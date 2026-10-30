package manageController

import (
	"os"
	"strings"
	"testing"
)

// 端点测试里发现的一类缺陷：写端点在"请求体为空"或"关键参数缺失"时照样回成功。
// 后台表单都是整份提交，空载荷只可能来自误调用或自动化试探；放行的后果是
// 配置被零值覆盖（robots.txt 就是这样被清成 0 字节的）、删除接口什么都不删却回"已删除"。
//
// 下面两个清单是行为契约：清单里的 handler 必须在源码里显式做拒绝校验。
// 只加校验却不登记清单，等于把这条规则留在某个人的记忆里。

// payloadGuardHandlers：必须以 readJSONPayload 读取请求体（空载荷直接拒绝）。
var payloadGuardHandlers = []string{
	"SettingAiForm",
	"SettingCurrencyForm",
	"SettingNavTypeForm",
	"ArchiveDelete",
	"UpdateArchiveSort",
	"CategoryDetailForm",
	"AttachmentChangeCategory",
	"PlaceDetailForm",
	"PluginRobotsForm",
	"PluginLinkDetailForm",
	"PluginAnchorDetailForm",
	"PluginAnchorDelete",
	"PluginGuestbookDelete",
	"PluginGuestbookUpdateStatus",
	"PluginKeywordDelete",
	"PluginKeywordDetailForm",
	"PluginSaveKeywordSetting",
	"PluginMaterialCategoryDetailForm",
	"PluginMaterialDetailForm",
	"PluginUserFieldsDelete",
	"PluginUserGroupDetailForm",
	"PluginWechatMenuSave",
	"PluginWechatReplyRuleForm",
	"PluginSaveOrderReview",
	"PluginSavePaymentAccount",
	"PluginRetailerApply",
	"PluginRetailerSetRealName",
	"PluginTimeFactorSettingSave",
	"PluginSaveMultiLangSite",
	"PluginRemoveMultiLangSite",
	"PluginRemoveTranslateHtmlCache",
	"HandleAiGenerateSettingSave",
	"AuthAiGenerateImageConfirm",
	"HandleSaveCollectSetting",
	"PluginActivityDetailForm",
	"CommunicationOrderClose",
	"CommunicationMessageDelete",
	"SaveCommunicationSetting",
	"AddCouponArchives",
	"CouponDetailForm",
	"DeleteCouponArchives",
	"DeleteCouponCode",
	"PluginGoogleMerchantAccountSelect",
	"PluginGoogleMerchantConfigSave",
	"PluginGoogleMerchantSyncDelete",
	"PluginRemoveBlockedIP",
	"DeleteSubscriberCategory",
	"DeleteDesignInfo",
	"DeleteSettingBanner",
	"PluginSendmailSaveTemplate",
	"PluginSaveShippingSetting",
}

// targetGuardHandlers：删除/定向修改类端点必须校验"目标存在"，否则空载荷或
// 不存在的 id 会静默成功。map 的值是必须出现在 handler 里的校验调用片段。
var targetGuardHandlers = map[string]string{
	"ArchiveDelete":                  "requireFound(ctx, found",
	"UpdateArchiveSort":              "requirePositive(ctx, req.Id",
	"AttachmentChangeCategory":       "requireAnyID(ctx, 0, req.Ids",
	"PlaceDetailForm":                "currentSite.GetPlaceById(req.Id)",
	"PlaceDelete":                    "requirePositive(ctx, req.Id",
	"SettingNavTypeForm":             "currentSite.GetNavTypeById(req.Id)",
	"SettingNavTypeDelete":           "requirePositive(ctx, req.Id",
	"PluginKeywordDetailForm":        "currentSite.GetKeywordById(req.Id)",
	"PluginMaterialDetailForm":       "currentSite.GetMaterialById(req.Id)",
	"PluginUserFieldsDelete":         "requireNonEmpty(ctx, req.FieldName",
	"PluginSaveOrderReview":          "req.Id == 0 && req.OrderId ==",
	"PluginRetailerApply":            "requirePositive(ctx, req.Id",
	"PluginRetailerSetRealName":      "requirePositive(ctx, req.Id",
	"DeleteSubscriberCategory":       "requirePositive(ctx, req.Id",
	"CommunicationMessageDelete":     "requirePositive(ctx, req.Id",
	"CommunicationOrderClose":        "requirePositive(ctx, req.Id",
	"AddCouponArchives":              "requireAnyID(ctx, req.CouponId, req.ArchiveIds",
	"DeleteCouponArchives":           "requirePositive(ctx, req.CouponId",
	"DeleteCouponCode":               "requireAnyID(ctx, req.Id, req.Ids",
	"PluginAnchorDelete":             "requireAnyID(ctx, req.Id, req.Ids",
	"PluginGuestbookDelete":          "requireAnyID(ctx, req.Id, req.Ids",
	"PluginKeywordDelete":            "requireAnyID(ctx, req.Id, req.Ids",
	"PluginRemoveBlockedIP":          "requireNonEmpty(ctx, req.Ip",
	"DeleteSettingBanner":            "requireFound(ctx, found",
	"DeleteDesignInfo":               "requireNonEmpty(ctx, req.Package",
	"PluginRemoveTranslateHtmlCache": "len(req.Uris) == 0",
	"PluginGoogleMerchantSyncDelete": "requireAnyID(ctx, 0, req.Ids",
}

