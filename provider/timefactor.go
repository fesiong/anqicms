package provider

import (
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"time"
)

func (w *Website) TryToRunTimeFactor() {
	setting := w.GetTimeFactorSetting()
	if !setting.Open && !setting.ReleaseOpen {
		return
	}

	// 开始尝试执行更新任务
	if len(setting.ModuleIds) == 0 {
		return
	}

	go w.TimeRenewArchives(&setting)

	go w.TimeReleaseArchives(&setting)
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

	db := w.DB.Model(&model.Archive{}).Where("`status` = 1 and module_id IN (?)", setting.ModuleIds).Limit(100)
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
	addStamp := (setting.StartDay - setting.EndDay) * 86400
	updateFields := map[string]interface{}{}
	for _, field := range setting.Types {
		if field == "created_time" {
			updateFields["created_time"] = gorm.Expr("`created_time` + ?", addStamp)
		}
		if field == "updated_time" {
			updateFields["updated_time"] = gorm.Expr("`updated_time` + ?", addStamp)
		}
	}
	var archives []*model.Archive
	if setting.DoPublish {
		// 重新推送
		db.Find(&archives)
	}
	db.UpdateColumns(updateFields)

	if setting.DoPublish && len(archives) > 0 {
		// 重新推送
		for _, archive := range archives {
			go w.PushArchive(archive.Link)
			// 清除缓存
			w.DeleteArchiveCache(archive.Id)
			w.DeleteArchiveExtraCache(archive.Id)
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
		w.SaveSettingValue(TimeFactorKey, setting)
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

	db := w.DB.Model(&model.Archive{}).Where("`status` = 0 and module_id IN (?)", setting.ModuleIds)
	if len(setting.CategoryIds) > 0 {
		db = db.Where("category_id NOT IN (?)", setting.CategoryIds)
	}
	var archive *model.Archive
	// 一次最多读取1个
	err := db.Order("id asc").Take(&archive).Error
	if err != nil {
		// 没文章
		return
	}
	archive.CreatedTime = nowStamp
	archive.UpdatedTime = nowStamp
	archive.Status = config.ContentStatusOK
	w.DB.Updates(archive)
	_ = w.SuccessReleaseArchive(archive, true)
	// 清除缓存
	w.DeleteArchiveCache(archive.Id)
	w.DeleteArchiveExtraCache(archive.Id)

	setting.TodayCount++
	setting.LastSent = nowStamp
	w.SaveSettingValue(TimeFactorKey, setting)
}
