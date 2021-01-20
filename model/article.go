package model

import (
	"gorm.io/gorm"
	"irisweb/config"
	"path/filepath"
	"strings"
	"time"
)

type Article struct {
	Model
	Id          uint         `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primary_key"`
	Title       string       `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
	Keywords    string       `json:"keywords" gorm:"column:keywords;type:varchar(250) not null;default:''"`
	Description string       `json:"description" gorm:"column:description;type:varchar(250) not null;default:''"`
	CategoryId  uint         `json:"category_id" gorm:"column:category_id;type:int(10) unsigned not null;default:0;index:idx_category_id"`
	Views       uint         `json:"views" gorm:"column:views;type:int(10) unsigned not null;default:0;index:idx_views"`
	Logo        string       `json:"logo" gorm:"column:logo;type:varchar(250) not null;default:''"`
	Status      uint         `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_status"`
	CreatedTime int64        `json:"created_time" gorm:"column:created_time;type:int(11) not null;default:0;index:idx_created_time"`
	UpdatedTime int64        `json:"updated_time" gorm:"column:updated_time;type:int(11) not null;default:0;index:idx_updated_time"`
	DeletedTime int64        `json:"-" gorm:"column:deleted_time;type:int(11) not null;default:0"`
	Category    *Category    `json:"category" gorm:"-"`
	ArticleData *ArticleData `json:"data" gorm:"-"`
	Thumb       string       `json:"thumb" gorm:"-"`
}

type ArticleData struct {
	Model
	Id      uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null;primary_key"`
	Content string `json:"content" gorm:"column:content;type:longtext default null"`
}

func (article *Article) AddViews(db *gorm.DB) error {
	article.Views = article.Views + 1
	db.Model(Article{}).Where("`id` = ?", article.Id).Update("views", article.Views)
	return nil
}

func (article *Article) Save(db *gorm.DB) error {
	if article.Id == 0 {
		article.CreatedTime = time.Now().Unix()
	}
	article.UpdatedTime = time.Now().Unix()

	if err := db.Debug().Save(article).Error; err != nil {
		return err
	}
	if article.ArticleData != nil {
		article.ArticleData.Id = article.Id
		if err := db.Debug().Save(article.ArticleData).Error; err != nil {
			return err
		}
	}

	return nil
}

func (article *Article) Delete(db *gorm.DB) error {
	if err := db.Model(article).Updates(Article{Status: 99, DeletedTime: time.Now().Unix()}).Error; err != nil {
		return err
	}

	return nil
}

func (article *Article) GetThumb() string {
	//如果是一个远程地址，则缩略图和原图地址一致
	if strings.HasPrefix(article.Logo, "http") {
		article.Thumb = article.Logo
	} else if article.Logo != "" {
		paths, fileName := filepath.Split(article.Logo)
		article.Thumb = config.JsonData.System.BaseUrl + paths + "thumb_" + fileName
	} else if config.JsonData.Content.DefaultThumb != "" {
		article.Thumb = config.JsonData.System.BaseUrl + config.JsonData.Content.DefaultThumb
	}

	return article.Thumb
}