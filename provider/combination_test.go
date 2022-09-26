package provider

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"log"
	"testing"
)

func TestCollectCombinationMaterials(t *testing.T) {
	keyword := &model.Keyword{Title: "搞笑"}
	config.KeywordConfig.FromEngine = config.Engin360
	result := collectCombinationMaterials(keyword)

	log.Println(result)
}
