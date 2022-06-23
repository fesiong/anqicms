package provider

import (
	"log"
	"testing"
)

func TestPushBing(t *testing.T) {
	urls := []string{"https://www.kandaoni.com/article/132"}

	err := PushBing(urls)
	log.Println(err)
}
