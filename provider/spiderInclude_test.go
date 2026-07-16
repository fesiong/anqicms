package provider

import "testing"

func TestQuerySpiderInclude(t *testing.T) {
	dbSite, err := GetDBWebsiteInfo(1)
	if err != nil {
		t.Fatal(err)
	}
	dbSite.Status = 1
	InitWebsite(dbSite)
	w := GetWebsite(1)
	w.QuerySpiderInclude()
}
