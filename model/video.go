package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"kandaoni.com/anqicms/config"
)

const (
	VideoTaskStatusPending    = 0
	VideoTaskStatusProcessing = 1
	VideoTaskStatusSuccess    = 2
	VideoTaskStatusFailed     = 3
	VideoTaskStatusCancelled  = 4
)

type VideoVariant struct {
	Name      string `json:"name"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	URL       string `json:"url"`
	File      string `json:"file,omitempty"`
	Size      int64  `json:"size,omitempty"`
	IsDefault bool   `json:"is_default,omitempty"`
}

type VideoPlaylist struct {
	Format      string         `json:"format"`
	DefaultURL  string         `json:"default_url"`
	DefaultType string         `json:"default_type"`
	PosterURL   string         `json:"poster_url,omitempty"`
	Variants    []VideoVariant `json:"variants"`
}

type VideoTask struct {
	Id            uint                         `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime   int64                        `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	UpdatedTime   int64                        `json:"updated_time" gorm:"column:updated_time;type:bigint(20);autoUpdateTime"`
	AttachmentId  uint                         `json:"attachment_id" gorm:"column:attachment_id;type:int(10) unsigned not null;index:idx_attachment_status,priority:1;index:idx_status_retry,priority:1"`
	Status        int                          `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_attachment_status,priority:2;index:idx_status_retry,priority:2"`
	Progress      int                          `json:"progress" gorm:"column:progress;type:tinyint(3) unsigned not null;default:0"`
	Attempts      int                          `json:"attempts" gorm:"column:attempts;type:int(10) unsigned not null;default:0"`
	ErrorMsg      string                       `json:"error_msg" gorm:"column:error_msg;type:varchar(1000) not null;default:''"`
	ConfigJSON    string                       `json:"-" gorm:"column:config;type:text"`
	PlaylistJSON  string                       `json:"-" gorm:"column:playlist;type:text"`
	ArtifactsJSON string                       `json:"-" gorm:"column:artifacts;type:text"`
	NextRetryAt   int64                        `json:"next_retry_at" gorm:"column:next_retry_at;type:bigint(20);not null;default:0;index:idx_status_retry,priority:3"`
	StartedAt     int64                        `json:"started_at" gorm:"column:started_at;type:bigint(20);not null;default:0"`
	FinishedAt    int64                        `json:"finished_at" gorm:"column:finished_at;type:bigint(20);not null;default:0"`
	LeaseUntil    int64                        `json:"-" gorm:"column:lease_until;type:bigint(20);not null;default:0;index:idx_status_retry,priority:4"`
	LeaseOwner    string                       `json:"-" gorm:"column:lease_owner;type:varchar(64) not null;default:''"`
	Config        config.VideoProcessingConfig `json:"config,omitempty" gorm:"-"`
	Playlist      *VideoPlaylist               `json:"playlist,omitempty" gorm:"-"`
	Artifacts     []string                     `json:"artifacts,omitempty" gorm:"-"`
	Attachment    *Attachment                  `json:"attachment,omitempty" gorm:"-"`
}

func (t *VideoTask) AfterFind(_ *gorm.DB) error {
	t.Config = config.DefaultVideoProcessingConfig()
	if t.ConfigJSON != "" {
		if err := json.Unmarshal([]byte(t.ConfigJSON), &t.Config); err != nil {
			return err
		}
	}
	t.Playlist = nil
	if t.PlaylistJSON != "" {
		t.Playlist = &VideoPlaylist{Variants: []VideoVariant{}}
		if err := json.Unmarshal([]byte(t.PlaylistJSON), t.Playlist); err != nil {
			return err
		}
		if t.Playlist.Variants == nil {
			t.Playlist.Variants = []VideoVariant{}
		}
	}
	t.Artifacts = []string{}
	if t.ArtifactsJSON != "" {
		if err := json.Unmarshal([]byte(t.ArtifactsJSON), &t.Artifacts); err != nil {
			return err
		}
		if t.Artifacts == nil {
			t.Artifacts = []string{}
		}
	}
	return nil
}

func (t *VideoTask) ResetForManualRetry(now time.Time) {
	if t.Status != VideoTaskStatusFailed {
		return
	}
	t.Status = VideoTaskStatusPending
	t.Progress = 0
	t.Attempts = 0
	t.NextRetryAt = now.Unix()
	t.StartedAt = 0
	t.FinishedAt = 0
	t.LeaseUntil = 0
	t.LeaseOwner = ""
	t.Playlist = nil
	t.PlaylistJSON = ""
	t.Artifacts = []string{}
	t.ArtifactsJSON = ""
}

func (t *VideoTask) MarkProcessing(now time.Time) {
	if t.Status != VideoTaskStatusPending {
		return
	}
	t.Status = VideoTaskStatusProcessing
	t.Progress = 0
	t.Attempts++
	t.StartedAt = now.Unix()
	t.FinishedAt = 0
	t.NextRetryAt = 0
	t.ErrorMsg = ""
	t.LeaseUntil = now.Add(2 * time.Minute).Unix()
}

func (t *VideoTask) MarkFailure(message string, maxRetries int, now time.Time) {
	if t.Status != VideoTaskStatusPending && t.Status != VideoTaskStatusProcessing {
		return
	}
	t.ErrorMsg = message
	t.LeaseUntil = 0
	t.LeaseOwner = ""
	if maxRetries > 0 && t.Attempts <= maxRetries {
		t.Status = VideoTaskStatusPending
		t.Progress = 0
		delay := time.Duration(1<<(t.Attempts-1)) * 30 * time.Second
		if delay > 5*time.Minute {
			delay = 5 * time.Minute
		}
		t.NextRetryAt = now.Add(delay).Unix()
		t.FinishedAt = 0
		return
	}
	t.Status = VideoTaskStatusFailed
	t.FinishedAt = now.Unix()
	t.NextRetryAt = 0
}

func (t *VideoTask) MarkSuccess(playURL string, now time.Time) {
	if t.Status != VideoTaskStatusPending && t.Status != VideoTaskStatusProcessing {
		return
	}
	t.Status = VideoTaskStatusSuccess
	t.Progress = 100
	t.ErrorMsg = ""
	t.FinishedAt = now.Unix()
	t.NextRetryAt = 0
	t.LeaseUntil = 0
	t.LeaseOwner = ""
	if playURL != "" {
		playlist := VideoPlaylist{DefaultURL: playURL}
		if t.Playlist != nil {
			playlist = *t.Playlist
			playlist.DefaultURL = playURL
		}
		encoded, _ := json.Marshal(playlist)
		t.PlaylistJSON = string(encoded)
	}
}
