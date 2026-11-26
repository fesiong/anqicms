package provider

import (
	"context"
	"fmt"
	"image/jpeg"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/storage"
)

type videoTaskExecutionOptions struct {
	SiteID         uint
	TaskID         uint
	Attachment     *model.Attachment
	Config         config.VideoProcessingConfig
	ThumbWidth     int
	ThumbHeight    int
	ThumbCrop      int
	Quality        int
	PublicPath     string
	StorageURL     string
	UseLocal       bool
	KeepLocal      bool
	Store          storage.Storage
	PosterLocation string
	OwnsTask       func() bool
	Render         func(context.Context, videoRenderOptions) (*videoRenderResult, error)
	Progress       func(int)
}

type videoTaskExecutionResult struct {
	Playlist  model.VideoPlaylist
	Artifacts []string
	Uploaded  []string
	Width     int
	Height    int
}

func executeVideoTask(
	ctx context.Context,
	opts videoTaskExecutionOptions,
) (*videoTaskExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if opts.Attachment == nil || opts.Store == nil {
		return nil, fmt.Errorf("video task requires an attachment and storage")
	}
	if opts.SiteID == 0 || opts.TaskID == 0 || opts.Attachment.Id == 0 {
		return nil, fmt.Errorf("video task requires site, attachment and task identifiers")
	}
	if opts.OwnsTask != nil && !opts.OwnsTask() {
		return nil, fmt.Errorf("video task lease is no longer owned by this worker")
	}
	tempDir, err := os.MkdirTemp("", "anqicms-video-")
	if err != nil {
		return nil, fmt.Errorf("create video temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	sourcePath, err := prepareVideoSource(ctx, opts, tempDir)
	if err != nil {
		return nil, err
	}
	if opts.OwnsTask != nil && !opts.OwnsTask() {
		return nil, fmt.Errorf("video task lease is no longer owned by this worker")
	}
	cfg := opts.Config
	cfg.Normalize()
	rootLocation := videoTaskArtifactRoot(opts.SiteID, opts.Attachment.Id, opts.TaskID)
	if cfg.PosterEnabled != 0 {
		opts.PosterLocation, err = newVideoPosterLocation(opts.Attachment.FileLocation)
		if err != nil {
			return nil, err
		}
	}
	render := opts.Render
	if render == nil {
		render = renderVideo
	}
	rendered, err := render(ctx, videoRenderOptions{
		SourcePath:     sourcePath,
		TempDir:        tempDir,
		RootLocation:   rootLocation,
		PosterLocation: opts.PosterLocation,
		StorageURL:     opts.StorageURL,
		Config:         cfg,
		Progress: func(progress int) {
			reportVideoProgress(opts.Progress, progress*9/10)
		},
	})
	if err != nil {
		return nil, err
	}
	result := &videoTaskExecutionResult{
		Playlist:  rendered.Playlist,
		Artifacts: make([]string, 0, len(rendered.Artifacts)+1),
		Uploaded:  make([]string, 0, len(rendered.Artifacts)+1),
		Width:     rendered.Width,
		Height:    rendered.Height,
	}
	for _, artifact := range rendered.Artifacts {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := putVideoArtifact(ctx, opts, artifact); err != nil {
			return result, fmt.Errorf("upload video artifact %s: %w", artifact.Location, err)
		}
		result.Artifacts = append(result.Artifacts, artifact.Location)
		result.Uploaded = append(result.Uploaded, artifact.Location)
		if artifact.Location != opts.PosterLocation {
			continue
		}
		thumb, err := buildVideoPosterThumbnail(artifact, opts.PosterLocation, &opts)
		if err != nil {
			return result, fmt.Errorf("build video poster thumbnail: %w", err)
		}
		if err := putVideoArtifact(ctx, opts, thumb); err != nil {
			return result, fmt.Errorf("upload video artifact %s: %w", thumb.Location, err)
		}
		result.Artifacts = append(result.Artifacts, thumb.Location)
		result.Uploaded = append(result.Uploaded, thumb.Location)
	}
	reportVideoProgress(opts.Progress, 99)
	return result, nil
}

func prepareVideoSource(
	ctx context.Context,
	opts videoTaskExecutionOptions,
	tempDir string,
) (string, error) {
	location, err := safeVideoStorageLocation(opts.Attachment.FileLocation)
	if err != nil {
		return "", err
	}
	if opts.UseLocal {
		localPath := filepath.Join(opts.PublicPath, filepath.FromSlash(location))
		if info, statErr := os.Stat(localPath); statErr == nil && !info.IsDir() {
			return localPath, nil
		}
	}
	reader, err := opts.Store.Get(ctx, location)
	if err != nil {
		return "", fmt.Errorf("download video source %s: %w", location, err)
	}
	defer reader.Close()
	extension := strings.ToLower(filepath.Ext(location))
	if extension == "" {
		extension = ".video"
	}
	destination := filepath.Join(tempDir, "source"+extension)
	file, err := os.Create(destination)
	if err != nil {
		return "", fmt.Errorf("create local video source: %w", err)
	}
	_, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	if copyErr != nil {
		return "", fmt.Errorf("save local video source: %w", copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close local video source: %w", closeErr)
	}
	return destination, nil
}

func putVideoArtifact(
	ctx context.Context,
	opts videoTaskExecutionOptions,
	artifact videoArtifact,
) error {
	if opts.OwnsTask != nil && !opts.OwnsTask() {
		return fmt.Errorf("video task lease is no longer owned by this worker")
	}
	location, err := safeVideoExecutionArtifactLocation(
		videoTaskArtifactRoot(opts.SiteID, opts.Attachment.Id, opts.TaskID),
		opts.PosterLocation,
		artifact.Location,
	)
	if err != nil {
		return err
	}
	file, err := os.Open(artifact.Path)
	if err != nil {
		return err
	}
	defer file.Close()
	if opts.KeepLocal {
		localPath := filepath.Join(opts.PublicPath, filepath.FromSlash(location))
		if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
			return err
		}
		localFile, err := os.Create(localPath)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(localFile, file)
		closeErr := localFile.Close()
		if copyErr != nil {
			_ = os.Remove(localPath)
			return copyErr
		}
		if closeErr != nil {
			_ = os.Remove(localPath)
			return closeErr
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			_ = os.Remove(localPath)
			return err
		}
		if err := opts.Store.Put(ctx, location, file); err != nil {
			_ = os.Remove(localPath)
			return err
		}
		return nil
	}
	return opts.Store.Put(ctx, location, file)
}

func newVideoPosterLocation(source string) (string, error) {
	sourceLocation, err := safeVideoStorageLocation(source)
	if err != nil {
		return "", err
	}
	token, err := newVideoArtifactToken()
	if err != nil {
		return "", err
	}
	return path.Join(path.Dir(sourceLocation), "poster_"+token+".jpg"), nil
}

func buildVideoPosterThumbnail(
	poster videoArtifact,
	posterLocation string,
	opts *videoTaskExecutionOptions,
) (videoArtifact, error) {
	thumbLocation, ok := videoPosterThumbLocation(posterLocation)
	if !ok {
		return videoArtifact{}, fmt.Errorf("invalid video poster location %q", posterLocation)
	}
	file, err := os.Open(poster.Path)
	if err != nil {
		return videoArtifact{}, err
	}
	defer file.Close()
	img, err := jpeg.Decode(file)
	if err != nil {
		return videoArtifact{}, err
	}
	thumb := library.ThumbnailCrop(
		opts.ThumbWidth,
		opts.ThumbHeight,
		img,
		opts.ThumbCrop,
	)
	buf, _, err := encodeImage(thumb, "jpg", opts.Quality)
	if err != nil {
		return videoArtifact{}, err
	}
	thumbPath := filepath.Join(filepath.Dir(poster.Path), "thumb_"+filepath.Base(poster.Path))
	if err := os.WriteFile(thumbPath, buf, 0o600); err != nil {
		return videoArtifact{}, err
	}
	return videoArtifact{Location: thumbLocation, Path: thumbPath}, nil
}

func videoPosterThumbLocation(posterLocation string) (string, bool) {
	poster, ok := safeVideoTaskPosterLocation(posterLocation)
	if !ok {
		return "", false
	}
	return path.Join(path.Dir(poster), "thumb_"+path.Base(poster)), true
}

func videoPosterLocationFromURL(source, posterURL, storageURL string) (string, bool) {
	location := strings.TrimSpace(posterURL)
	base := strings.TrimRight(strings.TrimSpace(storageURL), "/")
	if base != "" {
		if !strings.HasPrefix(location, base+"/") {
			return "", false
		}
		location = strings.TrimPrefix(location, base+"/")
	} else if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		return "", false
	}
	return safeVideoSourcePosterLocation(source, location)
}

func safeVideoSourcePosterLocation(source, location string) (string, bool) {
	cleaned, ok := safeVideoTaskPosterLocation(location)
	if !ok {
		return "", false
	}
	sourceLocation, err := safeVideoStorageLocation(source)
	if err != nil || cleaned == sourceLocation || path.Dir(cleaned) != path.Dir(sourceLocation) {
		return "", false
	}
	return cleaned, true
}

func safeVideoTaskPosterLocation(location string) (string, bool) {
	cleaned, err := safeVideoStorageLocation(location)
	if err != nil {
		return "", false
	}
	name := path.Base(cleaned)
	if !strings.HasPrefix(name, "poster_") || !strings.HasSuffix(name, ".jpg") {
		return "", false
	}
	token := strings.TrimSuffix(strings.TrimPrefix(name, "poster_"), ".jpg")
	if len(token) != 32 {
		return "", false
	}
	for _, char := range token {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return "", false
		}
	}
	return cleaned, true
}

