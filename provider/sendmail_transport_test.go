package provider

import (
	"bytes"
	"encoding/base64"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
)

type mailTransportCapture struct {
	recipient string
	raw       []byte
}

// Only loopback sockets are used; no configured site or real mail account is loaded.
func newMailTransportSite(t *testing.T, failures map[string]string) (*Website, <-chan mailTransportCapture) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	captures := make(chan mailTransportCapture, 16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				wire := textproto.NewConn(conn)
				_ = wire.PrintfLine("220 localhost test SMTP")
				recipient := ""
				for {
					line, err := wire.ReadLine()
					if err != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO"):
						_ = wire.PrintfLine("250-localhost")
						_ = wire.PrintfLine("250 AUTH PLAIN")
					case strings.HasPrefix(line, "AUTH PLAIN"):
						_ = wire.PrintfLine("235 authenticated")
					case strings.HasPrefix(line, "HELO"), strings.HasPrefix(line, "MAIL FROM:"):
						_ = wire.PrintfLine("250 OK")
					case strings.HasPrefix(line, "RCPT TO:"):
						recipient = strings.TrimSuffix(strings.TrimPrefix(line, "RCPT TO:<"), ">")
						if reason := failures[recipient]; reason != "" {
							captures <- mailTransportCapture{recipient: recipient}
							_ = wire.PrintfLine("550 %s", reason)
						} else {
							_ = wire.PrintfLine("250 OK")
						}
					case line == "DATA":
						_ = wire.PrintfLine("354 send message")
						raw, err := io.ReadAll(wire.DotReader())
						if err != nil {
							return
						}
						captures <- mailTransportCapture{recipient: recipient, raw: raw}
						_ = wire.PrintfLine("250 queued")
					case line == "QUIT":
						_ = wire.PrintfLine("221 bye")
						return
					default:
						_ = wire.PrintfLine("502 unsupported")
					}
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
	})
	return &Website{
		System: &config.SystemConfig{SiteName: "Test site"},
		PluginSendmail: &config.PluginSendmail{
			Server: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Account: "sender@example.invalid",
		},
		CachePath: t.TempDir() + "/",
	}, captures
}

func nextMailTransportCapture(t *testing.T, captures <-chan mailTransportCapture) mailTransportCapture {
	t.Helper()
	select {
	case capture := <-captures:
		return capture
	case <-time.After(6 * time.Second):
		t.Fatal("SMTP did not receive recipient/message")
		return mailTransportCapture{}
	}
}

type mailTransportPart struct {
	mediaType string
	filename  string
	body      []byte
}

func readMailTransportParts(t *testing.T, header textproto.MIMEHeader, body io.Reader) []mailTransportPart {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(mediaType, "multipart/") {
		reader := multipart.NewReader(body, params["boundary"])
		var parts []mailTransportPart
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				return parts
			}
			if err != nil {
				t.Fatal(err)
			}
			parts = append(parts, readMailTransportParts(t, part.Header, part)...)
		}
	}
	switch header.Get("Content-Transfer-Encoding") {
	case "base64":
		body = base64.NewDecoder(base64.StdEncoding, body)
	case "quoted-printable":
		body = quotedprintable.NewReader(body)
	}
	content, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	_, disposition, _ := mime.ParseMediaType(header.Get("Content-Disposition"))
	return []mailTransportPart{{mediaType: mediaType, filename: disposition["filename"], body: content}}
}

