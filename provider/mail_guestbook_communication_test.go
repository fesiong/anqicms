package provider

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/mail"
	"net/textproto"
	"os"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

// Removing user from the real entry's context loses the default greeting.
// Removing the recipient gate delivers an unwanted reply (including empty RCPT).
func TestGuestbookMailReply(t *testing.T) {
	for _, tc := range []struct{ name, contact, extra, recipient string }{
		{"contact", "reader@example.invalid", "other@example.invalid", "reader@example.invalid"},
		{"extra", "phone", "extra@example.invalid", "extra@example.invalid"},
		{"missing", "", "", ""},
		{"invalid", "phone", "not an email", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			site, captures := newMailTransportSite(t, nil)
			site.System = &config.SystemConfig{SiteName: "Fixture"}
			site.PluginSendmail.Recipient = "admin@example.invalid"
			site.PluginSendmail.Templates = []config.EmailTemplate{
				{Key: "guestbook", Open: true, Subject: "admin notice", Content: "<p>Admin {{guestbook.UserName}}</p>"},
				{Key: "auto_reply", Open: true, Subject: "reply"},
			}
			site.SendGuestbookToMail(&model.Guestbook{UserName: "留言客户", Contact: tc.contact, ExtraData: map[string]interface{}{"email": tc.extra, "nonstring": 42}})
			admin := nextMailTransportCapture(t, captures)
			if admin.recipient != "admin@example.invalid" {
				t.Fatalf("admin recipient: %q", admin.recipient)
			}
			if tc.recipient != "" {
				reply := nextMailTransportCapture(t, captures)
				if reply.recipient != tc.recipient {
					t.Errorf("reply recipient: %q, want %q", reply.recipient, tc.recipient)
				}
				message, err := mail.ReadMessage(bytes.NewReader(reply.raw))
				if err != nil {
					t.Fatal(err)
				}
				parts := readMailTransportParts(t, textproto.MIMEHeader(message.Header), message.Body)
				if len(parts) != 1 || parts[0].mediaType != "text/html" || !strings.Contains(string(parts[0].body), "尊敬的 留言客户，您好") {
					t.Errorf("default greeting missing: %+v", parts)
				}
			}
			// SendGuestbookToMail is synchronous, so all deliveries are now captured.
			select {
			case extra := <-captures:
				t.Errorf("unexpected automatic reply to %q", extra.recipient)
			default:
			}
		})
	}
}

func TestCommunicationMailStripTagsBehavior(t *testing.T) {
	for _, tc := range []struct{ html, text string }{
		{"<p>第一段</p><p>第二段<br>换行</p>", "第一段\n第二段\n换行"},
		{"前缀 <strong>内容</strong><script>alert(1)</script><style>p{color:red}</style>", "前缀\n内容"},
		{"plain content", "plain content"},
	} {
		if got := library.StripTags(tc.html); got != tc.text {
			t.Errorf("StripTags(%q)=%q, want %q", tc.html, got, tc.text)
		}
	}
}

// These DB-backed entries cannot be invoked without persistence and customer
// side effects. Guard the exact body expression and asynchronous admin call.
func communicationMailGuard(fn *ast.FuncDecl, owner string) error {
	var body ast.Expr
	var sends []*ast.GoStmt
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if a, ok := n.(*ast.AssignStmt); ok {
			for i, lhs := range a.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == "content" && len(a.Rhs) == len(a.Lhs) {
					body = a.Rhs[i]
				}
			}
		}
		if g, ok := n.(*ast.GoStmt); ok {
			if s, ok := g.Call.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "sendMail" {
				sends = append(sends, g)
			}
		}
		return true
	})
	converted, raw := 0, 0
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && len(call.Args) == 1 {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "library" && sel.Sel.Name == "StripTags" {
					if field, ok := call.Args[0].(*ast.SelectorExpr); ok {
						if id, ok := field.X.(*ast.Ident); ok && id.Name == owner && field.Sel.Name == "Content" {
							converted++
							return false
						}
					}
				}
			}
		}
		if field, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := field.X.(*ast.Ident); ok && id.Name == owner && field.Sel.Name == "Content" {
				raw++
			}
		}
		return true
	})
	if converted != 1 || raw != 0 {
		return fmt.Errorf("body must convert %s.Content exactly once (converted=%d raw=%d)", owner, converted, raw)
	}
	if len(sends) != 1 {
		return fmt.Errorf("want one asynchronous admin send, got %d", len(sends))
	}
	call := sends[0].Call
	if len(call.Args) != 6 {
		return fmt.Errorf("unexpected mail arguments")
	}
	for i, want := range []string{"subject", "content", "nil", "nil", "false", "true"} {
		id, ok := call.Args[i].(*ast.Ident)
		if !ok || id.Name != want {
			return fmt.Errorf("mail argument %d must be %s", i, want)
		}
	}
	return nil
}

func TestCommunicationAdminMailGuard(t *testing.T) {
	source, err := os.ReadFile("communication.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, owner string }{
		{"CommunicationOrderCreate", "order"}, {"CommunicationAcceptQuote", "communicationOrder"}, {"CommunicationMessageSend", "msg1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parse := func() *ast.FuncDecl {
				f, err := parser.ParseFile(token.NewFileSet(), "communication.go", source, 0)
				if err != nil {
					t.Fatal(err)
				}
				for _, d := range f.Decls {
					if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == tc.name {
						return fn
					}
				}
				t.Fatalf("missing %s", tc.name)
				return nil
			}
			if err := communicationMailGuard(parse(), tc.owner); err != nil {
				t.Error(err)
			}
			// Mutate only in-memory ASTs; never touch production files for controls.
			for _, mutation := range []string{"raw_html", "wrong_field", "html_mail", "no_log", "wrong_body"} {
				fn := parse()
				changed := false
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if call, ok := n.(*ast.CallExpr); ok {
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
							if sel.Sel.Name == "StripTags" && len(call.Args) == 1 {
								if field, ok := call.Args[0].(*ast.SelectorExpr); ok {
									if mutation == "raw_html" {
										sel.Sel.Name = "Unconverted"
										changed = true
									}
									if mutation == "wrong_field" {
										field.Sel.Name = "Title"
										changed = true
									}
								}
							}
							if sel.Sel.Name == "sendMail" && len(call.Args) == 6 {
								switch mutation {
								case "html_mail":
									call.Args[4] = ast.NewIdent("true")
									changed = true
								case "no_log":
									call.Args[5] = ast.NewIdent("false")
									changed = true
								case "wrong_body":
									call.Args[1] = ast.NewIdent("subject")
									changed = true
								}
							}
						}
					}
					return true
				})
				// During RED, conversion does not exist yet; GREEN must exercise every control.
				if !changed {
					if communicationMailGuard(parse(), tc.owner) == nil {
						t.Errorf("control %s did not mutate", mutation)
					}
					continue
				}
				if communicationMailGuard(fn, tc.owner) == nil {
					t.Errorf("accepted broken %s", mutation)
				}
			}
		})
	}
}