func videoPlaybackLocation(
	root string,
	storageURL string,
	playlist *model.VideoPlaylist,
	artifacts []string,
) (string, error) {
	if playlist == nil {
		return "", fmt.Errorf("video playlist is empty")
	}
	location, err := videoPlaybackURLStorageLocation(playlist.DefaultURL, storageURL)
	if err != nil {
		return "", err
	}
	controlled, err := safeVideoArtifactLocation(root, location)
	if err != nil {
		return "", err
	}
	artifactSet := make(map[string]bool, len(artifacts))
	for _, artifact := range artifacts {
		cleaned, cleanErr := safeVideoStorageLocation(artifact)
		if cleanErr == nil {
			artifactSet[cleaned] = true
		}
	}
	if !artifactSet[controlled] {
		return "", fmt.Errorf("video playback %q is not a task artifact", controlled)
	}

	switch playlist.Format {
	case config.VideoFormatMP4:
		if !strings.EqualFold(path.Ext(controlled), ".mp4") {
			return "", fmt.Errorf("mp4 playback %q is not an mp4 file", controlled)
		}
		matched := false
		for _, variant := range playlist.Variants {
			if variant.Name != playlist.DefaultType {
				continue
			}
			fileLocation, fileErr := safeVideoStorageLocation(variant.File)
			if fileErr == nil && fileLocation == controlled {
				matched = true
				break
			}
		}
		if !matched {
			return "", fmt.Errorf("mp4 playback %q does not match default variant %q", controlled, playlist.DefaultType)
		}
	case config.VideoFormatHLS:
		cleanedRoot, rootErr := safeVideoStorageLocation(root)
		if rootErr != nil || !strings.EqualFold(path.Ext(controlled), ".m3u8") || path.Dir(controlled) != cleanedRoot {
			return "", fmt.Errorf("hls playback %q is not a master playlist", controlled)
		}
		for _, variant := range playlist.Variants {
			fileLocation, fileErr := safeVideoStorageLocation(variant.File)
			if fileErr == nil && fileLocation == controlled {
				return "", fmt.Errorf("hls playback %q is a rendition playlist", controlled)
			}
		}
	default:
		return "", fmt.Errorf("unsupported video playback format %q", playlist.Format)
	}
	return controlled, nil
}

