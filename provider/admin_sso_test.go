package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"testing"
	"time"

	"kandaoni.com/anqicms/library"
)

func mockSSOSite(secret string) *Website {
	return &Website{
		Id:          2,
		TokenSecret: secret,
		Cache:       library.InitMemoryCache(),
	}
}

func TestVerifyAdminSSO(t *testing.T) {
	const userName = "sinclair"
	site := mockSSOSite("unit-test-token-secret")

	t.Run("正常票据可用且只能用一次", func(t *testing.T) {
		nonce := MintAdminSSONonce(AdminSSOTTL)
		if nonce == "" {
			t.Fatal("MintAdminSSONonce 返回空")
		}
		sign := SignAdminSSO(site.TokenSecret, userName, nonce)
		if err := site.VerifyAdminSSO(userName, nonce, sign); err != nil {
			t.Fatalf("首次校验应通过，got %v", err)
		}
		if err := site.VerifyAdminSSO(userName, nonce, sign); err != ErrAdminSSOUsed {
			t.Fatalf("重复使用应报 ErrAdminSSOUsed，got %v", err)
		}
	})

	t.Run("过期票据", func(t *testing.T) {
		nonce := MintAdminSSONonce(-time.Minute)
		sign := SignAdminSSO(site.TokenSecret, userName, nonce)
		if err := site.VerifyAdminSSO(userName, nonce, sign); err != ErrAdminSSOExpired {
			t.Fatalf("got %v, want ErrAdminSSOExpired", err)
		}
	})

	t.Run("超长有效期票据", func(t *testing.T) {
		nonce := MintAdminSSONonce(72 * time.Hour)
		sign := SignAdminSSO(site.TokenSecret, userName, nonce)
		if err := site.VerifyAdminSSO(userName, nonce, sign); err != ErrAdminSSOInvalid {
			t.Fatalf("got %v, want ErrAdminSSOInvalid", err)
		}
	})

	t.Run("换一个管理员就失效", func(t *testing.T) {
		nonce := MintAdminSSONonce(AdminSSOTTL)
		sign := SignAdminSSO(site.TokenSecret, "another-admin", nonce)
		if err := site.VerifyAdminSSO(userName, nonce, sign); err != ErrAdminSSOInvalid {
			t.Fatalf("got %v, want ErrAdminSSOInvalid", err)
		}
	})

	t.Run("篡改nonce", func(t *testing.T) {
		nonce := MintAdminSSONonce(AdminSSOTTL)
		sign := SignAdminSSO(site.TokenSecret, userName, nonce)
		flipped := byte('0')
		if nonce[len(nonce)-1] == '0' {
			flipped = '1'
		}
		tampered := nonce[:len(nonce)-1] + string(flipped)
		if err := site.VerifyAdminSSO(userName, tampered, sign); err != ErrAdminSSOInvalid {
			t.Fatalf("got %v, want ErrAdminSSOInvalid", err)
		}
	})

	t.Run("跨站点票据", func(t *testing.T) {
		other := mockSSOSite("other-site-secret")
		nonce := MintAdminSSONonce(AdminSSOTTL)
		sign := SignAdminSSO(other.TokenSecret, userName, nonce)
		if err := site.VerifyAdminSSO(userName, nonce, sign); err != ErrAdminSSOInvalid {
			t.Fatalf("got %v, want ErrAdminSSOInvalid", err)
		}
	})

	// 事故复盘：攻击者从 admins.password 拖出哈希后，用 sha256(哈希 + 自填 nonce) 就能登录。
	// 这条路径必须在改造后彻底失效。
	t.Run("密码哈希不再是登录密钥", func(t *testing.T) {
		stolenHash := "$2a$12$wk7QS4uHdSYP6d/BWUBOWOT2YuNq3IM46F4qctNNJRizdRMYosjAW"
		legacyNonce := strings.ReplaceAll(time.Now().Format("20060102150405.000000"), ".", "")
		legacySign := sha256.Sum256([]byte(stolenHash + legacyNonce))
		if err := site.VerifyAdminSSO(userName, legacyNonce, hex.EncodeToString(legacySign[:])); err != ErrAdminSSOInvalid {
			t.Fatalf("got %v, want ErrAdminSSOInvalid", err)
		}
	})

	t.Run("畸形输入", func(t *testing.T) {
		validSign := SignAdminSSO(site.TokenSecret, userName, "123:abc")
		cases := []struct {
			nonce, sign string
		}{
			{"", ""},
			{"", validSign},
			{"no-colon", validSign},
			{":abc", validSign},
			{"notanumber:abc", validSign},
			{"99999999999999999999:abc", validSign},
			{MintAdminSSONonce(AdminSSOTTL), ""},
		}
		for _, c := range cases {
			if err := site.VerifyAdminSSO(userName, c.nonce, c.sign); err != ErrAdminSSOInvalid {
				t.Errorf("nonce=%q sign=%q got %v, want ErrAdminSSOInvalid", c.nonce, c.sign, err)
			}
		}
	})

	t.Run("站点没有密钥时一律拒绝", func(t *testing.T) {
		noSecret := mockSSOSite("")
		nonce := MintAdminSSONonce(AdminSSOTTL)
		sign := SignAdminSSO("", userName, nonce)
		if err := noSecret.VerifyAdminSSO(userName, nonce, sign); err != ErrAdminSSOInvalid {
			t.Fatalf("got %v, want ErrAdminSSOInvalid", err)
		}
	})
}

