package provider

import (
	"log"
	"testing"
)

func (w *Website) TestPushBing(t *testing.T) {
	urls := []string{"https://www.anqicms.com/help-basic/112.html"}

	err := w.PushBing(urls)
	log.Println(err)
}
