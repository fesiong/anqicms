package config

type KeywordJson struct {
	AutoDig      bool             `json:"auto_dig"` //关键词是否自动拓词
	FromEngine   string           `json:"from_engine"`
	FromWebsite  string           `json:"from_website"`
	Language     string           `json:"language"` // zh|en|cr
	TitleExclude []string         `json:"title_exclude"`
	TitleReplace []ReplaceKeyword `json:"title_replace"`
}

var defaultKeywordConfig = KeywordJson{
	AutoDig:     false,
	FromEngine:  EnginBaidu,
	Language:    LanguageZh,
	FromWebsite: "",
}
