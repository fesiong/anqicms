package model

import "gorm.io/gorm"

const (
	StatusWait = uint(0)
	StatusOk   = uint(1)
)

/**
 * 说明 改用soft delete
 */
type Model struct {
	//默认字段
	Id          uint  `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64 `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64 `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	//删除字段不包含在json中
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

type CustomField struct {
	Name        string      `json:"name"`
	Value       interface{} `json:"value"`
	Default     interface{} `json:"default"`
	FollowLevel bool        `json:"follow_level"`
	Type        string      `json:"-"`
	FieldName   string      `json:"-"`
}
