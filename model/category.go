package model

import (
    "gorm.io/gorm"
    "time"
)

const (
    CategoryTypeArticle = 1
    CategoryTypeProduct = 2
    CategoryTypePage    = 3
)

type Category struct {
    Model
    Id          uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primary_key"`
    Title       string `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
    UrlToken    string `json:"url_token" gorm:"column:url_token;type:varchar(250) not null;default:'';index"`
    Description string `json:"description" gorm:"column:description;type:varchar(250) not null;default:''"`
    Content     string `json:"content" gorm:"column:content;type:longtext default null"`
    ParentId    uint   `json:"parent_id" gorm:"column:parent_id;type:int(10) unsigned not null;default:0;index:idx_parent_id"`
    Type        uint   `json:"type" gorm:"column:type;type:int(10) unsigned not null;default:0;index:idx_type"`
    Sort        uint   `json:"sort" gorm:"column:sort;type:int(10) unsigned not null;default:99;index:idx_sort"`
    Status      uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_status"`
    CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11) not null;default:0;index:idx_created_time"`
    UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;type:int(11) not null;default:0;index:idx_updated_time"`
    DeletedTime int64  `json:"-" gorm:"column:deleted_time;type:int(11) not null;default:0"`
    Spacer      string `json:"spacer" gorm:"-"`
    HasChildren bool   `json:"has_children" gorm:"-"`
}

func (category *Category) Save(db *gorm.DB) error {
    if category.Id == 0 {
        category.CreatedTime = time.Now().Unix()
    }
    category.UpdatedTime = time.Now().Unix()

    if err := db.Save(category).Error; err != nil {
        return err
    }

    return nil
}

func (category *Category) Delete(db *gorm.DB) error {
    if err := db.Model(category).Updates(Category{Status: 99, DeletedTime: time.Now().Unix()}).Error; err != nil {
        return err
    }
    //删除后，如果存在下级分类，则需要将它们的分类级别上移，文章也需要
    db.Model(&Category{}).Where("`parent_id` = ?", category.Id).Update("parent_id", category.ParentId)
    if category.Type == CategoryTypeArticle {
        //文章
        db.Model(&Article{}).Where("`category_id` = ?", category.Id).Update("category_id", category.ParentId)
    } else if category.Type == CategoryTypeProduct {
        //产品
        db.Model(&Product{}).Where("`category_id` = ?", category.Id).Update("category_id", category.ParentId)
    }

    return nil
}
