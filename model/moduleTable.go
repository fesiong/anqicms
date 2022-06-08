package model

type ModuleTable struct {
	Id    uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	Table string `json:"table_name" gorm:"-"`
}

func (m ModuleTable) TableName() string {
	return m.Table
}