// 主站签发、子站核销：密钥取的是目标站的 TokenSecret，链接要经过一次 URL 编解码。
func TestAdminSSOCrossSiteRoundTrip(t *testing.T) {
	parent := mockSSOSite("parent-site-secret")
	sub := mockSSOSite("sub-site-secret")
	const userName = "site admin"

	// controller/manageController/website.go LoginSubWebsite 的等价步骤
	nonce := MintAdminSSONonce(AdminSSOTTL)
	sign := SignAdminSSO(sub.TokenSecret, userName, nonce)
	link := "https://sub.example.com/system/login?admin-login=true&site_id=2&user_name=" +
		url.QueryEscape(userName) + "&sign=" + sign + "&nonce=" + url.QueryEscape(nonce)

	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("链接无法解析: %v", err)
	}
	q := parsed.Query()
	if q.Get("admin-login") != "true" {
		t.Fatalf("缺少 admin-login 标记: %s", link)
	}
	// 浏览器侧解码后必须拿回原样字段，否则签名对不上
	if got := q.Get("user_name"); got != userName {
		t.Fatalf("user_name 往返后变了: %q", got)
	}
	if got := q.Get("nonce"); got != nonce {
		t.Fatalf("nonce 往返后变了: %q", got)
	}

	// 子站登录接口在切到子站上下文后校验
	if err := sub.VerifyAdminSSO(q.Get("user_name"), q.Get("nonce"), q.Get("sign")); err != nil {
		t.Fatalf("子站应接受本站票据，got %v", err)
	}
	// 主站密钥不同，不能拿同一张票登录主站
	if err := parent.VerifyAdminSSO(q.Get("user_name"), q.Get("nonce"), q.Get("sign")); err != ErrAdminSSOInvalid {
		t.Fatalf("主站应拒绝子站票据，got %v", err)
	}
	// 用过一次之后子站也不再接受
	if err := sub.VerifyAdminSSO(q.Get("user_name"), q.Get("nonce"), q.Get("sign")); err != ErrAdminSSOUsed {
		t.Fatalf("子站应拒绝重复票据，got %v", err)
	}
}

func TestMintAdminSSONonceIsUnpredictable(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		nonce := MintAdminSSONonce(AdminSSOTTL)
		if nonce == "" {
			t.Fatal("MintAdminSSONonce 返回空")
		}
		if seen[nonce] {
			t.Fatalf("nonce 重复: %s", nonce)
		}
		seen[nonce] = true
		if !strings.Contains(nonce, ":") {
			t.Fatalf("nonce 缺少过期时间: %s", nonce)
		}
	}
}