// TestIsEmptyJSONPayload 空载荷的判定口径：只有真"什么都没带"才算空。
func TestIsEmptyJSONPayload(t *testing.T) {
	empty := []string{"", "   ", "\n\t", "{}", "{ }", " {\n } ", "null", " null "}
	for _, body := range empty {
		if !isEmptyJSONPayload([]byte(body)) {
			t.Errorf("空载荷被当成有内容：%q", body)
		}
	}
	full := []string{`{"id":1}`, `{"ids":[]}`, `{"a":0}`, `{"a":null}`, `{"a":""}`}
	for _, body := range full {
		if isEmptyJSONPayload([]byte(body)) {
			t.Errorf("有内容的请求体被当成空：%q", body)
		}
	}
}

// TestWriteHandlersRejectEmptyPayload 源码级护栏：清单里的 handler 必须走 readJSONPayload。
// 只测辅助函数锁不住"handler 忘了用"——那正是原缺陷的形态。
func TestWriteHandlersRejectEmptyPayload(t *testing.T) {
	src := loadControllerSources(t)
	for _, name := range payloadGuardHandlers {
		body, ok := src[name]
		if !ok {
			t.Errorf("清单里的 handler 不存在：%s", name)
			continue
		}
		if !strings.Contains(body, "readJSONPayload(ctx, &req)") {
			t.Errorf("%s 没有用 readJSONPayload 读取请求体，空载荷仍会被放行", name)
		}
		if strings.Contains(body, "ctx.ReadJSON(&req)") {
			t.Errorf("%s 仍在直接 ctx.ReadJSON(&req)，空载荷不会被拒绝", name)
		}
	}
}

// TestWriteHandlersCheckTarget 源码级护栏：删除/定向修改类端点必须校验目标。
func TestWriteHandlersCheckTarget(t *testing.T) {
	src := loadControllerSources(t)
	for name, want := range targetGuardHandlers {
		body, ok := src[name]
		if !ok {
			t.Errorf("清单里的 handler 不存在：%s", name)
			continue
		}
		if !strings.Contains(body, want) {
			t.Errorf("%s 缺少目标校验 %q，空载荷或不存在的 id 会被当成成功", name, want)
		}
	}
}

// loadControllerSources 取本包所有 handler 的函数体，按函数名索引。
func loadControllerSources(t *testing.T) map[string]string {
	t.Helper()
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读取包目录失败：%v", err)
	}
	out := map[string]string{}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(f.Name())
		if err != nil {
			t.Fatalf("读取 %s 失败：%v", f.Name(), err)
		}
		src := string(raw)
		for _, chunk := range strings.Split(src, "\nfunc ") {
			chunk = "func " + chunk
			// 取函数名：func Name( 或 func (r T) Name(
			open := strings.Index(chunk, "(")
			if open < 0 {
				continue
			}
			name := chunk[len("func "):open]
			if strings.HasPrefix(name, "(") || strings.HasPrefix(chunk, "func (") {
				// 方法：func (x *T) Name(...)
				close := strings.Index(chunk, ")")
				if close < 0 {
					continue
				}
				rest := chunk[close+1:]
				p := strings.Index(rest, "(")
				if p < 0 {
					continue
				}
				name = strings.TrimSpace(rest[:p])
			}
			name = strings.TrimSpace(name)
			if name == "" || strings.ContainsAny(name, " \t") {
				continue
			}
			out[name] = chunk
		}
	}
	return out
}
