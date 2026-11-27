package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

const (
	videoWorkerInterval    = 2 * time.Second
	videoTaskLeaseTTL      = 2 * time.Minute
	videoTaskHeartbeat     = 30 * time.Second
	videoWorkerBatch       = 20
	videoWorkerShutdown    = 5 * time.Second
	videoGlobalConcurrency = 2
)

var (
	videoWorkerOnInit   atomic.Bool
	videoWorkerRegistry = struct {
		sync.Mutex
		bySite map[uint]*videoWorker
	}{bySite: make(map[uint]*videoWorker)}
	videoFFmpegSlots       = make(chan struct{}, videoGlobalConcurrency)
	videoWorkerSequence    atomic.Uint64
	startVideoWorkerOnInit = func(w *Website) {
		w.StartVideoWorker(context.Background())
	}
)

func EnableVideoWorkerOnInit() {
	videoWorkerOnInit.Store(true)
}

func (w *Website) StartVideoWorker(ctx context.Context) {
	if w == nil || w.DB == nil {
		return
	}
	videoWorkerRegistry.Lock()
	if existing, exists := videoWorkerRegistry.bySite[w.Id]; exists {
		if existing.servesSite(w) {
			videoWorkerRegistry.Unlock()
			return
		}
		delete(videoWorkerRegistry.bySite, w.Id)
		videoWorkerRegistry.Unlock()
		existing.stop()
		videoWorkerRegistry.Lock()
	}
	worker := &videoWorker{site: w, owner: newVideoWorkerOwner()}
	videoWorkerRegistry.bySite[w.Id] = worker
	videoWorkerRegistry.Unlock()
	worker.start(ctx)
}

func (w *Website) StopVideoWorker() {
	if w == nil {
		return
	}
	videoWorkerRegistry.Lock()
	worker := videoWorkerRegistry.bySite[w.Id]
	delete(videoWorkerRegistry.bySite, w.Id)
	videoWorkerRegistry.Unlock()
	if worker != nil {
		worker.stop()
	}
}

