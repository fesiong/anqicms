package intent

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// 这组测试锁住 2026-10-03 实测发现的「静默给错数据」问题——都是「不报错但结果错」：
//
//  1. list 的 order_by 填不存在的列时静默返回空列表却 ok=true；
//  2. publish/delete 对不存在的 id 回 ok:true「文章已更新/已删除」；
//     且这两条路径成功时没有 structuredContent，调用方只能去解析中文文案；
//  3. 局部更新清空未传字段 —— 已从「意图层回查补齐」改为「端点 PATCH 语义」
//     （partial=true，由 provider 的 injectPartialUpdate 统一注入）。
//     意图层只保留「哪些端点不需要补齐」的判断，见 catalog_domains.go。
//
// 第 3 条的教训值得记住：补齐（把旧值查出来写回去）是**绕过**症状，
// 根因在端点无条件 `req.UpdateAll = true`。而且补齐本身有害——
// 读端点会覆写字段（GetNavList 用 GetUrl 覆盖 link），拿派生值写回就是改坏数据。

// articleInvoker 造一个按 method+path 应答的 api_invoke 桩，
// 让用例能分别模拟「读回旧文档」与「写请求」。
func articleInvoker(t *testing.T, getResp, postResp string, captured *map[string]any) CapInvoker {
	t.Helper()
	return func(_ context.Context, name string, m map[string]any) (string, error) {
		if name != "api_invoke" {
			t.Errorf("应经 api_invoke 落到真实端点，实际 %q", name)
		}
		method := fmt.Sprint(m["method"])
		p, _ := m["params"].(map[string]any)
		if p == nil {
			p = map[string]any{}
		}
		switch method {
		case "GET":
			return getResp, nil
		case "POST":
			if captured != nil {
				*captured = p
			}
			return postResp, nil
		}
		return `{"ok":true,"data":null}`, nil
	}
}



// TestListRejectsUnknownOrderBy 无效排序列必须报错，不能静默返回空列表。
//
// 实测：order_by="nonexistent_col" 返回 ok=true + list:[] + total:1856。
// 端点 ParseOrderBy 只做词法校验不校验列存在性，MySQL 报 Unknown column
// 而控制器丢弃了 Find 的 error。AI 看到 ok=true 会误判「筛选条件太窄」。
func TestListRejectsUnknownOrderBy(t *testing.T) {
	inv := articleInvoker(t, `{"ok":true,"data":null}`, ``, nil)
	spec := mustSpec(t, "content_article")
	_, err := spec.Compose(context.Background(),
		map[string]any{"action": "list", "order_by": "nonexistent_col"}, inv)
	if err == nil {
		t.Fatal("无效 order_by 应返回 error，而不是静默返回空列表")
	}
	if !strings.Contains(err.Error(), "nonexistent_col") {
		t.Errorf("错误信息应回显用户传的值便于纠正，实际=%v", err)
	}
}

// TestListAcceptsRealOrderColumns 真实列名不能被误伤——白名单的第一道防线是别拦错。
func TestListAcceptsRealOrderColumns(t *testing.T) {
	for _, col := range []string{"created_time", "id", "views", "title"} {
		inv := articleInvoker(t, `{"ok":true,"data":{"total":1,"data":[{"id":1}]}}`, ``, nil)
		spec := mustSpec(t, "content_article")
		if _, err := spec.Compose(context.Background(),
			map[string]any{"action": "list", "order_by": col}, inv); err != nil {
			t.Errorf("合法排序列 %q 被误拦：%v", col, err)
		}
	}
}

// TestListRejectsBadOrderDir order_dir 非法值必须报错，不能被端点静默转成 desc。
func TestListRejectsBadOrderDir(t *testing.T) {
	inv := articleInvoker(t, `{"ok":true,"data":null}`, ``, nil)
	spec := mustSpec(t, "content_article")
	_, err := spec.Compose(context.Background(),
		map[string]any{"action": "list", "order_dir": "sideways"}, inv)
	if err == nil {
		t.Fatal("非法 order_dir 应返回 error")
	}
}

// TestPublishRejectsMissingArchive publish 不存在的 id 必须报错。
//
// 实测：端点 UpdateArchiveStatus 用 GORM Find 批量查，查不到得到空切片、
// error 为 nil，循环零次执行后照样回 ok:true「文章已更新」。
func TestPublishRejectsMissingArchive(t *testing.T) {
	inv := articleInvoker(t,
		`{"ok":false,"code":-1,"msg":"record not found","data":null}`, ``, nil)
	spec := mustSpec(t, "content_article")
	_, err := spec.Compose(context.Background(),
		map[string]any{"action": "publish", "id": 999999, "status": "ok"}, inv)
	if err == nil {
		t.Fatal("对不存在的 id 执行 publish 应返回 error，不能假成功")
	}
	if !strings.Contains(err.Error(), "999999") {
		t.Errorf("错误信息应含出错的 id 便于排查，实际=%v", err)
	}
}

