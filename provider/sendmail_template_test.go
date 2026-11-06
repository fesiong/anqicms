package provider_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/flosch/pongo2/v6"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/tags"
)

func init() {
	if err := pongo2.RegisterTag("system", tags.TagSystemParser); err != nil {
		panic(err)
	}
}

// Missing function registration must break both saved function-style templates
// and defaults; filter-style templates must remain compatible.
func TestEmailTemplateFormatting(t *testing.T) {
	w := &provider.Website{System: &config.SystemConfig{SiteName: "Test site"}}
	data := map[string]interface{}{"amount": int64(12345), "stamp": int64(1704067200)}
	for _, source := range []string{
		`{{priceFormat(amount)}}|{{stampToDate(stamp, "2006")}}`,
		`{{amount|priceFormat}}|{{stamp|dateFormat:"2006"}}`,
	} {
		tmpl := &config.EmailTemplate{Subject: source, Content: source}
		got, err := w.RenderEmailTemplate(tmpl, data)
		if err != nil {
			t.Fatal(err)
		}
		if got.Subject != "123.45|2024" || !strings.Contains(got.Content, "123.45|2024") {
			t.Errorf("source %q: got subject %q, content %q", source, got.Subject, got.Content)
		}
	}
	if len(data) != 2 {
		t.Fatalf("render mutated input: %#v", data)
	}
}

// A malformed subject or body must return an error, not crash the caller.
func TestEmailTemplateInvalidSyntax(t *testing.T) {
	for _, field := range []string{"subject", "content"} {
		t.Run(field, func(t *testing.T) {
			defer func() {
				if p := recover(); p != nil {
					t.Errorf("render panicked: %v", p)
				}
			}()
			tmpl := &config.EmailTemplate{Subject: "valid", Content: "valid"}
			if field == "subject" {
				tmpl.Subject = "{% if %}"
			} else {
				tmpl.Content = "{% if %}"
			}
			if _, err := (&provider.Website{System: &config.SystemConfig{SiteName: "Test site"}}).RenderEmailTemplate(tmpl, nil); err == nil {
				t.Fatal("invalid syntax accepted")
			}
		})
	}
}

// Whitespace before HTML must not send it through Markdown conversion.
func TestEmailTemplateLeadingWhitespaceHTML(t *testing.T) {
	source := " \n\t<div>\nplain\n</div>"
	got, err := (&provider.Website{System: &config.SystemConfig{SiteName: "Test site"}}).RenderEmailTemplate(&config.EmailTemplate{Content: source}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != source {
		t.Fatalf("HTML changed: %q", got.Content)
	}
}

func emailDefault(t *testing.T, key string) config.EmailTemplate {
	t.Helper()
	for _, tmpl := range provider.DefaultEmailTemplates {
		if tmpl.Key == key {
			return tmpl
		}
	}
	t.Fatalf("missing default %s", key)
	return config.EmailTemplate{}
}

// Reading cart.Items/cart.Amount instead of the actual order must lose these
// independently specified details, and a literal {url} must lose the link.
func TestEmailTemplateRecoveryDefaults(t *testing.T) {
	w := &provider.Website{System: &config.SystemConfig{SiteName: "Test site"}}
	order := &model.Order{OrderId: "ORDER-42", Amount: 24690, Details: []*model.OrderDetail{{Goods: &model.Archive{Title: "Fixture product"}, Quantity: 2, Price: 12345}}}
	for _, key := range []string{"recover_abandoned_checkout", "recover_abandoned_cart"} {
		t.Run(key, func(t *testing.T) {
			tmpl := emailDefault(t, key)
			data := map[string]interface{}{"order": order, "user": &model.User{UserName: "Buyer"}, "url": "https://front.example/order/detail?order_id=ORDER-42"}
			if key == "recover_abandoned_cart" {
				data["cart"] = map[string]interface{}{"Items": []map[string]interface{}{{"Title": "Fixture product", "Quantity": 2, "Price": int64(12345)}}, "Amount": int64(24690)}
			}
			got, err := w.RenderEmailTemplate(&tmpl, data)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"Fixture product", "x2", "123.45", "246.90", `href="https://front.example/order/detail?order_id=ORDER-42"`} {
				if !strings.Contains(got.Content, want) {
					t.Errorf("missing %q in %s", want, got.Content)
				}
			}
		})
	}
}

// Capture the actual persistence payload without a database or SMTP service.
// Omitting url or selecting BaseUrl when FrontUrl exists must break this test.
func TestEmailTemplateRecoveryPlanURL(t *testing.T) {
	for _, front := range []string{"", "https://front.example"} {
		t.Run(front, func(t *testing.T) {
			db, err := gorm.Open(mysql.New(mysql.Config{DSN: "unused:unused@tcp(127.0.0.1:1)/unused", SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true})
			if err != nil {
				t.Fatal(err)
			}
			var captured *model.MailPlan
			stop := errors.New("stop before database write")
			if err := db.Callback().Create().Before("gorm:create").Register("test:capture_mail", func(tx *gorm.DB) {
				plan := tx.Statement.Dest.(*model.MailPlan)
				copy := *plan
				captured = &copy
				tx.AddError(stop)
			}); err != nil {
				t.Fatal(err)
			}
			tmpl := emailDefault(t, "recover_abandoned_checkout")
			tmpl.Open = true
			tmpl.Content = `{{cart.Amount|priceFormat}} [Pay]({{url|safe}})`
			w := &provider.Website{DB: db, System: &config.SystemConfig{SiteName: "Test site"}, PluginSendmail: &config.PluginSendmail{Account: "sender@example.com", Templates: []config.EmailTemplate{tmpl}}}
			w.System.BaseUrl = "https://base.example"
			w.System.FrontUrl = front
			order := &model.Order{OrderId: "ORDER-42", Amount: 12345}
			if err := w.SendRecoverAbandonedOrderPlan(&model.User{Email: "buyer@example.com"}, order); !errors.Is(err, stop) {
				t.Errorf("lost persistence error: %v", err)
			}
			if captured == nil {
				t.Fatal("no mail plan reached persistence")
			}
			if !strings.Contains(captured.Content, "123.45") {
				t.Errorf("saved cart template lost amount: %s", captured.Content)
			}
			host := "https://base.example"
			if front != "" {
				host = "https://front.example"
			}
			want := `href="` + host + `/order/checkout?order_id=ORDER-42"`
			if !strings.Contains(captured.Content, want) {
				t.Fatalf("missing %s in %s", want, captured.Content)
			}
		})
	}
}
