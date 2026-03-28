package provider

import (
	"kandaoni.com/anqicms/request"
	"log"
	"testing"
	"time"
)

func TestSyncMultiLangSiteContent(t *testing.T) {
	InitWebsites()
	w := GetWebsite(1)

	status, err := w.NewMultiLangSync()
	if err != nil {
		t.Fatal(err)
	}
	go status.SyncMultiLangSiteContent(&request.PluginMultiLangSiteRequest{Id: 3, ParentId: 1, Focus: false})

	for {
		time.Sleep(1 * time.Second)
		log.Printf("%v \n", status)
	}
}