// TestDeleteRejectsMissingArchive delete 同样不能假成功。
func TestDeleteRejectsMissingArchive(t *testing.T) {
	inv := articleInvoker(t,
		`{"ok":false,"code":-1,"msg":"record not found","data":null}`, ``, nil)
	spec := mustSpec(t, "content_article")
	_, err := spec.Compose(context.Background(),
		map[string]any{"action": "delete", "id": 999999}, inv)
	if err == nil {
		t.Fatal("删除不存在的文档应返回 error，不能回「文章已删除」")
	}
}

// TestPublishAndDeleteReturnStructuredReceipt 写操作成功时必须有结构化回执。
//
// 实测：publish/delete 成功时 structuredContent 为 null，调用方只能解析
// 「文章已更新」这类中文文案。意图层的存在意义就是给出稳定结构。
func TestPublishAndDeleteReturnStructuredReceipt(t *testing.T) {
	existResp := `{"ok":true,"data":{"code":0,"msg":"","data":{"id":1882,"title":"t"}}}`

	spec := mustSpec(t, "content_article")
	res, err := spec.Compose(context.Background(),
		map[string]any{"action": "publish", "id": 1882, "status": "ok"},
		articleInvoker(t, existResp, `{"ok":true,"data":{"code":0,"msg":"文章已更新","data":null}}`, nil))
	if err != nil {
		t.Fatalf("上架已有文档不应报错：%v", err)
	}
	pub, _ := res.Data.(map[string]any)
	if pub == nil || fmt.Sprint(pub["id"]) != "1882" {
		t.Errorf("publish 应返回含 id 的结构化回执，实际=%#v", res.Data)
	}
	// status 必须是对外语义（ok/draft），不能是端点要的裸数字 1/0：
	// 意图层契约里 status 一律是字符串（save 的回执就是 "ok"），
	// 回一个 int64 会让 AI 每换动作就要重猜取值含义。
	if got := fmt.Sprint(pub["status"]); got != "ok" {
		t.Errorf("publish 上架回执 status 应为 \"ok\"，实际=%#v", pub["status"])
	}
	if got := fmt.Sprint(pub["status_code"]); got != "1" {
		t.Errorf("status_code 应保留端点数值 1 便于排查，实际=%#v", pub["status_code"])
	}

	res, err = spec.Compose(context.Background(),
		map[string]any{"action": "delete", "id": 1882},
		articleInvoker(t, existResp, `{"ok":true,"data":{"code":0,"msg":"文章已删除","data":null}}`, nil))
	if err != nil {
		t.Fatalf("删除已有文档不应报错：%v", err)
	}
	del, _ := res.Data.(map[string]any)
	if del == nil || fmt.Sprint(del["id"]) != "1882" {
		t.Errorf("delete 应返回含 id 的结构化回执，实际=%#v", res.Data)
	}
	// 正式文档删除是移入回收站（archive_drafts status=99），不是物理删除。
	// 不告诉调用方，它会以为数据没了而放弃恢复，也无法解释「删除后 get 仍查得到」。
	if del["moved_to_trash"] != true {
		t.Errorf("删除正式文档的回执应标注 moved_to_trash=true，实际=%#v", del)
	}
}

// TestPublishPassesThroughWhenExistenceCheckFails 存在性校验读到异常（非 not found）时
// 应放行，不能把端点临时故障误报成「文档不存在」。
func TestPublishPassesThroughWhenExistenceCheckFails(t *testing.T) {
	var posted bool
	inv := func(_ context.Context, _ string, m map[string]any) (string, error) {
		if fmt.Sprint(m["method"]) == "GET" {
			return `{"ok":true,"status":500,"msg":""}`, nil
		}
		posted = true
		return `{"ok":true,"data":{"code":0,"msg":"文章已更新","data":null}}`, nil
	}
	spec := mustSpec(t, "content_article")
	if _, err := spec.Compose(context.Background(),
		map[string]any{"action": "publish", "id": 1882, "status": "ok"}, inv); err != nil {
		t.Fatalf("读取端点 5xx 时不应误报为「文档不存在」：%v", err)
	}
	if !posted {
		t.Error("存在性校验读失败后仍应把写请求发给端点")
	}
}

