package config

type AiGenerateConfig struct {
	Open            bool             `json:"open"`         // 是否自动写作
	Language        string           `json:"language"`     // zh|en|cr
	DoubleTitle     bool             `json:"double_title"` // 是否生成双标题
	DoubleSplit     int              `json:"double_split"` // 双标题形式
	Demand          string           `json:"demand"`       // 通用Demand
	InsertImage     int              `json:"insert_image"` // 是否插入图片, 0 移除图片，2 插入自定义图片，3，图片库分类
	Images          []string         `json:"images"`
	ImageCategoryId int              `json:"image_category_id"` // 选定的图片分类
	ContentReplace  []ReplaceKeyword `json:"content_replace"`
	CategoryId      uint             `json:"category_id"`   //默认分类
	CategoryIds     []uint           `json:"category_ids"`  // 默认分类，支持多个
	SaveType        uint             `json:"save_type"`     // 文档处理方式
	StartHour       int              `json:"start_hour"`    //每天开始时间
	EndHour         int              `json:"end_hour"`      //每天结束时间
	DailyLimit      int              `json:"daily_limit"`   //每日限额
	AiEngine        string           `json:"ai_engine"`     // ai 引擎，default 官方接口，openai 自定义openai，spark 星火大模型，DeepSeek
	AiTranslate     bool             `json:"ai_translate"`  // 是否使用AI翻译
	OpenAIKeys      []OpenAIKey      `json:"open_ai_keys"`  // self openai key
	ApiValid        bool             `json:"api_valid"`     // api地址是否可用
	KeyIndex        int              `json:"-"`             // 上一次调用的key id
	OpenAiApi       string           `json:"open_ai_api"`   // openai api地址
	OpenAIModel     string           `json:"open_ai_model"` // 选用的模型
	Spark           SparkSetting     `json:"spark"`
}

type OpenAIKey struct {
	Key     string `json:"key"`
	Invalid bool   `json:"invalid"` // 是否不可用
}

type SparkSetting struct {
	Version   string `json:"version"`
	AppID     string `json:"app_id"`
	APISecret string `json:"api_secret"`
	APIKey    string `json:"api_key"`
}

const (
	AiEngineDefault  = ""
	AiEngineOpenAI   = "openai"
	AiEngineSpark    = "spark"
	AiEngineDeepSeek = "deepseek"
)

const (
	TranslateEngineDefault = ""
	TranslateEngineBaidu   = "baidu"
	TranslateEngineYoudao  = "youdao"
	TranslateEngineGoogle  = "google"
	TranslateEngineAi      = "ai"
	TranslateEngineDeepl   = "deepl"
)
