package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

type videoCommandRunner func(context.Context, string, []string, []string, func(string)) error

type videoArtifact struct {
	Location string
	Path     string
}

type videoRenderOptions struct {
	SourcePath     string
	TempDir        string
	RootLocation   string
	PosterLocation string
	StorageURL     string
	FFmpegPath     string
	FFprobePath    string
	Duration       float64
	Config         config.VideoProcessingConfig
	Run            videoCommandRunner
	Progress       func(int)
}

type videoRenderResult struct {
	Playlist  model.VideoPlaylist
	Artifacts []videoArtifact
	Width     int
	Height    int
}

func renderVideo(
	ctx context.Context,
	opts videoRenderOptions,
) (*videoRenderResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if opts.Run == nil {
		opts.Run = runFFmpegCommand
	}
	if info, err := os.Stat(opts.SourcePath); err != nil || info.IsDir() {
		if err == nil {
			err = fmt.Errorf("source is a directory")
		}
		return nil, fmt.Errorf("read video source: %w", err)
	}
	if strings.TrimSpace(opts.RootLocation) == "" {
		return nil, fmt.Errorf("video artifact location is empty")
	}
	cfg := opts.Config
	cfg.Normalize()
	if cfg.PosterEnabled != 0 {
		posterLocation, err := safeVideoStorageLocation(opts.PosterLocation)
		if err != nil {
			return nil, fmt.Errorf("invalid video poster location: %w", err)
		}
		rootLocation, err := safeVideoStorageLocation(opts.RootLocation)
		if err != nil {
			return nil, err
		}
		if posterLocation == rootLocation || strings.HasPrefix(posterLocation, rootLocation+"/") {
			return nil, fmt.Errorf("video poster must be stored outside the video artifact namespace")
		}
		opts.PosterLocation = posterLocation
	}
	ffprobePath := opts.FFprobePath
	if ffprobePath == "" {
		var pathErr error
		ffprobePath, pathErr = getFFprobePath()
		if pathErr != nil {
			return nil, fmt.Errorf("ffprobe is not installed: %w", pathErr)
		}
	}
	probe, err := probeVideoFile(ctx, ffprobePath, opts.SourcePath, opts.Run)
	if err != nil {
		return nil, err
	}
	opts.Duration = probe.Duration
	renditions, err := selectVideoRenditions(cfg, probe)
	if err != nil {
		return nil, err
	}
	rootDir := filepath.Join(opts.TempDir, "artifacts")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("create video artifact directory: %w", err)
	}
	result := &videoRenderResult{
		Playlist: model.VideoPlaylist{
			Format:   cfg.Format,
			Variants: make([]model.VideoVariant, 0, len(renditions)),
		},
		Artifacts: make([]videoArtifact, 0),
		Width:     probe.Width,
		Height:    probe.Height,
	}
	if cfg.PosterEnabled != 0 {
		poster, err := renderVideoPoster(ctx, opts, probe, rootDir)
		if err != nil {
			return nil, err
		}
		ffmpegPath, err := videoFFmpegPath(opts.FFmpegPath)
		if err != nil {
			return nil, err
		}
		posterErr := executeVideoCommand(
			ctx,
			ffmpegPath,
			poster.args,
			opts.Run,
			func(string) {},
		)
		if posterErr != nil {
			_ = os.Remove(poster.path)
			firstAttemptErr := posterErr
			retryArgs := append([]string(nil), poster.args...)
			seekIndex := slices.Index(retryArgs, "-ss")
			retryArgs[seekIndex+1] = "0.000"
			posterErr = executeVideoCommand(
				ctx,
				ffmpegPath,
				retryArgs,
				opts.Run,
				func(string) {},
			)
			if posterErr != nil {
				posterErr = errors.Join(firstAttemptErr, posterErr)
			}
		}
		if posterErr != nil {
			_ = os.Remove(poster.path)
			log.Printf("[video] 生成封面失败，继续转码（文件 %s）：%v", opts.SourcePath, posterErr)
		} else {
			result.Playlist.PosterURL = videoStorageURL(opts.StorageURL, opts.PosterLocation)
			result.Artifacts = append(result.Artifacts, videoArtifact{Location: opts.PosterLocation, Path: poster.path})
			reportVideoProgress(opts.Progress, 5)
		}
	}

	ffmpegPath, err := videoFFmpegPath(opts.FFmpegPath)
	if err != nil {
		return nil, err
	}
	for index, rendition := range renditions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		variant, artifacts, err := renderVideoRendition(
			ctx,
			ffmpegPath,
			opts,
			cfg,
			rendition,
			index,
			len(renditions),
		)
		if err != nil {
			return nil, err
		}
		result.Playlist.Variants = append(result.Playlist.Variants, variant)
		result.Artifacts = append(result.Artifacts, artifacts...)
	}
	if cfg.Format == config.VideoFormatHLS {
		masterToken, err := newVideoArtifactToken()
		if err != nil {
			return nil, err
		}
		masterName := masterToken + ".m3u8"
		masterLocation := joinStorageLocation(opts.RootLocation, masterName)
		masterPath := filepath.Join(rootDir, masterName)
		if err := os.WriteFile(masterPath, []byte(buildHLSMasterPlaylist(result.Playlist.Variants)), 0o644); err != nil {
			return nil, fmt.Errorf("write HLS master playlist: %w", err)
		}
		result.Artifacts = append(result.Artifacts, videoArtifact{Location: masterLocation, Path: masterPath})
		result.Playlist.DefaultURL = videoStorageURL(opts.StorageURL, masterLocation)
		result.Playlist.DefaultType = result.Playlist.Variants[0].Name
	} else {
		defaultIndex := 0
		for index := 1; index < len(result.Playlist.Variants); index++ {
			candidate := result.Playlist.Variants[index]
			current := result.Playlist.Variants[defaultIndex]
			if candidate.Name != config.VideoQualityOriginal &&
				(current.Name == config.VideoQualityOriginal || candidate.Height > current.Height) {
				defaultIndex = index
			}
		}
		result.Playlist.DefaultURL = result.Playlist.Variants[defaultIndex].URL
		result.Playlist.DefaultType = result.Playlist.Variants[defaultIndex].Name
	}
	reportVideoProgress(opts.Progress, 100)
	return result, nil
}

