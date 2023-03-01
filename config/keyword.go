package config

type KeywordJson struct {
	AutoDig      bool             `json:"auto_dig"` //关键词是否自动拓词
	Language     string           `json:"language"` // zh|en|cr
	MaxCount     int64            `json:"max_count"`
	TitleExclude []string         `json:"title_exclude"`
	TitleReplace []ReplaceKeyword `json:"title_replace"`
}

var DefaultKeywordConfig = KeywordJson{
	AutoDig:  false,
	Language: LanguageZh,
	MaxCount: 100000,
}
