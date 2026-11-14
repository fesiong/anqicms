package manageController

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
)

// ---------------------------------------------------------------------------
// 声明式字段清单 + 反射/go-ast 穷举（R96/R102：不写死「12」这个总数常量）。
//
// smsSecretSpecs 是参与鉴权的全部凭据字段，以 config/sms.go 为权威：
//   - 阿里云 Domestic/International 各 AccessKeyId / AccessKeySecret（4）
//   - 腾讯云 Domestic/International 各 SecretId / SecretKey（4）
//   - Twilio 仅 International 的 AccountSid / AuthToken / ApiKeySid / ApiKeySecret（4）
//
// 每项只写「点路径」，读写一律走反射（smsGet/smsSet），避免手写 12 组闭包时
// 抄错一个名字漏掉掩码。反射穷举（TestSmsSecretFieldsMasked）会用 config 结构体的
// 全部 string 叶子反推「应有凭据集合」，保证清单既不多也不少。
// ---------------------------------------------------------------------------

var smsSecretSpecs = []string{
	"Aliyun.Domestic.AccessKeyId",
	"Aliyun.Domestic.AccessKeySecret",
	"Aliyun.International.AccessKeyId",
	"Aliyun.International.AccessKeySecret",
	"Tencent.Domestic.SecretId",
	"Tencent.Domestic.SecretKey",
	"Tencent.International.SecretId",
	"Tencent.International.SecretKey",
	"Twilio.International.AccountSid",
	"Twilio.International.AuthToken",
	"Twilio.International.ApiKeySid",
	"Twilio.International.ApiKeySecret",
}

// smsNonSecretStringPaths 是「不掩码」的全部 string 叶子字段：掩掉它们会让站长
// 无法核对配置（规格 §3.4）。反射穷举会用「全部 string 叶子 − 本白名单」反推凭据集合，
// 因此 config/sms.go 若新增 string 字段，清单没跟上就会变红，而不是静默漏掩码。
var smsNonSecretStringPaths = []string{
	"Provider",
	"Aliyun.Domestic.Endpoint", "Aliyun.Domestic.SignName", "Aliyun.Domestic.ParamName",
	"Aliyun.International.Endpoint", "Aliyun.International.SignName", "Aliyun.International.ParamName",
	"Tencent.Domestic.SmsSdkAppId", "Tencent.Domestic.Region", "Tencent.Domestic.SignName",
	"Tencent.International.SmsSdkAppId", "Tencent.International.Region", "Tencent.International.SignName",
	"Twilio.International.From", "Twilio.International.MessagingServiceSid",
}

// smsStringLeafPaths 反射递归收集 config.PluginSmsConfig 里所有 string 类型叶子字段的点路径。
func smsStringLeafPaths() map[string]bool {
	out := map[string]bool{}
	var walk func(reflect.Value, string)
	walk = func(v reflect.Value, prefix string) {
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			path := f.Name
			if prefix != "" {
				path = prefix + "." + f.Name
			}
			fv := v.Field(i)
			switch fv.Kind() {
			case reflect.String:
				out[path] = true
			case reflect.Struct:
				walk(fv, path)
			}
		}
	}
	walk(reflect.ValueOf(config.PluginSmsConfig{}), "")
	return out
}

// smsFieldByPath 按点路径取可设置字段（仅供本测试文件反射读写用）。
func smsFieldByPath(c *config.PluginSmsConfig, path string) reflect.Value {
	v := reflect.ValueOf(c).Elem()
	for _, part := range strings.Split(path, ".") {
		v = v.FieldByName(part)
		if !v.IsValid() {
			panic("sms 字段路径无效: " + path)
		}
	}
	return v
}

func smsGet(c *config.PluginSmsConfig, path string) string {
	return smsFieldByPath(c, path).String()
}

func smsSet(c *config.PluginSmsConfig, path, val string) {
	smsFieldByPath(c, path).SetString(val)
}

