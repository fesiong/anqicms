package model

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestVideoTaskTimeFieldsAreUnixSeconds(t *testing.T) {
	taskType := reflect.TypeOf(VideoTask{})
	for _, name := range []string{"CreatedTime", "UpdatedTime", "NextRetryAt", "StartedAt", "FinishedAt", "LeaseUntil"} {
		field, ok := taskType.FieldByName(name)
		if !ok || field.Type.Kind() != reflect.Int64 {
			t.Fatalf("VideoTask.%s must be an int64 Unix-seconds field", name)
		}
	}

	now := time.Unix(2000, 987654321)
	task := VideoTask{Status: VideoTaskStatusProcessing, Attempts: 1}
	task.MarkFailure("transcode failed", 2, now)
	value := reflect.ValueOf(task)
	if got := value.FieldByName("NextRetryAt").Int(); got != 2030 {
		t.Fatalf("next_retry_at = %d, want Unix seconds 2030", got)
	}
	if got := value.FieldByName("FinishedAt").Int(); got != 0 {
		t.Fatalf("retry finished_at = %d, want 0", got)
	}
	if got := value.FieldByName("LeaseUntil").Int(); got != 0 {
		t.Fatalf("retry lease_until = %d, want 0", got)
	}

	task = VideoTask{Status: VideoTaskStatusPending}
	task.MarkProcessing(now)
	value = reflect.ValueOf(task)
	if got := value.FieldByName("StartedAt").Int(); got != 2000 {
		t.Fatalf("started_at = %d, want Unix seconds 2000", got)
	}
	if got := value.FieldByName("LeaseUntil").Int(); got != 2120 {
		t.Fatalf("lease_until = %d, want Unix seconds 2120", got)
	}
	if got := value.FieldByName("NextRetryAt").Int(); got != 0 {
		t.Fatalf("processing next_retry_at = %d, want 0", got)
	}
}

func TestVideoTaskRetryState(t *testing.T) {
	tests := []struct {
		name       string
		task       VideoTask
		maxRetries int
		wantStatus int
		wantError  bool
	}{
		{
			name:       "first failure schedules retry",
			task:       VideoTask{Status: VideoTaskStatusProcessing, Attempts: 1},
			maxRetries: 2,
			wantStatus: VideoTaskStatusPending,
			wantError:  true,
		},
		{
			name:       "exhausted retries fail permanently",
			task:       VideoTask{Status: VideoTaskStatusProcessing, Attempts: 3},
			maxRetries: 2,
			wantStatus: VideoTaskStatusFailed,
			wantError:  true,
		},
		{
			name:       "disabled retries fail immediately",
			task:       VideoTask{Status: VideoTaskStatusPending, Attempts: 1},
			maxRetries: 0,
			wantStatus: VideoTaskStatusFailed,
			wantError:  true,
		},
		{
			name:       "successful task is not changed",
			task:       VideoTask{Status: VideoTaskStatusSuccess, Attempts: 1},
			maxRetries: 2,
			wantStatus: VideoTaskStatusSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Unix(2000, 0)
			tt.task.MarkFailure("transcode failed", tt.maxRetries, now)
			if tt.task.Status != tt.wantStatus {
				t.Fatalf("status = %d, want %d", tt.task.Status, tt.wantStatus)
			}
			if (tt.task.ErrorMsg != "") != tt.wantError {
				t.Fatalf("error = %q, wantError %v", tt.task.ErrorMsg, tt.wantError)
			}
			if tt.wantStatus == VideoTaskStatusPending && tt.task.NextRetryAt != now.Add(30*time.Second).Unix() {
				t.Fatalf("next retry = %d, want %d", tt.task.NextRetryAt, now.Add(30*time.Second).Unix())
			}
			if tt.wantStatus == VideoTaskStatusFailed && tt.task.FinishedAt != now.Unix() {
				t.Fatalf("finished at = %d, want %d", tt.task.FinishedAt, now.Unix())
			}
			if tt.task.LeaseUntil != 0 {
				t.Fatalf("failure must release lease: %d", tt.task.LeaseUntil)
			}
		})
	}
}