type videoPosterRender struct {
	path string
	args []string
}

func probeVideoFile(
	ctx context.Context,
	ffprobePath string,
	sourcePath string,
	run videoCommandRunner,
) (videoProbe, error) {
	var output strings.Builder
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration:stream=codec_type,width,height",
		"-of", "json",
		sourcePath,
	}
	if err := executeVideoCommand(
		ctx,
		ffprobePath,
		args,
		run,
		func(line string) { output.WriteString(line); output.WriteByte('\n') },
	); err != nil {
		return videoProbe{}, fmt.Errorf("probe video: %w", err)
	}
	probe, err := parseVideoProbe(output.String())
	if err != nil {
		return videoProbe{}, fmt.Errorf("probe video: %w", err)
	}
	return probe, nil
}

func renderVideoPoster(
	ctx context.Context,
	opts videoRenderOptions,
	probe videoProbe,
	rootDir string,
) (videoPosterRender, error) {
	if err := ctx.Err(); err != nil {
		return videoPosterRender{}, err
	}
	seconds := float64(opts.Config.PosterTime)
	if seconds <= 0 {
		seconds = 1
	}
	if seconds > probe.Duration {
		seconds = probe.Duration / 2
	}
	quality := opts.Config.PosterQuality
	if quality < 1 || quality > 100 {
		quality = 85
	}
	poster := videoPosterRender{
		path: filepath.Join(rootDir, "poster.jpg"),
	}
	poster.args = []string{
		"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-ss", strconv.FormatFloat(seconds, 'f', 3, 64),
		"-i", opts.SourcePath,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale=min(iw\\,%d):-2", opts.Config.PosterWidth),
		"-q:v", strconv.Itoa(posterQualityScale(quality)),
		poster.path,
	}
	return poster, nil
}

func posterQualityScale(quality int) int {
	return 31 - int(math.Round(float64(quality-1)*29/99))
}

