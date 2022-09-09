package model

import (
	"github.com/lib/pq"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"path/filepath"
	"strings"
)

type Archive struct {
	Model
	Title        string         `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
	SeoTitle     string         `json:"seo_title" gorm:"column:seo_title;type:varchar(250) not null;default:''"`
	UrlToken     string         `json:"url_token" gorm:"column:url_token;type:varchar(250) not null;default:'';index"`
	Keywords     string         `json:"keywords" gorm:"column:keywords;type:varchar(250) not null;default:''"`
	Description  string         `json:"description" gorm:"column:description;type:varchar(250) not null;default:''"`
	ModuleId     uint           `json:"module_id" gorm:"column:module_id;type:int(10) unsigned not null;default:1;index:idx_module_id"`
	CategoryId   uint           `json:"category_id" gorm:"column:category_id;type:int(10) unsigned not null;default:0;index:idx_category_id"`
	Views        uint           `json:"views" gorm:"column:views;type:int(10) unsigned not null;default:0;index:idx_views"`
	CommentCount uint           `json:"comment_count" gorm:"column:comment_count;type:int(10) unsigned not null;default:0;index"`
	Images       pq.StringArray `json:"images" gorm:"column:images;type:text default null"`
	Template     string         `json:"template" gorm:"column:template;type:varchar(250) not null;default:''"`
	Status       uint           `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`
	CanonicalUrl string         `json:"canonical_url" gorm:"column:canonical_url;type:varchar(250) not null;default:''"`         // 规范链接
	FixedLink    string         `json:"fixed_link" gorm:"column:fixed_link;type:varchar(250) default null;index:idx_fixed_link"` // 固化的链接
	Flag         string         `json:"flag" gorm:"column:flag;type:set('c','h','p','f','s','j','a','b') default null;index"`    //推荐标签
	UserId       uint           `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`
	Price        int64          `json:"price" gorm:"column:price;type:bigint(20) not null;default:0"`
	Stock        int64          `json:"stock" gorm:"column:stock;type:bigint(20) not null;default:9999999"`
	//采集专用
	HasPseudo   int    `json:"has_pseudo" gorm:"column:has_pseudo;type:tinyint(1) not null;default:0"`
	KeywordId   uint   `json:"keyword_id" gorm:"column:keyword_id;type:bigint(20) not null;default:0"`
	OriginUrl   string `json:"origin_url" gorm:"column:origin_url;type:varchar(250) not null;default:'';index:idx_origin_url"`
	OriginTitle string `json:"origin_title" gorm:"column:origin_title;type:varchar(250) not null;default:'';index:idx_origin_title"`
	// 其他内容
	Category    *Category               `json:"category" gorm:"-"`
	ModuleName  string                  `json:"module_name" gorm:"-"`
	ArchiveData *ArchiveData            `json:"data" gorm:"-"`
	Logo        string                  `json:"logo" gorm:"-"`
	Thumb       string                  `json:"thumb" gorm:"-"`
	Extra       map[string]*CustomField `json:"extra" gorm:"-"`
	Link        string                  `json:"link" gorm:"-"`
	Tags        []string                `json:"tags,omitempty" gorm:"-"`
}

type ArchiveData struct {
	Model
	Content string `json:"content" gorm:"column:content;type:longtext default null"`
}

func (a *Archive) BeforeSave(tx *gorm.DB) error {
	if len(a.Images) > 0 {
		for i := range a.Images {
			a.Images[i] = strings.TrimPrefix(a.Images[i], config.JsonData.PluginStorage.StorageUrl)
		}
	}
	return nil
}

func (a *Archive) AfterFind(tx *gorm.DB) error {
	a.GetThumb()
	return nil
}

func (a *Archive) AddViews(db *gorm.DB) error {
	db.Model(&Archive{}).Where("`id` = ?", a.Id).UpdateColumn("views", gorm.Expr("`views` + 1"))
	return nil
}

func (a *Archive) Save(db *gorm.DB) error {
	if err := db.Save(a).Error; err != nil {
		return err
	}
	if a.ArchiveData != nil {
		a.ArchiveData.Id = a.Id
		if err := db.Save(a.ArchiveData).Error; err != nil {
			return err
		}
	}

	return nil
}

func (a *Archive) Delete(db *gorm.DB) error {
	if err := db.Delete(a).Error; err != nil {
		return err
	}
	// 同时删除对应module的内容

	return nil
}

func (a *Archive) GetThumb() string {
	//取第一张
	if len(a.Images) > 0 {
		for i := range a.Images {
			if !strings.HasPrefix(a.Images[i], "http") && !strings.HasPrefix(a.Images[i], "//") {
				a.Images[i] = config.JsonData.PluginStorage.StorageUrl + "/" + strings.TrimPrefix(a.Images[i], "/")
			}
		}
		a.Logo = a.Images[0]
		paths, fileName := filepath.Split(a.Logo)
		a.Thumb = paths + "thumb_" + fileName
	} else if config.JsonData.Content.DefaultThumb != "" {
		a.Logo = config.JsonData.Content.DefaultThumb
		if !strings.HasPrefix(a.Logo, "http") && !strings.HasPrefix(a.Logo, "//") {
			a.Logo = config.JsonData.PluginStorage.StorageUrl + "/" + strings.TrimPrefix(a.Logo, "/")
		}
		a.Thumb = a.Logo
	}

	return a.Thumb
}