type videoWorker struct {
	site   *Website
	owner  string
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (worker *videoWorker) servesSite(site *Website) bool {
	return worker.site == site && worker.site.DB == site.DB
}

func (worker *videoWorker) start(parent context.Context) {
	if parent == nil {
		parent = context.Background()
	}
	worker.ctx, worker.cancel = context.WithCancel(parent)
	worker.wg.Add(1)
	go func() {
		defer worker.wg.Done()
		for {
			worker.site.runVideoWorkerCycle(worker.ctx, worker.owner)
			select {
			case <-worker.ctx.Done():
				return
			case <-time.After(videoWorkerInterval):
			}
		}
	}()
}

func (worker *videoWorker) stop() {
	if worker.cancel == nil {
		return
	}
	worker.cancel()
	done := make(chan struct{})
	go func() {
		worker.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(videoWorkerShutdown):
		log.Printf("[video] 站点 %d 的视频工作者停止超时", worker.site.Id)
	}
}

func newVideoWorkerOwner() string {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	sequence := videoWorkerSequence.Add(1)
	owner := fmt.Sprintf("%s:%d:%d", hostname, os.Getpid(), sequence)
	if len(owner) > 64 {
		owner = owner[:64]
	}
	return owner
}

func (w *Website) runVideoWorkerCycle(ctx context.Context, owner string) {
	if w == nil || w.DB == nil || ctx.Err() != nil {
		return
	}
	if !w.DB.Migrator().HasTable(&model.VideoTask{}) {
		return
	}
	w.recoverStaleVideoTasks(time.Now())
	ids, err := w.dueVideoTaskIDs(time.Now(), videoWorkerBatch)
	if err != nil {
		w.logVideoTaskError(0, err)
		return
	}
	concurrency := 1
	if w.Content != nil && w.Content.Video.Concurrency > 0 {
		concurrency = w.Content.Video.Concurrency
	}
	if concurrency > videoGlobalConcurrency {
		concurrency = videoGlobalConcurrency
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
tasks:
	for _, id := range ids {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break tasks
		}
		task, claimErr := w.claimVideoTask(id, owner, time.Now())
		if claimErr != nil {
			<-sem
			w.logVideoTaskError(id, claimErr)
			continue
		}
		if task == nil {
			<-sem
			continue
		}
		wg.Add(1)
		go func(task *model.VideoTask) {
			defer wg.Done()
			defer func() { <-sem }()
			w.processVideoTask(ctx, task)
		}(task)
	}
	wg.Wait()
}

func (w *Website) dueVideoTaskIDs(now time.Time, limit int) ([]uint, error) {
	ids := make([]uint, 0)
	if limit < 1 {
		limit = videoWorkerBatch
	}
	err := w.DB.Model(&model.VideoTask{}).
		Where("`status` = ? AND `next_retry_at` <= ?", model.VideoTaskStatusPending, now.Unix()).
		Order("`id` asc").Limit(limit).Pluck("`id`", &ids).Error
	return ids, err
}

func (w *Website) claimVideoTask(id uint, owner string, now time.Time) (*model.VideoTask, error) {
	result := w.DB.Model(&model.VideoTask{}).
		Where("`id` = ? AND `status` = ? AND `next_retry_at` <= ?", id, model.VideoTaskStatusPending, now.Unix()).
		Updates(map[string]any{
			"status":        model.VideoTaskStatusProcessing,
			"progress":      0,
			"attempts":      gorm.Expr("`attempts` + 1"),
			"error_msg":     "",
			"started_at":    now.Unix(),
			"finished_at":   0,
			"next_retry_at": 0,
			"lease_until":   now.Add(videoTaskLeaseTTL).Unix(),
			"lease_owner":   owner,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, nil
	}
	return w.GetVideoTask(id)
}

func (w *Website) recoverStaleVideoTasks(now time.Time) {
	result := w.DB.Model(&model.VideoTask{}).
		Where("`status` = ? AND `lease_until` < ?", model.VideoTaskStatusProcessing, now.Unix()).
		Updates(map[string]any{
			"status":        model.VideoTaskStatusPending,
			"progress":      0,
			"next_retry_at": now.Unix(),
			"lease_until":   0,
			"lease_owner":   "",
			"error_msg":     "任务因工作者中断或租约过期而重新排队",
		})
	if result.Error != nil {
		w.logVideoTaskError(0, result.Error)
		return
	}
	if result.RowsAffected > 0 {
		log.Printf("[video] 站点 %d 恢复 %d 个过期视频任务", w.Id, result.RowsAffected)
	}
}

func (w *Website) processVideoTask(ctx context.Context, task *model.VideoTask) {
	if task == nil {
		return
	}
	cfg := task.Config
	cfg.Normalize()
	timeout := time.Duration(cfg.Timeout) * time.Second
	processCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(videoTaskHeartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-processCtx.Done():
				return
			case <-ticker.C:
				if err := w.renewVideoTaskLease(task, time.Now()); err != nil {
					log.Printf("[video] 站点 %d 的任务 #%d 失去租约：%s", w.Id, task.Id, err)
					cancel()
					return
				}
			}
		}
	}()

	select {
	case videoFFmpegSlots <- struct{}{}:
		defer func() { <-videoFFmpegSlots }()
	case <-processCtx.Done():
		cancel()
		<-heartbeatDone
		w.saveVideoTaskFailure(task, processCtx.Err(), nil)
		return
	}

	var attachment model.Attachment
	err := w.DB.Where("`id` = ?", task.AttachmentId).First(&attachment).Error
	var execution *videoTaskExecutionResult
	if err == nil && !w.ownsVideoTaskLease(task, time.Now()) {
		err = fmt.Errorf("video task lease is no longer owned by this worker")
	}
	if err == nil {
		var thumbWidth, thumbHeight, thumbCrop, quality int
		if w.Content != nil {
			thumbWidth = w.Content.ThumbWidth
			thumbHeight = w.Content.ThumbHeight
			thumbCrop = w.Content.ThumbCrop
			quality = w.Content.Quality
		}
		execution, err = executeVideoTask(processCtx, videoTaskExecutionOptions{
			SiteID:      w.Id,
			TaskID:      task.Id,
			Attachment:  &attachment,
			Config:      cfg,
			ThumbWidth:  thumbWidth,
			ThumbHeight: thumbHeight,
			ThumbCrop:   thumbCrop,
			Quality:     quality,
			PublicPath:  w.PublicPath,
			StorageURL:  w.PluginStorage.StorageUrl,
			UseLocal:    w.PluginStorage.StorageType == "local" || w.PluginStorage.KeepLocal,
			KeepLocal:   w.PluginStorage.KeepLocal && w.PluginStorage.StorageType != "local",
			Store:       w.Storage,
			OwnsTask: func() bool {
				return w.ownsVideoTaskLease(task, time.Now())
			},
			Progress: func(progress int) {
				w.updateVideoTaskProgress(task, progress)
			},
		})
	}
	cancel()
	<-heartbeatDone
	if err != nil {
		w.saveVideoTaskFailure(task, err, execution)
		return
	}
	if err := w.saveVideoTaskSuccess(task, execution, &attachment); err != nil {
		w.cleanupVideoTaskArtifacts(task, execution, &attachment)
		w.saveVideoTaskFailure(task, err, execution)
		return
	}
	if err := w.deleteVideoTaskSource(task, &attachment); err != nil {
		log.Printf("[video] 站点 %d 的任务 #%d 已成功，但删除原片失败：%s", w.Id, task.Id, err)
	}
}

