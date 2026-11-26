package provider

import (
	"context"
	"crypto/md5"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"mime/multipart"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

func TestIsVideoFileExt(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		want bool
	}{
		{name: "mp4", ext: ".mp4", want: true},
		{name: "m4v", ext: ".m4v", want: true},
		{name: "quicktime", ext: ".mov", want: true},
		{name: "matroska", ext: ".mkv", want: true},
		{name: "uppercase", ext: ".WEBM", want: true},
		{name: "audio", ext: ".mp3", want: false},
		{name: "empty", ext: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isVideoFileExt(tt.ext); got != tt.want {
				t.Fatalf("isVideoFileExt(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestVideoTaskArtifactLocations(t *testing.T) {
	task := &model.VideoTask{
		Id:           9,
		AttachmentId: 7,
		Artifacts:    []string{"video/3/7/9/poster.jpg", "video/3/7/9/720/index.m3u8", "uploads/video/source.mp4"},
		Playlist: &model.VideoPlaylist{
			PosterURL: "https://cdn.example/video/3/7/9/poster.jpg",
			Variants: []model.VideoVariant{
				{Name: "720p", File: "video/3/7/9/720/index.m3u8", URL: "https://cdn.example/video/3/7/9/720/index.m3u8"},
			},
		},
	}
	got := videoTaskArtifactLocations(3, task, "uploads/video/source.mp4", "https://cdn.example")
	want := []string{"video/3/7/9/poster.jpg", "video/3/7/9/720/index.m3u8"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("artifacts = %#v, want %#v", got, want)
	}
}

func TestVideoTaskArtifactLocationsIncludesSourceDirectoryPoster(t *testing.T) {
	poster := "uploads/video/poster_0123456789abcdef0123456789abcdef.jpg"
	task := &model.VideoTask{
		Id:           9,
		AttachmentId: 7,
		Artifacts: []string{
			"video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4",
			poster,
			"uploads/video/unrelated.jpg",
			"uploads/other/poster_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.jpg",
			"uploads/video/source.mp4",
		},
		Playlist: &model.VideoPlaylist{
			PosterURL: "https://cdn.example/" + poster,
			Variants: []model.VideoVariant{
				{Name: "720p", File: "video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4"},
			},
		},
	}
	got := videoTaskArtifactLocations(3, task, "uploads/video/source.mp4", "https://cdn.example")
	want := []string{
		"video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4",
		poster,
		"uploads/video/thumb_poster_0123456789abcdef0123456789abcdef.jpg",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("artifacts = %#v, want task video and exact source-directory poster with thumbnail %#v", got, want)
	}
}

func TestVideoTaskArtifactLocationsIncludesLocalPosterURL(t *testing.T) {
	poster := "uploads/video/poster_0123456789abcdef0123456789abcdef.jpg"
	task := &model.VideoTask{
		Id:           9,
		AttachmentId: 7,
		Playlist: &model.VideoPlaylist{
			PosterURL: "/uploads/" + poster,
			Variants:  []model.VideoVariant{},
		},
	}
	got := videoTaskArtifactLocations(3, task, "uploads/video/source.mp4", "/uploads")
	want := []string{poster, "uploads/video/thumb_poster_0123456789abcdef0123456789abcdef.jpg"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("artifacts = %#v, want local poster with thumbnail %#v", got, want)
	}
}

func TestVideoTaskArtifactLocationsIncludesRecordedPosterAfterBackfill(t *testing.T) {
	poster := "uploads/video/poster_0123456789abcdef0123456789abcdef.jpg"
	task := &model.VideoTask{
		Id:           9,
		AttachmentId: 7,
		Artifacts: []string{
			"video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4",
			"video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.mp4",
			poster,
		},
		Playlist: &model.VideoPlaylist{
			PosterURL: "https://cdn.example/" + poster,
			Variants: []model.VideoVariant{
				{Name: "720p", File: "video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.mp4"},
			},
		},
	}
	got := videoTaskArtifactLocations(3, task, "video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4", "https://cdn.example")
	want := []string{
		"video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.mp4",
		poster,
		"uploads/video/thumb_poster_0123456789abcdef0123456789abcdef.jpg",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("artifacts = %#v, want task artifacts and recorded poster with thumbnail %#v", got, want)
	}
}

func TestDeleteAttachmentCleansBackfilledHLSAndRecordedPoster(t *testing.T) {
	const master = "video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.m3u8"
	const media = "video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/index.m3u8"
	const segment = "video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/segment_000.ts"
	const poster = "uploads/202611/poster_cccccccccccccccccccccccccccccccc.jpg"
	const thumb = "uploads/202611/thumb_poster_cccccccccccccccccccccccccccccccc.jpg"
	const playlistJSON = `{"format":"hls","default_url":"/uploads/video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.m3u8","default_type":"480p","poster_url":"/uploads/uploads/202611/poster_cccccccccccccccccccccccccccccccc.jpg","variants":[{"name":"480p","file":"video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/index.m3u8","url":"/uploads/video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/index.m3u8"}]}`
	const artifactsJSON = `["video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.m3u8","video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/index.m3u8","video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/segment_000.ts","uploads/202611/poster_cccccccccccccccccccccccccccccccc.jpg"]`
	d := &videoAttachmentDeleteSQL{playlistJSON: playlistJSON, artifactsJSON: artifactsJSON}
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
	store := &videoTestStorage{data: map[string][]byte{
		master:  []byte("master"),
		media:   []byte("media"),
		segment: []byte("segment"),
		poster:  []byte("poster"),
		thumb:   []byte("thumb"),
	}}
	store.beforeDelete = func(string) error {
		if !d.committed {
			return errors.New("storage deleted before attachment transaction committed")
		}
		return nil
	}
	w := &Website{
		Id:            3,
		DB:            db,
		Storage:       store,
		PluginStorage: &config.PluginStorageConfig{StorageType: config.StorageTypeLocal, StorageUrl: "/uploads"},
	}
	if err := w.DeleteAttachment(&model.Attachment{Id: 7, FileLocation: master, IsImage: 2}); err != nil {
		t.Fatal(err)
	}
	if !d.committed || d.rolledBack {
		t.Fatalf("delete transaction committed = %v, rolledBack = %v", d.committed, d.rolledBack)
	}
	if len(store.data) != 0 {
		t.Fatalf("delete left video artifacts or poster behind: %#v", store.data)
	}
}

type videoAttachmentDeleteSQL struct {
	playlistJSON  string
	artifactsJSON string
	committed     bool
	rolledBack    bool
}

func (d *videoAttachmentDeleteSQL) Connect(context.Context) (driver.Conn, error) { return d, nil }
func (d *videoAttachmentDeleteSQL) Driver() driver.Driver                        { return d }
func (d *videoAttachmentDeleteSQL) Open(string) (driver.Conn, error)             { return d, nil }
func (d *videoAttachmentDeleteSQL) Close() error                                 { return nil }
func (d *videoAttachmentDeleteSQL) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (d *videoAttachmentDeleteSQL) Begin() (driver.Tx, error) { return d, nil }
func (d *videoAttachmentDeleteSQL) Commit() error {
	d.committed = true
	return nil
}
func (d *videoAttachmentDeleteSQL) Rollback() error {
	d.rolledBack = true
	return nil
}
func (d *videoAttachmentDeleteSQL) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	lower := strings.ToLower(query)
	switch {
	case strings.Contains(lower, "from `attachments`"):
		return &videoTaskRows{
			columns: []string{"id", "file_location", "file_md5", "logo", "is_image"},
			data:    [][]driver.Value{{int64(7), "video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.m3u8", "old-md5", "", int64(2)}},
		}, nil
	case strings.Contains(lower, "from `video_tasks`"):
		return &videoTaskRows{
			columns: []string{"id", "attachment_id", "playlist", "artifacts"},
			data:    [][]driver.Value{{int64(9), int64(7), d.playlistJSON, d.artifactsJSON}},
		}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}
func (d *videoAttachmentDeleteSQL) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}

func TestAttachmentUploadEnqueuesVideoWithoutBlockingOnFFmpeg(t *testing.T) {
	fn := mustFindFunc(t, "attachment.go", "AttachmentUpload")
	var savePos, enqueuePos token.Pos
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := videoCallName(call.Fun)
		if name == "exec.Command" {
			t.Error("AttachmentUpload 不得同步执行 ffmpeg，视频处理必须进入异步队列")
		}
		if name == "attachment.Save" && savePos == 0 {
			savePos = call.Pos()
		}
		if name == "w.EnqueueVideoTask" {
			enqueuePos = call.Pos()
		}
		return true
	})
	if savePos == 0 || enqueuePos == 0 {
		t.Fatal("AttachmentUpload 必须先保存附件再创建视频任务")
	}
	if savePos > enqueuePos {
		t.Error("视频任务必须在附件落库后创建")
	}
}

func TestReplaceVideoAttachmentDatabaseFailurePreservesOriginal(t *testing.T) {
	w, d, replacement, file := newVideoReplacementFixture(t, errors.New("attachment update failed"))
	if err := w.replaceVideoAttachment(replacement, replacement.FileLocation, file, "old-md5"); err == nil {
		t.Fatal("replacement must report the database update failure")
	}
	if d.committed || !d.rolledBack {
		t.Fatalf("failed replacement must roll back: committed=%v rolledBack=%v", d.committed, d.rolledBack)
	}
	store := w.Storage.(*videoTestStorage)
	if got := string(store.data["uploads/source.mp4"]); got != "old source" {
		t.Fatalf("database rollback left new bytes at the old URL: %q", got)
	}
	if len(store.data) != 1 {
		t.Fatalf("database rollback left an orphaned replacement: %#v", store.data)
	}
}

func TestReplaceVideoAttachmentStorageFailurePreservesOriginal(t *testing.T) {
	w, d, replacement, file := newVideoReplacementFixture(t, nil)
	w.Storage.(*videoTestStorage).putErr = errors.New("storage unavailable")
	if err := w.replaceVideoAttachment(replacement, replacement.FileLocation, file, "old-md5"); err == nil {
		t.Fatal("replacement must report the storage failure")
	}
	if got := string(w.Storage.(*videoTestStorage).data["uploads/source.mp4"]); got != "old source" {
		t.Fatalf("storage failure changed the original source: %q", got)
	}
	if d.committed || d.attachmentUpdates != 0 {
		t.Fatalf("storage failure reached the database: committed=%v updates=%d", d.committed, d.attachmentUpdates)
	}
}

func TestReplaceVideoAttachmentCommitsNewLocationBeforeDeletingOriginal(t *testing.T) {
	w, d, replacement, file := newVideoReplacementFixture(t, nil)
	if err := w.replaceVideoAttachment(replacement, replacement.FileLocation, file, "old-md5"); err != nil {
		t.Fatal(err)
	}
	if !d.committed || d.rolledBack {
		t.Fatalf("successful replacement must commit: committed=%v rolledBack=%v", d.committed, d.rolledBack)
	}
	if !regexp.MustCompile(`^uploads/source_[0-9a-f]{32}\.mp4$`).MatchString(replacement.FileLocation) {
		t.Fatalf("replacement location = %q, want an independent random source key", replacement.FileLocation)
	}
	store := w.Storage.(*videoTestStorage)
	if got := string(store.data[replacement.FileLocation]); got != "new source" {
		t.Fatalf("new source = %q", got)
	}
	if _, exists := store.data["uploads/source.mp4"]; exists {
		t.Fatal("original source must be deleted only after the database commit")
	}
}

func TestAttachmentUploadReplacesProcessedHLSUsingUploadedVideoExtension(t *testing.T) {
	w, d, _, file := newVideoReplacementFixture(t, nil)
	const currentLocation = "video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.m3u8"
	d.attachmentLocation = currentLocation
	d.attachmentMD5 = "old-md5"
	d.fileName = "source.mp4"
	store := w.Storage.(*videoTestStorage)
	store.data = map[string][]byte{currentLocation: []byte("old master")}
	store.beforeDelete = func(location string) error {
		if location == currentLocation && !d.committed {
			return errors.New("original playback deleted before database commit")
		}
		return nil
	}
	w.Content = &config.ContentConfig{Video: config.VideoProcessingConfig{
		Enabled: 0,
		Format:  config.VideoFormatMP4,
	}}

	got, err := w.AttachmentUpload(file, &multipart.FileHeader{
		Filename: "replacement.mp4",
		Size:     int64(len("new source")),
	}, 0, 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^uploads/[0-9]{6}/[0-9]{2}/source_[0-9a-f]{32}\.mp4$`).MatchString(got.FileLocation) {
		t.Fatalf("replacement location = %q, want a fresh uploads key with the uploaded mp4 extension", got.FileLocation)
	}
	if string(store.data[got.FileLocation]) != "new source" {
		t.Fatalf("new source bytes are missing at %q", got.FileLocation)
	}
	if _, exists := store.data[currentLocation]; exists {
		t.Fatal("old default playback must be deleted after replacement commits")
	}
}

func TestAttachmentUploadKeepsProcessedPlaybackWhenContentUnchanged(t *testing.T) {
	w, d, _, file := newVideoReplacementFixture(t, nil)
	const currentLocation = "video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.m3u8"
	d.attachmentLocation = currentLocation
	d.attachmentMD5 = fmt.Sprintf("%x", md5.Sum([]byte("new source")))
	d.fileName = "source.mp4"
	store := w.Storage.(*videoTestStorage)
	store.data = map[string][]byte{currentLocation: []byte("master")}
	w.Content = &config.ContentConfig{Video: config.VideoProcessingConfig{
		Enabled: 0,
		Format:  config.VideoFormatMP4,
	}}

	got, err := w.AttachmentUpload(file, &multipart.FileHeader{
		Filename: "replacement.mp4",
		Size:     int64(len("new source")),
	}, 0, 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.FileLocation != currentLocation {
		t.Fatalf("unchanged replacement location = %q, want current playback %q", got.FileLocation, currentLocation)
	}
	if d.attachmentUpdates != 0 || d.committed {
		t.Fatalf("unchanged replacement wrote the attachment: updates=%d committed=%v", d.attachmentUpdates, d.committed)
	}
	if len(store.data) != 1 || string(store.data[currentLocation]) != "master" {
		t.Fatalf("unchanged replacement changed stored playback: %#v", store.data)
	}
}

func TestAttachmentUploadDuplicateDoesNotOverwriteProcessedPlayback(t *testing.T) {
	for _, currentLocation := range []string{
		"video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.m3u8",
		"video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.mp4",
	} {
		t.Run(currentLocation, func(t *testing.T) {
			w, d, _, file := newVideoReplacementFixture(t, nil)
			d.duplicate = true
			d.attachmentLocation = currentLocation
			d.attachmentMD5 = fmt.Sprintf("%x", md5.Sum([]byte("new source")))
			store := w.Storage.(*videoTestStorage)
			store.data = map[string][]byte{currentLocation: []byte("processed playback")}
			w.Content = &config.ContentConfig{Video: config.VideoProcessingConfig{
				Enabled: 0,
				Format:  config.VideoFormatMP4,
			}}

			got, err := w.AttachmentUpload(file, &multipart.FileHeader{
				Filename: "duplicate.mp4",
				Size:     int64(len("new source")),
			}, 0, 0, 1)
			if err != nil {
				t.Fatal(err)
			}
			if got.FileLocation != currentLocation || got.IsImage != 2 {
				t.Fatalf("duplicate attachment = %q, is_image=%d; want processed video %q", got.FileLocation, got.IsImage, currentLocation)
			}
			if len(store.puts) != 0 || d.attachmentUpdates != 0 {
				t.Fatalf("duplicate upload rewrote playback: puts=%#v updates=%d", store.puts, d.attachmentUpdates)
			}
			if string(store.data[currentLocation]) != "processed playback" {
				t.Fatalf("duplicate upload changed playback bytes: %q", store.data[currentLocation])
			}
		})
	}
}

func TestReplaceVideoAttachmentCleansAllPreviousTaskArtifacts(t *testing.T) {
	w, d, _, file := newVideoReplacementFixture(t, nil)
	const master = "video/3/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.m3u8"
	const media = "video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/index.m3u8"
	const segment = "video/3/7/9/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/segment_000.ts"
	const poster = "uploads/202611/poster_cccccccccccccccccccccccccccccccc.jpg"
	const thumb = "uploads/202611/thumb_poster_cccccccccccccccccccccccccccccccc.jpg"
	d.attachmentLocation = master
	d.oldPlaylistJSON = fmt.Sprintf(`{"format":"hls","default_url":"/%s","poster_url":"/%s","variants":[{"name":"480p","file":"%s"}]}`, master, poster, media)
	d.oldArtifactsJSON = fmt.Sprintf(`["%s","%s","%s","%s"]`, master, media, segment, poster)
	store := w.Storage.(*videoTestStorage)
	store.data = map[string][]byte{
		master:  []byte("master"),
		media:   []byte("media"),
		segment: []byte("segment"),
		poster:  []byte("poster"),
		thumb:   []byte("thumb"),
	}
	oldLocations := map[string]bool{master: true, media: true, segment: true, poster: true, thumb: true}
	store.beforeDelete = func(location string) error {
		if oldLocations[location] && !d.committed {
			return errors.New("previous video artifact deleted before replacement committed")
		}
		return nil
	}

	if err := w.replaceVideoAttachment(&model.Attachment{
		Id:           7,
		FileName:     "source.mp4",
		FileLocation: "uploads/202611/26/source.mp4",
		FileMd5:      "new-md5",
		IsImage:      2,
		Status:       1,
	}, master, file, "old-md5"); err != nil {
		t.Fatal(err)
	}
	if len(store.data) != 1 {
		t.Fatalf("replacement left previous task artifacts behind: %#v", store.data)
	}
	for location := range store.data {
		if !strings.HasPrefix(location, "uploads/202611/26/source_") {
			t.Fatalf("unexpected file left after replacement: %q", location)
		}
	}
	if len(d.taskUpdates) == 0 {
		t.Fatal("replacement must invalidate previous task metadata")
	}
}

func TestReplaceVideoAttachmentReconcilesUncertainCommitBeforeCleanup(t *testing.T) {
	w, d, replacement, file := newVideoReplacementFixture(t, nil)
	d.commitErr = errors.New("commit result unknown")
	d.commitApplied = true
	err := w.replaceVideoAttachment(replacement, replacement.FileLocation, file, "old-md5")
	store := w.Storage.(*videoTestStorage)
	if string(store.data[replacement.FileLocation]) != "new source" {
		t.Fatalf("committed replacement source was deleted after an uncertain commit: err=%v files=%#v", err, store.data)
	}
	if _, exists := store.data["uploads/source.mp4"]; exists {
		t.Fatalf("original source survived a confirmed replacement commit: err=%v files=%#v", err, store.data)
	}
	if len(store.data) != 1 {
		t.Fatalf("unexpected files after uncertain commit reconciliation: err=%v files=%#v", err, store.data)
	}
	if err != nil {
		t.Fatalf("confirmed commit must finish replacement cleanup: %v", err)
	}
}

func TestReplaceVideoAttachmentPreservesSourceWhenCommitErrorReadsOldState(t *testing.T) {
	w, d, replacement, file := newVideoReplacementFixture(t, nil)
	d.commitErr = errors.New("commit result unknown")
	err := w.replaceVideoAttachment(replacement, replacement.FileLocation, file, "old-md5")
	if err == nil {
		t.Fatal("an uncertain commit must still report the transaction failure")
	}
	store := w.Storage.(*videoTestStorage)
	if string(store.data[replacement.FileLocation]) != "new source" {
		t.Fatalf("possibly committed replacement source was deleted after reading old state: %#v", store.data)
	}
	if string(store.data["uploads/source.mp4"]) != "old source" {
		t.Fatalf("replacement changed original storage before commit verification: %#v", store.data)
	}
	if len(store.data) != 2 {
		t.Fatalf("uncertain commit must preserve both possible sources: %#v", store.data)
	}
}

func TestReplaceVideoAttachmentPreservesSourceWhenCommitCannotBeVerified(t *testing.T) {
	w, d, replacement, file := newVideoReplacementFixture(t, nil)
	d.commitErr = errors.New("commit result unknown")
	d.commitApplied = true
	d.queryErr = errors.New("verification unavailable")
	err := w.replaceVideoAttachment(replacement, replacement.FileLocation, file, "old-md5")
	if err == nil {
		t.Fatal("replacement must report an unverifiable commit")
	}
	store := w.Storage.(*videoTestStorage)
	if string(store.data[replacement.FileLocation]) != "new source" {
		t.Fatalf("possibly committed replacement source was deleted: %#v", store.data)
	}
	if string(store.data["uploads/source.mp4"]) != "old source" {
		t.Fatalf("replacement changed original storage before commit verification: %#v", store.data)
	}
}

func newVideoReplacementFixture(t *testing.T, saveErr error) (*Website, *videoReplacementSQL, *model.Attachment, *os.File) {
	t.Helper()
	filePath := filepath.Join(t.TempDir(), "replacement.mp4")
	if err := os.WriteFile(filePath, []byte("new source"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	d := &videoReplacementSQL{saveErr: saveErr}
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
	d.attachmentLocation = "uploads/source.mp4"
	d.attachmentMD5 = "old-md5"
	d.fileName = "source.mp4"
	store := &videoTestStorage{data: map[string][]byte{d.attachmentLocation: []byte("old source")}}
	store.beforeDelete = func(location string) error {
		if location == d.attachmentLocation && !d.committed {
			return errors.New("original source deleted before database commit")
		}
		return nil
	}
	w := &Website{
		Id:            3,
		DB:            db,
		Storage:       store,
		PluginStorage: &config.PluginStorageConfig{StorageType: config.StorageTypeLocal},
	}
	replacement := &model.Attachment{
		Id:           7,
		FileName:     "source.mp4",
		FileLocation: d.attachmentLocation,
		FileSize:     10,
		FileMd5:      "new-md5",
		IsImage:      2,
		Status:       1,
	}
	return w, d, replacement, file
}

type videoReplacementSQL struct {
	saveErr            error
	commitErr          error
	queryErr           error
	commitApplied      bool
	duplicate          bool
	oldPlaylistJSON    string
	oldArtifactsJSON   string
	persistedLocation  string
	persistedMD5       string
	attachmentLocation string
	attachmentMD5      string
	fileName           string
	attachmentUpdates  int
	taskUpdates        []string
	committed          bool
	rolledBack         bool
}

func (d *videoReplacementSQL) Connect(context.Context) (driver.Conn, error) { return d, nil }
func (d *videoReplacementSQL) Driver() driver.Driver                        { return d }
func (d *videoReplacementSQL) Open(string) (driver.Conn, error)             { return d, nil }
func (d *videoReplacementSQL) Close() error                                 { return nil }
func (d *videoReplacementSQL) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (d *videoReplacementSQL) Begin() (driver.Tx, error) { return d, nil }
func (d *videoReplacementSQL) Commit() error {
	d.committed = d.commitApplied || d.commitErr == nil
	return d.commitErr
}
func (d *videoReplacementSQL) Rollback() error {
	d.rolledBack = true
	return nil
}
func (d *videoReplacementSQL) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	lower := strings.ToLower(query)
	switch {
	case strings.Contains(lower, "from `attachments`"):
		if strings.Contains(lower, "`file_md5` = ?") {
			if d.duplicate {
				return &videoTaskRows{
					columns: []string{"id", "file_location", "file_md5", "file_name", "is_image"},
					data:    [][]driver.Value{{int64(7), d.attachmentLocation, d.attachmentMD5, d.fileName, int64(2)}},
				}, nil
			}
			return &videoTaskRows{columns: []string{"id", "file_location", "file_md5", "file_name", "is_image"}}, nil
		}
		location, checksum := d.attachmentLocation, d.attachmentMD5
		if d.attachmentUpdates > 0 && d.queryErr != nil {
			return nil, d.queryErr
		}
		if d.committed && d.persistedLocation != "" {
			location, checksum = d.persistedLocation, d.persistedMD5
		}
		return &videoTaskRows{
			columns: []string{"id", "file_location", "file_md5", "file_name", "is_image"},
			data:    [][]driver.Value{{int64(7), location, checksum, d.fileName, int64(2)}},
		}, nil
	case strings.Contains(lower, "from `video_tasks`") && strings.Contains(lower, "count(*)"):
		return &videoTaskRows{columns: []string{"count"}, data: [][]driver.Value{{int64(0)}}}, nil
	case strings.Contains(lower, "from `video_tasks`"):
		if d.duplicate {
			playlist := fmt.Sprintf(`{"default_url":"/uploads/%s","variants":[]}`, d.attachmentLocation)
			return &videoTaskRows{
				columns: []string{"id", "attachment_id", "status", "playlist", "artifacts"},
				data:    [][]driver.Value{{int64(9), int64(7), int64(2), playlist, "[]"}},
			}, nil
		}
		if d.oldPlaylistJSON != "" {
			return &videoTaskRows{
				columns: []string{"id", "attachment_id", "status", "playlist", "artifacts"},
				data:    [][]driver.Value{{int64(9), int64(7), int64(2), d.oldPlaylistJSON, d.oldArtifactsJSON}},
			}, nil
		}
		return &videoTaskRows{columns: []string{"id"}}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}
func (d *videoReplacementSQL) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if strings.HasPrefix(strings.ToUpper(query), "UPDATE `ATTACHMENTS`") {
		d.attachmentUpdates++
		if d.attachmentUpdates == 1 && d.saveErr != nil {
			return nil, d.saveErr
		}
		for _, arg := range args {
			value, ok := arg.Value.(string)
			if !ok {
				continue
			}
			if strings.HasPrefix(value, "uploads/") && strings.HasSuffix(value, ".mp4") {
				d.persistedLocation = value
			}
			if value == "new-md5" {
				d.persistedMD5 = value
			}
		}
	}
	if strings.HasPrefix(strings.ToUpper(query), "UPDATE `VIDEO_TASKS`") {
		d.taskUpdates = append(d.taskUpdates, query)
	}
	return driver.RowsAffected(1), nil
}

func videoCallName(expr ast.Expr) string {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.SelectorExpr:
		return videoCallName(node.X) + "." + node.Sel.Name
	default:
		return ""
	}
}
