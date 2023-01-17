package model

type Setting struct {
	Key   string `json:"key" gorm:"column:key;type:varchar(190) not null;primaryKey"`
	Value string `json:"value" gorm:"column:value;type:longtext default null"`
}
