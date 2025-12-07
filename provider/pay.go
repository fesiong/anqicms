package provider

import (
	"context"
	"encoding/json"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/paypal"
	"github.com/go-pay/xlog"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"log"
	"strings"
	"time"
)

type PaypalWebhookResource struct {
	Id         string `json:"id"`
	CreateTime string `json:"create_time"`
	UpdateTime string `json:"update_time"`
	State      string `json:"state"`
	Amount     struct {
		Total    string `json:"total"`
		Currency string `json:"currency"`
		Details  struct {
			Subtotal string `json:"subtotal"`
		} `json:"details"`
	} `json:"amount"`
	ParentPayment string `json:"parent_payment"`
	ValidUntil    string `json:"valid_until"`
}

func (w *Website) ProcessPaypalEvent(event *paypal.WebhookEvent) {
	// 添加超时控制
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 使用事件ID进行幂等性检查
	if w.isDuplicateEvent(ctx, event.Id) {
		xlog.Warnf("Duplicate event detected: %s", event.Id)
		return
	}
	// 获取payment信息
	var resp PaypalWebhookResource
	err := json.Unmarshal(event.Resource, &resp)
	if err != nil {
		xlog.Errorf("Failed to unmarshal resource: %v, %v", err, string(event.Resource))
		return
	}
	payment, err := w.GetPaymentInfoByTerraceId(resp.Id)
	if err != nil {
		xlog.Errorf("Failed to get payment info: %v", err)
		return
	}

	client, err := paypal.NewClient(w.PluginPay.PaypalClientId, w.PluginPay.PaypalClientSecret, w.PluginPay.PaypalSandbox == false)
	if err != nil {
		// 处理token获取失败
		return
	}
	library.DebugLog(w.CachePath, "paypal_webhook", string(event.Resource))
	// 根据事件类型路由处理
	switch event.EventType {
	case "CHECKOUT.ORDER.APPROVED":
		// 执行capture
		captureRes, err := client.OrderCapture(context.Background(), payment.TerraceId, nil)
		if err != nil {
			log.Println("capture err", err)
			return
		}
		library.DebugLog(w.CachePath, "paypalCapture", captureRes.Response)
		if captureRes.Code == 0 {
			// todo
		} else {
			log.Println("captureRes err", captureRes.Error)
		}
	case "PAYMENT.CAPTURE.COMPLETED":
		// 付款捕获完成（资金已到账）
		payment.PayWay = config.PayWayPaypal
		err = w.TraceQuery(payment)
	case "PAYMENT.CAPTURE.DENIED":
		order, err := w.GetOrderInfoByOrderId(payment.OrderId)
		if err == nil {
			_ = w.SetOrderCanceled(order)
		}
	case "PAYMENT.CAPTURE.REFUNDED":
		//order, err := w.GetOrderInfoByOrderId(payment.OrderId)
		//if err == nil {
		//	err = w.ApplyOrderRefund(order)
		//	err = w.SetOrderRefund(order, 1)
		//}
	case "CHECKOUT.ORDER.COMPLETED":
		payment.PayWay = config.PayWayPaypal
		err = w.TraceQuery(payment)
	default:
		xlog.Warnf("Unhandled event type: %s", event.EventType)
	}

}

func (w *Website) isDuplicateEvent(ctx context.Context, eventId string) bool {
	var eventIdExists bool
	err := w.Cache.Get(eventId, &eventIdExists)
	if err != nil {
		return false
	}
	// 保留30秒的缓存
	_ = w.Cache.Set(eventId, true, 30)

	return false
}

func (w *Website) UpdatePaypalWebhook() {
	if w.PluginPay.PaypalOpen == false || w.PluginPay.PaypalClientId == "" || w.PluginPay.PaypalClientSecret == "" {
		return
	}
	if strings.Contains(w.System.BaseUrl, "127.0.0.1") {
		return
	}
	// 不支持http
	if strings.HasPrefix(w.System.BaseUrl, "http://") {
		return
	}
	client, err := paypal.NewClient(w.PluginPay.PaypalClientId, w.PluginPay.PaypalClientSecret, w.PluginPay.PaypalSandbox == false)
	if err != nil {
		// 处理token获取失败
		return
	}
	resp, err := client.ListWebhook(context.Background())
	if err != nil {
		// 处理获取失败
		log.Println("paypal webhook list error", err.Error())
		return
	}
	if resp.Code != paypal.Success {
		// 不处理
		log.Println("paypal webhook list error", resp.Error)
		return
	}

	var eventTypes = []*paypal.WebhookEventType{
		{Name: "PAYMENT.CAPTURE.COMPLETED"},
		{Name: "PAYMENT.CAPTURE.DENIED"},
		{Name: "PAYMENT.CAPTURE.REFUNDED"},
		{Name: "CHECKOUT.ORDER.APPROVED"},
		{Name: "CHECKOUT.ORDER.COMPLETED"},
	}
	// anqicms 的 webhook url 是 /notify/paypal/pay
	var webhookId = w.PluginPay.PaypalWebhookId
	var webhookUrl = w.System.BaseUrl + "/notify/paypal/pay"
	var findUrl string
	if len(resp.Response.Webhooks) > 0 {
		var exist bool
		for _, webhook := range resp.Response.Webhooks {
			if webhook.Url == webhookUrl || webhook.Id == webhookId {
				webhookId = webhook.Id
				findUrl = webhook.Url
				exist = true
				break
			}
		}
		if !exist {
			webhookId = ""
		}
	}
	if webhookId == "" || findUrl != webhookUrl {
		if webhookId == "" {
			// 创建 webhook
			bm := make(gopay.BodyMap)
			bm.Set("url", webhookUrl).
				Set("event_types", eventTypes)
			createRsp, err := client.CreateWebhook(context.Background(), bm)
			if err != nil {
				// 处理创建失败
				log.Println("paypal webhook create error", err.Error())
				return
			}
			if createRsp.Code != paypal.Success {
				log.Println("paypal webhook create error", createRsp.ErrorResponse.Message)
				return
			}
			w.PluginPay.PaypalWebhookId = createRsp.Response.Id
			log.Println("paypal webhook create success", createRsp.Response.Id)
		} else {
			// 更新
			var ps = []*paypal.Patch{
				{
					Op:    "replace",
					Path:  "/url",
					Value: webhookUrl,
				},
				{
					Op:    "replace",
					Path:  "/event_types",
					Value: eventTypes,
				},
			}
			ppRsp, err := client.UpdateWebhook(context.Background(), webhookId, ps)
			if err != nil {
				xlog.Error(err)
				return
			}
			if ppRsp.Code != paypal.Success {
				xlog.Debugf("paypal webhook update error: %+v", ppRsp.Error)
				return
			}
			w.PluginPay.PaypalWebhookId = webhookId
			log.Println("paypal webhook update success", ppRsp.Response.Id)
		}
		// 保存
		err = w.SaveSettingValue(PaySettingKey, w.PluginPay)
		if err != nil {
			log.Println("paypal webhook update error", err)
			return
		}
	}
}
