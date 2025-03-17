package provider

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"time"
)

func (w *Website) TryToRunTimeFactor() {
	if !w.PluginTimeFactor.Open && !w.PluginTimeFactor.ReleaseOpen {
		return
	}

	// 开始尝试执行更新任务
	if len(w.PluginTimeFactor.ModuleIds) == 0 {
		return
	}

	go w.TimeRenewArchives(w.PluginTimeFactor)

	go w.TimeReleaseArchives(w.PluginTimeFactor)
}

func (w *Website) TimeRenewArchives(setting *config.PluginTimeFactor) {
	if !setting.Open {
		return
	}
	if len(setting.Types) == 0 {
		return
	}
	if setting.StartDay == 0 {
		return
	}
	if setting.UpdateRunning {
		return
	}
	setting.UpdateRunning = true
	defer func() {
		setting.UpdateRunning = false
	}()
	if setting.TodayUpdate > 0 && time.Unix(setting.LastUpdate, 0).Day() != time.Now().Day() {
		setting.TodayUpdate = 0
		// 更新数量
		_ = w.SaveSettingValue(TimeFactorKey, setting)
	}
	// 计算每篇间隔
	if setting.DailyUpdate > 0 && setting.TodayUpdate >= setting.DailyUpdate {
		return
	}
	if setting.EndTime == 0 {
		setting.EndTime = 23
	}
	var diffSecond = 1
	if setting.DailyUpdate > 0 {
		diffSecond = (setting.EndTime + 1 - setting.StartTime) * 3600 / setting.DailyUpdate
	}
	if diffSecond < 1 {
		diffSecond = 1
	}
	nowStamp := time.Now().Unix()
	if setting.DailyUpdate > 0 && setting.LastUpdate > nowStamp+int64(diffSecond) {
		// 间隔未到
		return
	}

	db := w.DB.Model(&model.Archive{}).Limit(100)
	if len(setting.ModuleIds) == 1 {
		db = db.Where("module_id = ?", setting.ModuleIds[0])
	} else {
		db = db.Where("module_id IN (?)", setting.ModuleIds)
	}
	if len(setting.CategoryIds) > 0 {
		db = db.Where("category_id NOT IN (?)", setting.CategoryIds)
	}
	startStamp := time.Now().AddDate(0, 0, -setting.StartDay).Unix()
	for _, field := range setting.Types {
		if field == "created_time" {
			db = db.Where("`created_time` < ?", startStamp)
		}
		if field == "updated_time" {
			db = db.Where("`updated_time` < ?", startStamp)
		}
	}
	addStamp := time.Now()
	if setting.EndDay > 0 {
		addStamp = addStamp.AddDate(0, 0, -setting.EndDay)
	}
	updateFields := map[string]interface{}{}
	for _, field := range setting.Types {
		if field == "created_time" {
			updateFields["created_time"] = addStamp.Unix()
		}
		if field == "updated_time" {
			updateFields["updated_time"] = addStamp.Unix()
		}
	}
	var archives []*model.Archive
	db.Find(&archives)
	spend := 0
	if len(archives) > 0 {
		for _, archive := range archives {
			// 更新时间
			w.DB.Model(archive).UpdateColumns(updateFields)
			if setting.DoPublish {
				archive.Link = w.GetUrl("archive", archive, 0)
				// 重新推送
				go w.PushArchive(archive.Link)
				// 清除缓存
				w.DeleteArchiveCache(archive.Id, archive.Link)
				w.DeleteArchiveExtraCache(archive.Id)
			}
			// 如果有限制时间，则在这里进行等待，并且小于1分钟，才进行等待
			if setting.DailyUpdate > 0 {
				spend += diffSecond
				if spend > 60 {
					// 超过1分钟，就退出
					return
				}
				if diffSecond < 60 {
					time.Sleep(time.Second * time.Duration(diffSecond))
				}
			}
			// 对下一次更新的文章增加 diffSecond
			addStamp = addStamp.Add(time.Second * time.Duration(diffSecond))
			if _, ok := updateFields["created_time"]; ok {
				updateFields["created_time"] = addStamp.Unix()
			}
			if _, ok := updateFields["updated_time"]; ok {
				updateFields["updated_time"] = addStamp.Unix()
			}
		}
	}
}

func (w *Website) TimeReleaseArchives(setting *config.PluginTimeFactor) {
	if !setting.ReleaseOpen {
		return
	}
	if setting.TodayCount > 0 && time.Unix(setting.LastSent, 0).Day() != time.Now().Day() {
		setting.TodayCount = 0
		// 更新数量
		_ = w.SaveSettingValue(TimeFactorKey, setting)
	}
	if setting.StartTime > 0 && time.Now().Hour() < setting.StartTime {
		return
	}
	if setting.EndTime > 0 && time.Now().Hour() > setting.EndTime {
		return
	}
	// 计算每篇间隔
	if setting.TodayCount >= setting.DailyLimit {
		return
	}
	if setting.EndTime == 0 {
		setting.EndTime = 23
	}
	diffSecond := (setting.EndTime + 1 - setting.StartTime) * 3600 / setting.DailyLimit
	if diffSecond < 1 {
		diffSecond = 1
	}
	nowStamp := time.Now().Unix()
	if setting.LastSent > nowStamp+int64(diffSecond) {
		// 间隔未到
		return
	}

	// 从草稿箱发布
	db := w.DB.Model(&model.ArchiveDraft{}).Where("`status` = 0")
	db = db.Where("module_id IN (?)", setting.ModuleIds)
	if len(setting.CategoryIds) > 0 {
		db = db.Where("category_id NOT IN (?)", setting.CategoryIds)
	}
	var draft *model.ArchiveDraft
	// 一次最多读取1个
	err := db.Order("id asc").Take(&draft).Error
	if err != nil {
		// 没文章
		return
	}
	archive := &draft.Archive
	archive.CreatedTime = nowStamp
	archive.UpdatedTime = nowStamp
	err = w.DB.Save(archive).Error
	if err != nil {
		// err
		return
	}
	// 删除草稿
	w.DB.Delete(draft)
	archive.Link = w.GetUrl("archive", archive, 0)
	_ = w.SuccessReleaseArchive(archive, true)
	// 清除缓存
	w.DeleteArchiveCache(archive.Id, archive.Link)
	w.DeleteArchiveExtraCache(archive.Id)

	setting.TodayCount++
	setting.LastSent = nowStamp
	_ = w.SaveSettingValue(TimeFactorKey, setting)
}
