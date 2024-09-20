package provider

import "testing"

func TestQuerySpiderInclude(t *testing.T) {
	dbSite, _ := GetDBWebsiteInfo(1)
	InitWebsite(dbSite)
	w := GetWebsite(1)
	w.QuerySpiderInclude()
}
