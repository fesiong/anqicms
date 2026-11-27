package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

var videoFileExts = map[string]bool{
	".mp4":  true,
	".webm": true,
	".mov":  true,
	".avi":  true,
	".mkv":  true,
	".flv":  true,
	".wmv":  true,
	".m4v":  true,
}

func isVideoFileExt(ext string) bool {
	return videoFileExts[strings.ToLower(strings.TrimSpace(ext))]
}

func (w *Website) EnqueueVideoTask(
	attachment *model.Attachment,
	cfg config.VideoProcessingConfig,
	force bool,
) (*model.VideoTask, error) {
	if w == nil || w.DB == nil || attachment == nil || attachment.IsImage != 2 {
		return nil, nil
	}
	cfg.Normalize()
	if cfg.Enabled == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	var task model.VideoTask
	err = w.DB.Transaction(func(tx *gorm.DB) error {
		var current model.Attachment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, attachment.Id).Error; err != nil {
			return err
		}
		if current.IsImage != 2 {
			return fmt.Errorf("attachment %d is not a video", current.Id)
		}

		var existing model.VideoTask
		queryErr := tx.Unscoped().Where("`attachment_id` = ?", current.Id).
			Order("`id` desc").First(&existing).Error
		if queryErr == nil {
			active := existing.Status == model.VideoTaskStatusPending ||
				existing.Status == model.VideoTaskStatusProcessing
			if !force || active {
				task = existing
				return nil
			}
		} else if !errors.Is(queryErr, gorm.ErrRecordNotFound) {
			return queryErr
		}

		created := model.VideoTask{
			AttachmentId: current.Id,
			Status:       model.VideoTaskStatusPending,
			NextRetryAt:  time.Now().Unix(),
			ConfigJSON:   string(encoded),
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		task = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (w *Website) GetVideoTask(id uint) (*model.VideoTask, error) {
	var task model.VideoTask
	if err := w.DB.Where("`id` = ?", id).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (w *Website) ListVideoTasks(status int, page int, pageSize int) ([]*model.VideoTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	builder := w.DB.Model(&model.VideoTask{})
	if status >= 0 {
		builder = builder.Where("`status` = ?", status)
	}
	var total int64
	if err := builder.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	tasks := make([]*model.VideoTask, 0)
	if err := builder.Order("`id` desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	if len(tasks) == 0 {
		return tasks, total, nil
	}

	ids := make([]uint, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.AttachmentId)
	}
	attachments := make([]*model.Attachment, 0, len(ids))
	if err := w.DB.Where("`id` IN ?", ids).Find(&attachments).Error; err != nil {
		return nil, 0, err
	}
	byID := make(map[uint]*model.Attachment, len(attachments))
	for _, attachment := range attachments {
		attachment.GetThumb(w.PluginStorage.StorageUrl)
		byID[attachment.Id] = attachment
	}
	for _, task := range tasks {
		task.Attachment = byID[task.AttachmentId]
	}
	return tasks, total, nil
}

func (w *Website) RetryVideoTask(id uint) (*model.VideoTask, error) {
	var task model.VideoTask
	err := w.DB.Transaction(func(tx *gorm.DB) error {
		var initial model.VideoTask
		if err := tx.Where("`id` = ?", id).First(&initial).Error; err != nil {
			return err
		}
		if initial.Status != model.VideoTaskStatusFailed {
			return fmt.Errorf("only failed video tasks can be retried")
		}
		var source model.Attachment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("`id` = ?", initial.AttachmentId).First(&source).Error; err != nil {
			return fmt.Errorf("load video source for retry: %w", err)
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("`id` = ?", id).First(&task).Error; err != nil {
			return err
		}
		if task.AttachmentId != source.Id || task.Status != model.VideoTaskStatusFailed {
			return fmt.Errorf("video task %d is no longer failed", task.Id)
		}
		if len(task.Artifacts) > 0 || task.Playlist != nil {
			// 持行锁清理，避免并发重试删掉下一次尝试刚上传的制品。
			if err := w.deleteVideoTaskArtifacts(&task, &source); err != nil {
				return fmt.Errorf("cleanup previous video artifacts: %w", err)
			}
		}
		posterTask := task
		task.ResetForManualRetry(time.Now())
		updateResult := tx.Model(&model.VideoTask{}).
			Where("`id` = ? AND `status` = ?", task.Id, model.VideoTaskStatusFailed).
			Updates(map[string]any{
				"status":        task.Status,
				"progress":      task.Progress,
				"attempts":      task.Attempts,
				"next_retry_at": task.NextRetryAt,
				"started_at":    task.StartedAt,
				"finished_at":   task.FinishedAt,
				"lease_until":   task.LeaseUntil,
				"lease_owner":   task.LeaseOwner,
				"playlist":      "",
				"artifacts":     "[]",
			})
		if updateResult.Error != nil {
			return updateResult.Error
		}
		if updateResult.RowsAffected != 1 {
			return fmt.Errorf("video task %d is no longer failed", task.Id)
		}
		return w.clearVideoTaskPosterReference(tx, &posterTask, &source)
	})
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (w *Website) EnqueueHistoricalVideoTasks(force bool) (int, int, int, error) {
	const pageSize = 200
	created, retried, skipped := 0, 0, 0
	videoConfig := config.DefaultVideoProcessingConfig()
	if w.Content != nil {
		videoConfig = w.Content.Video
	}
	for page := 1; ; page++ {
		attachments := make([]*model.Attachment, 0, pageSize)
		if err := w.DB.Where("`is_image` = ? AND `status` = ?", 2, 1).
			Order("`id` asc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&attachments).Error; err != nil {
			return created, retried, skipped, err
		}
		if len(attachments) == 0 {
			break
		}
		for _, attachment := range attachments {
			if videoConfig.Enabled == 0 {
				skipped++
				continue
			}
			var latest model.VideoTask
			err := w.DB.Unscoped().Where("`attachment_id` = ?", attachment.Id).
				Order("`id` desc").First(&latest).Error
			switch {
			case err == nil && (latest.Status == model.VideoTaskStatusPending ||
				latest.Status == model.VideoTaskStatusProcessing):
				skipped++
				continue
			case err == nil && !force && latest.Status == model.VideoTaskStatusSuccess:
				skipped++
				continue
			case err == nil && !force && latest.Status == model.VideoTaskStatusFailed:
				if _, retryErr := w.RetryVideoTask(latest.Id); retryErr != nil {
					return created, retried, skipped, retryErr
				}
				retried++
				continue
			case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
				return created, retried, skipped, err
			}

			task, enqueueErr := w.EnqueueVideoTask(attachment, videoConfig, force)
			if enqueueErr != nil {
				return created, retried, skipped, enqueueErr
			}
			if task == nil {
				skipped++
				continue
			}
			created++
		}
		if len(attachments) < pageSize {
			break
		}
	}
	return created, retried, skipped, nil
}

func (w *Website) attachVideoTasks(attachments []*model.Attachment) error {
	ids := make([]uint, 0, len(attachments))
	for _, attachment := range attachments {
		if attachment != nil && attachment.IsImage == 2 {
			ids = append(ids, attachment.Id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	tasks := make([]*model.VideoTask, 0, len(ids))
	if err := w.DB.Where("`attachment_id` IN ?", ids).Order("`id` desc").Find(&tasks).Error; err != nil {
		return err
	}
	latest := make(map[uint]*model.VideoTask, len(tasks))
	for _, task := range tasks {
		if _, exists := latest[task.AttachmentId]; !exists {
			latest[task.AttachmentId] = task
		}
	}
	for _, attachment := range attachments {
		if attachment == nil || attachment.IsImage != 2 {
			continue
		}
		task := latest[attachment.Id]
		if task == nil {
			continue
		}
		copyTask := *task
		copyTask.Attachment = nil
		attachment.VideoTask = &copyTask
		if task.Playlist != nil {
			attachment.PlayURL = task.Playlist.DefaultURL
		}
	}
	return nil
}

func videoTaskArtifactLocations(siteID uint, task *model.VideoTask, source string, storageURL string) []string {
	if task == nil || siteID == 0 || task.AttachmentId == 0 || task.Id == 0 {
		return []string{}
	}
	root := videoTaskArtifactRoot(siteID, task.AttachmentId, task.Id)
	sourceLocation, err := safeVideoStorageLocation(source)
	if err != nil {
		return []string{}
	}
	storageURL = strings.TrimRight(storageURL, "/")
	recordedPoster, _ := videoRecordedPosterLocation(task, storageURL)
	seen := make(map[string]bool)
	locations := make([]string, 0)
	add := func(location string) {
		rawLocation := strings.TrimSpace(location)
		location = strings.TrimLeft(rawLocation, "/")
		if location == "" || location == sourceLocation || seen[location] {
			return
		}
		base := strings.TrimRight(storageURL, "/")
		isAbsolute := strings.HasPrefix(rawLocation, "http://") || strings.HasPrefix(rawLocation, "https://")
		if isAbsolute || (base != "" && strings.HasPrefix(rawLocation, base+"/")) {
			if base == "" || !strings.HasPrefix(rawLocation, base+"/") {
				return
			}
			location = strings.TrimPrefix(rawLocation, base+"/")
		}
		location, err = safeVideoStorageLocation(location)
		if err != nil || location == sourceLocation || seen[location] {
			return
		}
		controlled, artifactErr := safeVideoArtifactLocation(root, location)
		if artifactErr != nil {
			if recordedPoster != "" && location == recordedPoster {
				controlled = location
			} else {
				var ok bool
				controlled, ok = safeVideoSourcePosterLocation(sourceLocation, location)
				if !ok {
					return
				}
			}
		}
		if controlled == sourceLocation || seen[controlled] {
			return
		}
		seen[controlled] = true
		locations = append(locations, controlled)
		if poster, ok := safeVideoTaskPosterLocation(controlled); ok {
			if thumb, ok := videoPosterThumbLocation(poster); ok && !seen[thumb] {
				seen[thumb] = true
				locations = append(locations, thumb)
			}
		}
	}
	for _, artifact := range task.Artifacts {
		add(artifact)
	}
	if task.Playlist != nil {
		add(task.Playlist.PosterURL)
		for _, variant := range task.Playlist.Variants {
			add(variant.File)
			if variant.File == "" {
				add(variant.URL)
			}
		}
	}
	return locations
}

func videoRecordedPosterLocation(task *model.VideoTask, storageURL string) (string, bool) {
	if task == nil || task.Playlist == nil {
		return "", false
	}
	location := strings.TrimSpace(task.Playlist.PosterURL)
	base := strings.TrimRight(strings.TrimSpace(storageURL), "/")
	if base != "" {
		if !strings.HasPrefix(location, base+"/") {
			return "", false
		}
		location = strings.TrimPrefix(location, base+"/")
	} else if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		return "", false
	}
	return safeVideoTaskPosterLocation(location)
}

func (w *Website) logVideoTaskError(taskID uint, err error) {
	if err == nil {
		return
	}
	log.Printf("[video] 站点 %d 的任务 #%d 失败：%s", w.Id, taskID, err.Error())
}
