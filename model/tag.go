package model

import (
	"gorm.io/gorm"
	"path/filepath"
	"strings"
)

type Tag struct {
	Model
	Title       string `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
	CategoryId  uint   `json:"category_id" gorm:"column:category_id;type:int(10) unsigned not null;default:0;index"`
	SeoTitle    string `json:"seo_title" gorm:"column:seo_title;type:varchar(250) not null;default:''"`
	Keywords    string `json:"keywords" gorm:"column:keywords;type:varchar(250) not null;default:''"`
	UrlToken    string `json:"url_token" gorm:"column:url_token;type:varchar(190) not null;default:'';index"`
	Description string `json:"description" gorm:"column:description;type:varchar(1000) not null;default:''"`
	FirstLetter string `json:"first_letter" gorm:"column:first_letter;type:char(1) not null;default:'';index"`
	Template    string `json:"template" gorm:"column:template;type:varchar(250) not null;default:''"`
	Logo        string `json:"logo" gorm:"column:logo;type:varchar(250) not null;default:''"`
	Status      uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_status"`

	Link          string `json:"link" gorm:"-"`
	Thumb         string `json:"thumb" gorm:"-"`
	Content       string `json:"content,omitempty" gorm:"-"`
	CategoryTitle string `json:"category_title,omitempty" gorm:"-"`
}

type TagData struct {
	Id     uint  `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	TagId  uint  `json:"tag_id" gorm:"column:tag_id;type:int(10) not null;default:0;index"`
	ItemId int64 `json:"item_id" gorm:"column:item_id;type:bigint(20) not null;default:0;index:idx_item_id"`
}

type TagContent struct {
	Id      uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	Content string `json:"content" gorm:"column:content;type:longtext default null"`
}

func (tag *Tag) GetThumb(storageUrl, defaultThumb string) string {
	if tag.Logo != "" {
		//如果是一个远程地址，则缩略图和原图地址一致
		if strings.HasPrefix(tag.Logo, "http") || strings.HasPrefix(tag.Logo, "//") {
			tag.Thumb = tag.Logo
		} else {
			tag.Logo = storageUrl + "/" + strings.TrimPrefix(tag.Logo, "/")
			paths, fileName := filepath.Split(tag.Logo)
			tag.Thumb = paths + "thumb_" + fileName
			if strings.HasSuffix(tag.Logo, ".svg") {
				tag.Thumb = tag.Logo
			}
		}
	} else if defaultThumb != "" {
		tag.Thumb = defaultThumb
		if !strings.HasPrefix(tag.Thumb, "http") && !strings.HasPrefix(tag.Thumb, "//") {
			tag.Thumb = storageUrl + "/" + strings.TrimPrefix(tag.Thumb, "/")
		}
	}

	return tag.Thumb
}

func GetNextTagId(tx *gorm.DB) uint {
	var lastId int64
	tx.Model(Tag{}).Order("id desc").Limit(1).Pluck("id", &lastId)

	return uint(lastId) + 1
}