func videoTaskLeaseScope(task *model.VideoTask, now time.Time) (string, []any) {
	if task == nil || task.LeaseOwner == "" {
		return "`id` = ? AND 0 = 1", []any{uint(0)}
	}
	return "`id` = ? AND `status` = ? AND `lease_owner` = ? AND `lease_until` > ?", []any{
		task.Id,
		model.VideoTaskStatusProcessing,
		task.LeaseOwner,
		now.Unix(),
	}
}

func (w *Website) ownsVideoTaskLease(task *model.VideoTask, now time.Time) bool {
	if w == nil || w.DB == nil || task == nil {
		return false
	}
	query, args := videoTaskLeaseScope(task, now)
	var id uint
	result := w.DB.Model(&model.VideoTask{}).Where(query, args...).Select("`id`").Limit(1).Scan(&id)
	return result.Error == nil && id == task.Id
}

func (w *Website) renewVideoTaskLease(task *model.VideoTask, now time.Time) error {
	query, args := videoTaskLeaseScope(task, now)
	result := w.DB.Model(&model.VideoTask{}).Where(query, args...).
		Update("lease_until", now.Add(videoTaskLeaseTTL).Unix())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("video task lease is no longer owned by this worker")
	}
	return nil
}

func (w *Website) updateVideoTaskProgress(task *model.VideoTask, progress int) {
	if task == nil || progress < 0 || progress > 99 {
		return
	}
	query, args := videoTaskLeaseScope(task, time.Now())
	w.DB.Model(&model.VideoTask{}).Where(query, args...).Update("progress", progress)
}