// smsFullConfig 构造「全字段都有可辨认真值」的配置，便于逐字段断言保留/清空。
func smsFullConfig() config.PluginSmsConfig {
	c := config.PluginSmsConfig{
		Provider:           config.SmsProviderAliyun,
		CodeLength:         6,
		CodeTTL:            300,
		ResendInterval:     60,
		MaxVerifyAttempts:  5,
		DailyLimitPerPhone: 10,
		HourlyLimitPerIp:   10,
		DailyLimitPerIp:    30,
		LogRetentionDays:   90,
		Scenes:             []string{"login", "register"},
	}
	for i, p := range smsSecretSpecs {
		smsSet(&c, p, fmt.Sprintf("REAL_%02d_%s", i, strings.ReplaceAll(p, ".", "_")))
	}
	c.Aliyun.Domestic.Endpoint = "dysmsapi.aliyuncs.com"
	c.Aliyun.Domestic.SignName = "SignDom"
	c.Aliyun.Domestic.ParamName = "code"
	c.Aliyun.Domestic.Templates = map[string]string{"login": "SMS_D"}
	c.Aliyun.International.Endpoint = "dysmsapi.ap-southeast-1.aliyuncs.com"
	c.Aliyun.International.SignName = "SignIntl"
	c.Aliyun.International.ParamName = "code"
	c.Aliyun.International.Templates = map[string]string{"login": "SMS_I"}
	c.Tencent.Domestic.SmsSdkAppId = "1400000001"
	c.Tencent.Domestic.Region = "ap-guangzhou"
	c.Tencent.Domestic.SignName = "TxDom"
	c.Tencent.Domestic.Templates = map[string]string{"login": "1001"}
	c.Tencent.International.SmsSdkAppId = "1400000002"
	c.Tencent.International.Region = "ap-singapore"
	c.Tencent.International.SignName = "TxIntl"
	c.Tencent.International.Templates = map[string]string{"login": "1002"}
	c.Twilio.International.From = "+15550001"
	c.Twilio.International.MessagingServiceSid = "MG_fake"
	c.Twilio.International.Texts = map[string]string{"login": "code {{code}}"}
	return c
}

func smsFullConfigPtr() *config.PluginSmsConfig {
	c := smsFullConfig()
	return &c
}

// smsApplyPayload 复现 PluginSmsSettingForm 的「以现值为基底 → MergeJSONPayload →
// 哨兵回写」三步，不开 HTTP 上下文即可测真实语义。它调用的是真实的
// library.MergeJSONPayload 与真实的 restoreSmsSecretSentinels；handler 是否真的
// 按这个序列调用，由 TestSmsSettingFormKeepsNamedReadJSON 的 go/ast 护栏保证。
func smsApplyPayload(t *testing.T, base config.PluginSmsConfig, body string) config.PluginSmsConfig {
	t.Helper()
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("解析 payload %q 失败：%v", body, err)
	}
	req := base
	if err := library.MergeJSONPayload(payload, &req); err != nil {
		t.Fatalf("MergeJSONPayload(%q) 失败：%v", body, err)
	}
	restoreSmsSecretSentinels(&req, &base)
	return req
}

// ---------------------------------------------------------------------------
// 源码级护栏检查器：接受「函数源码字符串」，返回第一个违规原因。
// 同一检查器同时喂「真实源码」与「合成坏源码」，实现三段式反向对照
// （真实绿 / 合成坏源码必红 / 恢复绿），不污染磁盘上的生产文件。
// ---------------------------------------------------------------------------

// smsFuncSource 用 go/ast 从本包源文件里截取某个函数的完整源码（含函数体注释）。
func smsFuncSource(t *testing.T, file, funcName string) string {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("读取 %s 失败：%v", file, err)
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", file, err)
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName {
			continue
		}
		return string(src[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset])
	}
	t.Fatalf("%s 中找不到函数 %s", file, funcName)
	return ""
}

// smsParseFuncSrc 把「单个函数源码」补 package 头后解析成 AST，供检查器做结构判定。
func smsParseFuncSrc(funcSrc string) (*ast.File, error) {
	fset := token.NewFileSet()
	return parser.ParseFile(fset, "pluginSms.go", "package manageController\n"+funcSrc, 0)
}

// smsRequireAnchors 要求源码逐字段命中清单里每个凭据的点路径锚点；漏任一即报错。
// 这是「穷举」而非数总数：清单每项单独生成一个锚点正则，合成漏掉任一字段的
// 源码（把该路径替换掉）必然少一个锚点而判红。
func smsRequireAnchors(src string, specs []string) error {
	for _, p := range specs {
		anchor := regexp.MustCompile(`\.` + regexp.QuoteMeta(p) + `\b`)
		if !anchor.MatchString(src) {
			return fmt.Errorf("缺字段锚点 %s（源码没有逐字段处理这个凭据）", p)
		}
	}
	return nil
}

