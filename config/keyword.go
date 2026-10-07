package config

type KeywordJson struct {
	AutoDig      bool             `json:"auto_dig"`      //关键词是否自动拓词
	Language     string           `json:"language"`      // zh|en|cr
	MaxCount     int64            `json:"max_count"`     // 最大挖掘数量
	TitleExclude []string         `json:"title_exclude"` // 关键词过滤
	TitleReplace []ReplaceKeyword `json:"title_replace"` // 关键词替换
}

var DefaultKeywordConfig = KeywordJson{
	AutoDig:  false,
	Language: LanguageZh,
	MaxCount: 100000,
}
