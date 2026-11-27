package intent

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMaskSecrets 凭据字段必须掩码，非凭据字段必须原样保留。
//
// 2026-10-27 全接口测试发现 6 个只读意图明文吐凭据：
// storage_get 的 s3 access_key/secret、wechat_get 的 app_secret、
// google_get 的 client_secret、siteops_website list 的 mysql.password、
// contentops_import setting_get、site_anqi 的 JWT 密钥。
func TestMaskSecrets(t *testing.T) {
	in := map[string]any{
		"storage_type":  "r2",
		"s3_access_key": "AKIA123",
		"s3_secret_key": "s3cr3t",
		"app_secret":    "wechat-secret",
		"client_secret": "google-secret",
		"token_secret":  "jwt-key",
		"mysql": map[string]any{
			"host":     "127.0.0.1",
			"user":     "root",
			"password": "root",
		},
		"list": []any{
			map[string]any{"name": "site1", "password": "p1"},
			map[string]any{"name": "site2", "password": ""},
		},
		// 非凭据字段不能被误伤：它们只是名字里带类似词根。
		"token_expire": 3600,
		"total":        3,
		"keyword":      "hello",
	}
	got, ok := MaskSecrets(in).(map[string]any)
	if !ok {
		t.Fatal("MaskSecrets 应返回 map")
	}
	for _, k := range []string{"s3_access_key", "s3_secret_key", "app_secret", "client_secret", "token_secret"} {
		if got[k] != MaskedSecret {
			t.Errorf("%s = %#v，期望掩码", k, got[k])
		}
	}
	mysql := got["mysql"].(map[string]any)
	if mysql["password"] != MaskedSecret {
		t.Errorf("嵌套 password 未掩码: %#v", mysql["password"])
	}
	if mysql["host"] != "127.0.0.1" || mysql["user"] != "root" {
		t.Error("非凭据字段被误改")
	}
	list := got["list"].([]any)
	if list[0].(map[string]any)["password"] != MaskedSecret {
		t.Error("数组内 password 未掩码")
	}
	// 空密码保持空串：区分「没配」与「配了但被藏了」。
	if list[1].(map[string]any)["password"] != "" {
		t.Error("空值不该被替换成掩码，否则读方无法区分「未配置」")
	}
	if got["token_expire"] != 3600 || got["total"] != 3 || got["keyword"] != "hello" {
		t.Errorf("非凭据字段被误伤: %#v", got)
	}
	// 入参不能被就地修改（Compose 可能还要用原值）。
	if in["app_secret"] != "wechat-secret" {
		t.Error("MaskSecrets 不该修改入参")
	}
}

// TestMaskSecretsInJSON 纯文本通道（Data 为 nil）也要脱敏。
func TestMaskSecretsInJSON(t *testing.T) {
	raw := `{"ok":true,"data":{"app_id":"wx1","app_secret":"s3cr3t"}}`
	got := MaskSecretsInJSON(raw)
	if strings.Contains(got, "s3cr3t") {
		t.Errorf("JSON 文本里的密钥没被掩码: %s", got)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(got), &v); err != nil {
		t.Fatalf("掩码后仍是合法 JSON: %v", err)
	}
	// 非 JSON 文本原样返回（文件工具的成功/失败文案都走这条路）。
	if s := "文件写入成功: cache/a.txt (12 字节)"; MaskSecretsInJSON(s) != s {
		t.Error("非 JSON 文本不该被改写")
	}
}

// TestContainsMaskedSecret 写前拦截要能递归发现掩码。
func TestContainsMaskedSecret(t *testing.T) {
	if ContainsMaskedSecret(map[string]any{"a": "b"}) {
		t.Error("普通值不该命中")
	}
	if !ContainsMaskedSecret(map[string]any{"values": map[string]any{"password": MaskedSecret}}) {
		t.Error("嵌套掩码应被识别")
	}
	if !ContainsMaskedSecret([]any{MaskedSecret}) {
		t.Error("数组内掩码应被识别")
	}
}

// TestMaskSecretsExtendedCredentials 锁住三条既有欠账字段，并反向锁住
// 「不要把 _token 加宽成后缀」——url_token / link_token 是 URL 别名，
// 掩码后写回会直接毁链接。
//
// 反向对照：从 secretFieldNames 删掉 google_credentials_json → 本用例红。
func TestMaskSecretsExtendedCredentials(t *testing.T) {
	in := map[string]any{
		// GCP 服务账号 JSON：里面含 private_key，但整段是一个字符串，
		// 按 map key 递归永远看不到 private_key 这个键。
		"google_credentials_json": `{"type":"service_account","private_key":"-----BEGIN PRIVATE KEY-----\nMIIB\n-----END PRIVATE KEY-----\n"}`,
		"recaptcha_private_key":   "recaptcha-secret",
		"refresh_token":           "1//refresh-token",
		// 非凭据的 URL 别名：不能被误掩码。
		"url_token":  "about",
		"link_token": "news",
	}

	got, ok := MaskSecrets(in).(map[string]any)
	if !ok {
		t.Fatal("MaskSecrets 应返回 map")
	}
	for _, k := range []string{"google_credentials_json", "recaptcha_private_key", "refresh_token"} {
		if got[k] != MaskedSecret {
			t.Errorf("%s = %#v，期望掩码（否则含私钥的 JSON 会原样出站）", k, got[k])
		}
	}
	for _, k := range []string{"url_token", "link_token"} {
		if got[k] == MaskedSecret {
			t.Errorf("%s 是非凭据 URL 别名，掩码后写回会毁链接", k)
		}
	}

	// 掩码后的 JSON 文本里不得残留私钥原文。
	raw := MaskSecretsInJSON(`{"data":{"google_credentials_json":"-----BEGIN PRIVATE KEY-----MIIB"}}`)
	if strings.Contains(raw, "BEGIN PRIVATE KEY") {
		t.Errorf("纯文本通道仍泄露 GCP 私钥: %s", raw)
	}
}
