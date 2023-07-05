package config

type AiGenerateConfig struct {
	Open           bool             `json:"open"`         // 是否自动写作
	Language       string           `json:"language"`     // zh|en|cr
	Demand         string           `json:"demand"`       // 通用Demand
	InsertImage    int              `json:"insert_image"` // 是否插入图片, 0 移除图片，2 插入自定义图片
	Images         []string         `json:"images"`
	ContentReplace []ReplaceKeyword `json:"content_replace"`
	CategoryId     uint             `json:"category_id"`  //默认分类
	SaveType       uint             `json:"save_type"`    // 文档处理方式
	StartHour      int              `json:"start_hour"`   //每天开始时间
	EndHour        int              `json:"end_hour"`     //每天结束时间
	DailyLimit     int              `json:"daily_limit"`  //每日限额
	UseSelfKey     bool             `json:"use_self_key"` // 使用自有key
	OpenAIKeys     []OpenAIKey      `json:"open_ai_keys"` // self openai key
	ApiValid       bool             `json:"api_valid"`    // api地址是否可用
	KeyIndex       int              `json:"-"`            // 上一次调用的key id
}

type OpenAIKey struct {
	Key     string `json:"key"`
	Invalid bool   `json:"invalid"` // 是否不可用
}
