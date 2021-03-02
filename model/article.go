package model

import (
	"github.com/lib/pq"
	"gorm.io/gorm"
	"irisweb/config"
	"path/filepath"
	"strings"
)

type Article struct {
	Model
	Title       string         `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
	UrlToken    string         `json:"url_token" gorm:"column:url_token;type:varchar(250) not null;default:'';index"`
	Keywords    string         `json:"keywords" gorm:"column:keywords;type:varchar(250) not null;default:''"`
	Description string         `json:"description" gorm:"column:description;type:varchar(250) not null;default:''"`
	CategoryId  uint           `json:"category_id" gorm:"column:category_id;type:int(10) unsigned not null;default:0;index:idx_category_id"`
	Views       uint           `json:"views" gorm:"column:views;type:int(10) unsigned not null;default:0;index:idx_views"`
	Images      pq.StringArray `json:"images" gorm:"column:images;type:text default null"`
	Status      uint           `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_status"`
	Category    *Category      `json:"category" gorm:"-"`
	ArticleData *ArticleData   `json:"data" gorm:"-"`
	Logo        string         `json:"logo" gorm:"-"`
	Thumb       string         `json:"thumb" gorm:"-"`
}

type ArticleData struct {
	Model
	Content string `json:"content" gorm:"column:content;type:longtext default null"`
}

func (article *Article) AddViews(db *gorm.DB) error {
	article.Views = article.Views + 1
	db.Model(Article{}).Where("`id` = ?", article.Id).Update("views", article.Views)
	return nil
}

func (article *Article) Save(db *gorm.DB) error {
	if err := db.Save(article).Error; err != nil {
		return err
	}
	if article.ArticleData != nil {
		article.ArticleData.Id = article.Id
		if err := db.Save(article.ArticleData).Error; err != nil {
			return err
		}
	}

	return nil
}

func (article *Article) Delete(db *gorm.DB) error {
	if err := db.Delete(article).Error; err != nil {
		return err
	}

	return nil
}

func (article *Article) GetThumb() string {
	//取第一张
	if len(article.Images) > 0 {
		article.Logo = article.Images[0]
		//如果是一个远程地址，则缩略图和原图地址一致
		if strings.HasPrefix(article.Logo, "http") {
			article.Thumb = article.Logo
		} else {
			paths, fileName := filepath.Split(article.Logo)
			article.Thumb = config.JsonData.System.BaseUrl + paths + "thumb_" + fileName
		}
	} else if config.JsonData.Content.DefaultThumb != "" {
		article.Logo = config.JsonData.System.BaseUrl + config.JsonData.Content.DefaultThumb
		article.Thumb = article.Logo
	}

	return article.Thumb
}
