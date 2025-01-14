package model

type AiArticlePlan struct {
	Model
	Type       int    `json:"type" gorm:"column:type;type:tinyint(1)  not null;default:0;index:idx_type"` // 0 预设无用，1 AI写作，2 AI翻译，3，AI改写，4 自媒体重写
	ReqId      int64  `json:"req_id" gorm:"column:req_id;type:bigint(20) not null;default:0"`             // 服务端返回的可查询ID
	Language   string `json:"language" gorm:"column:language;type:varchar(10) not null;default:''"`
	ToLanguage string `json:"to_language" gorm:"column:to_language;type:varchar(10) not null;default:''"`
	Keyword    string `json:"keyword" gorm:"column:keyword;type:varchar(250) not null;default:''"`
	Demand     string `json:"demand" gorm:"column:demand;type:varchar(250) not null;default:''"`      // demand最多支持250字
	ArticleId  int64  `json:"article_id" gorm:"column:article_id;type:bigint(20) not null;default:0"` // 生成的文章ID
	PayCount   int64  `json:"pay_count" gorm:"column:pay_count;type:int(10) not null;default:0"`      // 这篇文章需要支付的数量，1000字等于1篇
	UseSelf    bool   `json:"use_self" gorm:"column:use_self;type:tinyint(1)  not null;default:0"`    // 0 安企，1 自己的。
	Status     int    `json:"status" gorm:"column:status;type:tinyint(1)  not null;default:0"`        // 0 未使用，1 已推送进行中，2已完成，4写作出错

	Title string `json:"title" gorm:"-"`
}