// Dropping attach or classifying untrimmed HTML as text must break these wire assertions.
func TestMailTransportMIME(t *testing.T) {
	for _, tc := range []struct {
		name, body, mediaType string
		attachment            bool
	}{
		{"attachment", "plain body", "text/plain", true},
		{"leading_whitespace_html", " \t<html><body>正文</body></html>  ", "text/html", false},
		{"plain_without_attachment", "  plain < text", "text/plain", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			site, captures := newMailTransportSite(t, nil)
			var attachment *library.Attachment
			payload := []byte{0, 1, 2, 255, '\n', 'x'}
			if tc.attachment {
				attachment = &library.Attachment{Filename: "report.bin", Content: payload,
					Header: textproto.MIMEHeader{"Content-Type": {"application/octet-stream"}}}
			}
			if err := site.SendMail("transport subject", tc.body, attachment, "reader@example.invalid"); err != nil {
				t.Fatal(err)
			}
			capture := nextMailTransportCapture(t, captures)
			message, err := mail.ReadMessage(bytes.NewReader(capture.raw))
			if err != nil {
				t.Fatal(err)
			}
			if message.Header.Get("Subject") != "transport subject" || message.Header.Get("To") != "reader@example.invalid" {
				t.Fatalf("unexpected envelope headers: %v", message.Header)
			}
			parts := readMailTransportParts(t, textproto.MIMEHeader(message.Header), message.Body)
			wantCount := 1
			if tc.attachment {
				wantCount++
			}
			if len(parts) != wantCount {
				t.Fatalf("got %d MIME parts, want %d (body plus optional attachment)", len(parts), wantCount)
			}
			if parts[0].mediaType != tc.mediaType || string(parts[0].body) != tc.body {
				t.Errorf("body: got %s %q, want %s %q", parts[0].mediaType, parts[0].body, tc.mediaType, tc.body)
			}
			if tc.attachment && (parts[1].filename != "report.bin" || parts[1].mediaType != "application/octet-stream" || !bytes.Equal(parts[1].body, payload)) {
				t.Errorf("attachment changed: %+v", parts[1])
			}
		})
	}
}

// Returning only the final send error loses early failures or earlier error details.
func TestMailTransportRecipientErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		failures map[string]string
	}{
		{"failure_then_success", map[string]string{"first@example.invalid": "first rejected"}},
		{"success_then_failure", map[string]string{"second@example.invalid": "second rejected"}},
		{"both_fail", map[string]string{"first@example.invalid": "first rejected", "second@example.invalid": "second rejected"}},
		{"both_succeed", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			site, captures := newMailTransportSite(t, tc.failures)
			recipients := []string{"first@example.invalid", "second@example.invalid"}
			err := site.sendMail("subject", "body", nil, recipients, false, false)
			if len(tc.failures) == 0 {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal("recipient failures were lost")
				}
				for _, reason := range tc.failures {
					if !strings.Contains(err.Error(), reason) {
						t.Errorf("missing %q in %v", reason, err)
					}
				}
				joined, ok := err.(interface{ Unwrap() []error })
				if !ok || len(joined.Unwrap()) != len(tc.failures) {
					t.Errorf("expected joined recipient errors, got %T: %v", err, err)
				}
			}
			for _, recipient := range recipients {
				capture := nextMailTransportCapture(t, captures)
				if capture.recipient != recipient {
					t.Errorf("got recipient %q, want %q", capture.recipient, recipient)
				}
				if tc.failures[recipient] == "" && len(capture.raw) == 0 {
					t.Errorf("successful recipient %s received no MIME", recipient)
				}
			}
		})
	}
}

// Default ports cannot safely be bound in tests. Guard the final transport assignment
// so the already-computed 25/465 fallback cannot be discarded.
func TestMailTransportUsesComputedPort(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "sendmail.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var portValue ast.Expr
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "sendMail" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			assignment, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, lhs := range assignment.Lhs {
				selector, ok := lhs.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Port" {
					continue
				}
				if owner, ok := selector.X.(*ast.Ident); ok && owner.Name == "email" {
					portValue = assignment.Rhs[i]
				}
			}
			return true
		})
	}
	usesComputedPort := func(expr ast.Expr) bool { id, ok := expr.(*ast.Ident); return ok && id.Name == "port" }
	if !usesComputedPort(portValue) {
		t.Errorf("email.Port must use computed port, got %T", portValue)
	}
	if usesComputedPort(&ast.SelectorExpr{X: ast.NewIdent("setting"), Sel: ast.NewIdent("Port")}) {
		t.Fatal("negative control accepted raw setting.Port")
	}
}