func (w *Website) saveVideoTaskSuccess(
	task *model.VideoTask,
	result *videoTaskExecutionResult,
	source *model.Attachment,
) error {
	if task == nil || result == nil || source == nil {
		return fmt.Errorf("video task result or source is empty")
	}
	root := videoTaskArtifactRoot(w.Id, task.AttachmentId, task.Id)
	posterLocation, _ := videoPosterLocationFromURL(source.FileLocation, result.Playlist.PosterURL, w.PluginStorage.StorageUrl)
	for _, location := range result.Artifacts {
		if _, err := safeVideoExecutionArtifactLocation(root, posterLocation, location); err != nil {
			return err
		}
	}
	playlist := result.Playlist
	if playlist.Variants == nil {
		playlist.Variants = []model.VideoVariant{}
	}
	for index := range playlist.Variants {
		playlist.Variants[index].IsDefault = playlist.Variants[index].Name == playlist.DefaultType
	}
	playbackLocation, err := videoPlaybackLocation(root, w.PluginStorage.StorageUrl, &playlist, result.Artifacts)
	if err != nil {
		return err
	}
	playlistJSON, err := json.Marshal(playlist)
	if err != nil {
		return err
	}
	artifactsJSON, err := json.Marshal(result.Artifacts)
	if err != nil {
		return err
	}
	completed := *task
	completed.Playlist = &playlist
	completed.Artifacts = append([]string{}, result.Artifacts...)
	completed.MarkSuccess(playlist.DefaultURL, time.Now())
	return w.DB.Transaction(func(tx *gorm.DB) error {
		var current model.Attachment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("`id` = ?", task.AttachmentId).First(&current).Error; err != nil {
			return err
		}
		if current.FileLocation != source.FileLocation || current.FileMd5 != source.FileMd5 {
			return fmt.Errorf("video source changed while task %d was processing", task.Id)
		}
		query, args := videoTaskLeaseScope(task, time.Now())
		updateResult := tx.Model(&model.VideoTask{}).Where(query, args...).
			Updates(map[string]any{
				"status":        completed.Status,
				"progress":      completed.Progress,
				"error_msg":     completed.ErrorMsg,
				"playlist":      string(playlistJSON),
				"artifacts":     string(artifactsJSON),
				"finished_at":   completed.FinishedAt,
				"next_retry_at": completed.NextRetryAt,
				"lease_until":   completed.LeaseUntil,
				"lease_owner":   completed.LeaseOwner,
			})
		if updateResult.Error != nil {
			return updateResult.Error
		}
		if updateResult.RowsAffected != 1 {
			return fmt.Errorf("video task lease is no longer owned by this worker")
		}
		return tx.Model(&model.Attachment{}).Where("`id` = ?", task.AttachmentId).
			Updates(map[string]any{
				"file_location": playbackLocation,
				"logo":          playlist.PosterURL,
				"width":         result.Width,
				"height":        result.Height,
			}).Error
	})
}

