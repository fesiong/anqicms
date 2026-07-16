package model

import (
	"path/filepath"
	"strings"

	"github.com/lib/pq"
)

type ParentPlace struct {
	Id    uint   `json:"id"`
	Title string `json:"title"`
}

type Place struct {
	Id          uint             `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64            `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64            `json:"updated_time" gorm:"column:updated_time;type:bigint(20);autoUpdateTime;index:idx_updated_time"`
	Title       string           `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
	SeoTitle    string           `json:"seo_title" gorm:"column:seo_title;type:varchar(250) not null;default:''"`
	Keywords    string           `json:"keywords" gorm:"column:keywords;type:varchar(250) not null;default:''"`
	UrlToken    string           `json:"url_token" gorm:"column:url_token;type:varchar(190) not null;default:'';index"`
	Description string           `json:"description" gorm:"column:description;type:varchar(1000) not null;default:''"`
	Content     string           `json:"content" gorm:"column:content;type:longtext default null"`
	ParentId    uint             `json:"parent_id" gorm:"column:parent_id;type:int(10) unsigned not null;default:0;index:idx_parent_id"`
	Sort        uint             `json:"sort" gorm:"column:sort;type:int(10) unsigned not null;default:99;index:idx_sort"`
	Template    string           `json:"template" gorm:"column:template;type:varchar(250) not null;default:''"`       // 自定义模板
	IsInherit   uint             `json:"is_inherit" gorm:"column:is_inherit;type:int(1) unsigned not null;default:0"` // 模板是否被继承
	Images      pq.StringArray   `json:"images" gorm:"column:images;type:text default null"`
	Logo        string           `json:"logo" gorm:"column:logo;type:varchar(250) not null;default:''"`
	Extra       ExtraData        `json:"extra,omitempty" gorm:"column:extra;type:longtext default null"` // 自定义字段
	Latitude    float64          `json:"latitude" gorm:"column:latitude;type:decimal(10,6) not null;default:0"`
	Longitude   float64          `json:"longitude" gorm:"column:longitude;type:decimal(10,6) not null;default:0"`
	Timezone    string           `json:"timezone" gorm:"column:timezone;type:varchar(100) not null;default:''"`
	Status      uint             `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`
	Spacer      string           `json:"spacer" gorm:"-"`
	HasChildren bool             `json:"has_children" gorm:"-"`
	Link        string           `json:"link" gorm:"-"`
	Thumb       string           `json:"thumb" gorm:"-"`
	IsCurrent   bool             `json:"is_current" gorm:"-"`
	Parents     []ParentCategory `json:"parents" gorm:"-"`

	Children []*Place `json:"children,omitempty" gorm:"-"`
}

func (category *Place) GetThumb(storageUrl, defaultThumb string) string {
	//取第一张
	if len(category.Images) > 0 {
		for i := range category.Images {
			if !strings.HasPrefix(category.Images[i], "http") && !strings.HasPrefix(category.Images[i], "//") {
				category.Images[i] = storageUrl + "/" + strings.TrimPrefix(category.Images[i], "/")
			}
		}
	}
	if category.Logo != "" {
		//如果是一个远程地址，则缩略图和原图地址一致
		if !strings.HasPrefix(category.Logo, "http") && !strings.HasPrefix(category.Logo, "//") {
			category.Logo = storageUrl + "/" + strings.TrimPrefix(category.Logo, "/")
		}
		if strings.HasPrefix(category.Logo, storageUrl) && !strings.HasSuffix(category.Logo, ".svg") {
			paths, fileName := filepath.Split(category.Logo)
			category.Thumb = paths + "thumb_" + fileName
		} else {
			category.Thumb = category.Logo
		}
	} else if defaultThumb != "" {
		category.Thumb = defaultThumb
		if !strings.HasPrefix(category.Thumb, "http") && !strings.HasPrefix(category.Thumb, "//") {
			category.Thumb = storageUrl + "/" + strings.TrimPrefix(category.Thumb, "/")
		}
		paths, fileName := filepath.Split(category.Thumb)
		category.Thumb = paths + "thumb_" + fileName
	}

	return category.Thumb
}