func TestVideoTaskClaimAndComplete(t *testing.T) {
	now := time.Unix(2000, 0)
	task := VideoTask{Status: VideoTaskStatusPending}
	task.MarkProcessing(now)
	if task.Status != VideoTaskStatusProcessing || task.StartedAt != now.Unix() {
		t.Fatalf("claimed task = %#v", task)
	}
	if task.LeaseUntil != now.Add(2*time.Minute).Unix() || task.NextRetryAt != 0 || task.FinishedAt != 0 {
		t.Fatalf("claimed task timestamps = %#v", task)
	}

	task.MarkSuccess("uploads/video/master.m3u8", now.Add(time.Minute))
	if task.Status != VideoTaskStatusSuccess || task.Progress != 100 {
		t.Fatalf("completed task = %#v", task)
	}
	if task.ErrorMsg != "" || task.FinishedAt != now.Add(time.Minute).Unix() {
		t.Fatalf("completed task error/finished = %#v", task)
	}
	if task.NextRetryAt != 0 || task.LeaseUntil != 0 {
		t.Fatalf("completed task must clear retry and lease timestamps: %#v", task)
	}
}

func TestVideoTaskPlaylistJSON(t *testing.T) {
	playlist := VideoPlaylist{
		Format:      "hls",
		DefaultURL:  "https://cdn.example/master.m3u8",
		PosterURL:   "https://cdn.example/poster.jpg",
		DefaultType: "720p",
		Variants: []VideoVariant{
			{Name: "720p", Height: 720, URL: "https://cdn.example/720/index.m3u8"},
		},
	}
	encoded, err := json.Marshal(playlist)
	if err != nil {
		t.Fatal(err)
	}
	var decoded VideoPlaylist
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.DefaultURL != playlist.DefaultURL || len(decoded.Variants) != 1 ||
		decoded.Variants[0].Name != "720p" {
		t.Fatalf("decoded playlist = %#v", decoded)
	}
}

func TestVideoTaskAfterFindDecodesPayloads(t *testing.T) {
	task := VideoTask{
		ConfigJSON:    `{"enabled":1,"format":"hls","qualities":["720p"]}`,
		PlaylistJSON:  `{"format":"hls","default_url":"https://cdn.example/master.m3u8","variants":[]}`,
		ArtifactsJSON: `["uploads/video/poster.jpg","uploads/video/master.m3u8"]`,
	}
	if err := task.AfterFind(nil); err != nil {
		t.Fatal(err)
	}
	if task.Config.Format != "hls" || len(task.Config.Qualities) != 1 || task.Config.Qualities[0] != "720p" {
		t.Fatalf("decoded config = %#v", task.Config)
	}
	if task.Playlist == nil || task.Playlist.DefaultURL != "https://cdn.example/master.m3u8" {
		t.Fatalf("decoded playlist = %#v", task.Playlist)
	}
	if len(task.Artifacts) != 2 || task.Artifacts[0] != "uploads/video/poster.jpg" {
		t.Fatalf("decoded artifacts = %#v", task.Artifacts)
	}
}

func TestVideoTaskResetForManualRetry(t *testing.T) {
	now := time.Unix(2000, 0)
	task := VideoTask{
		Status:     VideoTaskStatusFailed,
		Attempts:   3,
		ErrorMsg:   "old failure",
		FinishedAt: now.Add(-time.Minute).Unix(),
	}
	task.ResetForManualRetry(now)
	if task.Status != VideoTaskStatusPending || task.Attempts != 0 || task.ErrorMsg != "old failure" {
		t.Fatalf("retry task = %#v", task)
	}
	if task.NextRetryAt != now.Unix() || task.FinishedAt != 0 || task.StartedAt != 0 || task.LeaseUntil != 0 {
		t.Fatalf("retry schedule = %#v", task)
	}
}
