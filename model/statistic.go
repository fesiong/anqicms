package model

type Statistic struct {
	Model
	Spider    string `json:"spider" gorm:"column:spider;type:varchar(20) not null;default:'';index"`
	Host      string `json:"host" gorm:"column:host;type:varchar(100) not null;default:'';index"`
	Url       string `json:"url" gorm:"column:url;type:varchar(250) not null;default:''"`
	Ip        string `json:"ip" gorm:"column:ip;type:varchar(15) not null;default:''"`
	Device    string `json:"device" gorm:"column:device;type:varchar(20) not null;default:'';index"`
	HttpCode  int    `json:"http_code" gorm:"column:http_code;type:int(3) not null;default:0"`
	UserAgent string `json:"user_agent" gorm:"column:user_agent;type:varchar(255) not null;default:''"`
}
