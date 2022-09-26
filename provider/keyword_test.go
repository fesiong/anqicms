package provider

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"log"
	"sync"
	"testing"
)

func TestCollectKeywords(t *testing.T) {
	collectedWords := &sync.Map{}
	keyword := &model.Keyword{Title: "yahoo"}
	config.KeywordConfig.Language = config.LanguageEn
	config.KeywordConfig.FromEngine = config.EnginBing
	err := collectKeyword(collectedWords, keyword)
	if err != nil {
		t.Fatal()
	}
}

func TestKeywordFilter(t *testing.T) {
	ss := []string{
		"0.00000",
		"12,4567",
		"99.98 88",
		"景点图片大全",
		"nlpv=test_bt_47",
		"so_home",
		"nginx'wangdun'xgb'zds",
	}

	for _, v := range ss {
		log.Println(KeywordFilter(v))
	}
}
