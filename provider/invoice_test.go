package provider

import (
	"kandaoni.com/anqicms/model"
	"log"
	"testing"
)

func TestGenerateInvoice(t *testing.T) {
	dbSite, _ := GetDBWebsiteInfo(1)
	dbSite.Status = 1
	InitWebsite(dbSite)
	w := GetWebsite(1)
	var payment model.Payment
	err := w.DB.Last(&payment).Error
	if err != nil {
		log.Println(err)
		return
	}
	order, err := w.GetOrderInfoByOrderId(payment.OrderId)
	if err != nil {
		log.Println(err)
		return
	}
	pathInfo, err := w.GenerateInvoice(order)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(pathInfo)
}
