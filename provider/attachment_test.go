package provider

import (
	"log"
	"testing"
)

func TestDownloadRemoteImage(t *testing.T) {
	link := "https://www.php.cn/static/images/sw/yamaxun_mob.jpg"
	alt := "4.png"

	result, err := DownloadRemoteImage(link, alt)
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("%#v", result)
}
