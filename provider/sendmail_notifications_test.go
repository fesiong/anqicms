package provider

import (
	"bytes"
	"encoding/json"
	"net/mail"
	"net/textproto"
	"os"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

const notificationBody = "<html><body>Notification fixture</body></html>"

func setNotificationTemplate(site *Website, key string, open bool) {
	site.System = &config.SystemConfig{BaseUrl: "https://shop.example.invalid"}
	site.PluginSendmail.Templates = []config.EmailTemplate{{
		Key: key, Open: open, Subject: "Notification fixture", Content: notificationBody,
	}}
}

func assertNotificationLogs(t *testing.T, site *Website, recipients []string, failures map[string]string) {
	t.Helper()
	content, err := os.ReadFile(site.CachePath + MailLogFile)
	if err != nil {
		t.Error(err)
		return
	}
	lines := bytes.Split(bytes.TrimSpace(content), []byte("\n"))
	if len(lines) != len(recipients) {
		t.Errorf("got %d mail logs, want %d: %s", len(lines), len(recipients), content)
	}
	for i, recipient := range recipients {
		if i >= len(lines) {
			break
		}
		var entry MailLog
		if err := json.Unmarshal(lines[i], &entry); err != nil {
			t.Error(err)
			continue
		}
		if entry.Address != recipient || entry.Subject != "Notification fixture" || entry.Status == "" {
			t.Errorf("unexpected log for %s: %+v", recipient, entry)
		}
		if reason := failures[recipient]; reason != "" && !strings.Contains(entry.Status, reason) {
			t.Errorf("log lost failure %q: %+v", reason, entry)
		}
	}
}

func assertNotificationMessage(t *testing.T, capture mailTransportCapture) {
	t.Helper()
	message, err := mail.ReadMessage(bytes.NewReader(capture.raw))
	if err != nil {
		t.Error(err)
		return
	}
	parts := readMailTransportParts(t, textproto.MIMEHeader(message.Header), message.Body)
	if len(parts) == 0 || parts[0].mediaType != "text/html" || string(parts[0].body) != notificationBody {
		t.Errorf("notification lost HTML body: %+v", parts)
	}
}

// The old admin sendMail(false, false) loses HTML and logs; ignoring either
// return value loses errors, and returning early prevents the other delivery.
func TestMailNotificationsOrderRecipients(t *testing.T) {
	for _, notification := range []struct {
		key  string
		send func(*Website, *model.User, *model.Order) error
	}{
		{"new_order", (*Website).SendNewOrderEmail},
		{"pay_order", (*Website).SendPayOrderEmail},
		{"order_custom_form", (*Website).SendOrderCustomFormEmail},
	} {
		for _, scenario := range []struct {
			name     string
			failures map[string]string
		}{
			{"success", nil},
			{"user_failure", map[string]string{"user@example.invalid": "user rejected"}},
			{"admin_failure", map[string]string{"admin@example.invalid": "admin rejected"}},
			{"both_fail", map[string]string{"user@example.invalid": "user rejected", "admin@example.invalid": "admin rejected"}},
		} {
			t.Run(notification.key+"/"+scenario.name, func(t *testing.T) {
				site, captures := newMailTransportSite(t, scenario.failures)
				setNotificationTemplate(site, notification.key, true)
				site.PluginSendmail.Recipient = "admin@example.invalid"
				order := &model.Order{Details: []*model.OrderDetail{{GoodsId: 1, OrderFields: []config.CustomField{{}}}}}
				err := notification.send(site, &model.User{Email: "user@example.invalid"}, order)
				if len(scenario.failures) == 0 && err != nil {
					t.Errorf("successful send returned %v", err)
				}
				if len(scenario.failures) > 0 {
					if err == nil {
						t.Error("notification discarded SMTP failures")
					} else {
						for _, reason := range scenario.failures {
							if !strings.Contains(err.Error(), reason) {
								t.Errorf("missing %q in %v", reason, err)
							}
						}
						joined, ok := err.(interface{ Unwrap() []error })
						if !ok || len(joined.Unwrap()) != len(scenario.failures) {
							t.Errorf("expected joined delivery errors, got %T: %v", err, err)
						}
					}
				}
				recipients := []string{"user@example.invalid", "admin@example.invalid"}
				for _, recipient := range recipients {
					capture := nextMailTransportCapture(t, captures)
					if capture.recipient != recipient {
						t.Errorf("got recipient %q, want %q", capture.recipient, recipient)
					}
					if scenario.failures[recipient] == "" {
						assertNotificationMessage(t, capture)
					}
				}
				assertNotificationLogs(t, site, recipients, scenario.failures)
			})
		}
	}
}

// User delivery remains gated, but closed templates, Brevo and absent users
// must not suppress the admin notification or its log.
func TestMailNotificationsAdminOnly(t *testing.T) {
	for _, notification := range []struct {
		key  string
		send func(*Website, *model.User, *model.Order) error
	}{
		{"new_order", (*Website).SendNewOrderEmail},
		{"pay_order", (*Website).SendPayOrderEmail},
		{"order_custom_form", (*Website).SendOrderCustomFormEmail},
	} {
		for _, gate := range []string{"closed", "brevo", "nil_user", "empty_email"} {
			t.Run(notification.key+"/"+gate, func(t *testing.T) {
				site, captures := newMailTransportSite(t, nil)
				setNotificationTemplate(site, notification.key, gate != "closed")
				site.PluginSendmail.UseBrevo = gate == "brevo"
				site.PluginSendmail.Recipient = "admin@example.invalid"
				user := &model.User{Email: "user@example.invalid"}
				if gate == "nil_user" {
					user = nil
				} else if gate == "empty_email" {
					user.Email = ""
				}
				order := &model.Order{Details: []*model.OrderDetail{{GoodsId: 1, OrderFields: []config.CustomField{{}}}}}
				if err := notification.send(site, user, order); err != nil {
					t.Fatal(err)
				}
				capture := nextMailTransportCapture(t, captures)
				if capture.recipient != "admin@example.invalid" {
					t.Errorf("user gate bypassed: %q", capture.recipient)
				}
				assertNotificationMessage(t, capture)
				select {
				case extra := <-captures:
					t.Errorf("unexpected delivery to %s", extra.recipient)
				default:
				}
				assertNotificationLogs(t, site, []string{"admin@example.invalid"}, nil)
			})
		}
	}
}

// Returning nil after SendMail must fail for every single-user notification.
// The invoice fixture also verifies the real PDF attachment survives delivery.
func TestMailNotificationsUserErrors(t *testing.T) {
	for _, key := range []string{"group_change", "invoice", "delivery_order", "finish_order", "refund_order"} {
		for _, reject := range []bool{false, true} {
			name := "success"
			failures := map[string]string{}
			if reject {
				name = "failure"
				failures["user@example.invalid"] = "notification rejected"
			}
			t.Run(key+"/"+name, func(t *testing.T) {
				site, captures := newMailTransportSite(t, failures)
				setNotificationTemplate(site, key, true)
				user, order := &model.User{Email: "user@example.invalid"}, &model.Order{}
				var err error
				switch key {
				case "group_change":
					err = site.SendUserGroupChangeEmail(user, &model.UserGroup{})
				case "invoice":
					site.DataPath = t.TempDir() + "/"
					if err := os.WriteFile(site.DataPath+"invoice.pdf", []byte("%PDF-test invoice"), 0600); err != nil {
						t.Fatal(err)
					}
					err = site.SendInvoiceMail(user, order, "invoice.pdf")
				case "delivery_order":
					err = site.SendDeliveryOrderEmail(user, order)
				case "finish_order":
					err = site.SendCompletedOrderEmail(user, order)
				case "refund_order":
					err = site.SendRefundOrderEmail(user, order)
				}
				if reject && (err == nil || !strings.Contains(err.Error(), "notification rejected")) {
					t.Errorf("SMTP failure was discarded: %v", err)
				} else if !reject && err != nil {
					t.Errorf("successful send returned %v", err)
				}
				capture := nextMailTransportCapture(t, captures)
				if capture.recipient != user.Email {
					t.Errorf("wrong recipient: %s", capture.recipient)
				}
				if !reject {
					assertNotificationMessage(t, capture)
					if key == "invoice" {
						message, err := mail.ReadMessage(bytes.NewReader(capture.raw))
						if err != nil {
							t.Fatal(err)
						}
						parts := readMailTransportParts(t, textproto.MIMEHeader(message.Header), message.Body)
						if len(parts) != 2 || parts[1].filename != "invoice.pdf" || parts[1].mediaType != "application/pdf" || string(parts[1].body) != "%PDF-test invoice" {
							t.Errorf("invoice attachment changed: %+v", parts)
						}
					}
				}
				assertNotificationLogs(t, site, []string{user.Email}, failures)
			})
		}
	}
}
