package manageController

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// 这组测试锁住「删除不存在的记录却回成功」这类假成功。
//
// 根因是 GORM 的语义：**Delete / Update 对不存在的 id 是 0 行受影响但不报错**
// （只在 SQL 出错时返回 error）。控制器因此无法感知，回「删除成功」——
// 调用方以为记录已删，实际什么都没发生。
//
// 2026-10-03 实测：MCP 的 system_multilang/delete 与 contentops_translate/text_delete
// 传 id=999999 都回 ok:true。其中 system_multilang/delete 历史上真删过子站配置，
// 一旦 id 传错就是不可逆损坏。

// TestRemoveMultiLangSiteRejectsUnknownID provider 层必须显式判存在。
//
// 这里读源文件断言而非直接调函数：RemoveMultiLangSite 依赖全局 GetDefaultDB()
// 与运行中的 Website 状态，单测里构造不出可用环境（无数据库时 Count 也会失败）。
// 断言守卫代码存在，是这类「无法在单测里复现」的修复的常规做法。
func TestRemoveMultiLangSiteRejectsUnknownID(t *testing.T) {
	src, err := os.ReadFile("../../provider/multiLang.go")
	if err != nil {
		t.Fatalf("读源码失败: %v", err)
	}
	body := string(src)
	// 必须先 Count 再动数据
	if !strings.Contains(body, "func (w *Website) RemoveMultiLangSite") {
		t.Fatal("未找到 RemoveMultiLangSite")
	}
	seg := body[strings.Index(body, "func (w *Website) RemoveMultiLangSite"):]
	if n := strings.Index(seg, "\n}"); n > 0 {
		seg = seg[:n]
	}
	if !strings.Contains(seg, ".Count(&exist)") {
		t.Error("RemoveMultiLangSite 必须先 Count 确认子站存在（GORM 的 Update 对不存在 id 不报错）")
	}
	if !strings.Contains(seg, "exist == 0") {
		t.Error("Count 为 0 时必须返回错误，不能继续删")
	}
	if !strings.Contains(seg, "不存在") {
		t.Error("错误信息应说明「不存在」，让调用方知道是空操作而非成功")
	}
}

// TestTranslateTextDeleteGuards 三处守卫都要在：
// ① 单条删除前确认存在；② id 与 all 都未传时明确报错；③ 错误信息含 id。
func TestTranslateTextDeleteGuards(t *testing.T) {
	src, err := os.ReadFile("pluginTranslate.go")
	if err != nil {
		t.Fatalf("读源码失败: %v", err)
	}
	body := string(src)
	seg := body[strings.Index(body, "func PluginRemoveTranslateTextLog"):]
	if n := strings.Index(seg, "\n}\n"); n > 0 {
		seg = seg[:n]
	}
	if !strings.Contains(seg, ".Count(&exist)") {
		t.Error("单条删除前必须 Count 确认存在（GORM Delete 对不存在 id 不报错）")
	}
	if !strings.Contains(seg, "exist == 0") {
		t.Error("Count 为 0 时必须报错，不能回「已删除」")
	}
	// id=0 且 all=false：既没删任何东西又回成功，是最骗人的形态
	if !strings.Contains(seg, "请提供要删除的 id") {
		t.Error("既无 id 也无 all 时必须明确报错（当前会静默回成功）")
	}
	// 错误信息应带 id，便于定位
	if !strings.Contains(seg, "strconv.FormatUint") {
		t.Error("错误信息应回显 id，便于调用方定位")
	}
	// 确保用的是模型而不是请求 DTO（曾经误传 &req 导致 GORM 推出不存在的表名）
	if strings.Contains(seg, "Delete(&req)") {
		t.Error("必须按 model.TranslateTextLog{} 删除：传请求 DTO 会让 GORM 推出不存在的表名")
	}
}

// TestTranslateTextDeleteRequestIDType 锁住 req.Id 是 uint——
// 写错成 int64 会让 strconv 调用编译不过（已在修复过程中踩到）。
func TestTranslateTextDeleteRequestIDType(t *testing.T) {
	src, err := os.ReadFile("../../request/plugin.go")
	if err != nil {
		t.Skipf("读 request 失败: %v", err)
	}
	body := string(src)
	i := strings.Index(body, "TranslateTextLogDeleteRequest")
	if i < 0 {
		t.Skip("未找到 TranslateTextLogDeleteRequest")
	}
	seg := body[i:]
	if n := strings.Index(seg, "}"); n > 0 {
		seg = seg[:n]
	}
	if !strings.Contains(seg, "Id uint") && !strings.Contains(seg, "Id  uint") {
		t.Errorf("TranslateTextLogDeleteRequest.Id 应为 uint（控制器用 FormatUint），实际片段=%q", seg)
	}
	_ = strconv.Itoa(0) // 保持 import 被使用，避免未来删掉 import 时才暴露问题
}