// smsRequireWriteBack 在字段覆盖之外，还要求回写函数精确匹配哨兵常量：
// 只有 `== SmsSecretMask` 才回写真值，空串必须真的清空。
func smsRequireWriteBack(src string, specs []string) error {
	if err := smsRequireAnchors(src, specs); err != nil {
		return err
	}
	if !regexp.MustCompile(`==\s*SmsSecretMask`).MatchString(src) {
		return errors.New("缺 `== SmsSecretMask` 哨兵判断（空串必须真的清空，只有精确匹配哨兵才回写真值）")
	}
	return nil
}

// smsIsCurrentSitePluginSms 判定表达式是否为 currentSite.PluginSms。
func smsIsCurrentSitePluginSms(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "PluginSms" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "currentSite"
}

// smsCheckValueCopyInSetting 用 go/ast 断言读端点在脱敏前做了 `*currentSite.PluginSms`
// 解引用值拷贝，且不存在 `setting := currentSite.PluginSms` 这种指针浅拷贝。
func smsCheckValueCopyInSetting(funcSrc string) error {
	file, err := smsParseFuncSrc(funcSrc)
	if err != nil {
		return fmt.Errorf("解析函数源码失败：%w", err)
	}
	foundDeref, foundPointerCopy := false, false
	ast.Inspect(file, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || as.Tok != token.DEFINE {
			return true
		}
		for _, rhs := range as.Rhs {
			switch e := rhs.(type) {
			case *ast.StarExpr:
				if smsIsCurrentSitePluginSms(e.X) {
					foundDeref = true
				}
			case *ast.SelectorExpr:
				if smsIsCurrentSitePluginSms(e) {
					foundPointerCopy = true
				}
			}
		}
		return true
	})
	if foundPointerCopy {
		return errors.New("出现 `setting := currentSite.PluginSms` 指针浅拷贝（会抹掉内存真值，2026-10-03 事故）")
	}
	if !foundDeref {
		return errors.New("脱敏前缺 `setting := *currentSite.PluginSms` 解引用值拷贝")
	}
	return nil
}

// smsCheckFormReadJSON 断言写端点：恰好一次具名 ctx.ReadJSON(&req)、req 是
// config.PluginSmsConfig（非 map）、且出现 library.MergeJSONPayload。
// 具名 ReadJSON(&req) 是 cmd/apicatalog 生成字段级 params 的唯一依据，换成 map
// 或改成第二次 ReadJSON 都会让 params 归零（R103/R105）。
func smsCheckFormReadJSON(funcSrc string) error {
	file, err := smsParseFuncSrc(funcSrc)
	if err != nil {
		return fmt.Errorf("解析函数源码失败：%w", err)
	}
	readCount, namedTarget := 0, false
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "ReadJSON" {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || id.Name != "ctx" {
			return true
		}
		readCount++
		if len(call.Args) == 1 {
			if u, ok := call.Args[0].(*ast.UnaryExpr); ok && u.Op == token.AND {
				if rid, ok := u.X.(*ast.Ident); ok && rid.Name == "req" {
					namedTarget = true
				}
			}
		}
		return true
	})
	if readCount != 1 {
		return fmt.Errorf("ctx.ReadJSON 出现 %d 次（必须恰好一次，否则 apicatalog params 归零）", readCount)
	}
	if !namedTarget {
		return errors.New("ctx.ReadJSON 的具名目标 &req 缺失（不能是 map/匿名结构）")
	}
	if !regexp.MustCompile(`var\s+req\s+config\.PluginSmsConfig`).MatchString(funcSrc) {
		return errors.New("req 未声明为具名 config.PluginSmsConfig")
	}
	if regexp.MustCompile(`var\s+req\s+map\[`).MatchString(funcSrc) {
		return errors.New("req 被声明为 map（apicatalog 无法生成字段级 params）")
	}
	if !strings.Contains(funcSrc, "library.MergeJSONPayload") {
		return errors.New("缺 library.MergeJSONPayload（部分更新语义丢失）")
	}
	return nil
}

// ---------------------------------------------------------------------------
// 六条测试
// ---------------------------------------------------------------------------

