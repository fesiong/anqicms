package provider

import (
	"log"
	"testing"
)

func TestDownloadRemoteImage(t *testing.T) {
	link := "https://mmbiz.qpic.cn/mmbiz_jpg/YNoY3yGicTIRicbeSpTCnzxK1icJ0vBLlnMwibl9icyZcNnL4ml0ic3YI1Yp3RyeK8FicBu9OFVvmibRuK89ky5u2faCnw/640?wx_fmt=jpeg"
	alt := ""

	result, err := DownloadRemoteImage(link, alt)
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("%#v", result)
}
