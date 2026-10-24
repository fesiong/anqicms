package provider

import (
	"context"
	"fmt"
	"time"

	brevo "github.com/getbrevo/brevo-go/lib"
	"kandaoni.com/anqicms/model"
)

// Brevo event list
// - product_view 浏览商品
// -
const (
	EventProductViewd   = "product_viewed"
	EventCartUpdated    = "cart_updated"
	EventOrderCreated   = "order_created"
	EventOrderCheckout  = "order_checkout"
	EventOrderPaid      = "order_paid"
	EventOrderCompleted = "order_completed"
	EventOrderRefund    = "order_refund"
	EventOrderDelivered = "order_delivered"
)

type Brevo struct {
	w      *Website
	ApiKey string `json:"api_key"`
	Client *brevo.APIClient
}

func (w *Website) BrevoClient(apiKey string) *Brevo {
	if apiKey == "" {
		apiKey = w.PluginSendmail.BrevoApiKey
	}
	if apiKey == "" {
		return nil
	}

	cfg := brevo.NewConfiguration()
	//Configure API key authorization: api-key
	cfg.AddDefaultHeader("api-key", apiKey)
	//Configure API key authorization: partner-key
	//cfg.AddDefaultHeader("partner-key","YOUR_API_KEY")

	br := brevo.NewAPIClient(cfg)

	if w.PluginSendmail.BrevoApiKey == "" {
		// 验证是否正确
		var ctx context.Context
		result, _, err := br.AccountApi.GetAccount(ctx)
		if err != nil {
			fmt.Println("Error when calling AccountApi->get_account: ", err.Error())
			return nil
		}
		fmt.Println("Account: ", result.Email)
	}

	return &Brevo{
		w:      w,
		ApiKey: apiKey,
		Client: br,
	}
}

func (b *Brevo) CreateContacts(focus bool) error {
	var users []*model.User
	tx := b.w.DB.Where("status = ?", 1).Order("id ASC")
	if focus {
		// 获取全部
	} else {
		// 跳过已同步的
		tx = tx.Where("brevo_id = ?", 0)
	}
	limit := 1000
	lastId := uint(0)
	for {
		tx.WithContext(context.Background()).Where("id > ?", lastId).Limit(limit).Find(&users)
		if len(users) == 0 {
			break
		}
		lastId = users[len(users)-1].Id

		for i := range users {
			b.CreateContact(users[i])
		}
	}

	return nil
}

func (b *Brevo) CreateContact(user *model.User) error {
	if user.Email == "" && user.Phone == "" {
		return nil
	}
	attribute := map[string]interface{}{}
	if user.FirstName != "" {
		attribute["FIRSTNAME"] = user.FirstName
		attribute["LASTNAME"] = user.LastName
	}
	if user.Phone != "" {
		attribute["SMS"] = user.Phone
	}
	if user.Birthday > 0 {
		attribute["BIRTHDAY"] = time.Unix(user.Birthday, 0).Format(time.RFC3339)
	}
	attribute["POINTS"] = 0
	attribute["GROUPID"] = user.GroupId
	if user.ExpireTime > 0 {
		attribute["EXPIREDATE"] = time.Unix(user.ExpireTime, 0).Format(time.RFC3339)
	}

	var ctx context.Context

	createContact := brevo.CreateContact{
		Attributes:    attribute,
		ExtId:         fmt.Sprint(user.Id),
		UpdateEnabled: true,
	}
	if user.Email != "" {
		createContact.Email = user.Email
	}
	result, resp, err := b.Client.ContactsApi.CreateContact(ctx, createContact)
	if err != nil {
		fmt.Println("Error when calling ContactsApi->create_contact: ", err.Error())
		fmt.Println("Response: ", resp)
		return err
	}
	fmt.Println("Status code: ", resp.StatusCode)
	fmt.Println("Response: ", result)

	return nil
}

// CreateEvent 异步执行
func (w *Website) CreateEvent(eventName string, eventProperties map[string]interface{}, user *model.User) {
	if user == nil || user.Id == 0 || w.PluginSendmail.UseBrevo == false {
		return
	}
	client := w.BrevoClient("")
	if client == nil {
		return
	}
	go func() {
		event := brevo.Event{
			EventName:       eventName,
			EventProperties: eventProperties,
			Identifiers: &brevo.EventIdentifiers{
				EmailId: user.Email,
				PhoneId: user.Phone,
				ExtId:   fmt.Sprint(user.Id),
			},
		}
		var ctx context.Context
		resp, err := client.Client.EventsApi.CreateEvent(ctx, event)
		if err != nil {
			fmt.Println("Error when calling EventsApi->create_event: ", err.Error())
			fmt.Println("Response: ", resp)
			return
		}
		fmt.Println("Status code: ", resp.StatusCode)
	}()
	return
}

func (w *Website) SyncBrevo() {
	if w.PluginSendmail.UseBrevo == false || w.PluginSendmail.BrevoApiKey == "" {
		return
	}
	// 同步的数据包括：
	// - 用户

	// 同步用户
	client := w.BrevoClient("")
	if client == nil {
		return
	}
	client.CreateContacts(false)
}
