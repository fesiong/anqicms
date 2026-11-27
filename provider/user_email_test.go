package provider

import "testing"

// 邮箱大小写不敏感；不归一化会让 " Alice@Example.COM " 与 "alice@example.com"
// 被当成两个账号，绕过注册查重。
func TestNormalizeEmail(t *testing.T) {
	cases := []struct{ in, want string }{
		{" Alice@Example.COM ", "alice@example.com"},
		{"a@b.com", "a@b.com"},
		{"  ", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeEmail(c.in); got != c.want {
			t.Errorf("normalizeEmail(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}

// 所有用户邮箱的查重/落库入口都必须走同一套归一化，
// 否则从某个入口写进去的 " A@x.com " 仍然能被另一个入口重复注册。
func TestUserEmailPathsNormalize(t *testing.T) {
	cases := []struct{ file, fn string }{
		{"user.go", "GetUserInfoByEmail"},
		{"user.go", "RegisterUser"},
		{"user.go", "LoginViaGoogle"},
		{"user.go", "UpdateUserInfo"},
		{"subscriber.go", "GetSubscriberByEmail"},
		{"subscriber.go", "SaveSubscriber"},
		{"subscriber.go", "SendSubscriberMail"},
		{"order.go", "CreateOrder"},
	}
	for _, c := range cases {
		fn := mustFindProviderFunc(t, c.file, c.fn)
		if callName(fn, "normalizeEmail") == "" {
			t.Errorf("%s 未调用 normalizeEmail：邮箱查重/落库口径不一致", c.fn)
		}
	}
}
