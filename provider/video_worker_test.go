package provider

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"go/ast"
	"go/token"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/storage"
)

type videoTestStorage struct {
	data         map[string][]byte
	gets         []string
	puts         []string
	deletes      []string
	putErr       error
	deleteErr    error
	beforeDelete func(string) error
}

func (s *videoTestStorage) Put(_ context.Context, key string, r io.Reader) error {
	if s.putErr != nil {
		return s.putErr
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if s.data == nil {
		s.data = make(map[string][]byte)
	}
	s.data[key] = data
	s.puts = append(s.puts, key)
	return nil
}

func (s *videoTestStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	s.gets = append(s.gets, key)
	data, ok := s.data[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

func (s *videoTestStorage) Delete(_ context.Context, key string) error {
	s.deletes = append(s.deletes, key)
	if s.deleteErr != nil {
		return s.deleteErr
	}
	if s.beforeDelete != nil {
		if err := s.beforeDelete(key); err != nil {
			return err
		}
	}
	delete(s.data, key)
	return nil
}

func (s *videoTestStorage) Exists(_ context.Context, key string) (bool, error) {
	_, ok := s.data[key]
	return ok, nil
}

func (s *videoTestStorage) Move(_ context.Context, src, dest string) error {
	data, ok := s.data[src]
	if !ok {
		return os.ErrNotExist
	}
	s.data[dest] = data
	delete(s.data, src)
	return nil
}

func TestExecuteVideoTaskUsesLocalSourceAndUploadsArtifacts(t *testing.T) {
	publicDir := t.TempDir()
	location := "uploads/source.mp4"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(publicDir, location)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, location), []byte("local source"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &videoTestStorage{}
	var renderedSource string
	result, err := executeVideoTask(context.Background(), videoTaskExecutionOptions{
		SiteID:     3,
		TaskID:     9,
		Attachment: &model.Attachment{Id: 7, FileLocation: location, FileSize: 12},
		Config:     config.DefaultVideoProcessingConfig(),
		PublicPath: publicDir,
		StorageURL: "/uploads",
		UseLocal:   true,
		Store:      store,
		Render: func(_ context.Context, opts videoRenderOptions) (*videoRenderResult, error) {
			data, readErr := os.ReadFile(opts.SourcePath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			renderedSource = string(data)
			artifactPath := filepath.Join(opts.TempDir, "720p.mp4")
			if writeErr := os.WriteFile(artifactPath, []byte("rendered"), 0o600); writeErr != nil {
				t.Fatal(writeErr)
			}
			return &videoRenderResult{
				Playlist: model.VideoPlaylist{
					Format:     config.VideoFormatMP4,
					DefaultURL: "/uploads/video/3/7/9/720p.mp4",
				},
				Artifacts: []videoArtifact{{Location: "video/3/7/9/720p.mp4", Path: artifactPath}},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if renderedSource != "local source" {
		t.Fatalf("rendered source = %q", renderedSource)
	}
	if len(store.gets) != 0 {
		t.Fatalf("local source should not be downloaded: %#v", store.gets)
	}
	if !reflect.DeepEqual(store.puts, []string{"video/3/7/9/720p.mp4"}) {
		t.Fatalf("uploads = %#v", store.puts)
	}
	if _, err := os.Stat(filepath.Join(publicDir, location)); err != nil {
		t.Fatalf("execution must not delete the source before task success: %v", err)
	}
	if !reflect.DeepEqual(result.Uploaded, []string{"video/3/7/9/720p.mp4"}) {
		t.Fatalf("uploaded = %#v", result.Uploaded)
	}
}

func TestExecuteVideoTaskDownloadsRemoteSource(t *testing.T) {
	store := &videoTestStorage{data: map[string][]byte{"uploads/remote.mov": []byte("remote source")}}
	var renderedSource string
	_, err := executeVideoTask(context.Background(), videoTaskExecutionOptions{
		SiteID:     3,
		TaskID:     10,
		Attachment: &model.Attachment{Id: 8, FileLocation: "uploads/remote.mov"},
		Config:     config.DefaultVideoProcessingConfig(),
		PublicPath: t.TempDir(),
		StorageURL: "https://cdn.example.com",
		Store:      store,
		Render: func(_ context.Context, opts videoRenderOptions) (*videoRenderResult, error) {
			data, readErr := os.ReadFile(opts.SourcePath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			renderedSource = string(data)
			return &videoRenderResult{Playlist: model.VideoPlaylist{Variants: []model.VideoVariant{}}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if renderedSource != "remote source" || !reflect.DeepEqual(store.gets, []string{"uploads/remote.mov"}) {
		t.Fatalf("source = %q, downloads = %#v", renderedSource, store.gets)
	}
}

func TestExecuteVideoTaskUploadsPosterAndThumbBesideSource(t *testing.T) {
	store := &videoTestStorage{data: map[string][]byte{"uploads/2026/source.mp4": []byte("source")}}
	posterPattern := regexp.MustCompile(`^uploads/2026/poster_[0-9a-f]{32}\.jpg$`)
	videoLocation := "video/3/8/10/0123456789abcdef0123456789abcdef.mp4"
	var posterLocation string
	result, err := executeVideoTask(context.Background(), videoTaskExecutionOptions{
		SiteID:     3,
		TaskID:     10,
		Attachment: &model.Attachment{Id: 8, FileLocation: "uploads/2026/source.mp4"},
		Config:     config.DefaultVideoProcessingConfig(),
		StorageURL: "https://cdn.example",
		Store:      store,
		Render: func(_ context.Context, opts videoRenderOptions) (*videoRenderResult, error) {
			posterLocation = opts.PosterLocation
			if !posterPattern.MatchString(posterLocation) {
				t.Fatalf("poster location = %q, want an independent random poster beside the source", posterLocation)
			}
			posterPath := filepath.Join(opts.TempDir, "poster.jpg")
			var poster bytes.Buffer
			if encodeErr := jpeg.Encode(&poster, image.NewRGBA(image.Rect(0, 0, 640, 360)), &jpeg.Options{Quality: 85}); encodeErr != nil {
				t.Fatal(encodeErr)
			}
			if writeErr := os.WriteFile(posterPath, poster.Bytes(), 0o600); writeErr != nil {
				t.Fatal(writeErr)
			}
			videoPath := filepath.Join(opts.TempDir, "video.mp4")
			if writeErr := os.WriteFile(videoPath, []byte("video"), 0o600); writeErr != nil {
				t.Fatal(writeErr)
			}
			return &videoRenderResult{
				Playlist: model.VideoPlaylist{
					DefaultURL: "https://cdn.example/" + videoLocation,
					PosterURL:  "https://cdn.example/" + posterLocation,
					Variants:   []model.VideoVariant{},
				},
				Artifacts: []videoArtifact{
					{Location: posterLocation, Path: posterPath},
					{Location: videoLocation, Path: videoPath},
				},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	thumbLocation := "uploads/2026/thumb_poster_" + strings.TrimPrefix(posterLocation, "uploads/2026/poster_")
	if result.Playlist.PosterURL != "https://cdn.example/"+posterLocation {
		t.Fatalf("poster URL = %q", result.Playlist.PosterURL)
	}
	if !reflect.DeepEqual(store.puts, []string{posterLocation, thumbLocation, videoLocation}) {
		t.Fatalf("uploads = %#v, want poster, poster thumb and task-owned video", store.puts)
	}
	thumbBytes, ok := store.data[thumbLocation]
	if !ok {
		t.Fatalf("poster thumbnail %q was not uploaded", thumbLocation)
	}
	thumb, decodeErr := jpeg.Decode(bytes.NewReader(thumbBytes))
	if decodeErr != nil {
		t.Fatalf("decode poster thumbnail: %v", decodeErr)
	}
	if thumb.Bounds().Dx() != 250 || thumb.Bounds().Dy() > 250 {
		t.Fatalf("thumbnail dimensions = %dx%d, want the default 250px bound", thumb.Bounds().Dx(), thumb.Bounds().Dy())
	}
	if !reflect.DeepEqual(result.Artifacts, []string{posterLocation, thumbLocation, videoLocation}) {
		t.Fatalf("artifacts = %#v", result.Artifacts)
	}
	if strings.Contains(videoLocation, strings.TrimSuffix(strings.TrimPrefix(posterLocation, "uploads/2026/poster_"), ".jpg")) {
		t.Fatal("video path must not reuse the poster token")
	}
}

func TestBuildVideoPosterThumbnailUsesConfiguredSizeAndCrop(t *testing.T) {
	tempDir := t.TempDir()
	posterPath := filepath.Join(tempDir, "poster.jpg")
	var poster bytes.Buffer
	if err := jpeg.Encode(&poster, image.NewRGBA(image.Rect(0, 0, 640, 360)), &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(posterPath, poster.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	const posterLocation = "uploads/2026/poster_0123456789abcdef0123456789abcdef.jpg"
	artifact, err := buildVideoPosterThumbnail(
		videoArtifact{Location: posterLocation, Path: posterPath},
		posterLocation,
		&videoTaskExecutionOptions{ThumbWidth: 120, ThumbHeight: 80, ThumbCrop: 2, Quality: 70},
	)
	if err != nil {
		t.Fatal(err)
	}
	const wantLocation = "uploads/2026/thumb_poster_0123456789abcdef0123456789abcdef.jpg"
	if artifact.Location != wantLocation {
		t.Fatalf("thumbnail location = %q, want %q", artifact.Location, wantLocation)
	}
	data, err := os.ReadFile(artifact.Path)
	if err != nil {
		t.Fatal(err)
	}
	thumb, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if thumb.Bounds().Dx() != 120 || thumb.Bounds().Dy() != 80 {
		t.Fatalf("thumbnail dimensions = %dx%d, want 120x80", thumb.Bounds().Dx(), thumb.Bounds().Dy())
	}
}

func TestExecuteVideoTaskUploadFailurePreservesSource(t *testing.T) {
	publicDir := t.TempDir()
	location := "uploads/source.mp4"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(publicDir, location)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, location), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &videoTestStorage{putErr: errors.New("storage unavailable")}
	_, err := executeVideoTask(context.Background(), videoTaskExecutionOptions{
		SiteID:     3,
		TaskID:     11,
		Attachment: &model.Attachment{Id: 9, FileLocation: location},
		Config:     config.DefaultVideoProcessingConfig(),
		PublicPath: publicDir,
		StorageURL: "/uploads",
		UseLocal:   true,
		Store:      store,
		Render: func(_ context.Context, opts videoRenderOptions) (*videoRenderResult, error) {
			artifactPath := filepath.Join(opts.TempDir, "720p.mp4")
			if writeErr := os.WriteFile(artifactPath, []byte("rendered"), 0o600); writeErr != nil {
				t.Fatal(writeErr)
			}
			return &videoRenderResult{
				Playlist:  model.VideoPlaylist{Variants: []model.VideoVariant{}},
				Artifacts: []videoArtifact{{Location: "video/3/9/11/720p.mp4", Path: artifactPath}},
			}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "storage unavailable") {
		t.Fatalf("error = %v", err)
	}
	if data, readErr := os.ReadFile(filepath.Join(publicDir, location)); readErr != nil || string(data) != "source" {
		t.Fatalf("source changed after upload failure: data=%q err=%v", data, readErr)
	}
}

func TestExecuteVideoTaskRemovesLocalArtifactWhenRemoteUploadFails(t *testing.T) {
	publicDir := t.TempDir()
	location := "uploads/source.mp4"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(publicDir, location)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, location), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &videoTestStorage{putErr: errors.New("storage unavailable")}
	_, err := executeVideoTask(context.Background(), videoTaskExecutionOptions{
		SiteID:     3,
		TaskID:     14,
		Attachment: &model.Attachment{Id: 12, FileLocation: location},
		Config:     config.DefaultVideoProcessingConfig(),
		PublicPath: publicDir,
		StorageURL: "/uploads",
		UseLocal:   true,
		KeepLocal:  true,
		Store:      store,
		Render: func(_ context.Context, opts videoRenderOptions) (*videoRenderResult, error) {
			artifactPath := filepath.Join(opts.TempDir, "720p.mp4")
			if writeErr := os.WriteFile(artifactPath, []byte("rendered"), 0o600); writeErr != nil {
				return nil, writeErr
			}
			return &videoRenderResult{
				Playlist:  model.VideoPlaylist{Variants: []model.VideoVariant{}},
				Artifacts: []videoArtifact{{Location: "video/3/12/14/720p.mp4", Path: artifactPath}},
			}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "storage unavailable") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(publicDir, "video/3/12/14/720p.mp4")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed upload left a local artifact: %v", statErr)
	}
}

func TestExecuteVideoTaskRejectsArtifactOutsideSiteTaskNamespace(t *testing.T) {
	store := &videoTestStorage{data: map[string][]byte{"uploads/source.mp4": []byte("source")}}
	_, err := executeVideoTask(context.Background(), videoTaskExecutionOptions{
		SiteID:     3,
		TaskID:     12,
		Attachment: &model.Attachment{Id: 10, FileLocation: "uploads/source.mp4"},
		Config:     config.DefaultVideoProcessingConfig(),
		StorageURL: "/uploads",
		Store:      store,
		Render: func(_ context.Context, opts videoRenderOptions) (*videoRenderResult, error) {
			if opts.RootLocation != "video/3/10/12" {
				t.Fatalf("artifact root = %q, want site-scoped task root", opts.RootLocation)
			}
			artifactPath := filepath.Join(opts.TempDir, "poster.jpg")
			if writeErr := os.WriteFile(artifactPath, []byte("poster"), 0o600); writeErr != nil {
				t.Fatal(writeErr)
			}
			return &videoRenderResult{
				Playlist:  model.VideoPlaylist{Variants: []model.VideoVariant{}},
				Artifacts: []videoArtifact{{Location: "video/4/10/12/poster.jpg", Path: artifactPath}},
			}, nil
		},
	})
	if err == nil {
		t.Fatal("expected cross-site artifact rejection")
	}
	if len(store.puts) != 0 {
		t.Fatalf("uncontrolled artifact reached storage: %#v", store.puts)
	}
}

func TestExecuteVideoTaskStopsWhenLeaseOwnershipIsLost(t *testing.T) {
	store := &videoTestStorage{data: map[string][]byte{"uploads/source.mp4": []byte("source")}}
	checks := 0
	rendered := false
	_, err := executeVideoTask(context.Background(), videoTaskExecutionOptions{
		SiteID:     3,
		TaskID:     13,
		Attachment: &model.Attachment{Id: 11, FileLocation: "uploads/source.mp4"},
		Config:     config.DefaultVideoProcessingConfig(),
		Store:      store,
		OwnsTask: func() bool {
			checks++
			return checks == 1
		},
		Render: func(_ context.Context, opts videoRenderOptions) (*videoRenderResult, error) {
			rendered = true
			artifactPath := filepath.Join(opts.TempDir, "720p.mp4")
			if writeErr := os.WriteFile(artifactPath, []byte("rendered"), 0o600); writeErr != nil {
				return nil, writeErr
			}
			return &videoRenderResult{
				Playlist:  model.VideoPlaylist{Variants: []model.VideoVariant{}},
				Artifacts: []videoArtifact{{Location: "video/3/11/13/720p.mp4", Path: artifactPath}},
			}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "lease") {
		t.Fatalf("error = %v, want lost lease", err)
	}
	if rendered {
		t.Fatal("renderer must not run after the lease is lost")
	}
	if len(store.puts) != 0 {
		t.Fatalf("artifacts uploaded after lease loss: %#v", store.puts)
	}
}

func TestVideoWorkerSQLUsesUnixSecondTimestamps(t *testing.T) {
	d := &videoWorkerCycleSQL{
		attachmentStarted: make(chan struct{}),
		releaseAttachment: make(chan struct{}),
	}
	close(d.releaseAttachment)
	w := newVideoWorkerCycleSite(t, d)
	now := time.Unix(2000, 0)
	if _, err := w.dueVideoTaskIDs(now, 20); err != nil {
		t.Fatal(err)
	}
	if len(d.queryArgs) != 1 || len(d.queryArgs[0]) < 2 {
		t.Fatalf("due query args = %#v", d.queryArgs)
	}
	if got, ok := d.queryArgs[0][1].Value.(int64); !ok || got != now.Unix() {
		t.Fatalf("due query timestamp = %#v, want int64(%d)", d.queryArgs[0][1].Value, now.Unix())
	}

	task := &model.VideoTask{Id: 1, Status: model.VideoTaskStatusProcessing, LeaseOwner: "worker-test"}
	query, args := videoTaskLeaseScope(task, now)
	if !strings.Contains(query, "`lease_until` > ?") || len(args) < 4 {
		t.Fatalf("lease scope = %q %#v", query, args)
	}
	if got, ok := args[3].(int64); !ok || got != now.Unix() {
		t.Fatalf("lease scope timestamp = %#v, want int64(%d)", args[3], now.Unix())
	}
	if err := w.renewVideoTaskLease(task, now); err != nil {
		t.Fatal(err)
	}
	if len(d.updateArgs) != 1 || len(d.updateArgs[0]) < 2 {
		t.Fatalf("renew update args = %#v", d.updateArgs)
	}
	for _, arg := range d.updateArgs[0] {
		if _, ok := arg.Value.(time.Time); ok {
			t.Fatalf("renew update passed time.Time to an integer column: %#v", d.updateArgs[0])
		}
	}
	if got, ok := d.updateArgs[0][0].Value.(int64); !ok || got != now.Add(videoTaskLeaseTTL).Unix() {
		t.Fatalf("renewed lease = %#v, want int64(%d)", d.updateArgs[0][0].Value, now.Add(videoTaskLeaseTTL).Unix())
	}

	if _, err := w.claimVideoTask(1, "worker-test", now); err != nil {
		t.Fatal(err)
	}
	claimArgs := d.updateArgs[len(d.updateArgs)-1]
	for _, arg := range claimArgs {
		if _, ok := arg.Value.(time.Time); ok {
			t.Fatalf("claim passed time.Time to an integer timestamp column: %#v", claimArgs)
		}
	}
}

func TestVideoTaskHeartbeatRunsWhileWaitingForGlobalFFmpegSlot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		d := &videoWorkerCycleSQL{
			attachmentStarted: make(chan struct{}),
			releaseAttachment: make(chan struct{}),
		}
		close(d.releaseAttachment)
		w := newVideoWorkerCycleSite(t, d)
		task := &model.VideoTask{
			Id:           1,
			AttachmentId: 7,
			Status:       model.VideoTaskStatusProcessing,
			Attempts:     1,
			Config:       config.DefaultVideoProcessingConfig(),
			LeaseOwner:   "worker-test",
			LeaseUntil:   time.Now().Add(videoTaskLeaseTTL).Unix(),
		}

		previousSlots := videoFFmpegSlots
		videoFFmpegSlots = make(chan struct{}, videoGlobalConcurrency)
		defer func() { videoFFmpegSlots = previousSlots }()
		videoFFmpegSlots <- struct{}{}
		videoFFmpegSlots <- struct{}{}

		done := make(chan struct{})
		go func() {
			defer close(done)
			w.processVideoTask(context.Background(), task)
		}()
		synctest.Wait()
		time.Sleep(videoTaskHeartbeat + time.Second)
		synctest.Wait()
		renewals := d.leaseRenewals()

		<-videoFFmpegSlots
		<-done
		if renewals == 0 {
			t.Fatal("task lease must be renewed while waiting for a global ffmpeg slot")
		}
	})
}

func TestVideoWorkerClaimsOnlyAfterSiteSlotIsAvailable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		d := &videoWorkerCycleSQL{
			attachmentStarted: make(chan struct{}),
			releaseAttachment: make(chan struct{}),
		}
		w := newVideoWorkerCycleSite(t, d)
		cfg := config.DefaultVideoProcessingConfig()
		cfg.Concurrency = 1
		w.Content = &config.ContentConfig{Video: cfg}

		previousSlots := videoFFmpegSlots
		videoFFmpegSlots = make(chan struct{}, videoGlobalConcurrency)
		defer func() { videoFFmpegSlots = previousSlots }()

		done := make(chan struct{})
		go func() {
			defer close(done)
			w.runVideoWorkerCycle(context.Background(), "worker-test")
		}()
		<-d.attachmentStarted
		synctest.Wait()

		claimed := d.claimedIDs()
		close(d.releaseAttachment)
		<-done
		if !reflect.DeepEqual(claimed, []uint{1}) {
			t.Fatalf("claimed while the only site slot was occupied: %v, want [1]", claimed)
		}
	})
}

type videoWorkerCycleSQL struct {
	mu                sync.Mutex
	claims            []uint
	renewals          int
	queryArgs         [][]driver.NamedValue
	updateArgs        [][]driver.NamedValue
	attachmentStarted chan struct{}
	releaseAttachment chan struct{}
	startedOnce       sync.Once
}

func (d *videoWorkerCycleSQL) claimedIDs() []uint {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]uint{}, d.claims...)
}

func (d *videoWorkerCycleSQL) leaseRenewals() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.renewals
}

func (d *videoWorkerCycleSQL) Connect(context.Context) (driver.Conn, error) { return d, nil }
func (d *videoWorkerCycleSQL) Driver() driver.Driver                        { return d }
func (d *videoWorkerCycleSQL) Open(string) (driver.Conn, error)             { return d, nil }
func (d *videoWorkerCycleSQL) Close() error                                 { return nil }
func (d *videoWorkerCycleSQL) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (d *videoWorkerCycleSQL) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}
func (d *videoWorkerCycleSQL) QueryContext(
	_ context.Context,
	query string,
	args []driver.NamedValue,
) (driver.Rows, error) {
	d.queryArgs = append(d.queryArgs, append([]driver.NamedValue{}, args...))
	lower := strings.ToLower(query)
	switch {
	case strings.HasPrefix(query, "SELECT DATABASE()"):
		return &videoTaskRows{columns: []string{"DATABASE()"}, data: [][]driver.Value{{"anqicms"}}}, nil
	case strings.Contains(lower, "information_schema.tables"):
		return &videoTaskRows{columns: []string{"count"}, data: [][]driver.Value{{int64(1)}}}, nil
	case strings.Contains(lower, "from `attachments`"):
		d.startedOnce.Do(func() { close(d.attachmentStarted) })
		<-d.releaseAttachment
		return nil, errors.New("attachment unavailable")
	case strings.Contains(lower, "from `video_tasks`") && strings.Contains(lower, "`next_retry_at` <= ?"):
		return &videoTaskRows{
			columns: []string{"id"},
			data:    [][]driver.Value{{int64(1)}, {int64(2)}},
		}, nil
	case strings.Contains(lower, "select * from `video_tasks`"):
		id := int64(1)
		if len(args) > 0 {
			id = args[0].Value.(int64)
		}
		return &videoTaskRows{
			columns: []string{"id", "attachment_id", "status", "attempts", "config", "lease_owner", "lease_until"},
			data: [][]driver.Value{{
				id,
				int64(7),
				int64(model.VideoTaskStatusProcessing),
				int64(1),
				"",
				"worker-test",
				time.Now().Add(videoTaskLeaseTTL).Unix(),
			}},
		}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}
func (d *videoWorkerCycleSQL) ExecContext(
	_ context.Context,
	query string,
	args []driver.NamedValue,
) (driver.Result, error) {
	d.updateArgs = append(d.updateArgs, append([]driver.NamedValue{}, args...))
	lower := strings.ToLower(query)
	if strings.Contains(lower, "update `video_tasks` set `lease_until`") {
		d.mu.Lock()
		d.renewals++
		d.mu.Unlock()
		return driver.RowsAffected(1), nil
	}
	if strings.Contains(lower, "attempts` + 1") {
		if len(args) == 0 {
			return nil, errors.New("claim without task id")
		}
		if len(args) < 3 {
			return nil, errors.New("claim without task id")
		}
		d.mu.Lock()
		d.claims = append(d.claims, uint(args[len(args)-3].Value.(int64)))
		d.mu.Unlock()
		return driver.RowsAffected(1), nil
	}
	return driver.RowsAffected(1), nil
}

func newVideoWorkerCycleSite(t *testing.T, d *videoWorkerCycleSQL) *Website {
	t.Helper()
	conn := sql.OpenDB(d)
	t.Cleanup(func() { _ = conn.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &Website{Id: 3, DB: db}
}

func TestRetryVideoTaskDoesNotResetTaskClaimedByWorker(t *testing.T) {
	d := &videoTaskSQL{}
	w := newVideoTaskSQLSite(t, d)
	if _, err := w.RetryVideoTask(9); err == nil {
		t.Fatal("retry must fail when the conditional update no longer owns a failed task")
	}
	if len(d.updates) != 1 {
		t.Fatalf("updates = %#v, want one conditional retry", d.updates)
	}
	query, args := d.updates[0], d.updateArgs[0]
	if !strings.Contains(query, "`status` = ?") {
		t.Fatalf("retry update must require failed status: %s", query)
	}
	if len(args) < 2 || args[len(args)-1].Value != int64(model.VideoTaskStatusFailed) {
		t.Fatalf("retry update args = %#v, want failed status guard", args)
	}
}

func TestRetryVideoTaskResetsFailedTaskWhenConditionalUpdateSucceeds(t *testing.T) {
	d := &videoTaskSQL{rowsAffected: 1}
	w := newVideoTaskSQLSite(t, d)
	before := time.Now().Unix()
	task, err := w.RetryVideoTask(9)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != model.VideoTaskStatusPending || task.Progress != 0 || task.Attempts != 0 {
		t.Fatalf("retried task = %#v, want a fresh pending task", task)
	}
	if task.NextRetryAt < before || task.NextRetryAt > time.Now().Unix() {
		t.Fatalf("next_retry_at = %d, want current Unix seconds", task.NextRetryAt)
	}
	if task.StartedAt != 0 || task.FinishedAt != 0 || task.LeaseUntil != 0 || task.LeaseOwner != "" {
		t.Fatalf("retried task retained execution state: %#v", task)
	}
}

func TestRetryVideoTaskDeletesPreviousArtifactsBeforeRequeue(t *testing.T) {
	poster := "uploads/2026/poster_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg"
	thumb := "uploads/2026/thumb_poster_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg"
	mp4 := "video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.mp4"
	master := "video/3/7/9/cccccccccccccccccccccccccccccccc.m3u8"
	segment := "video/3/7/9/dddddddddddddddddddddddddddddddd/eeeeeeeeeeeeeeeeeeeeeeeeeeee_000.ts"
	source := "uploads/2026/source.mp4"
	unrelated := "uploads/2026/unrelated.jpg"
	store := &videoTestStorage{data: map[string][]byte{
		source:    []byte("source"),
		poster:    []byte("poster"),
		thumb:     []byte("thumb"),
		mp4:       []byte("mp4"),
		master:    []byte("master"),
		segment:   []byte("segment"),
		unrelated: []byte("unrelated"),
	}}
	d := &videoRetryCleanupSQL{
		task: model.VideoTask{
			Id:            9,
			AttachmentId:  7,
			Status:        model.VideoTaskStatusFailed,
			Attempts:      2,
			PlaylistJSON:  `{"poster_url":"/uploads/` + poster + `","variants":[]}`,
			ArtifactsJSON: `["` + poster + `","` + mp4 + `","` + master + `","` + segment + `"]`,
		},
		attachmentLogo:            "/uploads/" + poster,
		attachmentRowsAffected:    1,
		attachmentRowsAffectedSet: true,
	}
	w := newVideoRetryCleanupSite(t, d, store)
	task, err := w.RetryVideoTask(9)
	if err != nil {
		t.Fatal(err)
	}
	if !d.committed || d.rolledBack {
		t.Fatalf("retry must commit after cleanup: committed=%v rolledBack=%v", d.committed, d.rolledBack)
	}
	for _, location := range []string{poster, thumb, mp4, master, segment} {
		if _, exists := store.data[location]; exists {
			t.Errorf("previous artifact survived retry: %s", location)
		}
	}
	for _, location := range []string{source, unrelated} {
		if _, exists := store.data[location]; !exists {
			t.Errorf("retry deleted an uncontrolled file: %s", location)
		}
	}
	if task.Status != model.VideoTaskStatusPending || task.Playlist != nil || len(task.Artifacts) != 0 {
		t.Fatalf("retried task retained stale output metadata: %#v", task)
	}
	if len(d.updates) != 2 {
		t.Fatalf("updates = %#v, want one conditional retry and one poster reference reset", d.updates)
	}
	var clearedArtifacts, clearedPlaylist, clearedAttachmentLogo bool
	for _, arg := range d.updateArgs[0] {
		if value, ok := arg.Value.(string); ok {
			clearedArtifacts = clearedArtifacts || value == "[]"
			clearedPlaylist = clearedPlaylist || value == ""
		}
	}
	if !strings.Contains(d.updates[1], "`logo` = ?") {
		t.Fatalf("poster reference reset must compare the exact logo: %s", d.updates[1])
	}
	matchesPoster := false
	for _, arg := range d.updateArgs[1] {
		if value, ok := arg.Value.(string); ok {
			clearedAttachmentLogo = clearedAttachmentLogo || value == ""
			matchesPoster = matchesPoster || value == "/uploads/"+poster
		}
	}
	if !clearedArtifacts || !clearedPlaylist || !clearedAttachmentLogo || !matchesPoster {
		t.Fatalf("retry did not clear persisted output metadata: task=%#v attachment=%#v", d.updateArgs[0], d.updateArgs[1])
	}
}

func TestRetryVideoTaskDoesNotClearUnneededPosterReference(t *testing.T) {
	poster := "uploads/2026/poster_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg"
	tests := []struct {
		name           string
		logo           string
		attachmentRows int64
	}{
		{
			name:           "empty logo",
			logo:           "",
			attachmentRows: 0,
		},
		{
			name:           "unrelated logo",
			logo:           "/uploads/2026/unrelated.jpg",
			attachmentRows: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &videoTestStorage{data: map[string][]byte{poster: []byte("poster")}}
			d := &videoRetryCleanupSQL{
				task: model.VideoTask{
					Id:            9,
					AttachmentId:  7,
					Status:        model.VideoTaskStatusFailed,
					PlaylistJSON:  `{"poster_url":"/uploads/` + poster + `","variants":[]}`,
					ArtifactsJSON: `["` + poster + `"]`,
				},
				attachmentLogo:            tt.logo,
				attachmentRowsAffected:    tt.attachmentRows,
				attachmentRowsAffectedSet: true,
			}
			w := newVideoRetryCleanupSite(t, d, store)
			task, err := w.RetryVideoTask(9)
			if err != nil {
				t.Fatal(err)
			}
			if task.Status != model.VideoTaskStatusPending {
				t.Fatalf("retried task status = %d, want pending", task.Status)
			}
			if len(d.updates) != 1 {
				t.Fatalf("updates = %#v, want only the task retry", d.updates)
			}
			if d.attachmentLogo != tt.logo {
				t.Fatalf("attachment logo = %q, want preserved %q", d.attachmentLogo, tt.logo)
			}
		})
	}
}

func TestRetryVideoTaskCleanupFailureDoesNotRequeue(t *testing.T) {
	poster := "uploads/2026/poster_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg"
	store := &videoTestStorage{
		data:      map[string][]byte{poster: []byte("poster")},
		deleteErr: errors.New("storage unavailable"),
	}
	d := &videoRetryCleanupSQL{
		task: model.VideoTask{
			Id:            9,
			AttachmentId:  7,
			Status:        model.VideoTaskStatusFailed,
			Artifacts:     []string{poster},
			ArtifactsJSON: `["` + poster + `"]`,
		},
	}
	w := newVideoRetryCleanupSite(t, d, store)
	if _, err := w.RetryVideoTask(9); err == nil || !strings.Contains(err.Error(), "storage unavailable") {
		t.Fatalf("retry error = %v, want cleanup failure", err)
	}
	if d.committed || !d.rolledBack || len(d.updates) != 0 {
		t.Fatalf("failed cleanup must not persist a retry: committed=%v rolledBack=%v updates=%#v", d.committed, d.rolledBack, d.updates)
	}
}

func TestSaveVideoTaskFailureCleansArtifactsBeforeAutomaticRetry(t *testing.T) {
	poster := "uploads/2026/poster_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg"
	thumb := "uploads/2026/thumb_poster_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg"
	previous := "video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.mp4"
	partial := "video/3/7/9/cccccccccccccccccccccccccccccccc.m3u8"
	source := "uploads/2026/source.mp4"
	store := &videoTestStorage{data: map[string][]byte{
		source:   []byte("source"),
		poster:   []byte("poster"),
		thumb:    []byte("thumb"),
		previous: []byte("previous"),
		partial:  []byte("partial"),
	}}
	d := &videoFailureSQL{attachmentLogo: "/uploads/" + poster}
	w := newVideoFailureSite(t, d, store)
	cfg := config.DefaultVideoProcessingConfig()
	cfg.MaxRetries = 2
	task := &model.VideoTask{
		Id:           9,
		AttachmentId: 7,
		Status:       model.VideoTaskStatusProcessing,
		Attempts:     1,
		Config:       cfg,
		Artifacts:    []string{poster, previous},
		Playlist:     &model.VideoPlaylist{PosterURL: "/uploads/" + poster},
		LeaseOwner:   "worker-test",
		LeaseUntil:   time.Now().Add(videoTaskLeaseTTL).Unix(),
	}
	w.saveVideoTaskFailure(task, errors.New("ffmpeg failed"), &videoTaskExecutionResult{Uploaded: []string{partial}})
	for _, location := range []string{poster, thumb, previous, partial} {
		if _, exists := store.data[location]; exists {
			t.Errorf("automatic retry retained artifact %s", location)
		}
	}
	if _, exists := store.data[source]; !exists {
		t.Fatal("automatic retry cleanup deleted the source")
	}
	if !d.committed || d.rolledBack {
		t.Fatalf("automatic retry must commit atomically: committed=%v rolledBack=%v", d.committed, d.rolledBack)
	}
	assertVideoFailureUpdate(t, d, model.VideoTaskStatusPending)
	taskArgs := videoFailureTaskArgs(d)
	if !videoUpdateContainsString(taskArgs, "[]") || !videoUpdateContainsString(taskArgs, "") {
		t.Fatalf("automatic retry retained output metadata: %#v", taskArgs)
	}
	clearedAttachmentLogo := false
	matchesPoster := false
	for index, query := range d.queries {
		if !strings.Contains(strings.ToLower(query), "update `attachments`") {
			continue
		}
		if !strings.Contains(query, "`logo` = ?") {
			t.Fatalf("poster reference reset must compare the exact logo: %s", query)
		}
		for _, arg := range d.updateArgs[index] {
			if value, ok := arg.Value.(string); ok {
				clearedAttachmentLogo = clearedAttachmentLogo || value == ""
				matchesPoster = matchesPoster || value == "/uploads/"+poster
			}
		}
	}
	if !clearedAttachmentLogo || !matchesPoster || d.attachmentLogo != "" {
		t.Fatalf("automatic retry left attachment.logo pointing at the deleted poster: logo=%q queries=%#v", d.attachmentLogo, d.queries)
	}
}

func TestSaveVideoTaskFailureDoesNotClearUnneededPosterReference(t *testing.T) {
	poster := "uploads/2026/poster_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg"
	tests := []struct {
		name           string
		logo           string
		attachmentRows int64
	}{
		{
			name:           "empty logo",
			logo:           "",
			attachmentRows: 0,
		},
		{
			name:           "unrelated logo",
			logo:           "/uploads/2026/unrelated.jpg",
			attachmentRows: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &videoTestStorage{data: map[string][]byte{poster: []byte("poster")}}
			d := &videoFailureSQL{
				attachmentLogo:            tt.logo,
				attachmentRowsAffected:    tt.attachmentRows,
				attachmentRowsAffectedSet: true,
			}
			w := newVideoFailureSite(t, d, store)
			cfg := config.DefaultVideoProcessingConfig()
			cfg.MaxRetries = 2
			task := &model.VideoTask{
				Id:            9,
				AttachmentId:  7,
				Status:        model.VideoTaskStatusProcessing,
				Attempts:      1,
				Config:        cfg,
				Artifacts:     []string{poster},
				ArtifactsJSON: `["` + poster + `"]`,
				Playlist:      &model.VideoPlaylist{PosterURL: "/uploads/" + poster},
				LeaseOwner:    "worker-test",
				LeaseUntil:    time.Now().Add(videoTaskLeaseTTL).Unix(),
			}
			w.saveVideoTaskFailure(task, errors.New("ffmpeg failed"), nil)
			assertVideoFailureUpdate(t, d, model.VideoTaskStatusPending)
			if !d.committed || d.rolledBack {
				t.Fatalf("automatic retry must commit: committed=%v rolledBack=%v", d.committed, d.rolledBack)
			}
			if d.attachmentLogo != tt.logo {
				t.Fatalf("attachment logo = %q, want preserved %q", d.attachmentLogo, tt.logo)
			}
			for _, query := range d.queries {
				if strings.Contains(strings.ToLower(query), "update `attachments`") {
					t.Fatalf("unneeded poster reference must not be updated: %s", query)
				}
			}
		})
	}
}

func TestSaveVideoTaskFailureLeaseLostDuringCleanupDoesNotClearPosterReference(t *testing.T) {
	poster := "uploads/2026/poster_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg"
	d := &videoFailureSQL{attachmentLogo: "/uploads/" + poster}
	store := &videoTestStorage{
		data: map[string][]byte{poster: []byte("poster")},
		beforeDelete: func(string) error {
			d.leaseLost = true
			return nil
		},
	}
	w := newVideoFailureSite(t, d, store)
	cfg := config.DefaultVideoProcessingConfig()
	cfg.MaxRetries = 2
	task := &model.VideoTask{
		Id:            9,
		AttachmentId:  7,
		Status:        model.VideoTaskStatusProcessing,
		Attempts:      1,
		Config:        cfg,
		Artifacts:     []string{poster},
		ArtifactsJSON: `["` + poster + `"]`,
		Playlist:      &model.VideoPlaylist{PosterURL: "/uploads/" + poster},
		LeaseOwner:    "worker-test",
		LeaseUntil:    time.Now().Add(videoTaskLeaseTTL).Unix(),
	}
	w.saveVideoTaskFailure(task, errors.New("ffmpeg failed"), nil)
	if d.attachmentLogo != "/uploads/"+poster {
		t.Fatalf("lost lease cleared the current poster reference: %q", d.attachmentLogo)
	}
	if d.committedTaskUpdates != 0 {
		t.Fatalf("lost lease committed %d task updates", d.committedTaskUpdates)
	}
	if !d.rolledBack {
		t.Fatalf("lost lease must roll back the retry transaction: queries=%#v", d.queries)
	}
}

func TestSaveVideoTaskFailurePosterUpdateFailureRollsBackRetry(t *testing.T) {
	poster := "uploads/2026/poster_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg"
	d := &videoFailureSQL{
		attachmentLogo:      "/uploads/" + poster,
		attachmentUpdateErr: errors.New("database unavailable"),
	}
	store := &videoTestStorage{data: map[string][]byte{poster: []byte("poster")}}
	w := newVideoFailureSite(t, d, store)
	cfg := config.DefaultVideoProcessingConfig()
	cfg.MaxRetries = 2
	task := &model.VideoTask{
		Id:            9,
		AttachmentId:  7,
		Status:        model.VideoTaskStatusProcessing,
		Attempts:      1,
		Config:        cfg,
		Artifacts:     []string{poster},
		ArtifactsJSON: `["` + poster + `"]`,
		Playlist:      &model.VideoPlaylist{PosterURL: "/uploads/" + poster},
		LeaseOwner:    "worker-test",
		LeaseUntil:    time.Now().Add(videoTaskLeaseTTL).Unix(),
	}
	w.saveVideoTaskFailure(task, errors.New("ffmpeg failed"), nil)
	if !d.rolledBack || d.committed {
		t.Fatalf("poster update failure must roll back retry: committed=%v rolledBack=%v", d.committed, d.rolledBack)
	}
	if d.attachmentLogo != "/uploads/"+poster {
		t.Fatalf("rolled back poster update changed logo: %q", d.attachmentLogo)
	}
	assertVideoFailureUpdate(t, d, model.VideoTaskStatusFailed)
	taskArgs := videoFailureTaskArgs(d)
	foundDatabaseError := false
	for _, arg := range taskArgs {
		if value, ok := arg.Value.(string); ok && strings.Contains(value, "database unavailable") {
			foundDatabaseError = true
			break
		}
	}
	if !foundDatabaseError {
		t.Fatalf("terminal failure does not explain the rolled back retry: %#v", taskArgs)
	}
}

func TestSaveVideoTaskFailureCleanupErrorCancelsAutomaticRetry(t *testing.T) {
	poster := "uploads/2026/poster_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg"
	store := &videoTestStorage{
		data:      map[string][]byte{poster: []byte("poster")},
		deleteErr: errors.New("storage unavailable"),
	}
	d := &videoFailureSQL{}
	w := newVideoFailureSite(t, d, store)
	cfg := config.DefaultVideoProcessingConfig()
	cfg.MaxRetries = 2
	task := &model.VideoTask{
		Id:            9,
		AttachmentId:  7,
		Status:        model.VideoTaskStatusProcessing,
		Attempts:      1,
		Config:        cfg,
		Artifacts:     []string{poster},
		ArtifactsJSON: `["` + poster + `"]`,
		LeaseOwner:    "worker-test",
		LeaseUntil:    time.Now().Add(videoTaskLeaseTTL).Unix(),
	}
	w.saveVideoTaskFailure(task, errors.New("ffmpeg failed"), nil)
	assertVideoFailureUpdate(t, d, model.VideoTaskStatusFailed)
	taskArgs := videoFailureTaskArgs(d)
	cleanupMessage := ""
	for _, arg := range taskArgs {
		if value, ok := arg.Value.(string); ok && strings.Contains(value, "storage unavailable") {
			cleanupMessage = value
		}
	}
	if cleanupMessage == "" {
		t.Fatalf("cleanup failure is not observable in task error: %#v", taskArgs)
	}
}

func TestSaveVideoTaskFailureTerminalRetainsArtifactsForManualRetry(t *testing.T) {
	artifact := "video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.mp4"
	store := &videoTestStorage{data: map[string][]byte{artifact: []byte("artifact")}}
	d := &videoFailureSQL{}
	w := newVideoFailureSite(t, d, store)
	task := &model.VideoTask{
		Id:           9,
		AttachmentId: 7,
		Status:       model.VideoTaskStatusProcessing,
		Attempts:     3,
		Config:       config.DefaultVideoProcessingConfig(),
		Artifacts:    []string{artifact},
		LeaseOwner:   "worker-test",
		LeaseUntil:   time.Now().Add(videoTaskLeaseTTL).Unix(),
	}
	w.saveVideoTaskFailure(task, errors.New("ffmpeg failed"), nil)
	if _, exists := store.data[artifact]; !exists {
		t.Fatal("terminal failure must retain artifacts until manual retry cleans them")
	}
	assertVideoFailureUpdate(t, d, model.VideoTaskStatusFailed)
}

func assertVideoFailureUpdate(t *testing.T, d *videoFailureSQL, status int) {
	t.Helper()
	args := videoFailureTaskArgs(d)
	if args == nil {
		t.Fatalf("updates = %#v, want one failure update", d.queries)
	}
	for _, arg := range args {
		var value int
		switch typed := arg.Value.(type) {
		case int:
			value = typed
		case int64:
			value = int(typed)
		default:
			continue
		}
		if value == status {
			return
		}
	}
	t.Fatalf("failure update does not contain status %d: %#v", status, args)
}

func videoFailureTaskArgs(d *videoFailureSQL) []driver.NamedValue {
	for index := len(d.queries) - 1; index >= 0; index-- {
		if strings.Contains(strings.ToLower(d.queries[index]), "update `video_tasks`") && index < len(d.updateArgs) {
			return d.updateArgs[index]
		}
	}
	return nil
}

func videoUpdateContainsString(args []driver.NamedValue, want string) bool {
	for _, arg := range args {
		if value, ok := arg.Value.(string); ok && value == want {
			return true
		}
	}
	return false
}

type videoRetryCleanupSQL struct {
	task                      model.VideoTask
	attachmentLogo            string
	attachmentRowsAffected    int64
	attachmentRowsAffectedSet bool
	updates                   []string
	updateArgs                [][]driver.NamedValue
	committed                 bool
	rolledBack                bool
}

func (d *videoRetryCleanupSQL) Connect(context.Context) (driver.Conn, error) { return d, nil }
func (d *videoRetryCleanupSQL) Driver() driver.Driver                        { return d }
func (d *videoRetryCleanupSQL) Open(string) (driver.Conn, error)             { return d, nil }
func (d *videoRetryCleanupSQL) Close() error                                 { return nil }
func (d *videoRetryCleanupSQL) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (d *videoRetryCleanupSQL) Begin() (driver.Tx, error) { return &videoRetryCleanupTx{d: d}, nil }
func (d *videoRetryCleanupSQL) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	lower := strings.ToLower(query)
	switch {
	case strings.Contains(lower, "from `video_tasks`"):
		return &videoTaskRows{
			columns: []string{"id", "attachment_id", "status", "attempts", "config", "playlist", "artifacts"},
			data: [][]driver.Value{{
				int64(d.task.Id), int64(d.task.AttachmentId), int64(d.task.Status), int64(d.task.Attempts),
				d.task.ConfigJSON, d.task.PlaylistJSON, d.task.ArtifactsJSON,
			}},
		}, nil
	case strings.Contains(lower, "from `attachments`"):
		return &videoTaskRows{
			columns: []string{"id", "file_location", "file_md5", "logo"},
			data:    [][]driver.Value{{int64(7), "uploads/2026/source.mp4", "source-md5", d.attachmentLogo}},
		}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}
func (d *videoRetryCleanupSQL) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	d.updates = append(d.updates, query)
	d.updateArgs = append(d.updateArgs, append([]driver.NamedValue{}, args...))
	if strings.Contains(strings.ToLower(query), "update `attachments`") {
		rows := int64(1)
		if d.attachmentRowsAffectedSet {
			rows = d.attachmentRowsAffected
		}
		if rows == 1 {
			d.attachmentLogo = ""
		}
		return driver.RowsAffected(rows), nil
	}
	return driver.RowsAffected(1), nil
}

type videoRetryCleanupTx struct{ d *videoRetryCleanupSQL }

func (tx *videoRetryCleanupTx) Commit() error {
	tx.d.committed = true
	return nil
}
func (tx *videoRetryCleanupTx) Rollback() error {
	tx.d.rolledBack = true
	return nil
}

func newVideoRetryCleanupSite(t *testing.T, d *videoRetryCleanupSQL, store *videoTestStorage) *Website {
	t.Helper()
	conn := sql.OpenDB(d)
	t.Cleanup(func() { _ = conn.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &Website{
		Id:            3,
		DB:            db,
		Storage:       store,
		PluginStorage: &config.PluginStorageConfig{StorageType: config.StorageTypeLocal, StorageUrl: "/uploads"},
	}
}

type videoFailureSQL struct {
	queries                   []string
	updateArgs                [][]driver.NamedValue
	attachmentLocation        string
	attachmentMD5             string
	attachmentLogo            string
	attachmentRowsAffected    int64
	attachmentRowsAffectedSet bool
	attachmentUpdateErr       error
	leaseLost                 bool
	committed                 bool
	rolledBack                bool
	committedTaskUpdates      int
	activeTx                  *videoFailureTx
}

func (d *videoFailureSQL) Connect(context.Context) (driver.Conn, error) { return d, nil }
func (d *videoFailureSQL) Driver() driver.Driver                        { return d }
func (d *videoFailureSQL) Open(string) (driver.Conn, error)             { return d, nil }
func (d *videoFailureSQL) Close() error                                 { return nil }
func (d *videoFailureSQL) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (d *videoFailureSQL) Begin() (driver.Tx, error) {
	d.activeTx = &videoFailureTx{d: d}
	return d.activeTx, nil
}
func (d *videoFailureSQL) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	lower := strings.ToLower(query)
	switch {
	case strings.Contains(lower, "select `id` from `video_tasks`"):
		if d.leaseLost {
			return &videoTaskRows{columns: []string{"id"}}, nil
		}
		return &videoTaskRows{columns: []string{"id"}, data: [][]driver.Value{{int64(9)}}}, nil
	case strings.Contains(lower, "from `attachments`"):
		location := d.attachmentLocation
		if location == "" {
			location = "uploads/2026/source.mp4"
		}
		fileMD5 := d.attachmentMD5
		if fileMD5 == "" {
			fileMD5 = "source-md5"
		}
		return &videoTaskRows{
			columns: []string{"id", "file_location", "file_md5", "logo"},
			data:    [][]driver.Value{{int64(7), location, fileMD5, d.attachmentLogo}},
		}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}
func (d *videoFailureSQL) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	d.queries = append(d.queries, query)
	d.updateArgs = append(d.updateArgs, append([]driver.NamedValue{}, args...))
	lower := strings.ToLower(query)
	switch {
	case strings.Contains(lower, "update `attachments`"):
		if d.attachmentUpdateErr != nil {
			return nil, d.attachmentUpdateErr
		}
		rows := int64(1)
		if d.attachmentRowsAffectedSet {
			rows = d.attachmentRowsAffected
		}
		if rows == 1 {
			if d.activeTx != nil {
				d.activeTx.attachmentLogo = new(string)
			} else {
				d.attachmentLogo = ""
			}
		}
		return driver.RowsAffected(rows), nil
	case strings.Contains(lower, "update `video_tasks`"):
		if d.leaseLost {
			return driver.RowsAffected(0), nil
		}
		if d.activeTx != nil {
			d.activeTx.taskUpdates++
		} else {
			d.committedTaskUpdates++
		}
		return driver.RowsAffected(1), nil
	default:
		return nil, errors.New("unexpected update: " + query)
	}
}

type videoFailureTx struct {
	d              *videoFailureSQL
	attachmentLogo *string
	taskUpdates    int
}

func (tx *videoFailureTx) Commit() error {
	if tx.attachmentLogo != nil {
		tx.d.attachmentLogo = *tx.attachmentLogo
	}
	tx.d.committedTaskUpdates += tx.taskUpdates
	tx.d.committed = true
	tx.d.activeTx = nil
	return nil
}

func (tx *videoFailureTx) Rollback() error {
	tx.d.rolledBack = true
	tx.d.activeTx = nil
	return nil
}

func newVideoFailureSite(t *testing.T, d *videoFailureSQL, store *videoTestStorage) *Website {
	t.Helper()
	conn := sql.OpenDB(d)
	t.Cleanup(func() { _ = conn.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &Website{
		Id:            3,
		DB:            db,
		Storage:       store,
		PluginStorage: &config.PluginStorageConfig{StorageType: config.StorageTypeLocal, StorageUrl: "/uploads"},
	}
}

type videoTaskSQL struct {
	queries      []string
	updates      []string
	updateArgs   [][]driver.NamedValue
	rowsAffected int64
}

func (d *videoTaskSQL) Connect(context.Context) (driver.Conn, error) { return d, nil }
func (d *videoTaskSQL) Driver() driver.Driver                        { return d }
func (d *videoTaskSQL) Open(string) (driver.Conn, error)             { return d, nil }
func (d *videoTaskSQL) Close() error                                 { return nil }
func (d *videoTaskSQL) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (d *videoTaskSQL) Begin() (driver.Tx, error) {
	return videoSourceDeleteTx{}, nil
}
func (d *videoTaskSQL) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	d.queries = append(d.queries, query)
	switch {
	case strings.HasPrefix(query, "SELECT * FROM `video_tasks`"):
		return &videoTaskRows{
			columns: []string{"id", "attachment_id", "status"},
			data:    [][]driver.Value{{int64(9), int64(7), int64(model.VideoTaskStatusFailed)}},
		}, nil
	case strings.HasPrefix(query, "SELECT * FROM `attachments`"):
		return &videoTaskRows{
			columns: []string{"id", "file_location", "file_md5", "logo"},
			data:    [][]driver.Value{{int64(7), "uploads/source.mp4", "source-md5", ""}},
		}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}
func (d *videoTaskSQL) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	d.updates = append(d.updates, query)
	d.updateArgs = append(d.updateArgs, append([]driver.NamedValue{}, args...))
	return driver.RowsAffected(d.rowsAffected), nil
}

type videoTaskRows struct {
	columns []string
	data    [][]driver.Value
}

func (r *videoTaskRows) Columns() []string { return r.columns }
func (r *videoTaskRows) Close() error      { return nil }
func (r *videoTaskRows) Next(dest []driver.Value) error {
	if len(r.data) == 0 {
		return io.EOF
	}
	copy(dest, r.data[0])
	r.data = r.data[1:]
	return nil
}

func newVideoTaskSQLSite(t *testing.T, d *videoTaskSQL) *Website {
	t.Helper()
	conn := sql.OpenDB(d)
	t.Cleanup(func() { _ = conn.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &Website{Id: 3, DB: db}
}

func TestProcessVideoTaskDeletesSourceOnlyAfterSuccess(t *testing.T) {
	fn := mustFindFunc(t, "videoWorker.go", "processVideoTask")
	var savePos, deletePos token.Pos
	ast.Inspect(fn, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch selector.Sel.Name {
		case "saveVideoTaskSuccess":
			savePos = selector.Pos()
		case "deleteVideoTaskSource":
			deletePos = selector.Pos()
		}
		return true
	})
	if savePos == 0 || deletePos == 0 || savePos > deletePos {
		t.Fatalf("source deletion must follow task success persistence: save=%d delete=%d", savePos, deletePos)
	}
	helper := mustFindFunc(t, "videoWorker.go", "deleteVideoTaskSource")
	body := videoASTSource(t, helper)
	for _, want := range []string{"clause.Locking", "FileLocation", "FileMd5", "VideoTaskStatusSuccess", "DeleteFile"} {
		if !strings.Contains(body, want) {
			t.Fatalf("source deletion guard must contain %q", want)
		}
	}
}

func newVideoSuccessExecutionResult(format string) *videoTaskExecutionResult {
	poster := "uploads/poster_cccccccccccccccccccccccccccccccc.jpg"
	thumb := "uploads/thumb_poster_cccccccccccccccccccccccccccccccc.jpg"
	if format == config.VideoFormatHLS {
		master := "video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.m3u8"
		media := "video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/index.m3u8"
		return &videoTaskExecutionResult{
			Playlist: model.VideoPlaylist{
				Format:      config.VideoFormatHLS,
				DefaultURL:  "/uploads/" + master,
				DefaultType: config.VideoQuality480,
				PosterURL:   "/uploads/" + poster,
				Variants: []model.VideoVariant{
					{
						Name:   config.VideoQuality480,
						File:   media,
						URL:    "/uploads/" + media,
						Width:  640,
						Height: 480,
					},
				},
			},
			Artifacts: []string{master, media, poster, thumb},
			Width:     640,
			Height:    480,
		}
	}
	low := "video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4"
	high := "video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.mp4"
	return &videoTaskExecutionResult{
		Playlist: model.VideoPlaylist{
			Format:      config.VideoFormatMP4,
			DefaultURL:  "/uploads/" + high,
			DefaultType: config.VideoQuality720,
			PosterURL:   "/uploads/" + poster,
			Variants: []model.VideoVariant{
				{
					Name:   config.VideoQuality480,
					File:   low,
					URL:    "/uploads/" + low,
					Width:  640,
					Height: 480,
				},
				{
					Name:   config.VideoQuality720,
					File:   high,
					URL:    "/uploads/" + high,
					Width:  960,
					Height: 720,
				},
			},
		},
		Artifacts: []string{low, high, poster, thumb},
		Width:     960,
		Height:    720,
	}
}

func newVideoSuccessSite(t *testing.T, d *videoFailureSQL) *Website {
	t.Helper()
	conn := sql.OpenDB(d)
	t.Cleanup(func() { _ = conn.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &Website{
		Id:            3,
		DB:            db,
		Storage:       &videoTestStorage{data: map[string][]byte{}},
		PluginStorage: &config.PluginStorageConfig{StorageType: config.StorageTypeLocal, StorageUrl: "/uploads"},
	}
}

func newVideoSuccessTask() *model.VideoTask {
	return &model.VideoTask{
		Id:           9,
		AttachmentId: 7,
		Status:       model.VideoTaskStatusProcessing,
		Config:       config.DefaultVideoProcessingConfig(),
		LeaseOwner:   "worker-a",
		LeaseUntil:   time.Now().Add(time.Minute).Unix(),
	}
}

func videoAttachmentUpdate(t *testing.T, d *videoFailureSQL) (string, []driver.NamedValue) {
	t.Helper()
	for index, query := range d.queries {
		if strings.Contains(strings.ToLower(query), "update `attachments`") {
			if index >= len(d.updateArgs) {
				t.Fatalf("attachment update args are missing for %s", query)
			}
			return query, d.updateArgs[index]
		}
	}
	return "", nil
}

func TestSaveVideoTaskSuccessBackfillsDefaultPlayableLocation(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   string
	}{
		{
			name:   "mp4 uses default variant file",
			format: config.VideoFormatMP4,
			want:   "video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.mp4",
		},
		{
			name:   "hls uses master playlist",
			format: config.VideoFormatHLS,
			want:   "video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.m3u8",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &videoFailureSQL{
				attachmentLocation: "uploads/source.mp4",
				attachmentMD5:      "source-md5",
			}
			w := newVideoSuccessSite(t, d)
			task := newVideoSuccessTask()
			source := &model.Attachment{Id: 7, FileLocation: "uploads/source.mp4", FileMd5: "source-md5"}
			result := newVideoSuccessExecutionResult(tt.format)

			if err := w.saveVideoTaskSuccess(task, result, source); err != nil {
				t.Fatalf("save success: %v", err)
			}
			if !d.committed || d.rolledBack {
				t.Fatalf("transaction committed = %v, rolledBack = %v", d.committed, d.rolledBack)
			}
			query, args := videoAttachmentUpdate(t, d)
			if !strings.Contains(strings.ToLower(query), "`file_location`=?") {
				t.Fatalf("attachment update does not write file_location: %s", query)
			}
			found := false
			for _, arg := range args {
				if value, ok := arg.Value.(string); ok && value == tt.want {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("attachment args %#v do not contain playable location %q", args, tt.want)
			}
		})
	}
}

func TestSaveVideoTaskSuccessRejectsInvalidPlayableLocation(t *testing.T) {
	tests := []struct {
		name   string
		format string
		mutate func(*videoTaskExecutionResult)
	}{
		{
			name:   "empty default url",
			format: config.VideoFormatMP4,
			mutate: func(result *videoTaskExecutionResult) {
				result.Playlist.DefaultURL = ""
			},
		},
		{
			name:   "foreign default url",
			format: config.VideoFormatMP4,
			mutate: func(result *videoTaskExecutionResult) {
				result.Playlist.DefaultURL = "https://evil.example/video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.mp4"
			},
		},
		{
			name:   "default file absent from task artifacts",
			format: config.VideoFormatMP4,
			mutate: func(result *videoTaskExecutionResult) {
				result.Artifacts = append(result.Artifacts[:1], result.Artifacts[2:]...)
			},
		},
		{
			name:   "mp4 default variant missing",
			format: config.VideoFormatMP4,
			mutate: func(result *videoTaskExecutionResult) {
				result.Playlist.DefaultType = "1080p"
			},
		},
		{
			name:   "mp4 default url does not match default variant file",
			format: config.VideoFormatMP4,
			mutate: func(result *videoTaskExecutionResult) {
				result.Playlist.DefaultURL = "/uploads/video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4"
			},
		},
		{
			name:   "hls media playlist is not master",
			format: config.VideoFormatHLS,
			mutate: func(result *videoTaskExecutionResult) {
				result.Playlist.DefaultURL = "/uploads/video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/index.m3u8"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &videoFailureSQL{
				attachmentLocation: "uploads/source.mp4",
				attachmentMD5:      "source-md5",
			}
			w := newVideoSuccessSite(t, d)
			result := newVideoSuccessExecutionResult(tt.format)
			tt.mutate(result)

			if err := w.saveVideoTaskSuccess(newVideoSuccessTask(), result, &model.Attachment{
				Id:           7,
				FileLocation: "uploads/source.mp4",
				FileMd5:      "source-md5",
			}); err == nil {
				t.Fatal("invalid playable location was accepted")
			}
			if d.committed {
				t.Fatal("invalid playable location committed a successful task")
			}
			if query, _ := videoAttachmentUpdate(t, d); query != "" {
				t.Fatalf("invalid playable location updated attachment: %s", query)
			}
		})
	}
}

func TestSaveVideoTaskSuccessRollsBackWhenSourceOrLeaseChanges(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*videoFailureSQL, *model.Attachment)
	}{
		{
			name: "source location changed",
			configure: func(d *videoFailureSQL, _ *model.Attachment) {
				d.attachmentLocation = "uploads/replacement.mp4"
			},
		},
		{
			name: "source checksum changed",
			configure: func(d *videoFailureSQL, _ *model.Attachment) {
				d.attachmentMD5 = "replacement-md5"
			},
		},
		{
			name: "lease lost",
			configure: func(d *videoFailureSQL, _ *model.Attachment) {
				d.leaseLost = true
			},
		},
		{
			name: "attachment update failed",
			configure: func(d *videoFailureSQL, _ *model.Attachment) {
				d.attachmentUpdateErr = errors.New("database unavailable")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &videoFailureSQL{
				attachmentLocation: "uploads/source.mp4",
				attachmentMD5:      "source-md5",
			}
			source := &model.Attachment{Id: 7, FileLocation: "uploads/source.mp4", FileMd5: "source-md5"}
			tt.configure(d, source)
			w := newVideoSuccessSite(t, d)

			if err := w.saveVideoTaskSuccess(
				newVideoSuccessTask(),
				newVideoSuccessExecutionResult(config.VideoFormatMP4),
				source,
			); err == nil {
				t.Fatal("source or lease change was accepted")
			}
			if d.committed || !d.rolledBack {
				t.Fatalf("transaction committed = %v, rolledBack = %v", d.committed, d.rolledBack)
			}
		})
	}
}

func TestDeleteVideoTaskSourceHonorsOriginalQualityAndSafetyGuards(t *testing.T) {
	const outputLocation = "video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4"
	const playlistJSON = `{"format":"mp4","default_url":"/uploads/video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4","default_type":"480p","variants":[{"name":"480p","file":"video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4","url":"/uploads/video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4"}]}`
	const artifactsJSON = `["video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4"]`
	tests := []struct {
		name               string
		qualities          []string
		taskStatus         int
		activeTasks        int64
		attachmentLocation string
		attachmentMD5      string
		queryErr           error
		wantDeleted        bool
		wantErr            bool
	}{
		{
			name:               "preserve source when original is selected",
			qualities:          []string{config.VideoQuality480, config.VideoQualityOriginal},
			taskStatus:         model.VideoTaskStatusSuccess,
			attachmentLocation: outputLocation,
			wantDeleted:        false,
		},
		{
			name:               "delete source after transcoded outputs succeed",
			qualities:          []string{config.VideoQuality480, config.VideoQuality720},
			taskStatus:         model.VideoTaskStatusSuccess,
			attachmentLocation: outputLocation,
			wantDeleted:        true,
		},
		{
			name:               "keep source while another task is active",
			qualities:          []string{config.VideoQuality480},
			taskStatus:         model.VideoTaskStatusSuccess,
			activeTasks:        1,
			attachmentLocation: outputLocation,
		},
		{
			name:       "keep source before task success",
			qualities:  []string{config.VideoQuality480},
			taskStatus: model.VideoTaskStatusProcessing,
			wantErr:    true,
		},
		{
			name:               "keep replacement source",
			qualities:          []string{config.VideoQuality480},
			taskStatus:         model.VideoTaskStatusSuccess,
			attachmentLocation: "uploads/replacement.mp4",
			wantErr:            true,
		},
		{
			name:       "keep source when database validation fails",
			qualities:  []string{config.VideoQuality480},
			taskStatus: model.VideoTaskStatusSuccess,
			queryErr:   errors.New("database unavailable"),
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publicDir := t.TempDir() + "/"
			location := "uploads/source.mp4"
			sourcePath := filepath.Join(publicDir, location)
			if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(sourcePath, []byte("source"), 0o600); err != nil {
				t.Fatal(err)
			}
			outputPath := filepath.Join(publicDir, outputLocation)
			if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(outputPath, []byte("rendered"), 0o600); err != nil {
				t.Fatal(err)
			}
			d := &videoSourceDeleteSQL{
				attachmentLocation: tt.attachmentLocation,
				attachmentMD5:      tt.attachmentMD5,
				completed:          tt.taskStatus == model.VideoTaskStatusSuccess,
				playlistJSON:       playlistJSON,
				artifactsJSON:      artifactsJSON,
				activeTasks:        tt.activeTasks,
				queryErr:           tt.queryErr,
			}
			if d.attachmentLocation == "" {
				d.attachmentLocation = location
			}
			if d.attachmentMD5 == "" {
				d.attachmentMD5 = "source-md5"
			}
			conn := sql.OpenDB(d)
			t.Cleanup(func() { _ = conn.Close() })
			db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{
				DisableAutomaticPing:   true,
				SkipDefaultTransaction: true,
				Logger:                 logger.Default.LogMode(logger.Silent),
			})
			if err != nil {
				t.Fatal(err)
			}
			store, err := storage.NewLocalStorage(&config.PluginStorageConfig{StorageType: config.StorageTypeLocal}, publicDir)
			if err != nil {
				t.Fatal(err)
			}
			w := &Website{
				Id:            3,
				DB:            db,
				PublicPath:    publicDir,
				Storage:       store,
				PluginStorage: &config.PluginStorageConfig{StorageType: config.StorageTypeLocal, StorageUrl: "/uploads"},
			}
			task := &model.VideoTask{
				Id:           9,
				AttachmentId: 7,
				Status:       tt.taskStatus,
				Config:       config.VideoProcessingConfig{Qualities: tt.qualities},
			}
			source := &model.Attachment{Id: 7, FileLocation: location, FileMd5: "source-md5"}
			err = w.deleteVideoTaskSource(task, source)
			if (err != nil) != tt.wantErr {
				t.Fatalf("delete error = %v, wantErr %v", err, tt.wantErr)
			}
			_, statErr := os.Stat(sourcePath)
			if tt.wantDeleted {
				if !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("source still exists after successful transcode: %v", statErr)
				}
			} else if statErr != nil {
				t.Fatalf("source must be preserved: %v", statErr)
			}
			output, readErr := os.ReadFile(outputPath)
			if readErr != nil || string(output) != "rendered" {
				t.Fatalf("playable output must be preserved: content=%q, err=%v", output, readErr)
			}
		})
	}
}

type videoSourceDeleteSQL struct {
	attachmentLocation string
	attachmentMD5      string
	completed          bool
	playlistJSON       string
	artifactsJSON      string
	activeTasks        int64
	queryErr           error
}

func (d *videoSourceDeleteSQL) Connect(context.Context) (driver.Conn, error) { return d, nil }
func (d *videoSourceDeleteSQL) Driver() driver.Driver                        { return d }
func (d *videoSourceDeleteSQL) Open(string) (driver.Conn, error)             { return d, nil }
func (d *videoSourceDeleteSQL) Close() error                                 { return nil }
func (d *videoSourceDeleteSQL) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (d *videoSourceDeleteSQL) Begin() (driver.Tx, error) { return videoSourceDeleteTx{}, nil }
func (d *videoSourceDeleteSQL) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if d.queryErr != nil {
		return nil, d.queryErr
	}
	lower := strings.ToLower(query)
	switch {
	case strings.Contains(lower, "from `attachments`"):
		return &videoTaskRows{
			columns: []string{"id", "file_location", "file_md5"},
			data:    [][]driver.Value{{int64(7), d.attachmentLocation, d.attachmentMD5}},
		}, nil
	case strings.Contains(lower, "from `video_tasks`") && strings.Contains(lower, "`status` = ?"):
		if !d.completed {
			return &videoTaskRows{columns: []string{"id"}}, nil
		}
		return &videoTaskRows{
			columns: []string{"id", "playlist", "artifacts"},
			data:    [][]driver.Value{{int64(9), d.playlistJSON, d.artifactsJSON}},
		}, nil
	case strings.Contains(lower, "from `video_tasks`") && strings.Contains(lower, "count(*)"):
		return &videoTaskRows{columns: []string{"count"}, data: [][]driver.Value{{d.activeTasks}}}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

type videoSourceDeleteTx struct{}

func (videoSourceDeleteTx) Commit() error   { return nil }
func (videoSourceDeleteTx) Rollback() error { return nil }

var _ storage.Storage = (*videoTestStorage)(nil)
