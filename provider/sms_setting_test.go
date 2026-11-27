package provider

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
)

// 反向对照已实跑（2026-11-14）：把 InitSetting 里的 w.LoadSmsSetting(settingMap[SmsSettingKey])
// 一行删掉后 TestInitSettingDispatchesSmsLoader 变红，恢复后变绿；
// 把 LoadSmsSetting 里的 sms.ApplyDefaults 调用删掉后 TestLoadSmsSettingFillsDefaults 变红。
// 本文件不连库：LoadSmsSetting 只动内存对象，用 &Website{} 即可测真实行为。

// TestSmsSettingKeyConstant 钉住裁决 R1 的精确取值：任务 12 不会再重复添加这个常量，
// 键名错了就会读不到库里的 sms 设置行（settings 表主键列是 key）。
func TestSmsSettingKeyConstant(t *testing.T) {
	if SmsSettingKey != "sms" {
		t.Errorf("SmsSettingKey = %q, want %q", SmsSettingKey, "sms")
	}
}

// TestWebsitePluginSmsIsPointer 钉住规格 §3.1：挂载字段必须是指针，与 PluginSendmail 一致。
func TestWebsitePluginSmsIsPointer(t *testing.T) {
	field, ok := reflect.TypeOf(Website{}).FieldByName("PluginSms")
	if !ok {
		t.Fatal("Website 没有 PluginSms 字段，后续任务读不到短信配置")
	}
	want := reflect.TypeOf((*config.PluginSmsConfig)(nil))
	if field.Type != want {
		t.Errorf("Website.PluginSms 类型 = %s, want %s", field.Type, want)
	}
}

// TestLoadSmsSettingFillsDefaults 钉住裁决 R1 的核心：8 个默认值必须在运行期真被填上，
// 不能只停在 config/sms.go 的字段注释里（任务 8 的 SendSmsCode 直接吃这些值）。
func TestLoadSmsSettingFillsDefaults(t *testing.T) {
	t.Run("空设置值填八个默认值", func(t *testing.T) {
		w := &Website{}
		w.LoadSmsSetting("")
		if w.PluginSms == nil {
			t.Fatal("PluginSms 必须被初始化成非 nil，否则调用方要到处判空")
		}
		assertSmsNumericDefaults(t, w.PluginSms)
	})

	t.Run("库里已有配置也补齐未设字段", func(t *testing.T) {
		w := &Website{}
		w.LoadSmsSetting(`{"provider":"aliyun","code_ttl":600}`)
		if w.PluginSms.Provider != config.SmsProviderAliyun {
			t.Errorf("Provider 未读入：%q", w.PluginSms.Provider)
		}
		if w.PluginSms.CodeTTL != 600 {
			t.Errorf("已配置的 CodeTTL 被改写：%d", w.PluginSms.CodeTTL)
		}
		if w.PluginSms.CodeLength != 6 || w.PluginSms.ResendInterval != 60 || w.PluginSms.LogRetentionDays != 90 {
			t.Errorf("未设字段没补默认值：%+v", w.PluginSms)
		}
	})

	t.Run("CodeLength双向钳制", func(t *testing.T) {
		for _, c := range []struct {
			payload string
			want    int
		}{
			{`{"code_length":3}`, 4},
			{`{"code_length":9}`, 8},
			{`{"code_length":7}`, 7},
		} {
			w := &Website{}
			w.LoadSmsSetting(c.payload)
			if w.PluginSms.CodeLength != c.want {
				t.Errorf("%s → CodeLength=%d, want %d", c.payload, w.PluginSms.CodeLength, c.want)
			}
		}
	})

	t.Run("区域默认值一并填好", func(t *testing.T) {
		w := &Website{}
		w.LoadSmsSetting(`{"provider":"aliyun"}`)
		if w.PluginSms.Aliyun.Domestic.Endpoint != "dysmsapi.aliyuncs.com" {
			t.Errorf("国内 endpoint = %q", w.PluginSms.Aliyun.Domestic.Endpoint)
		}
		if w.PluginSms.Aliyun.International.Endpoint != "dysmsapi.ap-southeast-1.aliyuncs.com" {
			t.Errorf("国际 endpoint = %q", w.PluginSms.Aliyun.International.Endpoint)
		}
		if w.PluginSms.Aliyun.Domestic.ParamName != "code" {
			t.Errorf("ParamName = %q", w.PluginSms.Aliyun.Domestic.ParamName)
		}
		if w.PluginSms.Tencent.Domestic.Region != "ap-guangzhou" {
			t.Errorf("腾讯云 Region = %q", w.PluginSms.Tencent.Domestic.Region)
		}
	})

	t.Run("读的是库里真值_加载阶段不掩码", func(t *testing.T) {
		w := &Website{}
		w.LoadSmsSetting(`{"provider":"aliyun","aliyun":{"domestic":{"access_key_id":"fake-id","access_key_secret":"fake-secret","sign_name":"FakeSign","templates":{"login":"SMS_1"}}}}`)
		got := w.PluginSms.Aliyun.Domestic
		if got.AccessKeyId != "fake-id" || got.AccessKeySecret != "fake-secret" {
			t.Errorf("加载阶段不得掩码（掩码属任务 12 的读端点）：%+v", got)
		}
		if got.SignName != "FakeSign" || got.Templates["login"] != "SMS_1" {
			t.Errorf("签名／模板未原样读入：%+v", got)
		}
	})

	t.Run("非法JSON不panic且仍填默认值", func(t *testing.T) {
		w := &Website{}
		w.LoadSmsSetting(`{"provider":`)
		if w.PluginSms == nil {
			t.Fatal("非法 JSON 也必须留下可用的配置对象")
		}
		assertSmsNumericDefaults(t, w.PluginSms)
	})

	t.Run("重新加载不残留上一份配置", func(t *testing.T) {
		w := &Website{}
		w.LoadSmsSetting(`{"provider":"twilio","code_ttl":900,"twilio":{"international":{"account_sid":"AC_fake"}}}`)
		w.LoadSmsSetting("")
		if w.PluginSms.Provider != "" || w.PluginSms.CodeTTL != 300 || w.PluginSms.Twilio.International.AccountSid != "" {
			t.Errorf("重载后残留了旧配置：%+v", w.PluginSms)
		}
	})
}

