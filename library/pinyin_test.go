package library

import (
	"log"
	"testing"
)

func TestGetPinyin(t *testing.T) {
	result := GetPinyin("如何利用SEO优化网站？")

	log.Println(result)
}