// TestSmsSecretFieldsMasked：12 个凭据非空才打掩码；不掩码字段原样保留；
// 空凭据保持空串；脱敏后内存真值逐字段不变；反射穷举 + 逐字段反向对照。
func TestSmsSecretFieldsMasked(t *testing.T) {
	// 反射穷举：清单必须与 config 里全部 string 叶子对齐（不写死总数）。
	all := smsStringLeafPaths()
	nonSecret := map[string]bool{}
	for _, p := range smsNonSecretStringPaths {
		nonSecret[p] = true
	}
	specNames := map[string]bool{}
	for _, p := range smsSecretSpecs {
		specNames[p] = true
	}
	for p := range nonSecret {
		if !all[p] {
			t.Fatalf("不掩码白名单里的 %s 不是 config 的 string 字段（清单已过时）", p)
		}
	}
	for p := range specNames {
		if !all[p] {
			t.Fatalf("脱敏清单里的 %s 不是 config 的 string 字段", p)
		}
		if nonSecret[p] {
			t.Fatalf("%s 同时被列为凭据与不掩码，自相矛盾", p)
		}
	}
	// 权威凭据集合 = 全部 string 字段 − 不掩码白名单，必须与清单双向相等。
	wantSecret := map[string]bool{}
	for p := range all {
		if !nonSecret[p] {
			wantSecret[p] = true
		}
	}
	for p := range wantSecret {
		if !specNames[p] {
			t.Errorf("反射发现凭据字段 %s 不在脱敏清单里：会泄露明文 AK/SK", p)
		}
	}
	for p := range specNames {
		if !wantSecret[p] {
			t.Errorf("脱敏清单里的 %s 不是凭据字段：会误掩码非凭据", p)
		}
	}

	// 行为：全真值 config 走脱敏纯函数，逐字段变掩码。
	cfg := smsFullConfig()
	before := cfg
	maskSmsSecrets(&cfg)
	for _, p := range smsSecretSpecs {
		if got := smsGet(&cfg, p); got != SmsSecretMask {
			t.Errorf("凭据 %s 脱敏后 = %q，want %q", p, got, SmsSecretMask)
		}
	}
	// 行为：除 12 个凭据外，其余字段（8 个数值、templates/texts、非凭据 string、
	// provider/scenes）原样保留 —— 把 12 字段设回真值后整份配置应逐字段全等。
	for _, p := range smsSecretSpecs {
		smsSet(&cfg, p, smsGet(&before, p))
	}
	if !reflect.DeepEqual(cfg, before) {
		t.Errorf("脱敏误改了非凭据字段：\n got=%+v\nwant=%+v", cfg, before)
	}

	// 行为：空凭据保持空串（不给空值打掩码，站长要能分清配没配）。
	empty := smsFullConfig()
	for _, p := range smsSecretSpecs {
		smsSet(&empty, p, "")
	}
	emptyBefore := empty
	maskSmsSecrets(&empty)
	if !reflect.DeepEqual(empty, emptyBefore) {
		t.Errorf("空凭据被打了掩码（应原样保留空串）：got=%+v", empty)
	}

	// 行为侧证据：脱敏后 currentSite.PluginSms 真值逐字段不变。
	// 复现 `setting := *currentSite.PluginSms` 值拷贝形态，证明副本被掩码而内存真值不动。
	global := smsFullConfigPtr()
	snapshot := *global
	setting := *global
	maskSmsSecrets(&setting)
	if !reflect.DeepEqual(*global, snapshot) {
		t.Errorf("值拷贝脱敏污染了内存真值（2026-10-03 事故形态）：\n got=%+v\nwant=%+v", *global, snapshot)
	}
	for _, p := range smsSecretSpecs {
		if got := smsGet(&setting, p); got != SmsSecretMask {
			t.Errorf("副本 %s 未被掩码：%q", p, got)
		}
		if got := smsGet(global, p); got != smsGet(&snapshot, p) {
			t.Errorf("内存真值 %s 被改动：%q", p, got)
		}
	}

	// 源码级护栏（三段式）：脱敏函数必须逐字段覆盖清单全部字段。
	real := smsFuncSource(t, "pluginSms.go", "maskSmsSecrets")
	if err := smsRequireAnchors(real, smsSecretSpecs); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	for _, p := range smsSecretSpecs {
		bad := strings.ReplaceAll(real, p, "MISSING_FIELD")
		if err := smsRequireAnchors(bad, smsSecretSpecs); err == nil {
			t.Errorf("反向对照失败：合成漏掉 %s 的源码仍判绿", p)
		} else {
			t.Logf("反向对照 %s：合成漏字段源码判红 ✓", p)
		}
	}
	if err := smsRequireAnchors(real, smsSecretSpecs); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

// TestSmsSettingMasksValueCopyNotPointer：go/ast 断言读端点脱敏前出现
// *currentSite.PluginSms 解引用；反向对照「setting := currentSite.PluginSms 必红」。
func TestSmsSettingMasksValueCopyNotPointer(t *testing.T) {
	real := smsFuncSource(t, "pluginSms.go", "PluginSmsSetting")
	if err := smsCheckValueCopyInSetting(real); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	bad := strings.Replace(real, "setting := *currentSite.PluginSms", "setting := currentSite.PluginSms", 1)
	if bad == real {
		t.Fatal("反向对照准备失败：真实源码里没有 setting := *currentSite.PluginSms")
	}
	if err := smsCheckValueCopyInSetting(bad); err == nil {
		t.Fatal("反向对照失败：合成 `setting := currentSite.PluginSms` 指针浅拷贝源码仍判绿")
	} else {
		t.Logf("反向对照：合成 `setting := currentSite.PluginSms` 判红 ✓（%v）", err)
	}
	if err := smsCheckValueCopyInSetting(real); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

// TestSmsMaskWriteBackProtected：12 字段逐个跑「哨兵→保留真值 / 新值→更新 /
// 空串→真的清空」；反向对照「删掉任一字段哨兵分支必红」。
func TestSmsMaskWriteBackProtected(t *testing.T) {
	for _, p := range smsSecretSpecs {
		cur := smsFullConfig()
		realVal := "REAL_" + strings.ReplaceAll(p, ".", "_")
		smsSet(&cur, p, realVal)
		cases := []struct {
			label    string
			incoming string
			want     string
		}{
			{"哨兵回传保留真值", SmsSecretMask, realVal},
			{"新值更新", "NEW_" + strings.ReplaceAll(p, ".", "_"), "NEW_" + strings.ReplaceAll(p, ".", "_")},
			{"空串真的清空", "", ""},
		}
		for _, tc := range cases {
			req := cur // 基底：其余 11 个凭据都是真值
			smsSet(&req, p, tc.incoming)
			restoreSmsSecretSentinels(&req, &cur)
			if got := smsGet(&req, p); got != tc.want {
				t.Errorf("%s / %s：got %q, want %q", p, tc.label, got, tc.want)
			}
			for _, other := range smsSecretSpecs {
				if other == p {
					continue
				}
				if got := smsGet(&req, other); got != smsGet(&cur, other) {
					t.Errorf("%s / %s 误改了同批字段 %s：got %q, want %q", p, tc.label, other, got, smsGet(&cur, other))
				}
			}
		}
	}

	// 源码级护栏（三段式）：回写函数必须逐字段覆盖清单，且精确匹配哨兵。
	real := smsFuncSource(t, "pluginSms.go", "restoreSmsSecretSentinels")
	if err := smsRequireWriteBack(real, smsSecretSpecs); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	for _, p := range smsSecretSpecs {
		bad := strings.ReplaceAll(real, p, "MISSING_FIELD")
		if err := smsRequireWriteBack(bad, smsSecretSpecs); err == nil {
			t.Errorf("反向对照失败：合成删掉 %s 哨兵分支的源码仍判绿", p)
		} else {
			t.Logf("反向对照 %s：合成删掉哨兵分支判红 ✓", p)
		}
	}
	if err := smsRequireWriteBack(real, smsSecretSpecs); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

// TestSmsSettingNullNestedMap 钉住 library/merge.go 的真实 null 行为。
// 前端不得提交 null map（templates/texts），见任务 14 的 payload 护栏：
// 顶层厂商 null 是 no-op，而嵌套 region 里的 map 传 null 会被静默清空（数据丢失，界面无感）。
func TestSmsSettingNullNestedMap(t *testing.T) {
	base := smsFullConfig()

	t.Run("顶层aliyun_null_配置不变", func(t *testing.T) {
		// ① 顶层 {"aliyun": null}：json 把 null 解进结构体是 no-op。
		got := smsApplyPayload(t, base, `{"aliyun":null}`)
		if !reflect.DeepEqual(got, base) {
			t.Errorf("顶层 {\"aliyun\":null} 应是 no-op，配置却被改动：\n got=%+v\nwant=%+v", got, base)
		}
		// 裁决 R104 字面还列了 {"aliyun":{"templates":null}}。实测：templates 不是
		// SmsAliyunConfig 的直接字段（它在 domestic/international region 里），
		// json 会忽略这个未知键，同样是不变。这里如实钉住真实行为，不伪造「清空」。
		got2 := smsApplyPayload(t, base, `{"aliyun":{"templates":null}}`)
		if !reflect.DeepEqual(got2, base) {
			t.Errorf("{\"aliyun\":{\"templates\":null}} 实测为 no-op（templates 在 region 层），却被改动：\n got=%+v", got2)
		}
	})

	t.Run("嵌套region_null_map_静默清空", func(t *testing.T) {
		// ② 真正触发静默数据丢失的形态：null 落到 region 层的 map 上。
		got := smsApplyPayload(t, base, `{"aliyun":{"domestic":{"templates":null}}}`)
		if got.Aliyun.Domestic.Templates != nil {
			t.Errorf("aliyun.domestic.templates 应被 null 清空成 nil，实际=%v", got.Aliyun.Domestic.Templates)
		}
		if !reflect.DeepEqual(got.Aliyun.International.Templates, base.Aliyun.International.Templates) {
			t.Errorf("不该清空 international.templates：%v", got.Aliyun.International.Templates)
		}
		if got.Aliyun.Domestic.AccessKeyId != base.Aliyun.Domestic.AccessKeyId {
			t.Errorf("清空 templates 不应连带抹掉同 region 的凭据：%q", got.Aliyun.Domestic.AccessKeyId)
		}
		// texts 同理（Twilio 的本地文案也是 region 层 map）。
		got2 := smsApplyPayload(t, base, `{"twilio":{"international":{"texts":null}}}`)
		if got2.Twilio.International.Texts != nil {
			t.Errorf("twilio.international.texts 应被 null 清空成 nil，实际=%v", got2.Twilio.International.Texts)
		}
	})
}

// TestSmsSettingPartialUpdateKeepsUnsentFields：只提交 {"code_ttl":600} →
// 其余字段（含三个厂商对象全部 12 个凭据）保持原值。
func TestSmsSettingPartialUpdateKeepsUnsentFields(t *testing.T) {
	base := smsFullConfig()
	got := smsApplyPayload(t, base, `{"code_ttl":600}`)
	if got.CodeTTL != 600 {
		t.Fatalf("code_ttl 应更新成 600，实际=%d", got.CodeTTL)
	}
	// 把唯一被改的字段设回原值后，整份配置（含三厂商 12 个凭据、templates/texts、
	// 其余 7 个数值、provider/scenes、非凭据 string）应逐字段全等。
	got.CodeTTL = base.CodeTTL
	if !reflect.DeepEqual(got, base) {
		t.Errorf("只提交 code_ttl 却改动了未提交字段：\n got=%+v\nwant=%+v", got, base)
	}
	// 显式再点名 12 个凭据（审查口径要求逐字段证明）。
	for _, p := range smsSecretSpecs {
		if gotV, baseV := smsGet(&got, p), smsGet(&base, p); gotV != baseV {
			t.Errorf("未提交的凭据 %s 被改动：got %q, want %q", p, gotV, baseV)
		}
	}
}

// TestSmsSettingFormKeepsNamedReadJSON：go/ast 断言 PluginSmsSettingForm 内恰好一次
// 具名 ctx.ReadJSON(&req)（req 非 map）且出现 library.MergeJSONPayload；
// 反向对照「把 ReadJSON 目标换成 map / 出现两次必红」。
func TestSmsSettingFormKeepsNamedReadJSON(t *testing.T) {
	real := smsFuncSource(t, "pluginSms.go", "PluginSmsSettingForm")
	if err := smsCheckFormReadJSON(real); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}

	// 反向对照①：把 ReadJSON 的具名目标换成 map（会让 apicatalog params 归零）必红。
	badMap := strings.Replace(real, "var req config.PluginSmsConfig", "var req map[string]interface{}", 1)
	if badMap == real {
		t.Fatal("反向对照准备失败：真实源码里没有 var req config.PluginSmsConfig")
	}
	if err := smsCheckFormReadJSON(badMap); err == nil {
		t.Fatal("反向对照失败：把 ReadJSON 目标换成 map 的源码仍判绿")
	} else {
		t.Logf("反向对照：合成 map 目标判红 ✓（%v）", err)
	}

	// 反向对照②：ctx.ReadJSON 出现两次（不是恰好一次）必红。
	badTwice := strings.Replace(real, "var req config.PluginSmsConfig", "var req config.PluginSmsConfig\n\t_ = ctx.ReadJSON(&req)", 1)
	if err := smsCheckFormReadJSON(badTwice); err == nil {
		t.Fatal("反向对照失败：ctx.ReadJSON 出现两次仍判绿")
	} else {
		t.Logf("反向对照：合成两次 ReadJSON 判红 ✓（%v）", err)
	}

	if err := smsCheckFormReadJSON(real); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}
