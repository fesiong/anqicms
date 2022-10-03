package provider

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"testing"
)

func TestGenerateCombination(t *testing.T) {
	keyword := &model.Keyword{Title: "sunglasses"}
	config.CollectorConfig.FromEngine = config.EnginBing
	config.CollectorConfig.InsertImage = true
	config.CollectorConfig.Language = config.LanguageEn
	_, err := GenerateCombination(keyword)
	if err != nil {
		t.Fatal(err)
	}
}
