package provider

import (
	"log"
	"testing"
)

func (w *Website) TestGetUrl(t *testing.T) {
	archive, err := w.GetArchiveById(12)
	if err != nil {
		t.Fatal(err)
	}

	link := w.GetUrl("archive", archive, 0)
	log.Println(link)
}
