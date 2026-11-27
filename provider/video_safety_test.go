package provider

import (
	"go/ast"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	"kandaoni.com/anqicms/model"
)

func TestVideoTaskArtifactLocationsRejectsUncontrolledPaths(t *testing.T) {
	task := &model.VideoTask{
		Id:           9,
		AttachmentId: 7,
		Artifacts: []string{
			"video/3/7/9/poster.jpg",
			"uploads/unrelated/poster.jpg",
			"../outside.jpg",
			"video/3/7/90/poster.jpg",
		},
	}
	got := videoTaskArtifactLocations(3, task, "uploads/source.mp4", "https://cdn.example")
	if len(got) != 1 || got[0] != "video/3/7/9/poster.jpg" {
		t.Fatalf("artifact locations = %#v, want only the task-owned poster", got)
	}
}

func TestVideoExecutionArtifactsAllowOnlyTaskRootOrExactPosterArtifacts(t *testing.T) {
	root := "video/3/7/9"
	poster := "uploads/2026/poster_0123456789abcdef0123456789abcdef.jpg"
	tests := []struct {
		name     string
		location string
		want     string
		allowed  bool
	}{
		{
			name:     "task owned video",
			location: root + "/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4",
			want:     root + "/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4",
			allowed:  true,
		},
		{
			name:     "exact source directory poster",
			location: poster,
			want:     poster,
			allowed:  true,
		},
		{
			name:     "exact poster thumbnail",
			location: "uploads/2026/thumb_poster_0123456789abcdef0123456789abcdef.jpg",
			want:     "uploads/2026/thumb_poster_0123456789abcdef0123456789abcdef.jpg",
			allowed:  true,
		},
		{
			name:     "another poster thumbnail in source directory",
			location: "uploads/2026/thumb_poster_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.jpg",
			allowed:  false,
		},
		{
			name:     "thumbnail for an unrelated source directory image",
			location: "uploads/2026/thumb_unrelated.jpg",
			allowed:  false,
		},
		{
			name:     "another poster in source directory",
			location: "uploads/2026/poster_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.jpg",
			allowed:  false,
		},
		{
			name:     "non poster in source directory",
			location: "uploads/2026/unrelated.jpg",
			allowed:  false,
		},
		{
			name:     "poster in another attachment directory",
			location: "uploads/other/poster_0123456789abcdef0123456789abcdef.jpg",
			allowed:  false,
		},
		{
			name:     "cross site task path",
			location: "video/4/7/9/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.mp4",
			allowed:  false,
		},
		{
			name:     "traversal",
			location: "../outside.jpg",
			allowed:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := safeVideoExecutionArtifactLocation(root, poster, tt.location)
			if tt.allowed {
				if err != nil || got != tt.want {
					t.Fatalf("location = %q, err = %v, want %q", got, err, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("location %q must be rejected, got %q", tt.location, got)
			}
		})
	}
}

func TestVideoPosterLocationFromURL(t *testing.T) {
	poster := "uploads/2026/poster_0123456789abcdef0123456789abcdef.jpg"
	tests := []struct {
		name       string
		posterURL  string
		storageURL string
		want       string
		allowed    bool
	}{
		{
			name:       "absolute storage URL",
			posterURL:  "https://cdn.example/" + poster,
			storageURL: "https://cdn.example",
			want:       poster,
			allowed:    true,
		},
		{
			name:       "local storage URL",
			posterURL:  "/uploads/" + poster,
			storageURL: "/uploads",
			want:       poster,
			allowed:    true,
		},
		{
			name:       "foreign URL",
			posterURL:  "https://other.example/" + poster,
			storageURL: "https://cdn.example",
			allowed:    false,
		},
		{
			name:       "non poster",
			posterURL:  "https://cdn.example/uploads/2026/unrelated.jpg",
			storageURL: "https://cdn.example",
			allowed:    false,
		},
		{
			name:       "another directory",
			posterURL:  "https://cdn.example/uploads/other/poster_0123456789abcdef0123456789abcdef.jpg",
			storageURL: "https://cdn.example",
			allowed:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := videoPosterLocationFromURL("uploads/2026/source.mp4", tt.posterURL, tt.storageURL)
			if ok != tt.allowed || got != tt.want {
				t.Fatalf("poster location = %q, allowed = %v, want %q, %v", got, ok, tt.want, tt.allowed)
			}
		})
	}
}

func TestVideoWorkerMutationsRequireUnexpiredLease(t *testing.T) {
	for _, name := range []string{"updateVideoTaskProgress", "saveVideoTaskSuccess", "saveVideoTaskFailure"} {
		t.Run(name, func(t *testing.T) {
			fn := mustFindFunc(t, "videoWorker.go", name)
			body := videoASTSource(t, fn)
			if !strings.Contains(body, "videoTaskLeaseScope") {
				t.Fatalf("%s must use the shared lease scope", name)
			}
		})
	}
}

func TestVideoLeaseScopeRejectsExpiredOwnership(t *testing.T) {
	fn := mustFindFunc(t, "videoWorker.go", "videoTaskLeaseScope")
	body := videoASTSource(t, fn)
	for _, want := range []string{"lease_owner", "lease_until", "> ?", "VideoTaskStatusProcessing"} {
		if !strings.Contains(body, want) {
			t.Fatalf("lease scope must contain %q: %s", want, body)
		}
	}
}

func TestVideoSuccessLocksAttachmentAndVerifiesSourceIdentity(t *testing.T) {
	fn := mustFindFunc(t, "videoWorker.go", "saveVideoTaskSuccess")
	body := videoASTSource(t, fn)
	for _, want := range []string{"clause.Locking", "FileLocation", "FileMd5", "model.Attachment"} {
		if !strings.Contains(body, want) {
			t.Fatalf("saveVideoTaskSuccess must verify source identity under an attachment lock; missing %q", want)
		}
	}
}

