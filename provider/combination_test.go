package provider

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"testing"
)

func (w *Website) TestGenerateCombination(t *testing.T) {
	keyword := &model.Keyword{Title: "sunglasses"}
	w.CollectorConfig.FromEngine = config.EnginBing
	w.CollectorConfig.InsertImage = true
	w.CollectorConfig.Language = config.LanguageEn
	_, err := w.GenerateCombination(keyword)
	if err != nil {
		t.Fatal(err)
	}
}
