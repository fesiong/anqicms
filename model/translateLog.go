package model

type TranslateLog struct {
	Id            uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime   int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	Md5           string `json:"md5" gorm:"column:md5;type:varchar(32);default:null;index:idx_md5"` // md5 的来源是 title-content-to_language
	OriginTitle   string `json:"origin_title" gorm:"column:origin_title;type:varchar(190) not null;default:''"`
	OriginContent string `json:"origin_content" gorm:"column:origin_content;type:longtext;default:null"`
	Title         string `json:"title" gorm:"column:title;type:varchar(190) not null;default:''"`
	Content       string `json:"content" gorm:"column:content;type:longtext;default:null"`
	Language      string `json:"language" gorm:"column:language;type:varchar(10);default:null"`
	ToLanguage    string `json:"to_language" gorm:"column:to_language;type:varchar(10);default:null"`
}

type TranslateHtmlLog struct {
	Id          uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	Uri         string `json:"uri" gorm:"column:uri;type:varchar(190) not null;default:''"`
	Language    string `json:"language" gorm:"column:language;type:varchar(10);default:null"`
	ToLanguage  string `json:"to_language" gorm:"column:to_language;type:varchar(10);default:null"`
	Count       int64  `json:"count" gorm:"column:count;type:bigint(20);default:0"`
	UseCount    int64  `json:"use_count" gorm:"column:use_count;type:bigint(20);default:0"`
	Status      int    `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`
	Remark      string `json:"remark" gorm:"column:remark;type:varchar(255);default:null"` // 备注信息，错误之类
}
