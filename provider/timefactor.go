package provider

import (
	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
	"time"
)

func (w *Website) TryToRunTimeFactor() {
	setting := w.GetTimeFactorSetting()
	if !setting.Open {
		return
	}

	// 开始尝试执行更新任务
	if len(setting.ModuleIds) == 0 || len(setting.Types) == 0 || setting.StartDay == 0 {
		return
	}

	db := w.DB.Model(&model.Archive{}).Where("`status` = 1 and module_id IN (?)", setting.ModuleIds)
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
	db.UpdateColumns(updateFields)
}
