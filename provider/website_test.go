package provider

import (
	"log"
	"net/url"
	"testing"
)

func TestParseUrl(t *testing.T) {
	for _, urlToCheck := range []string{"http://127.0.0.1:8001", "https://m.anqicms.com", "http://admin.anqicms.com"} {
		if urlToCheck == "" {
			continue
		}
		// 这里不处理根路径的 domain
		parsed, err := url.Parse(urlToCheck)
		log.Println(err, parsed.Hostname(), parsed.RequestURI())
		if err != nil || parsed.RequestURI() == "/" {
			continue
		}
	}
}
