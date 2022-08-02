package model

import (
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"path/filepath"
	"strings"
)

type Attachment struct {
	Model
	FileName     string `json:"file_name" gorm:"column:file_name;type:varchar(100) not null;default:''"`
	FileLocation string `json:"file_location" gorm:"column:file_location;type:varchar(250) not null;default:''"`
	FileSize     int64  `json:"file_size" gorm:"column:file_size;type:bigint(20) unsigned not null;default:0"`
	FileMd5      string `json:"file_md5" gorm:"column:file_md5;type:varchar(32) unique not null;default:''"`
	Width        int    `json:"width" gorm:"column:width;type:int(10) unsigned not null;default:0"`
	Height       int    `json:"height" gorm:"column:height;type:int(10) unsigned not null;default:0"`
	CategoryId   uint   `json:"category_id" gorm:"column:category_id;type:int(10) unsigned not null;default:0;index:idx_category_id"`
	IsImage      int    `json:"is_image" gorm:"column:is_image;type:tinyint(1) not null;default:0"`
	Status       uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_status"`
	Logo         string `json:"logo" gorm:"-"`
	Thumb        string `json:"thumb" gorm:"-"`
}

type AttachmentCategory struct {
	Model
	Title       string `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
	AttachCount uint   `json:"attach_count" gorm:"column:attach_count;type:int(10) unsigned not null;default:0"`
	Status      uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`
}

func (attachment *Attachment) AfterFind(tx *gorm.DB) error {
	// 兼容旧数据
	if strings.HasPrefix(attachment.FileLocation, "20") {
		attachment.FileLocation = "uploads/" + attachment.FileLocation
		tx.Model(attachment).UpdateColumn("file_location", attachment.FileLocation)
	}
	return nil
}

func (attachment *Attachment) GetThumb() {
	// 如果不是图片
	ext := filepath.Ext(attachment.FileLocation)
	if ext != ".jpg" &&
		ext != ".jpeg" &&
		ext != ".gif" &&
		ext != ".png" &&
		ext != ".bmp" &&
		ext != ".webp" {
		return
	}
	//如果是一个远程地址，则缩略图和原图地址一致
	if strings.HasPrefix(attachment.FileLocation, "http") && !strings.HasPrefix(attachment.FileLocation, "//") {
		attachment.Logo = attachment.FileLocation
		attachment.Thumb = attachment.FileLocation
	} else {
		// 兼容旧数据
		if strings.HasPrefix(attachment.FileLocation, "20") {
			attachment.FileLocation = "uploads/" + attachment.FileLocation
		}
		attachment.Logo = config.JsonData.PluginStorage.StorageUrl + "/" + attachment.FileLocation
		paths, fileName := filepath.Split(attachment.Logo)
		attachment.Thumb = paths + "thumb_" + fileName
	}
}

func (attachment *Attachment) Save(db *gorm.DB) error {
	var err error
	if attachment.Id > 0 {
		if err = db.Updates(attachment).Error; err != nil {
			return err
		}
	} else {
		if err = db.Save(attachment).Error; err != nil {
			return err
		}
	}

	// 统计数量
	if attachment.CategoryId > 0 {
		var attachCount int64
		db.Model(&Attachment{}).Where("`category_id` = ?", attachment.CategoryId).Count(&attachCount)
		db.Model(&AttachmentCategory{}).Where("`id` = ?", attachment.CategoryId).UpdateColumn("attach_count", attachCount)
	}
	attachment.GetThumb()

	return nil
}

func (attachment *Attachment) Delete(db *gorm.DB) error {
	if err := db.Delete(attachment).Error; err != nil {
		return err
	}

	return nil
}
