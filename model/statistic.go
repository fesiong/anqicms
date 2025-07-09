package model

import (
	"database/sql/driver"
	"encoding/json"
)

// StatisticLog * 每天浏览量
type StatisticLog struct {
	Id          uint        `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64       `json:"created_time" gorm:"column:created_time;type:int(11);unique"`
	SpiderCount SpiderCount `json:"spider_count" gorm:"column:spider_count;type:varchar(250) default null"`
	VisitCount  VisitCount  `json:"visit_count" gorm:"column:visit_count;type:varchar(250) default null"`
}

type VisitCount struct {
	IPCount int `json:"ip_count"`
	PVCount int `json:"pv_count"`
}

func (v VisitCount) Value() (driver.Value, error) {
	return json.Marshal(v)
}

func (v *VisitCount) Scan(data interface{}) error {
	return json.Unmarshal(data.([]byte), &v)
}

type SpiderCount map[string]int

func (s SpiderCount) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *SpiderCount) Scan(data interface{}) error {
	return json.Unmarshal(data.([]byte), &s)
}
