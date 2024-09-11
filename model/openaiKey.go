package model

type OpenaiKey struct {
	Model
	Key      string `json:"key" gorm:"column:key;type:varchar(190) not null;default:'';index"`
	Usage    int64  `json:"usage" gorm:"column:usage;type:int(10) unsigned not null;default:0"`
	Code     int64  `json:"code" gorm:"column:code;type:int(10) unsigned not null;default:0"`
	ErrorMsg string `json:"error_msg" gorm:"column:error_msg;type:varchar(1000) not null;default:''"`
}