func videoPlaybackURLStorageLocation(playURL, storageURL string) (string, error) {
	location := strings.TrimSpace(playURL)
	base := strings.TrimRight(strings.TrimSpace(storageURL), "/")
	if base != "" {
		if !strings.HasPrefix(location, base+"/") {
			return "", fmt.Errorf("video playback URL %q is outside storage URL", playURL)
		}
		location = strings.TrimPrefix(location, base+"/")
	} else if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		return "", fmt.Errorf("video playback URL %q is absolute without a storage URL", playURL)
	}
	return safeVideoStorageLocation(location)
}

func videoTaskArtifactRoot(siteID, attachmentID, taskID uint) string {
	return fmt.Sprintf("video/%d/%d/%d", siteID, attachmentID, taskID)
}

func safeVideoArtifactLocation(root, location string) (string, error) {
	cleaned, err := safeVideoStorageLocation(location)
	if err != nil {
		return "", err
	}
	cleanedRoot, err := safeVideoStorageLocation(root)
	if err != nil || !strings.HasPrefix(cleaned, cleanedRoot+"/") {
		return "", fmt.Errorf("video artifact %q is outside task namespace %q", location, root)
	}
	return cleaned, nil
}

func safeVideoExecutionArtifactLocation(root, posterLocation, location string) (string, error) {
	cleaned, err := safeVideoStorageLocation(location)
	if err != nil {
		return "", err
	}
	poster, posterErr := safeVideoStorageLocation(posterLocation)
	if posterLocation != "" && posterErr == nil {
		if cleaned == poster {
			return cleaned, nil
		}
		if thumb, ok := videoPosterThumbLocation(poster); ok && cleaned == thumb {
			return cleaned, nil
		}
	}
	return safeVideoArtifactLocation(root, cleaned)
}

func safeVideoStorageLocation(location string) (string, error) {
	normalized := strings.TrimLeft(strings.ReplaceAll(strings.TrimSpace(location), "\\", "/"), "/")
	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "\x00") {
		return "", fmt.Errorf("invalid video storage location %q", location)
	}
	return cleaned, nil
}
