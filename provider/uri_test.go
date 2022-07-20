package provider

import (
	"log"
	"testing"
)

func TestGetUrl(t *testing.T) {
	archive, err := GetArchiveById(12)
	if err != nil {
		t.Fatal(err)
	}

	link := GetUrl("archive", archive, 0)
	log.Println(link)
}
