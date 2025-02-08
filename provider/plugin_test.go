package provider

import (
	"context"
	"fmt"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/paypal"
	"github.com/google/uuid"
	"kandaoni.com/anqicms/model"
	"log"
	"testing"
)

func (w *Website) TestPushBing(t *testing.T) {
	urls := []string{"https://www.anqicms.com/help-basic/112.html"}

	err := w.PushBing(urls)
	log.Println(err)
}

func TestPaypalPayment(t *testing.T) {
	GetDefaultDB()
	dbSite, _ := GetDBWebsiteInfo(1)
	InitWebsite(dbSite)
	currentSite := CurrentSite(nil)

	log.Println("currentSite.PluginPay", currentSite.PluginPay)

	client, err := paypal.NewClient(currentSite.PluginPay.PaypalClientId, currentSite.PluginPay.PaypalClientSecret, false)
	if err != nil {
		t.Log(err)
		return
	}

	payment := &model.Payment{
		PaymentId: uuid.NewString(),
		Amount:    1800,
	}

	client.DebugSwitch = gopay.DebugOn

	var purchases = []*paypal.PurchaseUnit{
		{
			ReferenceId: payment.PaymentId,
			Amount: &paypal.Amount{
				CurrencyCode: "USD",
				Value:        fmt.Sprintf("%.2f", float32(payment.Amount)/100),
			},
		},
	}

	bm := make(gopay.BodyMap)
	bm.Set("intent", "CAPTURE").
		Set("purchase_units", purchases).
		SetBodyMap("payment_source", func(b gopay.BodyMap) {
			b.SetBodyMap("paypal", func(bb gopay.BodyMap) {
				bb.SetBodyMap("experience_context", func(bbb gopay.BodyMap) {
					bbb.Set("brand_name", currentSite.System.SiteName).
						Set("locale", "en-US").
						Set("shipping_preference", "NO_SHIPPING").
						Set("user_action", "PAY_NOW").
						Set("return_url", currentSite.System.BaseUrl+"/return/paypal/pay").
						Set("cancel_url", currentSite.System.BaseUrl+"/return/paypal/cancel")
				})
			})
		})
	ppRsp, err := client.CreateOrder(context.Background(), bm)
	if err != nil {
		t.Log(err)
		return
	}
	log.Printf("%#v", ppRsp)
	if ppRsp.Code != 200 {
		// do something
		t.Log(ppRsp)
		return
	}

	log.Printf("%#v", ppRsp.Response)
}

// http://cn.anqi.com/return/paypal/pay?token=3VV29137SN736974V&PayerID=B57TZGN9MSAC8

func TestPaypalReturn(t *testing.T) {
	GetDefaultDB()
	dbSite, _ := GetDBWebsiteInfo(1)
	InitWebsite(dbSite)
	currentSite := CurrentSite(nil)

	log.Println("currentSite.PluginPay", currentSite.PluginPay)

	client, err := paypal.NewClient(currentSite.PluginPay.PaypalClientId, currentSite.PluginPay.PaypalClientSecret, false)
	if err != nil {
		t.Log(err)
		return
	}

	client.DebugSwitch = gopay.DebugOn

	bm := make(gopay.BodyMap)

	ppRsp, err := client.OrderDetail(context.Background(), "0Y5371446W544664V", bm)
	if err != nil {
		t.Log(err)
		return
	}
	log.Printf("%#v", ppRsp)
	if ppRsp.Code != 200 {
		// do something
		t.Log(ppRsp)
		return
	}

	log.Printf("%#v", ppRsp.Response)
}

func TestPaypaCapture(t *testing.T) {
	GetDefaultDB()
	dbSite, _ := GetDBWebsiteInfo(1)
	InitWebsite(dbSite)
	currentSite := CurrentSite(nil)

	log.Println("currentSite.PluginPay", currentSite.PluginPay)

	client, err := paypal.NewClient(currentSite.PluginPay.PaypalClientId, currentSite.PluginPay.PaypalClientSecret, false)
	if err != nil {
		t.Log(err)
		return
	}

	client.DebugSwitch = gopay.DebugOn

	bm := make(gopay.BodyMap)
	// 3VV29137SN736974V
	ppRsp, err := client.OrderCapture(context.Background(), "0Y5371446W544664V", bm)
	if err != nil {
		t.Log(err)
		return
	}
	log.Printf("%#v", ppRsp)
	if ppRsp.Code != 200 {
		// do something
		t.Log(ppRsp)
		return
	}

	log.Printf("%#v", ppRsp.Response)
}
