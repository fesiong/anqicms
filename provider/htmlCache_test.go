package provider

import (
	"log"
	"testing"
	"time"
)

func TestBuildIndexCache(t *testing.T) {
	GetDefaultDB()
	dbSite, _ := GetDBWebsiteInfo(1)
	InitWebsite(dbSite)
	w := CurrentSite(nil)

	go func() {
		for {
			log.Printf("%#v", w.HtmlCacheStatus)
			time.Sleep(1 * time.Second)
		}
	}()
	w.BuildHtmlCache()
}
