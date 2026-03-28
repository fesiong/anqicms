package provider

import (
	"log"
	"regexp"
	"testing"
)

func TestGetUrl(t *testing.T) {
	link := "/article(/c-{combine})/{filename}(/{page}).html"

	re := regexp.MustCompile(`\(([^{page}]*?)\)`)
	link = re.ReplaceAllString(link, "")

	log.Println(link)
}
