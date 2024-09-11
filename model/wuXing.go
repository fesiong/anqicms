package model

type WuXing struct {
	Model
	Version uint   `json:"version" gorm:"column:version;type:int(11);not null;default:0"`
	First   string `json:"first" gorm:"column:first;type:varchar(2);not null;default:''"`
	Second  string `json:"second" gorm:"column:second;type:varchar(2);not null;default:''"`
	Third   string `json:"third" gorm:"column:third;type:varchar(2);not null;default:''"`
	Fortune string `json:"fortune" gorm:"column:fortune;type:varchar(8);not null;default:''"`
}