func (w *Website) saveVideoTaskFailure(task *model.VideoTask, taskErr error, result *videoTaskExecutionResult) {
	if taskErr == nil {
		return
	}
	artifacts := append([]string{}, task.Artifacts...)
	if result != nil {
		artifacts = append(artifacts, result.Uploaded...)
	}
	artifacts = uniqueVideoLocations(artifacts)
	encoded, _ := json.Marshal(artifacts)
	message := []rune(strings.TrimSpace(taskErr.Error()))
	if len(message) > 1000 {
		message = message[:1000]
	}
	now := time.Now()
	failed := *task
	failed.Artifacts = artifacts
	failed.MarkFailure(string(message), task.Config.MaxRetries, now)
	buildUpdates := func() map[string]any {
		return map[string]any{
			"status":        failed.Status,
			"progress":      failed.Progress,
			"error_msg":     failed.ErrorMsg,
			"artifacts":     string(encoded),
			"next_retry_at": failed.NextRetryAt,
			"finished_at":   failed.FinishedAt,
			"lease_until":   failed.LeaseUntil,
			"lease_owner":   failed.LeaseOwner,
		}
	}
	updates := buildUpdates()
	if failed.Status == model.VideoTaskStatusPending {
		updates["playlist"] = ""
		updates["artifacts"] = "[]"
	}
	retryCommitted := false
	if failed.Status == model.VideoTaskStatusPending && (len(artifacts) > 0 || task.Playlist != nil) {
		var source model.Attachment
		cleanupErr := w.DB.Where("`id` = ?", task.AttachmentId).First(&source).Error
		if cleanupErr == nil && !w.canCleanupVideoTaskArtifacts(task) {
			cleanupErr = fmt.Errorf("video task lease is no longer owned by this worker")
		}
		if cleanupErr == nil {
			cleanupErr = w.deleteVideoTaskArtifacts(task, &source)
		}
		if cleanupErr == nil && result != nil {
			partial := *task
			partial.Artifacts = append([]string{}, result.Uploaded...)
			partial.Playlist = &result.Playlist
			cleanupErr = w.deleteVideoTaskArtifacts(&partial, &source)
		}
		if cleanupErr == nil {
			cleanupErr = w.DB.Transaction(func(tx *gorm.DB) error {
				var current model.Attachment
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("`id` = ?", task.AttachmentId).First(&current).Error; err != nil {
					return err
				}
				if current.FileLocation != source.FileLocation || current.FileMd5 != source.FileMd5 {
					return fmt.Errorf("video source changed while task %d was being cleaned up", task.Id)
				}
				query, args := videoTaskLeaseScope(task, time.Now())
				updateResult := tx.Model(&model.VideoTask{}).Where(query, args...).Updates(updates)
				if updateResult.Error != nil {
					return updateResult.Error
				}
				if updateResult.RowsAffected != 1 {
					return fmt.Errorf("video task lease is no longer owned by this worker")
				}
				return w.clearVideoTaskPosterReference(tx, task, &current)
			})
		}
		if cleanupErr != nil {
			failed.MarkFailure(fmt.Sprintf("%s；自动重试前清理制品或提交重试失败：%s", string(message), cleanupErr), 0, now)
		} else {
			retryCommitted = true
			failed.Artifacts = []string{}
			failed.ArtifactsJSON = "[]"
			failed.Playlist = nil
			failed.PlaylistJSON = ""
			encoded = []byte("[]")
		}
	}
	message = []rune(failed.ErrorMsg)
	if len(message) > 1000 {
		message = message[:1000]
	}
	failed.ErrorMsg = string(message)
	if retryCommitted {
		w.logVideoTaskError(task.Id, taskErr)
		return
	}
	updates = buildUpdates()
	if failed.Status == model.VideoTaskStatusPending {
		updates["playlist"] = ""
	}
	query, args := videoTaskLeaseScope(task, time.Now())
	updateResult := w.DB.Model(&model.VideoTask{}).Where(query, args...).Updates(updates)
	if updateResult.Error != nil {
		w.logVideoTaskError(task.Id, updateResult.Error)
		return
	}
	if updateResult.RowsAffected == 1 {
		w.logVideoTaskError(task.Id, taskErr)
	}
}

func (w *Website) clearVideoTaskPosterReference(
	tx *gorm.DB,
	task *model.VideoTask,
	source *model.Attachment,
) error {
	if task == nil || source == nil || source.Logo == "" {
		return nil
	}
	posterURLs := make([]string, 0, 2)
	if task.Playlist != nil && task.Playlist.PosterURL != "" {
		if _, ok := videoPosterLocationFromURL(source.FileLocation, task.Playlist.PosterURL, w.PluginStorage.StorageUrl); ok {
			posterURLs = append(posterURLs, task.Playlist.PosterURL)
		}
	}
	for _, location := range videoTaskArtifactLocations(w.Id, task, source.FileLocation, w.PluginStorage.StorageUrl) {
		if _, ok := safeVideoSourcePosterLocation(source.FileLocation, location); ok {
			posterURLs = append(posterURLs, videoStorageURL(w.PluginStorage.StorageUrl, location))
		}
	}
	for _, posterURL := range uniqueVideoLocations(posterURLs) {
		if source.Logo != posterURL {
			continue
		}
		result := tx.Model(&model.Attachment{}).
			Where("`id` = ? AND `file_location` = ? AND `file_md5` = ? AND `logo` = ?",
				source.Id, source.FileLocation, source.FileMd5, posterURL).
			UpdateColumn("logo", "")
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("video source %d changed before poster reference reset", source.Id)
		}
		return nil
	}
	return nil
}