// TestInitSettingDispatchesSmsLoader 用 go/ast 钉住挂载链路：InitSetting 必须真的调
// w.LoadSmsSetting(settingMap[SmsSettingKey])。只加常量不加调用 = 配置永远读不到。
func TestInitSettingDispatchesSmsLoader(t *testing.T) {
	src, err := os.ReadFile("setting.go")
	if err != nil {
		t.Fatalf("读取 setting.go 失败：%v", err)
	}
	pairs, err := initSettingDispatchPairs(src)
	if err != nil {
		t.Fatalf("提取 InitSetting 分派表失败（护栏会空转）：%v", err)
	}
	if got := pairs["LoadSmsSetting"]; got != "SmsSettingKey" {
		t.Errorf("InitSetting 里 LoadSmsSetting 的分派 = %q, want SmsSettingKey（全表：%v）", got, pairs)
	}
	// 非空转对照：同一提取器必须看得到既有的 sendmail 分派。
	if got := pairs["LoadSendmailSetting"]; got != "SendmailSettingKey" {
		t.Errorf("提取器没看到既有 sendmail 分派（得到 %q），说明它抓错了函数：%v", got, pairs)
	}

	t.Run("反向对照_删掉分派行必须检出", func(t *testing.T) {
		broken := removeSmsDispatchLine(string(src))
		if broken == string(src) {
			t.Fatal("反向对照准备失败：源码里没有 w.LoadSmsSetting(settingMap[SmsSettingKey]) 这一行")
		}
		got, err := initSettingDispatchPairs([]byte(broken))
		if err != nil {
			t.Fatalf("删掉一行分派不该让提取器报错：%v", err)
		}
		if _, ok := got["LoadSmsSetting"]; ok {
			t.Fatal("反向对照不成立：删掉分派行后提取器仍说它在")
		}
		if got["LoadSendmailSetting"] != "SendmailSettingKey" {
			t.Fatal("反向对照把整张分派表都弄丢了，说明删错了位置")
		}
	})
}

