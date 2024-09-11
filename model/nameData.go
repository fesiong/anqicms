package model

type NameData struct {
	Model
	SourceId     uint   `json:"source_id" gorm:"column:source_id;type:int(10);not null"`
	FirstName    string `json:"first_name" gorm:"column:first_name;type:varchar(4);not null;default:''"`
	FirstStroke1 int    `json:"first_stroke_1" gorm:"column:first_stroke_1;type:tinyint(2);not null;default:0"`
	FirstStroke2 int    `json:"first_stroke_2" gorm:"column:first_stroke_2;type:tinyint(2);not null;default:0"`
	Total        int    `json:"total" gorm:"column:total;type:int(10);not null;default:0"`
	Male         int    `json:"male" gorm:"column:male;type:int(10);not null;default:0"`
	Female       int    `json:"female" gorm:"column:female;type:int(10);not null;default:0"`
}
