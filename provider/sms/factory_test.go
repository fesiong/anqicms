package sms

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
)

// 本文件锁死 Sender 工厂的分派契约：三个 config.SmsProvider* 常量各返回对应
// Name() 的发送器；未知 provider fail closed —— 返回 error，且**不返回 nil
// sender**（调用方漏判 err 也不会 nil panic，而是拿到一条 permanent 失败）。
// 分派清单用 go/ast 与 config 的常量声明双向穷举，不写死总数（AGENTS.md §6）。

// TestNewSender 三个已知 provider 各返回对应发送器；未知值返回 error 且 sender 非 nil。
// 抓它的破坏：漏掉某个 case、未知值返回 (nil, nil)、未知值返回 (nil, err)、
// 把未知值兜底成某一家（会拿空凭据真的出网）。
func TestNewSender(t *testing.T) {
	known := []struct {
		provider string
		wantName string
	}{
		{config.SmsProviderAliyun, "aliyun"},
		{config.SmsProviderTencent, "tencent"},
		{config.SmsProviderTwilio, "twilio"},
	}
	for _, c := range known {
		t.Run(c.provider, func(t *testing.T) {
			sender, err := NewSender(c.provider)
			if err != nil {
				t.Fatalf("已知 provider %q 不应报错: %v", c.provider, err)
			}
			if sender == nil {
				t.Fatalf("已知 provider %q 返回了 nil sender", c.provider)
			}
			if sender.Name() != c.wantName {
				t.Fatalf("Name() = %q, want %q", sender.Name(), c.wantName)
			}
		})
	}

	// "Aliyun" 大小写变体也必须在内：工厂不做归一化，未知即 fail closed。
	for _, provider := range []string{"", "unknown", "Aliyun"} {
		t.Run("未知_"+strconv.Quote(provider), func(t *testing.T) {
			sender, err := NewSender(provider)
			if err == nil {
				t.Fatalf("未知 provider %q 必须返回 error（fail closed）", provider)
			}
			if sender == nil {
				t.Fatalf("未知 provider %q 不得返回 nil sender：调用方漏判 err 会 nil panic", provider)
			}
			// 兜底发送器永不出网，Send 一律 permanent 失败。
			res, sendErr := sender.Send(context.Background(), Profile{}, twilioTestSendRequest())
			if sendErr == nil {
				t.Fatal("未知 provider 的 Send 必须失败，绝不能静默成功")
			}
			var se *SendError
			if !errors.As(sendErr, &se) {
				t.Fatalf("错误不是 *SendError: %v", sendErr)
			}
			if se.Class != ClassPermanent {
				t.Fatalf("未知 provider 应为 permanent, got %s", se.Class)
			}
			if !errors.Is(sendErr, ErrSmsSendFailed) {
				t.Fatal("errors.Is(err, ErrSmsSendFailed) 应为 true")
			}
			if res.BizId != "" {
				t.Fatalf("失败时不应有回执: %+v", res)
			}
		})
	}
}

// TestNewSenderCoversAllProviderConstants 用 go/ast 双向穷举：
// config/sms.go 声明的每个 SmsProvider* 常量都必须在 factory.go 的 switch 里有
// case；factory.go 引用的每个 SmsProvider* 常量也必须真的在 config 里声明。
// 新增服务商常量漏配工厂会红（否则那家永远 fail closed，静默不可用），
// 工厂引用了不存在的常量也会红。不写死总数。
func TestNewSenderCoversAllProviderConstants(t *testing.T) {
	constProviderNames := smsProviderConstantNames(t, "../../config/sms.go")
	factoryNames := smsFactoryCaseNames(t, "factory.go")

	// 反空转：任一侧解析为空时双向差集皆空会假绿。
	if len(constProviderNames) == 0 {
		t.Fatal("未从 config/sms.go 解析到任何 SmsProvider* 常量：护栏会空转")
	}
	if len(factoryNames) == 0 {
		t.Fatal("未从 factory.go 解析到任何 SmsProvider* case：护栏会空转")
	}
	for name := range constProviderNames {
		if !factoryNames[name] {
			t.Errorf("config 常量 %s 在 factory.go 的 switch 里没有对应 case", name)
		}
	}
	for name := range factoryNames {
		if !constProviderNames[name] {
			t.Errorf("factory.go 引用了 config 里不存在的常量 %s", name)
		}
	}
}

// smsProviderConstantNames 穷举文件里所有 SmsProvider* 常量名。
func smsProviderConstantNames(t *testing.T, path string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败: %v", path, err)
	}
	names := make(map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for _, name := range spec.Names {
			if strings.HasPrefix(name.Name, "SmsProvider") {
				names[name.Name] = true
			}
		}
		return true
	})
	return names
}

// smsFactoryCaseNames 穷举 factory.go 里 switch case 引用的 config.SmsProvider* 常量名。
func smsFactoryCaseNames(t *testing.T, path string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败: %v", path, err)
	}
	names := make(map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		cc, ok := n.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expr := range cc.List {
			sel, ok := expr.(*ast.SelectorExpr)
			if !ok || !strings.HasPrefix(sel.Sel.Name, "SmsProvider") {
				continue
			}
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "config" {
				t.Errorf("case 引用了非 config 包的 %s：分派清单必须落在 config.SmsProvider* 上", sel.Sel.Name)
				continue
			}
			names[sel.Sel.Name] = true
		}
		return true
	})
	return names
}