// TestListOnlyParamsStrippedOnNonListActions 列表专用参数不能流向非 list 的端点。
//
// provider 的穷举门禁 TestIntentParamsRecognizedByEndpoints 抓到过这个问题：
// 它报「order_by/order_dir 未被任何落点端点认识」。根因是这些参数在 get/save/
// publish/delete 时被原样透传，而那些端点（/archive/detail、/archive/status、
// /archive/delete）根本不认识它们——ReadJSON 静默丢弃，调用方却以为
// 分页/排序生效了。这正是该门禁要防的「看得见却调不动」。
func TestListOnlyParamsStrippedOnNonListActions(t *testing.T) {
	listOnly := []string{"page", "page_size", "category_id", "module_id",
		"parent_id", "keyword", "flag", "order_by", "order_dir"}
	// save 走 /archive/detail，该端点认 category_id/module_id/parent_id/keyword/flag
	// —— 它们在 save 上是**有意义的写入字段**，不能一刀切剔除，故 save 单独豁免。
	saveKeeps := map[string]bool{
		"category_id": true, "module_id": true, "parent_id": true,
		"keyword": true, "flag": true,
	}

	for _, action := range []string{"get", "publish", "delete", "save"} {
		for _, k := range listOnly {
			if action == "save" && saveKeeps[k] {
				continue
			}
			var sent map[string]any
			inv := func(_ context.Context, _ string, m map[string]any) (string, error) {
				if fmt.Sprint(m["method"]) == "POST" {
					sent, _ = m["params"].(map[string]any)
					return `{"ok":true,"data":{"code":0,"msg":"","data":{"id":1882}}}`, nil
				}
				// GET 是存在性探测
				return `{"ok":true,"data":{"code":0,"msg":"","data":{"id":1882,"module_id":1,"category_id":13}}}`, nil
			}
			args := map[string]any{"action": action, "id": 1882, k: sampleValFor(k)}
			if action == "save" {
				args["title"] = "新标题"
			}
			spec := mustSpec(t, "content_article")
			if _, err := spec.Compose(context.Background(), args, inv); err != nil {
				// 排序白名单在非 list 场景报错也算合理（没传到端点就行）
				continue
			}
			if sent == nil {
				continue // 该 action 没走到写请求
			}
			if _, ok := sent[k]; ok {
				t.Errorf("action=%s 不应把 %q 透传给端点（端点不认，会被静默忽略）", action, k)
			}
		}
	}
}


// sampleValFor 给不同类型的参数造一个合法样本值。
func sampleValFor(k string) any {
	switch k {
	case "page", "page_size", "category_id", "module_id", "parent_id":
		return 1
	case "order_by":
		return "created_time"
	case "order_dir":
		return "desc"
	case "flag":
		return "h"
	case "keyword":
		return "sample"
	}
	return "sample"
}

// TestListSupportsRecycleStatus 回收站必须可查，否则 AI 删完就找不到这篇文档。
//
// delete 正式文档是「移到 archive_drafts status=99」而非物理删除
// （provider.DeleteArchive），端点 ArchiveList 认 status="delete"。
// 白名单漏了它，AI 只能删完凭回执里的 id 猜，无法确认也无法后续恢复。
func TestListSupportsRecycleStatus(t *testing.T) {
	for _, in := range []string{"delete", "trash", "recycle", "deleted"} {
		var gotStatus string
		inv := func(_ context.Context, _ string, m map[string]any) (string, error) {
			p, _ := m["params"].(map[string]any)
			if p != nil {
				gotStatus = fmt.Sprint(p["status"])
			}
			return `{"ok":true,"data":{"total":1,"data":[{"id":9,"title":"已删"}]}}`, nil
		}
		spec := mustSpec(t, "content_article")
		if _, err := spec.Compose(context.Background(),
			map[string]any{"action": "list", "status": in}, inv); err != nil {
			t.Errorf("status=%q 应被接受（回收站别名）：%v", in, err)
			continue
		}
		if gotStatus != "delete" {
			t.Errorf("status=%q 应归一为端点的 \"delete\"，实际透传 %q", in, gotStatus)
		}
	}
}

// TestListStillRejectsUnknownStatus 加了回收站不能把闸门放松——
// 非法值仍必须报错，绝不能退化成「查草稿表不加过滤」。
func TestListStillRejectsUnknownStatus(t *testing.T) {
	inv := articleInvoker(t, `{"ok":true,"data":null}`, ``, nil)
	spec := mustSpec(t, "content_article")
	if _, err := spec.Compose(context.Background(),
		map[string]any{"action": "list", "status": "bogus"}, inv); err == nil {
		t.Fatal("未知 status 仍应返回 error")
	}
}

// TestArticleDescDocumentsNewGuarantees desc 必须写清新语义，
// 否则后续「优化描述」时无声丢失，调用方又会踩回静默错数据。
func TestArticleDescDocumentsNewGuarantees(t *testing.T) {
	spec := mustSpec(t, "content_article")
	for _, want := range []string{
		"标签/标记",         // 未传的关联字段保真
		"不会静默返回空列表",   // order_by 白名单
		"不会假成功",         // publish/delete 存在性校验
		"moved_to_trash",   // 删除是入回收站而非物理删除
	} {
		if !strings.Contains(spec.Desc, want) {
			t.Errorf("desc 缺少关键承诺 %q，实际=%s", want, spec.Desc)
		}
	}
	props := schemaPropsOf(spec)
	ob, _ := props["order_by"].(map[string]any)
	if ob == nil {
		t.Fatal("order_by 参数定义缺失")
	}
	if d, _ := ob["description"].(string); !strings.Contains(d, "不会静默返回空列表") {
		t.Errorf("order_by 的 desc 应说明非法值会报错，实际=%q", d)
	}
	od, _ := props["order_dir"].(map[string]any)
	enum, _ := od["enum"].([]string)
	if len(enum) != 2 {
		t.Errorf("order_dir 应用 enum 约束 asc/desc，实际=%#v", od)
	}
}
