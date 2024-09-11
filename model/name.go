package model

import (
	"gorm.io/gorm"
)

type NameHistory struct {
	Model
	Hash     string `json:"hash" gorm:"column:hash;type:varchar(32);unique;not null"`
	LastName string `json:"last_name" gorm:"column:last_name;type:varchar(4);not null;default:''"`
	Gender   string `json:"gender" gorm:"column:gender;type:varchar(10);not null;default:''"`
	Times    uint   `json:"times" gorm:"column:times;type:int(10);not null;default:0"`
	Views    uint   `json:"views" gorm:"column:views;type:int(10);not null;default:0"`
}

type NameSource struct {
	Model
	Title  string `json:"title" gorm:"column:title;type:varchar(100);not null;default:''"`
	Status uint   `json:"status" gorm:"column:status;type:tinyint(1);unsigned;not null;default:0"`
}

type NameSourceData struct {
	Model
	SourceId    uint   `json:"source_id" gorm:"column:source_id;type:int(10);unsigned;not null;"`
	Title       string `json:"title" gorm:"column:title;type:varchar(100);not null;default:''"`
	SearchKey   string `json:"search_key" gorm:"column:search_key;type:varchar(250);not null;default:''"`
	Description string `json:"description" gorm:"column:description;type:varchar(250);not null;default:''"`
	Content     string `json:"content" gorm:"column:content;type:text;default null"`
	Status      uint   `json:"status" gorm:"column:status;type:tinyint(1);unsigned;not null;default:0"`
}

type NameDetail struct {
	Model
	Hash      string `json:"hash" gorm:"column:hash;type:varchar(32);unique;not null"`
	LastName  string `json:"last_name" gorm:"column:last_name;type:varchar(4);not null;default:''"`
	FirstName string `json:"first_name" gorm:"column:first_name;type:varchar(4);not null;default:''"`
	Gender    string `json:"gender" gorm:"column:gender;type:varchar(10);not null;default:''"`
	Born      int    `json:"born" gorm:"column:born;type:int(11);not null;default:0"`
	Views     uint   `json:"views" gorm:"column:views;type:int(10);not null;default:0"`
}

type Surname struct {
	Model
	Hash        string `json:"hash" gorm:"column:hash;type:varchar(32);unique;not null"`
	Title       string `json:"title" gorm:"column:title;type:varchar(4);not null;default:''"`
	Keywords    string `json:"keywords" gorm:"column:keywords;type:varchar(250);not null;default:''"`
	Description string `json:"description" gorm:"column:description;type:varchar(250);not null;default:''"`
	Content     string `json:"content" gorm:"column:content;type:longtext;default null"`
	Status      uint   `json:"status" gorm:"column:status;type:tinyint(1);unsigned;not null;default:0"`
	Views       uint   `json:"views" gorm:"column:views;type:int(10);not null;default:0"`
}

func (nameDetail *NameDetail) GetFullName() string {
	return nameDetail.LastName + nameDetail.FirstName
}

func (history *NameHistory) AddViews(db *gorm.DB) error {
	history.Views = history.Views + 1
	db.Model(NameHistory{}).Where("`id` = ?", history.Id).Update("views", history.Views)
	return nil
}

func (nameDetail *NameDetail) AddViews(db *gorm.DB) error {
	nameDetail.Views = nameDetail.Views + 1
	db.Model(NameDetail{}).Where("`id` = ?", nameDetail.Id).Update("views", nameDetail.Views)
	return nil
}
