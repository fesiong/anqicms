package provider

import (
	"os"
	"testing"
)

// 这两个用例打的是真实第三方 API（百度/有道翻译），需要有效凭据。
//
// 原实现把密钥硬编码成 "xxx"/"xxxx"——那不是自动化测试，是手工调试用的草稿：
// 密钥写死在源码里既跑不通（服务端回 UNAUTHORIZED USER）又无法在 CI 注入，
// 只会让 `go test ./provider/` 永远红着，并掩盖真实的回归。
//
// 改为从环境变量读凭据，没配就跳过：
//   - 配了 → 真正验证翻译链路（含签名与结果解析）；
//   - 没配 → t.Skip，如实说明"未验证"，而不是伪装成通过或失败。
//
// 跑法：
//   BAIDU_TRANSLATE_APP_ID=xxx BAIDU_TRANSLATE_APP_SECRET=yyy \
//     go test ./provider/ -run TestNewBaiduTranslate
//   YOUDAO_TRANSLATE_APP_KEY=xxx YOUDAO_TRANSLATE_APP_SECRET=yyy \
//     go test ./provider/ -run TestNewYoudaoTranslate
const translateSample = "安企CMS自推出以来，已经逐步扩展到多站点功能，用户群体也在不断扩大。" +
	"在这个过程中，用户们对于多语言功能的呼声越来越高。" +
	"尤其是在国际化环境中，内容的多语言切换和自动翻译的需求变得尤为迫切。"

// requireTranslateEnv 读取凭据，缺任一项就跳过并说明缺哪个。
func requireTranslateEnv(t *testing.T, keys ...string) map[string]string {
	t.Helper()
	out := make(map[string]string, len(keys))
	var missing []string
	for _, k := range keys {
		v := os.Getenv(k)
		if v == "" {
			missing = append(missing, k)
			continue
		}
		out[k] = v
	}
	if len(missing) > 0 {
		t.Skipf("未配置翻译服务凭据（%v），跳过真实 API 调用；这是环境限制而非代码失败。"+
			"配好后本用例会真正验证翻译链路。", missing)
	}
	return out
}

func TestNewBaiduTranslate(t *testing.T) {
	env := requireTranslateEnv(t, "BAIDU_TRANSLATE_APP_ID", "BAIDU_TRANSLATE_APP_SECRET")
	tr := NewBaiduTranslate(env["BAIDU_TRANSLATE_APP_ID"], env["BAIDU_TRANSLATE_APP_SECRET"])

	content, err := tr.Translate(translateSample, "auto", "en")
	if err != nil {
		t.Fatalf("百度翻译调用失败: %v", err)
	}
	if content == "" {
		t.Fatal("翻译结果为空")
	}
	// 原样返回说明根本没翻，不算成功
	if content == translateSample {
		t.Fatal("返回内容与原文相同，翻译未生效")
	}
	t.Logf("百度翻译结果: %s", content)
}

func TestNewYoudaoTranslate(t *testing.T) {
	env := requireTranslateEnv(t, "YOUDAO_TRANSLATE_APP_KEY", "YOUDAO_TRANSLATE_APP_SECRET")
	tr := NewYoudaoTranslate(env["YOUDAO_TRANSLATE_APP_KEY"], env["YOUDAO_TRANSLATE_APP_SECRET"])

	content, err := tr.Translate(translateSample, "auto", "en")
	if err != nil {
		t.Fatalf("有道翻译调用失败: %v", err)
	}
	if content == "" {
		t.Fatal("翻译结果为空")
	}
	if content == translateSample {
		t.Fatal("返回内容与原文相同，翻译未生效")
	}
	t.Logf("有道翻译结果: %s", content)
}

// TestTranslateRejectsEmptyInput 不需要任何凭据的纯本地校验。
//
// 修 Translate 空输入校验时补的：原先空内容会照样发起网络请求
// （百度那条还会先占住 baiduChan 并 sleep 1s），服务端只会回一个无意义的错误。
//
// 断言的是**本地拦截**而非「返回了错误」——这两者必须区分：
// 服务端对空 q 同样会回错误，所以只断言 err != nil 的话，
// 把本地校验删掉这个用例照样绿（反向对照实测：耗时从 1.3s 涨到 5.6s，
// 说明请求真的发出去了，测试却没抓到）。
// 故此处校验错误文案，且不接受任何来自服务端的措辞。
func TestTranslateRejectsEmptyInput(t *testing.T) {
	const wantMsg = "翻译内容不能为空"

	cases := []struct {
		name string
		in   string
	}{
		{"空串", ""},
		{"纯空格", "   "},
		{"换行与制表", " \n\t "},
	}
	baidu := NewBaiduTranslate("dummy-id", "dummy-secret")
	youdao := NewYoudaoTranslate("dummy-key", "dummy-secret")
	for _, c := range cases {
		out, err := baidu.Translate(c.in, "auto", "en")
		if err == nil {
			t.Errorf("百度翻译对%s应报错，实际返回 %q", c.name, out)
			continue
		}
		if err.Error() != wantMsg {
			t.Errorf("百度翻译对%s应在本地拦截，实际错误来自网络层: %v", c.name, err)
		}
		out, err = youdao.Translate(c.in, "auto", "en")
		if err == nil {
			t.Errorf("有道翻译对%s应报错，实际返回 %q", c.name, out)
			continue
		}
		if err.Error() != wantMsg {
			t.Errorf("有道翻译对%s应在本地拦截，实际错误来自网络层: %v", c.name, err)
		}
	}

	// 非空内容不能被误伤：应走到网络层（假凭据会在那里失败，而非本地拦截）
	if _, err := baidu.Translate("非空", "auto", "en"); err != nil {
		if err.Error() == wantMsg {
			t.Error("非空内容被空输入校验误伤")
		}
	}
}