func renderVideoRendition(
	ctx context.Context,
	ffmpegPath string,
	opts videoRenderOptions,
	cfg config.VideoProcessingConfig,
	rendition videoRendition,
	index int,
	total int,
) (model.VideoVariant, []videoArtifact, error) {
	variant := model.VideoVariant{
		Name:   rendition.Name,
		Width:  rendition.Width,
		Height: rendition.Height,
	}
	token, err := newVideoArtifactToken()
	if err != nil {
		return model.VideoVariant{}, nil, err
	}
	name := token + ".mp4"
	dir := opts.TempDir
	if cfg.Format == config.VideoFormatHLS {
		name = token + "/" + token + ".m3u8"
		dir = filepath.Join(opts.TempDir, token)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return model.VideoVariant{}, nil, fmt.Errorf("create rendition directory: %w", err)
	}
	outputPath := filepath.Join(dir, filepath.Base(name))
	args := buildFFmpegRenderArgs(rendition, opts.SourcePath, outputPath, dir, cfg)
	start := 5 + index*95/total
	end := 5 + (index+1)*95/total
	var commandError error
	err = executeVideoCommand(
		ctx,
		ffmpegPath,
		args,
		opts.Run,
		func(line string) {
			percent := parseFFmpegProgress(line, opts.Duration)
			if percent > 0 {
				progress := start + (end-start)*percent/100
				reportVideoProgress(opts.Progress, progress)
			}
		},
	)
	if err != nil {
		commandError = fmt.Errorf("render %s: %w", rendition.Name, err)
		return model.VideoVariant{}, nil, commandError
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		return model.VideoVariant{}, nil, fmt.Errorf("read rendered %s: %w", rendition.Name, err)
	}
	location := joinStorageLocation(opts.RootLocation, name)
	variant.File = location
	variant.URL = videoStorageURL(opts.StorageURL, location)
	variant.Size = info.Size()
	artifact := []videoArtifact{{Location: location, Path: outputPath}}
	if cfg.Format == config.VideoFormatHLS {
		artifact, err = collectVideoArtifact(dir, joinStorageLocation(opts.RootLocation, token))
		if err != nil {
			return model.VideoVariant{}, nil, err
		}
	}
	reportVideoProgress(opts.Progress, end)
	return variant, artifact, nil
}

func collectVideoArtifact(dir, outputLocation string) ([]videoArtifact, error) {
	artifacts := make([]videoArtifact, 0)
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		location := joinStorageLocation(outputLocation, relative)
		if strings.HasPrefix(location, outputLocation+"/") {
			artifacts = append(artifacts, videoArtifact{Location: location, Path: path})
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("collect rendered video artifacts: %w", err)
	}
	return artifacts, nil
}

func executeVideoCommand(
	ctx context.Context,
	name string,
	args []string,
	run videoCommandRunner,
	emit func(string),
) error {
	var commandOutput []string
	err := run(
		ctx,
		name,
		args,
		nil,
		func(line string) {
			emit(line)
			commandOutput = append(commandOutput, line)
			if len(commandOutput) > 5 {
				commandOutput = commandOutput[len(commandOutput)-5:]
			}
		},
	)
	if err != nil {
		if len(commandOutput) > 0 {
			return fmt.Errorf("%w: %s", err, strings.Join(commandOutput, " | "))
		}
		return err
	}
	return nil
}

func videoFFmpegPath(configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	path, err := getFfmpegPath()
	if err != nil {
		return "", fmt.Errorf("ffmpeg is not installed: %w", err)
	}
	return path, nil
}

func reportVideoProgress(callback func(int), progress int) {
	if callback != nil && progress >= 0 && progress <= 100 {
		callback(progress)
	}
}

func newVideoArtifactToken() (string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", fmt.Errorf("generate video artifact name: %w", err)
	}
	return hex.EncodeToString(token[:]), nil
}

func joinStorageLocation(root, name string) string {
	return strings.TrimRight(root, "/") + "/" + strings.TrimLeft(filepath.ToSlash(name), "/")
}

func videoStorageURL(storageURL, location string) string {
	return strings.TrimRight(storageURL, "/") + "/" + strings.TrimLeft(location, "/")
}
