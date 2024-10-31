package model

import (
	"github.com/lib/pq"
	"gorm.io/gorm"
	"path/filepath"
	"strings"
)

type Category struct {
	Model
	Title          string         `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
	SeoTitle       string         `json:"seo_title" gorm:"column:seo_title;type:varchar(250) not null;default:''"`
	Keywords       string         `json:"keywords" gorm:"column:keywords;type:varchar(250) not null;default:''"`
	UrlToken       string         `json:"url_token" gorm:"column:url_token;type:varchar(190) not null;default:'';index"`
	Description    string         `json:"description" gorm:"column:description;type:varchar(1000) not null;default:''"`
	Content        string         `json:"content" gorm:"column:content;type:longtext default null"`
	ModuleId       uint           `json:"module_id" gorm:"column:module_id;type:int(10) unsigned not null;default:0;index:idx_module_id"`
	ParentId       uint           `json:"parent_id" gorm:"column:parent_id;type:int(10) unsigned not null;default:0;index:idx_parent_id"`
	Type           uint           `json:"type" gorm:"column:type;type:int(10) unsigned not null;default:0;index:idx_type"` // 1 archive, 3 page
	Sort           uint           `json:"sort" gorm:"column:sort;type:int(10) unsigned not null;default:99;index:idx_sort"`
	Template       string         `json:"template" gorm:"column:template;type:varchar(250) not null;default:''"`
	DetailTemplate string         `json:"detail_template" gorm:"column:detail_template;type:varchar(250) not null;default:''"`
	IsInherit      uint           `json:"is_inherit" gorm:"column:is_inherit;type:int(1) unsigned not null;default:0"` // 模板是否被继承
	Images         pq.StringArray `json:"images" gorm:"column:images;type:text default null"`
	Logo           string         `json:"logo" gorm:"column:logo;type:varchar(250) not null;default:''"`
	Extra          extraData      `json:"extra,omitempty" gorm:"column:extra;type:longtext default null"`                                             // 分类自定义字段
	ArchiveCount   int64          `json:"archive_count" gorm:"column:archive_count;type:int(10) unsigned not null;default:0;index:idx_archive_count"` // 内容数量统计
	Status         uint           `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`
	Spacer         string         `json:"spacer" gorm:"-"`
	HasChildren    bool           `json:"has_children" gorm:"-"`
	Link           string         `json:"link" gorm:"-"`
	Thumb          string         `json:"thumb" gorm:"-"`
	IsCurrent      bool           `json:"is_current" gorm:"-"`

	Children []*Category `json:"children,omitempty" gorm:"-"`
}

func (category *Category) GetThumb(storageUrl, defaultThumb string) string {
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
		if strings.HasPrefix(category.Logo, "http") || strings.HasPrefix(category.Logo, "//") {
			category.Thumb = category.Logo
		} else {
			category.Logo = storageUrl + "/" + strings.TrimPrefix(category.Logo, "/")
			paths, fileName := filepath.Split(category.Logo)
			category.Thumb = paths + "thumb_" + fileName
			if strings.HasSuffix(category.Logo, ".svg") {
				category.Thumb = category.Logo
			}
		}
	} else if defaultThumb != "" {
		category.Thumb = defaultThumb
		if !strings.HasPrefix(category.Thumb, "http") && !strings.HasPrefix(category.Thumb, "//") {
			category.Thumb = storageUrl + "/" + strings.TrimPrefix(category.Thumb, "/")
		}
	}

	return category.Thumb
}

func (category *Category) Save(db *gorm.DB) error {
	if err := db.Save(category).Error; err != nil {
		return err
	}

	return nil
}

func (category *Category) Delete(db *gorm.DB) error {
	if err := db.Delete(category).Error; err != nil {
		return err
	}
	//删除后，如果存在下级分类，则需要将它们的分类级别上移，文章也需要
	db.Model(&Category{}).Where("`parent_id` = ?", category.Id).Update("parent_id", category.ParentId)
	db.Model(&Archive{}).Where("`category_id` = ?", category.Id).Update("category_id", category.ParentId)

	return nil
}
