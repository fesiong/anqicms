package model

import (
	"gorm.io/gorm"
	"time"
)

const (
	StatusWait   = uint(0)
	StatusOk     = uint(1)
	StatusDelete = uint(99)
)

/**
 * 说明 所有status = 99 的表示删除
 */
type Model struct {
}

func (m *Model) SoftDelete(db *gorm.DB, model interface{}, query interface{}, args ...interface{}) error {
	err := db.Model(&model).Where(query, args...).Updates(map[string]interface{}{"status": StatusDelete, "deleted_time": uint(time.Now().Unix())}).Error
	return err
}

func (m *Model) DeleteSoft(db *gorm.DB) error {
	err := db.Model(&m).Updates(map[string]interface{}{"status": StatusDelete, "deleted_time": uint(time.Now().Unix())}).Error
	return err
}

func AutoMigrateDB(db *gorm.DB) error {
	//自动迁移数据库
	err := db.AutoMigrate(
		&Admin{},
		&Article{},
		&ArticleData{},
		&Attachment{},
		&Category{},
		&Nav{},
		&Link{},
		&Comment{},
		&Product{},
		&ProductData{},
	)

	return err
}