package provider

import (
	"kandaoni.com/anqicms/model"
	"log"
	"strings"
	"sync"
	"testing"
)

func (w *Website) TestCollectKeywords(t *testing.T) {
	collectedWords := &sync.Map{}
	keyword := &model.Keyword{Title: "yahoo"}
	err := w.collectKeyword(collectedWords, keyword)
	if err != nil {
		t.Fatal()
	}
}

func (w *Website) TestKeywordFilter(t *testing.T) {
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
		log.Println(w.KeywordFilter(v))
	}
}

func (w *Website) TestContainKeywords(t *testing.T) {
	s := "哪位来讲一下环氧地板漆怎么样?来说说_涂料/油漆_太平洋家居问答"

	res := w.ContainKeywords(s, "环氧地坪漆怎么样")

	log.Println(res)
}

func (w *Website) TestTrim(t *testing.T) {
	s := "环氧漆地坪质量怎么样?-环氧漆地坪口碑怎么样? - 小麦优选"
	title := strings.TrimSpace(s)
	title = strings.Trim(title, "...…")
	index := strings.IndexAny(title, "|-_?？.")
	log.Println(index)
	if index > 0 {
		title = title[:index]
		title = strings.TrimSpace(title)
	}

	log.Println(title)
}