// assertSmsNumericDefaults 逐个核对全局约束里的 8 个默认值。
func assertSmsNumericDefaults(t *testing.T, cfg *config.PluginSmsConfig) {
	t.Helper()
	checks := []struct {
		name string
		got  int
		want int
	}{
		{"CodeLength", cfg.CodeLength, 6},
		{"CodeTTL", cfg.CodeTTL, 300},
		{"ResendInterval", cfg.ResendInterval, 60},
		{"MaxVerifyAttempts", cfg.MaxVerifyAttempts, 5},
		{"DailyLimitPerPhone", cfg.DailyLimitPerPhone, 10},
		{"HourlyLimitPerIp", cfg.HourlyLimitPerIp, 10},
		{"DailyLimitPerIp", cfg.DailyLimitPerIp, 30},
		{"LogRetentionDays", cfg.LogRetentionDays, 90},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

// initSettingDispatchPairs 提取 InitSetting 里所有 w.LoadXxx(settingMap[YyyKey]) 分派，
// 返回 LoadXxx → YyyKey。找不到函数或一条分派都没有时返回 error，避免护栏空转。
func initSettingDispatchPairs(src []byte) (map[string]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "setting.go", src, 0)
	if err != nil {
		return nil, err
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "InitSetting" {
			fn = fd
			break
		}
	}
	if fn == nil || fn.Body == nil {
		return nil, errors.New("未找到 InitSetting 函数体")
	}
	pairs := map[string]string{}
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !strings.HasPrefix(sel.Sel.Name, "Load") {
			return true
		}
		if recv, ok := sel.X.(*ast.Ident); !ok || recv.Name != "w" {
			return true
		}
		index, ok := call.Args[0].(*ast.IndexExpr)
		if !ok {
			return true
		}
		mapIdent, ok := index.X.(*ast.Ident)
		if !ok || mapIdent.Name != "settingMap" {
			return true
		}
		keyIdent, ok := index.Index.(*ast.Ident)
		if !ok {
			return true
		}
		pairs[sel.Sel.Name] = keyIdent.Name
		return true
	})
	if len(pairs) == 0 {
		return nil, errors.New("InitSetting 里没提取到任何 settingMap 分派")
	}
	return pairs, nil
}

// removeSmsDispatchLine 删掉 sms 的那一行分派，用于反向对照。
func removeSmsDispatchLine(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, "w.LoadSmsSetting(settingMap[SmsSettingKey])") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// TestLoadSmsSettingAcceptsMarshalledConfig 用真实落库格式交叉核对：
// settings.value 存的是 json.Marshal(PluginSmsConfig) 的产物（SaveSettingValue），
// 上面几条用例手写 JSON 字面量，这条确保序列化往返后字段与默认值都对得上。
func TestLoadSmsSettingAcceptsMarshalledConfig(t *testing.T) {
	cfg := &config.PluginSmsConfig{
		Provider:   config.SmsProviderTencent,
		CodeLength: 5,
		Scenes:     []string{"login", "register"},
	}
	cfg.Tencent.Domestic.SecretId = "fake-id"
	cfg.Tencent.Domestic.SecretKey = "fake-key"
	cfg.Tencent.Domestic.SmsSdkAppId = "1400000001"
	cfg.Tencent.Domestic.SignName = "FakeSign"
	cfg.Tencent.Domestic.Templates = map[string]string{"login": "1001"}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	w := &Website{}
	w.LoadSmsSetting(string(raw))
	if w.PluginSms.Provider != config.SmsProviderTencent || w.PluginSms.CodeLength != 5 {
		t.Errorf("序列化往返后配置丢失：%+v", w.PluginSms)
	}
	if len(w.PluginSms.Scenes) != 2 || w.PluginSms.Tencent.Domestic.Templates["login"] != "1001" {
		t.Errorf("Scenes／Templates 未读入：%+v", w.PluginSms)
	}
	if w.PluginSms.CodeTTL != 300 || w.PluginSms.Tencent.Domestic.Region != "ap-guangzhou" {
		t.Errorf("未设字段没补默认值：%+v", w.PluginSms)
	}
}
