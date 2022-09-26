package config

type CombinationJson struct {
	AutoCollect    bool             `json:"auto_collect"` // 是否自动采集
	FromEngine     string           `json:"from_engine"`
	Language       string           `json:"language"`     // zh|en|cr
	InsertImage    bool             `json:"insert_image"` // 是否插入图片
	FromWebsite    string           `json:"from_website"`
	ContentExclude []string         `json:"content_exclude"`
	ContentReplace []ReplaceKeyword `json:"content_replace"`
	AutoDigKeyword bool             `json:"auto_dig_keyword"` //关键词是否自动拓词
	CategoryId     uint             `json:"category_id"`      //默认分类
	SaveType       uint             `json:"save_type"`        // 文档处理方式
	StartHour      int              `json:"start_hour"`       //每天开始时间
	EndHour        int              `json:"end_hour"`         //每天结束时间
	DailyLimit     int              `json:"daily_limit"`      //每日限额
}

var defaultCombinationConfig = CombinationJson{
	AutoCollect:    false,
	FromEngine:     EnginBing,
	Language:       LanguageZh,
	InsertImage:    false,
	AutoDigKeyword: false,
	CategoryId:     0,
	SaveType:       0,
	StartHour:      8,
	EndHour:        20,
	DailyLimit:     1000,
}