func (w *Website) cleanupVideoTaskArtifacts(
	task *model.VideoTask,
	result *videoTaskExecutionResult,
	source *model.Attachment,
) {
	if task == nil || result == nil || source == nil || !w.canCleanupVideoTaskArtifacts(task) {
		return
	}
	partial := *task
	partial.Artifacts = append([]string{}, result.Uploaded...)
	partial.Playlist = &result.Playlist
	if err := w.deleteVideoTaskArtifacts(&partial, source); err != nil {
		w.logVideoTaskError(task.Id, fmt.Errorf("cleanup video artifact: %w", err))
	}
}

func (w *Website) deleteVideoTaskArtifacts(task *model.VideoTask, source *model.Attachment) error {
	if task == nil || source == nil {
		return fmt.Errorf("video task and source are required for artifact cleanup")
	}
	for _, location := range videoTaskArtifactLocations(w.Id, task, source.FileLocation, w.PluginStorage.StorageUrl) {
		exists, err := w.Storage.Exists(context.Background(), location)
		if err != nil {
			return fmt.Errorf("check video artifact %s: %w", location, err)
		}
		if !exists {
			continue
		}
		if err := w.DeleteFile(location); err != nil {
			return fmt.Errorf("delete video artifact %s: %w", location, err)
		}
	}
	return nil
}

func (w *Website) canCleanupVideoTaskArtifacts(task *model.VideoTask) bool {
	if w.ownsVideoTaskLease(task, time.Now()) {
		return true
	}
	var current model.VideoTask
	err := w.DB.Where("`id` = ?", task.Id).First(&current).Error
	return errors.Is(err, gorm.ErrRecordNotFound) ||
		(err == nil && current.Status == model.VideoTaskStatusCancelled)
}

func (w *Website) deleteVideoTaskSource(task *model.VideoTask, source *model.Attachment) error {
	if task == nil || source == nil {
		return fmt.Errorf("video task source is empty")
	}
	if slices.Contains(task.Config.Qualities, config.VideoQualityOriginal) {
		return nil
	}
	sourceLocation, err := safeVideoStorageLocation(source.FileLocation)
	if err != nil {
		return err
	}
	return w.DB.Transaction(func(tx *gorm.DB) error {
		var current model.Attachment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("`id` = ?", source.Id).First(&current).Error; err != nil {
			return err
		}
		if current.FileMd5 != source.FileMd5 {
			return fmt.Errorf("video source changed before deletion")
		}
		var completed model.VideoTask
		if err := tx.Where("`id` = ? AND `attachment_id` = ? AND `status` = ?",
			task.Id, source.Id, model.VideoTaskStatusSuccess).First(&completed).Error; err != nil {
			return err
		}
		playbackLocation, err := videoPlaybackLocation(
			videoTaskArtifactRoot(w.Id, task.AttachmentId, task.Id),
			w.PluginStorage.StorageUrl,
			completed.Playlist,
			completed.Artifacts,
		)
		if err != nil {
			return err
		}
		if current.FileLocation != playbackLocation || sourceLocation == playbackLocation {
			return fmt.Errorf("video source changed before deletion")
		}
		var active int64
		if err := tx.Model(&model.VideoTask{}).
			Where("`attachment_id` = ? AND `status` IN ?", source.Id,
				[]int{model.VideoTaskStatusPending, model.VideoTaskStatusProcessing}).Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return nil
		}
		return w.DeleteFile(sourceLocation)
	})
}

func uniqueVideoLocations(locations []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(locations))
	for _, location := range locations {
		if location == "" || seen[location] {
			continue
		}
		seen[location] = true
		result = append(result, location)
	}
	return result
}
