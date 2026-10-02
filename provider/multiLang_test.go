package provider

import (
	"log"
	"testing"
	"time"

	"kandaoni.com/anqicms/request"
)

func TestSyncMultiLangSiteContent(t *testing.T) {
	InitWebsites()
	w := GetWebsite(1)

	status, err := w.NewMultiLangSync()
	if err != nil {
		t.Fatal(err)
	}
	go status.SyncMultiLangSiteContent(&request.PluginMultiLangSiteSyncRequest{Id: 3, ParentId: 1, Focus: false})

	for {
		time.Sleep(1 * time.Second)
		log.Printf("%v \n", status)
	}
}
