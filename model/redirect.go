package model

type Redirect struct {
	Model
	FromUrl string `json:"from_url" gorm:"column:from_url;type:varchar(250) not null;default:'';unique"`
	ToUrl   string `json:"to_url" gorm:"column:to_url;type:varchar(250) not null;default:''"`
}
