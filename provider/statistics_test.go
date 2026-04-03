package provider

import "testing"

func TestCalc(t *testing.T) {
	dbSite, _ := GetDBWebsiteInfo(1)
	InitWebsite(dbSite)
	w := GetWebsite(1)
	w.StatisticLog.Calc(w.DB.Debug())
}
