package model

type Tag struct {
	Model
	Title       string `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
	SeoTitle    string `json:"seo_title" gorm:"column:seo_title;type:varchar(250) not null;default:''"`
	Keywords    string `json:"keywords" gorm:"column:keywords;type:varchar(250) not null;default:''"`
	UrlToken    string `json:"url_token" gorm:"column:url_token;type:varchar(190) not null;default:'';index"`
	Description string `json:"description" gorm:"column:description;type:varchar(1000) not null;default:''"`
	FirstLetter string `json:"first_letter" gorm:"column:first_letter;type:char(1) not null;default:'';index"`
	Status      uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_status"`
	Link        string `json:"link" gorm:"-"`
}

type TagData struct {
	Model
	TagId  uint `json:"tag_id" gorm:"column:tag_id;type:int(10) not null;default:0;index"`
	ItemId uint `json:"item_id" gorm:"column:item_id;type:int(10) unsigned not null;default:0;index:idx_item_id"`
}