func TestVideoFailureCleanupCanIdentifySourceDirectoryPoster(t *testing.T) {
	checks := []struct {
		file string
		fn   string
		want string
	}{
		{file: "videoWorker.go", fn: "deleteVideoTaskArtifacts", want: "source.FileLocation"},
		{file: "videoWorker.go", fn: "saveVideoTaskFailure", want: "partial.Playlist = &result.Playlist"},
	}
	for _, check := range checks {
		fn := mustFindFunc(t, check.file, check.fn)
		body := videoASTSource(t, fn)
		if !strings.Contains(body, check.want) {
			t.Fatalf("%s must preserve exact poster identity; missing %q", check.fn, check.want)
		}
	}
}

func TestDeleteAttachmentCancelsAtomicallyBeforeStorageDeletion(t *testing.T) {
	fn := mustFindFunc(t, "attachment.go", "DeleteAttachment")
	var transaction *ast.CallExpr
	ast.Inspect(fn, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || transaction != nil {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "Transaction" {
			transaction = call
		}
		return true
	})
	if transaction == nil {
		t.Fatal("DeleteAttachment must cancel tasks and delete rows in one transaction")
	}
	if len(transaction.Args) != 1 {
		t.Fatalf("DeleteAttachment transaction has %d arguments, want one callback", len(transaction.Args))
	}
	callback, ok := transaction.Args[0].(*ast.FuncLit)
	if !ok {
		t.Fatal("DeleteAttachment transaction callback is missing")
	}
	body := videoASTSource(t, callback.Body)
	for _, want := range []string{"clause.Locking", "VideoTaskStatusCancelled", "model.VideoTask", "model.Attachment"} {
		if !strings.Contains(body, want) {
			t.Fatalf("DeleteAttachment transaction must contain %q", want)
		}
	}
	ast.Inspect(callback.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "DeleteFile" {
			t.Error("storage I/O must happen after the cancellation transaction commits")
		}
		return true
	})
}

func TestVideoReplacementRejectsActiveTaskBeforeWriting(t *testing.T) {
	fn := mustFindFunc(t, "attachment.go", "AttachmentUpload")
	if !strings.Contains(videoASTSource(t, fn), "w.replaceVideoAttachment") {
		t.Fatal("video replacement must use the serialized replacement helper")
	}
	helper := mustFindFunc(t, "attachment.go", "replaceVideoAttachment")
	var uploadPos, activePos, lockPos token.Pos
	ast.Inspect(helper, func(node ast.Node) bool {
		if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "Locking" {
			lockPos = selector.Pos()
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch videoCallName(call.Fun) {
		case "w.UploadFile":
			uploadPos = call.Pos()
		}
		body := videoASTSource(t, call)
		if strings.Contains(body, "VideoTaskStatusPending") && strings.Contains(body, "VideoTaskStatusProcessing") {
			activePos = call.Pos()
		}
		return true
	})
	if activePos == 0 || lockPos == 0 || uploadPos == 0 {
		t.Fatalf("video replacement needs a locked active-task check before UploadFile: active=%d lock=%d upload=%d", activePos, lockPos, uploadPos)
	}
	if activePos > uploadPos || lockPos > uploadPos {
		t.Fatal("active video replacement must be rejected before overwriting source bytes")
	}
}

func TestRetryVideoTaskLocksAttachmentBeforeTask(t *testing.T) {
	fn := mustFindFunc(t, "videoTasks.go", "RetryVideoTask")
	var attachmentPos, taskPos token.Pos
	ast.Inspect(fn, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "First" {
			return true
		}
		switch videoASTSource(t, call.Args[0]) {
		case "&task":
			taskPos = call.Pos()
		case "&source":
			attachmentPos = call.Pos()
		}
		return true
	})
	if attachmentPos == 0 || taskPos == 0 || attachmentPos > taskPos {
		t.Fatalf("retry must lock attachment before task to match replacement/deletion order: attachment=%d task=%d", attachmentPos, taskPos)
	}
}

func TestSameVersionMigrationRepairsIncompleteVideoTaskTable(t *testing.T) {
	fn := mustFindFunc(t, "siteMigration.go", "ensureVideoTaskTable")
	body := videoASTSource(t, fn)
	if !strings.Contains(body, "videoTaskSchemaStatus") {
		t.Fatal("ensureVideoTaskTable must inspect required columns, timestamp types and indexes, not only HasTable")
	}
	if !strings.Contains(body, "AutoMigrate") {
		t.Fatal("incomplete video_tasks tables must be repaired through AutoMigrate")
	}
}

func TestHistoricalVideoBatchHonorsDisabledConfig(t *testing.T) {
	fn := mustFindFunc(t, "videoTasks.go", "EnqueueHistoricalVideoTasks")
	body := videoASTSource(t, fn)
	if !strings.Contains(body, "Enabled == 0") || !strings.Contains(body, "skipped++") {
		t.Fatal("disabled video processing must skip historical attachments instead of retrying or enqueueing")
	}
	if !strings.Contains(body, "task == nil") {
		t.Fatal("historical accounting must distinguish a newly created task from a no-op")
	}
}

func videoASTSource(t *testing.T, node ast.Node) string {
	t.Helper()
	var builder strings.Builder
	if err := printer.Fprint(&builder, token.NewFileSet(), node); err != nil {
		t.Fatal(err)
	}
	return builder.String()
}
