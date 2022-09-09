package provider

import (
	"log"
	"testing"
)

func TestPushBing(t *testing.T) {
	urls := []string{"https://www.anqicms.com/help-basic/112.html"}

	err := PushBing(urls)
	log.Println(err)
}
