package provider

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	_ "github.com/wneessen/go-mail"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

const MailLogFile = "mail.log"

const (
	SendTypeGuestbook = 1 // 新留言
	SendTypeDaily     = 2 // 网站日报
	SendTypeNewOrder  = 3 // 新订单
	SendTypePayOrder  = 4 // 新订单
	SendTypeInvoice   = 5 // 支付发票
)

type MailLog struct {
	CreatedTime int64  `json:"created_time"`
	Subject     string `json:"subject"`
	Status      string `json:"status"`
	Address     string `json:"address"`
}

var DefaultEmailTemplates = []config.EmailTemplate{
	{
		Key:         "guestbook",
		Type:        "system",
		Name:        "留言通知",
		Readonly:    false,
		Description: "当前网站有留言时，会通过邮件发送给设置的邮箱。",
		Subject:     `您的[{% system with name="SiteName" %}]网站有来自[{{guestbook.UserName}}]的新留言`,
		Content: `您好，您的网站[{% system with name="SiteName" %}]有一条新的留言，内容如下：

用户：{{guestbook.UserName}}

联系方式：{{guestbook.Contact}}

内容：{{guestbook.Content}}

{% for key, value in guestbook.ExtraData %}
{{key}}: {{value}}
{% endfor %}

时间：{{stampToDate(guestbook.CreatedTime, "2006-01-02 15:04:05")}}

IP: {{guestbook.Ip}}

来源页面: {{guestbook.Refer}}

此邮件为系统自动发送，请勿回复。`,
	},
	{
		Key:         "auto_reply",
		Type:        "user",
		Name:        "留言自动回复",
		Readonly:    false,
		Description: "当客户提交留言时，会自动回复一封邮件给客户。",
		Subject:     `[{% system with name="SiteName" %}]: Thank you for your message`,
		Content: `Hi {{user.UserName}}, 

Your message has been received. We will get back to you as soon as possible.

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "report",
		Type:        "system",
		Name:        "网站日报",
		Readonly:    true,
		Description: "每日发送网站流量等数据给设置的邮箱",
	},
	{
		Key:         "register",
		Type:        "user",
		Name:        "注册验证邮件",
		Readonly:    false,
		Description: "当用户通过邮件注册账号时，会通过邮件发送验证码或验证链接给用户。",
		Subject:     `[{% system with name="SiteName" %}]: Please verify your email address`,
		Content: `Hi {{user.UserName}}, 

Thank you for registering with {% system with name="SiteName" %}. Please click the link below to verify your email address:

{{url|safe}}

If you did not register with us, please ignore this email.

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "welcome",
		Type:        "user",
		Name:        "欢迎邮件",
		Readonly:    false,
		Description: "当用户通过邮件成功注册账号时，会通过邮件发送欢迎邮件给用户。",
		Subject:     `Welcome to {% system with name="SiteName" %}`,
		Content: `Hi {{user.UserName}}, 

Welcome to {% system with name="SiteName" %}. We are glad to have you as a member of our community.

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "new_order",
		Type:        "system",
		Name:        "订单确认通知",
		Readonly:    false,
		Description: "当客户创建新订单时，会通过邮件发送给设置的邮箱。",
		Subject:     `[{% system with name="SiteName" %}]: Your order({{order.OrderId}}) has been placed`,
		Content: `Hi {{user.UserName}}, 

Thank you for placing your order with {% system with name="SiteName" %}. Your order has been placed and is currently pending payment. To ensure timely processing and shipment of your items, please complete the payment at your earliest convenience.

Order Details:
Order ID: {{order.OrderId}}
Order Date: {{stampToDate(order.CreatedTime, "15:04:05, Jan 02, 2006")}}
Total Amount: {{priceFormat(order.Amount)}}

Please proceed to payment using the method that best suits you. If you have already completed the payment, please disregard this reminder.

We will send you another email confirmation once your payment has been successfully processed and your order begins fulfillment.

If you have any questions regarding your order or need assistance with the payment process, please don't hesitate to contact us.

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "pay_order",
		Type:        "system",
		Name:        "支付成功通知",
		Readonly:    false,
		Description: "当客户支付订单时，会通过邮件发送给设置的邮箱。",
		Subject:     `[{% system with name="SiteName" %}]: Your order({{order.OrderId}}) has been paid`,
		Content: `Hi {{user.UserName}}, 

Thank you for completing your payment. Your order has been successfully paid and is currently being processed. We will send you an email once your order has been shipped.

Order Details:
Order ID: {{order.OrderId}}
Payment Time: {{stampToDate(order.PaidTime, "15:04:05, Jan 02, 2006")}}
Total Amount: {{priceFormat(order.Amount)}}

We appreciate your business and will ensure your order is handled promptly.

If you have any questions regarding your order, please don't hesitate to contact us.

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "delivery_order",
		Type:        "user",
		Name:        "发货通知",
		Readonly:    false,
		Description: "当订单发货时，会通过邮件发送给设置的邮箱。",
		Subject:     `[{% system with name="SiteName" %}]: Your order({{order.OrderId}}) has been shipped`,
		Content: `Hi {{user.UserName}}, 

Great news! Your order has been shipped and is on its way to you.

Shipping Details:
Order ID: {{order.OrderId}}
{%- if order.TrackingNumber %}
Carrier: {{order.ExpressCompany}}
Tracking Number: {{order.TrackingNumber}}
{%- endif %}
Shipped Date: {{stampToDate(order.DeliverTime, "15:04:05, Jan 02, 2006")}}

Estimated Delivery: The package should arrive within the next few days depending on your location.

You can track your shipment using the tracking number provided above. If you have any questions regarding your shipment, please don't hesitate to contact us.

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "finish_order",
		Type:        "user",
		Name:        "交易成功通知",
		Readonly:    false,
		Description: "当客户完成订单时，会通过邮件发送给设置的邮箱。",
		Subject:     `[{% system with name="SiteName" %}]: Your order({{order.OrderId}}) has been completed`,
		Content: `Hi {{user.UserName}}, 

Your order has been marked as completed. Thank you for choosing {% system with name="SiteName" %}.

Order Summary:
Order ID: {{order.OrderId}}
Completed Date: {{stampToDate(order.FinishedTime, "15:04:05, Jan 02, 2006")}}
Total Amount: {{priceFormat(order.Amount)}}

We hope you're satisfied with your purchase. If you have any feedback about your experience or the products, we'd love to hear from you.

Don't forget to join our loyalty program to earn points for future purchases.

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "review_order",
		Type:        "user",
		Name:        "订单评价邀请",
		Readonly:    false,
		Description: "当客户完成订单时，会通过邮件发送给设置的邮箱邀请评价。",
		Delay:       259200,
		Subject:     `[{% system with name="SiteName" %}]: How was your experience with us?`,
		Content: `Hi {{user.UserName}}, 

We hope you enjoyed your recent purchase from {% system with name="SiteName" %}. We'd love to hear about your experience!

Taking a moment to share your feedback would be incredibly helpful. Your insights allow us to continue improving our products and services.

Would you mind taking a few moments to rate your experience and share any thoughts you have about:
- Product quality
- Shipping speed
- Customer service
- Overall satisfaction

Your feedback is valuable to us and helps other customers make informed decisions.

Thank you for choosing {% system with name="SiteName" %}.

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "refund_order",
		Type:        "user",
		Name:        "退款状态通知",
		Readonly:    false,
		Description: "当客户申请退款时，会通过邮件发送给设置的邮箱。",
		Subject:     `[{% system with name="SiteName" %}]: Refund Update for Order {{order.OrderId}}`,
		Content: `Hi {{user.UserName}}, 

We're writing to update you on the refund request for your order.

Refund Details:
Order ID: {{orderRefund.OrderId}}
Refund Amount: {{priceFormat(orderRefund.Amount)}}
Refund Reason: {{orderRefund.Remark}}
Request Date: {{stampToDate(orderRefund.RefundTime, "15:04:05, Jan 02, 2006")}}

Current Status: {{orderRefund.Status}}

Once approved, the refund will be processed back to your original payment method. Processing times may vary depending on your financial institution, typically taking 3-5 business days.

If you have any questions about the refund process, please feel free to contact us.

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "invoice",
		Type:        "system",
		Name:        "发票推送通知",
		Readonly:    false,
		Description: "当客户创建新订单时，会发送发票到客户邮箱。",
		Delay:       300, // 300秒后发送
		Subject:     `[{% system with name="SiteName" %}]: Invoice for Order {{order.OrderId}}`,
		Content: `Hi {{user.UserName}}, 

Please find attached the invoice for your order.

Invoice Details:
Order ID: {{order.OrderId}}
Issue Date: {{stampToDate(order.CreatedTime, "15:04:05, Jan 02, 2006")}}
Total Amount: {{priceFormat(order.Amount)}}

The invoice is attached to this email as a PDF file. If you have any questions about the invoice or need additional information, please don't hesitate to contact us.

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "recover_abandoned_checkout",
		Type:        "user",
		Name:        "未支付订单召回",
		Readonly:    false,
		Description: "在客户下单后未完成支付时，系统会发送电子邮件，以鼓励他们完成购买。",
		Delay:       36000, // 10小时后发送
		Subject:     `[{% system with name="SiteName" %}]: Complete your order - Items waiting for you!`,
		Content: `Hi {{user.UserName}}, 

We noticed you left some items in your cart without completing the purchase. Don't worry, your selected items are still reserved for you!

Order Summary:
Items: 
{% for item in cart.Items %}
- {{item.Title}} x{{item.Quantity}} ({{priceFormat(item.Price)}} each)
{% endfor %}

Total Amount: {{priceFormat(cart.Amount)}}

Complete your purchase now to secure these items before they go back into inventory. We'd hate for you to miss out on what you've selected.

[Complete Purchase Now]({{url|safe}})

If you experienced any issues during checkout, please let us know and we'll assist you immediately.

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "recover_abandoned_cart",
		Type:        "user",
		Name:        "购物车召回",
		Readonly:    false,
		Description: "当客户在购物车中添加商品后未完成支付时，系统会发送电子邮件，以鼓励他们完成购买。",
		Delay:       14400, // 4小时后发送
		Subject:     `[{% system with name="SiteName" %}]: Don't forget your items!`,
		Content: `Hi {{user.UserName}}, 

It looks like you were interested in some products but didn't complete your purchase. Your shopping cart is waiting for you with the following items:

Items in Cart:
{% for item in cart.Items %}
- {{item.Title}} x{{item.Quantity}} ({{priceFormat(item.Price)}} each)
{% endfor %}

Total: {{priceFormat(cart.Amount)}}

These items might sell out soon, so we recommend completing your purchase quickly to avoid disappointment.

[Return to Cart]({url})

We've also included a special discount code valid for the next 24 hours to thank you for your interest.

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	/*	{
				Key:         "convert_abandoned_browse",
				Type:        "user",
				Name:        "浏览未购提醒",
				Readonly:    false,
				Description: "向查看过产品但未将其添加到购物车的客户发送营销电子邮件。",
				Delay:       7200,
				Subject:     `[{% system with name="SiteName" %}]: You looked at these items recently`,
				Content: `Hi {{user.UserName}},

		We noticed you viewed some products recently but didn't add them to your cart. Here's a quick reminder of what caught your eye:

		{% for item in archives %}
		Product: {{item.Title}}
		Price: {{priceFormat(item.Price)}}
		Link: {{item.Link}}
		{% endfor %}

		These products are still available and we think they would be perfect for you based on your browsing history. Why not take another look?

		[Shop Similar Products]({{url|safe}})

		Sincerely,
		The {% system with name="SiteName" %} Team`,
			}, */
	{
		Key:         "communication_order_submit",
		Type:        "system",
		Name:        "沟通订单提交通知",
		Readonly:    false,
		Description: "当客户提交沟通订单时，会通过邮件发送给设置的邮箱。",
		Subject:     `New customization on {% system with name="SiteName" %} submitted`,
		Content: `Hi {{user.UserName}}, 

Thank you for submitting the customization. We will review the design and get back to you during our working hours as soon as possible. We will keep you informed of any updates via email.

Have any questions? Please leave messages on your customization page but not this email. Thank you!

This is the permanent link of the customization - {{url|safe}}

What can you do on the customization page?

- Leave more messages to communicate with your stylist
- Check the fabric and color, provide your size\nPlace an order after the price is set
- Check the finished photos before shipment
- Write a review

Have questions? Reply to this email or message us on your order page. We're happy to help!

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "communication_order_quote",
		Type:        "user",
		Name:        "沟通订单报价通知",
		Readonly:    false,
		Description: "当沟通订单给客户发送报价时，会通过邮件发送给客户。",
		Subject:     `Your Quote from {% system with name="SiteName" %} is Ready`,
		Content: `Hi {{user.UserName}}, 

Your customization quote is now available!

Quote Summary:

{{priceFormat(order.QuotePrice)}}

Total: {{priceFormat(order.QuotePrice)}}

Valid until: {{stampToDate(order.QuoteExpire, "January 02, 2006")}}

View the complete quote details and accept it here:
{{url|safe}}

Have questions? Reply to this email or message us on your customization page. We're happy to help!

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "communication_order_reply",
		Type:        "user",
		Name:        "沟通订单回复通知",
		Readonly:    false,
		Description: "当沟通订单给客户发送回复时，会通过邮件发送给客户。",
		Subject:     `You Have New Message about Customization from {% system with name="SiteName" %}`,
		Content: `Hi {{user.UserName}}, 

Your customization received a new message!

Message:

{{message.Content}}

View the complete details here:
{{url|safe}}

Have questions? Reply to this email or message us on your customization page. We're happy to help!

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	/*	{
				Key:         "recommendation",
				Type:        "user",
				Name:        "个性化推荐邮件",
				Readonly:    false,
				Description: "不定期根据用户浏览过的产品，向用户发送个性化推荐邮件。",
				Subject:     `[{% system with name="SiteName" %}]: Recommended for you`,
				Content: `Hi {{user.UserName}},

		Based on your previous browsing and purchase history, we think you'll love these recommendations:

		{% for item in archives %}
		Product: {{item.Title}}
		Price: {{priceFormat(item.Price)}}
		Rating: 4/5 stars
		Link: {{item.Link}}
		{% endfor %}

		Special Offer: Use code for 20% off your next purchase.

		Check out these personalized picks just for you!

		Sincerely,
		The {% system with name="SiteName" %} Team`,
			}, */
	{
		Key:         "newsletter_confirm",
		Type:        "user",
		Name:        "邮件订阅确认",
		Readonly:    false,
		Description: "当用户订阅邮件时，会通过邮件发送确认邮件给用户。",
		Subject:     `[{% system with name="SiteName" %}]: Welcome to our newsletter!`,
		Content: `Hi {{user.Email}}, 

Thank you for subscribing to our newsletter. You're now part of our exclusive community where you'll receive:

- Early access to sales and promotions
- New product announcements
- Special member-only discounts
- Valuable tips and industry insights

You'll start receiving emails from us shortly. We promise to only send you relevant and valuable content.

If you ever wish to unsubscribe, simply click the 'Unsubscribe' link at the bottom of any email.

Welcome aboard!

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "group_change",
		Type:        "user",
		Name:        "会员等级变动通知",
		Readonly:    false,
		Description: "当用户等级发生变化时，会通过邮件发送给用户。",
		Subject:     `[{% system with name="SiteName" %}]: Congratulations! Your membership level has been upgraded`,
		Content: `Hi {{user.UserName}}, 

Congratulations! We're excited to inform you that your membership level has been upgraded from {{oldGroup.Title}} to {{user.Group.Title}}.

As a {{user.Group.Title}} member, you now enjoy additional benefits including:
- Exclusive discounts
- Priority customer support
- Early access to new products
- Free shipping on orders

Thank you for your continued loyalty. We look forward to serving you better with your new status.

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	/*	{
				Key:         "group_will_expire",
				Type:        "user",
				Name:        "会员即将过期提醒",
				Readonly:    false,
				Description: "当会员即将到期时，会通过邮件发送给用户。",
				Subject:     `[{% system with name="SiteName" %}]: Your membership is expiring soon`,
				Content: `Hi {{user.UserName}},

		Your {{user.Group.Title}} membership is scheduled to expire in {{stampToDate(user.ExpireTime, "friendly")}} days. After expiration, you'll lose access to your member benefits including:

		- Exclusive discounts
		- Priority customer support
		- Free shipping benefits
		- Member-only products

		Renew your membership now to continue enjoying these valuable perks. As a valued member, we're offering you a special renewal discount.

		[Renew Membership Now]({{url|safe}})

		If you have any questions about your membership, please contact our support team.

		Sincerely,
		The {% system with name="SiteName" %} Team`,
			},
			{
				Key:         "group_expired",
				Type:        "user",
				Name:        "会员已过期提醒",
				Readonly:    false,
				Description: "当会员已到期时，会通过邮件发送给用户。",
				Subject:     `[{% system with name="SiteName" %}]: Your membership has expired`,
				Content: `Hi {{user.UserName}},

		Your {{oldGroup.Title}} membership has expired. You no longer have access to your member benefits such as:

		- Exclusive discounts
		- Priority customer support
		- Free shipping benefits
		- Member-only products

		Renew your membership today to regain all your valuable perks and continue enjoying the benefits of being part of our premium membership program.

		[Renew Membership Now]({{url|safe}})

		We hope to welcome you back as an active member soon.

		Sincerely,
		The {% system with name="SiteName" %} Team`,
			},
			{
				Key:         "points_change",
				Type:        "user",
				Name:        "会员积分变动邮件",
				Readonly:    false,
				Description: "当用户积分发生变化时，会通过邮件发送给用户。",
				Subject:     `[{% system with name="SiteName" %}]: Your points balance has changed`,
				Content: `Hi {{user.UserName}},

		Your points balance has been updated:

		Previous Balance: {{integralLog.AfterIntegral-integralLog.Integral}} points
		Change: {{integralLog.Integral}} points
		New Balance: {{integralLog.AfterIntegral}} points
		Transaction Date: {{stampToDate(integralLog.CreatedTime, "15:04:05, Jan 02, 2006")}}
		Reason: {{user.Remark}}

		You can use your points for discounts on future purchases. Visit your account to see how to redeem your points.

		Sincerely,
		The {% system with name="SiteName" %} Team`,
			},
			{
				Key:         "birthday",
				Type:        "user",
				Name:        "会员生日祝福",
				Readonly:    false,
				Description: "当用户生日时，会通过邮件发送给用户。",
				Subject:     `Happy Birthday, {{user.UserName}}!`,
				Content: `Hi {{user.UserName}},

		Happy Birthday! 🎉

		On this special day, we want to celebrate you and thank you for being a valued member of our community. As a birthday gift, we're offering you a special discount on your next purchase.

		Birthday Gift: 20% off your entire order

		Use this code at checkout to enjoy your special birthday savings. Treat yourself to something special!

		Wishing you a wonderful day filled with joy and happiness.

		Warmest regards,
		The {% system with name="SiteName" %} Team`,
			},
			{
				Key:         "promotion",
				Type:        "user",
				Name:        "促销活动预告",
				Readonly:    false,
				Description: "当促销活动即将开始时，会通过邮件发送给用户。",
				Subject:     `[{% system with name="SiteName" %}]: Big Sale Starts Tomorrow!`,
				Content: `Hi {{user.UserName}},

		Get ready for our upcoming promotion! Starting tomorrow, enjoy incredible savings on your favorite items:

		Special Offers:
		{% for item in archives %}
		- {{item.Title}}: {{priceFormat(item.Price)}}
		{% endfor %}

		Exclusive for Members: As a valued member, you get early access to these deals starting 24 hours before the general public.

		[Shop Early Access]({{url|safe}})

		Don't miss out on these limited-time offers!

		Sincerely,
		The {% system with name="SiteName" %} Team`,
			}, */
	{
		Key:         "repurchase_promotion",
		Type:        "user",
		Name:        "首次购物后回购营销",
		Readonly:    false,
		Description: "当用户首次购物一定时间后，会通过邮件发送给用户。",
		Delay:       1209600, // 14天后发送
		Subject:     `[{% system with name="SiteName" %}]: We Miss You! Special Offer Inside`,
		Content: `Hi {{user.UserName}}, 

It's been a while since your first purchase with us, and we miss having you as a customer! We hope you're enjoying your products.

As a returning customer, we'd love to offer you an incentive to shop with us again:

Special Offers:
{% archiveList archives with moduleId=2 limit=5 order="views desc" %}
{% for item in archives %}
- {{item.Title}}: {{priceFormat(item.Price)}}
{% endfor %}
{% endarchiveList %}

We've also added new products that we think you'll love based on your previous purchase history.

[Shop New Arrivals]({{url|safe}})

Thank you for being part of our community. We hope to serve you again soon!

Sincerely,
The {% system with name="SiteName" %} Team`,
	},
	{
		Key:         "recover_old_user",
		Type:        "user",
		Name:        "流失用户召回",
		Readonly:    false,
		Description: "用户长时间未发生互动时，系统会发送邮件给用户。",
		Delay:       2592000,
		Subject:     `[{% system with name="SiteName" %}]: We've Saved Something Special For You`,
		Content: `Hi {{user.UserName}}, 

We noticed it's been quite a while since your last visit, and we wanted to reach out to say we miss you!

To welcome you back, we've prepared an exclusive offer just for you:

Special Offers:
{% archiveList archives with moduleId=2 limit=5 order="views desc" %}
{% for item in archives %}
- {{item.Title}}: {{priceFormat(item.Price)}}
{% endfor %}
{% endarchiveList %}

Since you were last here, we've added many exciting new products and features that we think you'll love.

[Explore New Products]({{url|safe}})

Your feedback and presence mean a lot to us. We hope to see you again soon!

With appreciation,
The {% system with name="SiteName" %} Team`,
	},
}

func (w *Website) GetEmailTemplates() []config.EmailTemplate {
	templates := append([]config.EmailTemplate{}, DefaultEmailTemplates...)
	for i := range templates {
		for _, v := range w.PluginSendmail.Templates {
			if v.Key == templates[i].Key {
				templates[i].Open = v.Open
				if v.Subject != "" {
					templates[i].Subject = v.Subject
				}
				if v.Content != "" {
					templates[i].Content = v.Content
				}
				if v.Delay > 0 {
					templates[i].Delay = v.Delay
				}
				break
			}
		}
	}

	return templates
}

// 获取合并后的
func (w *Website) GetEmailTemplateInfo(key string) (*config.EmailTemplate, bool) {
	var exist bool
	var template config.EmailTemplate
	for _, v := range DefaultEmailTemplates {
		if v.Key == key {
			template = v
			exist = true
			break
		}
	}
	for _, v := range w.PluginSendmail.Templates {
		if v.Key == key {
			template.Open = v.Open
			if !template.Readonly {
				if v.Subject != "" {
					template.Subject = v.Subject
				}
				if v.Content != "" {
					template.Content = v.Content
				}
				if v.Delay > 0 {
					template.Delay = v.Delay
				}
			}
			break
		}
	}

	return &template, exist
}

func (w *Website) SaveEmailTemplateInfo(req *config.EmailTemplate) error {
	template := config.EmailTemplate{
		Open:    req.Open,
		Key:     req.Key,
		Delay:   req.Delay,
		Subject: req.Subject,
		Content: req.Content,
	}
	var exist bool
	for i, v := range w.PluginSendmail.Templates {
		if v.Key == req.Key {
			w.PluginSendmail.Templates[i] = template
			exist = true
			break
		}
	}
	if !exist {
		w.PluginSendmail.Templates = append(w.PluginSendmail.Templates, template)
	}

	// 保存到数据库
	return w.SaveSettingValue(SendmailSettingKey, w.PluginSendmail)
}

func (w *Website) RenderEmailTemplate(template *config.EmailTemplate, data map[string]interface{}) (*config.EmailTemplate, error) {
	var err error
	var err2 error
	template.Subject, err2 = pongo2.RenderTemplateString(template.Subject, data)
	if err2 != nil {
		// 渲染失败
		err = err2
		template.Subject = err.Error()
	}
	// render content
	// content先从Markdown转换为HTML
	template.Content, err2 = pongo2.RenderTemplateString(template.Content, data)
	if err2 != nil {
		// 渲染失败败
		err = err2
		template.Content = err.Error()
	}
	if !strings.HasPrefix(template.Content, "<") {
		template.Content = library.MarkdownToHTML(template.Content)
	}

	return template, err
}

func (w *Website) GetLastSendmailList(currentPage, pageSize int) ([]*MailLog, int64) {
	offset := (currentPage - 1) * pageSize
	var total int64

	var mailLogs []*MailLog
	//获取20条数据
	filePath := w.CachePath + MailLogFile
	logFile, err := os.Open(filePath)
	if nil != err {
		//打开失败
		return mailLogs, 0
	}
	defer logFile.Close()
	var fileSize int64
	// 获取文件大小
	if fileInfo, err := logFile.Stat(); err == nil {
		fileSize = fileInfo.Size()
	} else {
		return nil, 0
	}

	reader := bufio.NewReader(logFile)
	// 每次读取8K
	buffer := make([]byte, 8192)
	lineBuffer := ""

	// 按limit行读取
	var lines = make([]string, 0, pageSize)
	var curPos = fileSize
	var curLine = 0

	// 倒序读取文件内容
	for curPos > 0 {
		// 定位到需要读取的位置
		bytesToRead := int64(len(buffer))
		if curPos-bytesToRead < 0 {
			bytesToRead = curPos
		}

		// 调整文件指针到合适位置
		curPos -= bytesToRead
		logFile.Seek(curPos, io.SeekStart)

		// 读取文件数据到缓冲区
		n, err := reader.Read(buffer[:bytesToRead])
		if err != nil && err != io.EOF {
			return nil, 0
		}

		// 将读到的数据加入行缓冲
		lineBuffer = string(buffer[:n]) + lineBuffer

		// 处理行
		for {
			newLineIdx := len(lineBuffer) - 1
			for newLineIdx >= 0 && lineBuffer[newLineIdx] != '\n' {
				newLineIdx--
			}

			// 找到完整的行
			if newLineIdx == -1 {
				break
			}

			line := lineBuffer[newLineIdx+1:]
			lineBuffer = lineBuffer[:newLineIdx]
			if line != "" {
				curLine++
				if curLine <= offset {
					continue
				}
				lines = append(lines, line)

				// 如果已经获取到需要的行数，跳出循环
				if len(lines) >= pageSize {
					break
				}
			}
		}

		// 处理剩余内容
		if curPos == 0 && lineBuffer != "" && len(lines) < pageSize {
			lines = append(lines, lineBuffer)
		}
		// 如果已经获取到需要的行数，跳出循环
		if len(lines) >= pageSize {
			break
		}
	}
	for _, line := range lines {
		var mailLog MailLog
		err := json.Unmarshal([]byte(line), &mailLog)
		if err == nil {
			mailLogs = append(mailLogs, &mailLog)
		}
	}

	// 如果文件计数缓存，则重新计数
	err = w.Cache.Get("send-mail-total", &total)
	if err != nil {
		logFile.Seek(0, io.SeekStart)
		sc := bufio.NewScanner(reader)
		for sc.Scan() {
			total++
		}
		// 如果是当天的文件，则不缓存
		w.Cache.Set("send-mail-total", total, 300)
	}
	if pageSize > len(mailLogs) {
		total = int64(offset + len(mailLogs))
	}
	if total < int64(len(mailLogs)) {
		total = int64(len(mailLogs))
	}

	return mailLogs, total
}

func (w *Website) SendInvoiceMail(user *model.User, order *model.Order, invoicePath string) error {
	if user == nil || user.Email == "" || w.PluginSendmail.UseBrevo == true {
		return errors.New("no user")
	}
	template, exist := w.GetEmailTemplateInfo("invoice")
	if !exist || !template.Open {
		return errors.New("no template")
	}

	data := map[string]interface{}{
		"website": w,
		"order":   order,
		"user":    user,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	pdfBuf, err := os.ReadFile(w.DataPath + invoicePath)
	if err != nil {
		return err
	}
	attachment := library.Attachment{
		Filename: filepath.Base(invoicePath),
		Header: textproto.MIMEHeader{
			"Content-Disposition": []string{fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(invoicePath))},
			"Content-Type":        []string{"application/pdf"},
		},
		Content: pdfBuf,
	}

	// 发给用户
	w.SendMail(template.Subject, template.Content, &attachment, user.Email)

	return nil
}

func (w *Website) SendMail(subject, content string, attach *library.Attachment, recipients ...string) error {
	setting := w.PluginSendmail
	if setting.Account == "" {
		//成功配置，则跳过
		return errors.New(w.Tr("PleaseConfigureSender"))
	}
	userHtml := false
	if strings.HasPrefix(content, "<") {
		userHtml = true
	}
	err := w.sendMail(subject, content, nil, recipients, userHtml, true)

	return err
}

func (w *Website) sendMail(subject, content string, attachments []*library.Attachment, recipients []string, useHtml bool, setLog bool) error {
	setting := w.PluginSendmail
	port := setting.Port
	if port == 0 {
		//默认使用25端口
		port = 25
	}
	if setting.UseSSL == 1 && port == 25 {
		//如果使用ssl，设置了25端口，则使用465
		port = 465
	}

	if setting.Account == "" {
		//成功配置，则跳过
		return errors.New(w.Tr("PleaseConfigureSender"))
	}

	//开始发送
	email := library.NewEMail(`{"port":25}`)
	email.From = setting.Account
	email.Host = setting.Server
	email.Port = setting.Port
	email.Username = setting.Account
	if setting.UseSSL == 1 {
		email.Secure = "SSL"
	}
	email.Password = setting.Password
	for _, attach := range attachments {
		email.Attach(bytes.NewReader(attach.Content), attach.Filename, attach.Header.Get("Content-Type"))
	}

	if len(recipients) == 0 {
		if setting.Recipient != "" {
			tmp := strings.Split(setting.Recipient, ",")
			for _, v := range tmp {
				v = strings.TrimSpace(v)
				if v != "" {
					recipients = append(recipients, v)
				}
			}
		}
		if len(recipients) == 0 {
			recipients = append(recipients, setting.Account)
		}
	}
	// 多个收件地址的时候，分开发送
	var err error
	for _, to := range recipients {
		email.To = []string{to}
		email.Subject = subject
		if useHtml {
			email.HTML = content
		} else {
			email.Text = content
		}

		if err = email.Send(); err != nil {
			if setLog {
				w.logMailError(to, subject, err.Error())
			}
			continue
		}
		if setLog {
			w.logMailError(to, subject, w.Tr("SentSuccessfully"))
		}
	}
	return err
}

func (w *Website) logMailError(address, subject, status string) {
	mailLog := MailLog{
		CreatedTime: time.Now().Unix(),
		Subject:     subject,
		Status:      status,
		Address:     address,
	}

	content, err := json.Marshal(mailLog)

	if err == nil {
		library.DebugLog(w.CachePath, MailLogFile, string(content))
	}
}

func (w *Website) SendVerifyEmail(user *model.User, state string) error {
	// 是否需要邮箱验证
	template, exist := w.GetEmailTemplateInfo("register")
	if !exist || !template.Open {
		return errors.New("no template")
	}

	token := library.Md5(user.Email + user.Password)
	verifyCode := library.CodeCache.Generate(token)
	verifyUrl := w.System.BaseUrl + "/api/verify/email?token=" + token + "&code=" + verifyCode + "&state=" + state + "&email=" + user.Email
	data := map[string]interface{}{
		"website": w,
		"user":    user,
		"code":    verifyCode,
		"url":     verifyUrl,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	return w.SendMail(template.Subject, template.Content, nil, user.Email)
}

func (w *Website) SendCommunicationSummitedEmail(order *model.CommunicationOrder) error {
	user, err := w.GetUserInfoById(uint(order.UserId))
	if err != nil || user.Email != "" {
		return errors.New("no user")
	}

	template, exist := w.GetEmailTemplateInfo("communication_order_submit")
	if !exist || !template.Open {
		return errors.New("no template")
	}

	order.QuoteExpire = time.Now().AddDate(0, 0, 7).Unix()
	frontUrl := w.System.BaseUrl
	if w.System.FrontUrl != "" {
		frontUrl = w.System.FrontUrl
	}
	jumpUrl := frontUrl + "/order/communication?id=" + strconv.FormatInt(order.Id, 10)
	data := map[string]interface{}{
		"website": w,
		"user":    user,
		"url":     jumpUrl,
		"order":   order,
	}

	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	return w.SendMail(template.Subject, template.Content, nil, user.Email)
}

func (w *Website) SendCommunicationQuoteEmail(order *model.CommunicationOrder) error {
	user, err := w.GetUserInfoById(uint(order.UserId))
	if err != nil || user.Email != "" {
		return errors.New("no user")
	}

	template, exist := w.GetEmailTemplateInfo("communication_order_quote")
	if !exist || !template.Open {
		return errors.New("no template")
	}

	order.QuoteExpire = time.Now().AddDate(0, 0, 7).Unix()
	frontUrl := w.System.BaseUrl
	if w.System.FrontUrl != "" {
		frontUrl = w.System.FrontUrl
	}
	jumpUrl := frontUrl + "/order/communication?id=" + strconv.FormatInt(order.Id, 10)
	data := map[string]interface{}{
		"website": w,
		"user":    user,
		"url":     jumpUrl,
		"order":   order,
	}

	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	return w.SendMail(template.Subject, template.Content, nil, user.Email)
}

// SendCommunicationNewMessageEmail send new message email to customer
func (w *Website) SendCommunicationNewMessageEmail(message *model.CommunicationMessage) error {
	order, err := w.GetCommunicationOrderById(message.CommOrderId)
	if err != nil {
		return err
	}
	user, err := w.GetUserInfoById(uint(order.UserId))
	if err != nil || user.Email != "" {
		return errors.New("no user")
	}

	template, exist := w.GetEmailTemplateInfo("communication_order_reply")
	if !exist || !template.Open {
		return errors.New("no template")
	}
	frontUrl := w.System.BaseUrl
	if w.System.FrontUrl != "" {
		frontUrl = w.System.FrontUrl
	}
	jumpUrl := frontUrl + "/order/communication?id=" + strconv.FormatInt(order.Id, 10)
	data := map[string]interface{}{
		"website": w,
		"user":    user,
		"url":     jumpUrl,
		"order":   order,
		"message": message,
	}

	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	return w.SendMail(template.Subject, template.Content, nil, user.Email)
}

func (w *Website) SendWelcomeEmail(user *model.User) error {
	if user.Email == "" {
		return errors.New("no email")
	}
	template, exist := w.GetEmailTemplateInfo("welcome")
	if !exist || !template.Open {
		return errors.New("no template")
	}

	data := map[string]interface{}{
		"website": w,
		"user":    user,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	return w.SendMail(template.Subject, template.Content, nil, user.Email)
}

func (w *Website) SendNewSubscriberEmail(subscriber *model.Subscriber) error {
	if subscriber.Email == "" {
		return errors.New("no email")
	}
	template, exist := w.GetEmailTemplateInfo("newsletter_confirm")
	if !exist || !template.Open {
		return errors.New("no template")
	}

	data := map[string]interface{}{
		"website": w,
		"user":    subscriber,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	return w.SendMail(template.Subject, template.Content, nil, subscriber.Email)
}

func (w *Website) SendUserGroupChangeEmail(user *model.User, oldGroup *model.UserGroup) error {
	if user.Email == "" {
		return errors.New("no email")
	}
	template, exist := w.GetEmailTemplateInfo("group_change")
	if !exist || !template.Open || w.PluginSendmail.UseBrevo == true {
		return errors.New("no template")
	}
	data := map[string]interface{}{
		"website":  w,
		"oldGroup": oldGroup,
		"user":     user,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	w.SendMail(template.Subject, template.Content, nil, user.Email)

	return nil
}

func (w *Website) SendNewOrderEmail(user *model.User, order *model.Order) error {
	if user == nil {
		user = &model.User{}
	}
	template, exist := w.GetEmailTemplateInfo("new_order")
	if !exist {
		return errors.New("no template")
	}

	data := map[string]interface{}{
		"website": w,
		"order":   order,
		"user":    user,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	// 开启了 brevo 则不发送给用户，因此要判断 UseBrevo == false 才发送
	if user.Email != "" && template.Open && w.PluginSendmail.UseBrevo == false {
		// 发给用户
		w.SendMail(template.Subject, template.Content, nil, user.Email)
	}
	// 发给网站管理员
	_ = w.sendMail(template.Subject, template.Content, nil, nil, false, false)

	return nil
}

func (w *Website) SendPayOrderEmail(user *model.User, order *model.Order) error {
	if user == nil {
		user = &model.User{}
	}
	template, exist := w.GetEmailTemplateInfo("pay_order")
	if !exist {
		return errors.New("no template")
	}

	data := map[string]interface{}{
		"website": w,
		"order":   order,
		"user":    user,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	// 开启了 brevo 则不发送给用户，因此要判断 UseBrevo == false 才发送
	if user.Email != "" && template.Open && w.PluginSendmail.UseBrevo == false {
		// 发给用户
		w.SendMail(template.Subject, template.Content, nil, user.Email)
	}
	// 发给网站管理员
	_ = w.sendMail(template.Subject, template.Content, nil, nil, false, false)

	return nil
}

func (w *Website) SendDeliveryOrderEmail(user *model.User, order *model.Order) error {
	if user == nil || user.Email == "" || w.PluginSendmail.UseBrevo == true {
		return errors.New("no user")
	}
	template, exist := w.GetEmailTemplateInfo("delivery_order")
	if !exist || !template.Open {
		return errors.New("no template")
	}

	data := map[string]interface{}{
		"website": w,
		"order":   order,
		"user":    user,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	// 发给用户
	w.SendMail(template.Subject, template.Content, nil, user.Email)

	return nil
}

func (w *Website) SendCompletedOrderEmail(user *model.User, order *model.Order) error {
	if user == nil || user.Email == "" || w.PluginSendmail.UseBrevo == true {
		return errors.New("no user")
	}
	template, exist := w.GetEmailTemplateInfo("finish_order")
	if !exist || !template.Open {
		return errors.New("no template")
	}

	data := map[string]interface{}{
		"website": w,
		"order":   order,
		"user":    user,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	// 发给用户
	w.SendMail(template.Subject, template.Content, nil, user.Email)

	return nil
}

func (w *Website) SendRefundOrderEmail(user *model.User, order *model.Order) error {
	if user == nil || user.Email == "" || w.PluginSendmail.UseBrevo == true {
		return errors.New("no user")
	}
	template, exist := w.GetEmailTemplateInfo("refund_order")
	if !exist || !template.Open {
		return errors.New("no template")
	}

	data := map[string]interface{}{
		"website": w,
		"order":   order,
		"user":    user,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	// 发给用户
	w.SendMail(template.Subject, template.Content, nil, user.Email)

	return nil
}

func (w *Website) SendReviewOrderPlan(user *model.User, order *model.Order) error {
	if user == nil || user.Email == "" || w.PluginSendmail.UseBrevo == true {
		return errors.New("no user")
	}
	template, exist := w.GetEmailTemplateInfo("review_order")
	if !exist || !template.Open {
		return errors.New("no template")
	}

	data := map[string]interface{}{
		"website": w,
		"order":   order,
		"user":    user,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	// 添加到邮件计划
	plan := model.MailPlan{
		Identifier: "order_review_" + order.OrderId,
		Recipients: []string{user.Email},
		Subject:    template.Subject,
		Content:    template.Content,
		SendTime:   time.Now().Add(time.Second * time.Duration(template.Delay)).Unix(),
	}
	err = w.AddMailPlan(&plan)

	return nil
}

func (w *Website) SendRecoverAbandonedOrderPlan(user *model.User, order *model.Order) error {
	if user == nil || user.Email == "" || w.PluginSendmail.UseBrevo == true {
		return errors.New("no user")
	}
	template, exist := w.GetEmailTemplateInfo("recover_abandoned_checkout")
	if !exist || !template.Open {
		return errors.New("no template")
	}

	data := map[string]interface{}{
		"website": w,
		"order":   order,
		"user":    user,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	// 添加到邮件计划
	plan := model.MailPlan{
		Identifier: "recover_abandoned_checkout_" + order.OrderId,
		Recipients: []string{user.Email},
		Subject:    template.Subject,
		Content:    template.Content,
		SendTime:   time.Now().Add(time.Second * time.Duration(template.Delay)).Unix(),
	}
	err = w.AddMailPlan(&plan)

	return nil
}

func (w *Website) AddMailPlan(plan *model.MailPlan) error {
	// 如果没开启邮件服务，则不添加计划
	if w.PluginSendmail.Account == "" {
		//成功配置，则跳过
		return errors.New(w.Tr("PleaseConfigureSender"))
	}
	err := w.DB.Create(plan).Error
	if err == nil {
		// 添加到计划
		if w.MailService != nil {
			w.MailService.Add(plan)
		}
	}
	return err
}

func (w *Website) CancelMailPlan(identifier string, isDel bool) error {
	// 删除计划
	var plan model.MailPlan
	err := w.DB.Where("`identifier` = ?", identifier).First(&plan).Error
	if err == nil && plan.Status != 2 {
		if isDel {
			return w.DB.Where("`identifier` = ?", identifier).Delete(&model.MailPlan{}).Error
		} else {
			w.DB.Model(&model.MailPlan{}).Where("`identifier` = ?", identifier).UpdateColumn("status", -1)
		}
	}
	return nil
}
