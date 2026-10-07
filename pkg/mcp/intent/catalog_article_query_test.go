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
//
// 2026-10-05：order_by 的 enum 从「白名单全量」收窄为「5 个推荐列」后，
// 这个用例从只抽查 title 扩成**遍历白名单每一列**，因为收窄 enum 最容易犯的错
// 是顺手把校验也收窄成推荐集——那样 AI 一填 title/comment_count 就会被拒，
// 而这些列在 archives 里真实存在，属于「能力凭空消失且无报错」。
// 只有推荐集之外的列仍然全通，才能证明 enum 只是展示层的收窄。
func TestListAcceptsRealOrderColumns(t *testing.T) {
	// 遍历白名单全量（含 5 个推荐列与 10 个非推荐列），而非抽查。
	for col := range archiveOrderColumns {
		inv := articleInvoker(t, `{"ok":true,"data":{"total":1,"data":[{"id":1}]}}`, ``, nil)
		spec := mustSpec(t, "content_article")
		if _, err := spec.Compose(context.Background(),
			map[string]any{"action": "list", "order_by": col}, inv); err != nil {
			t.Errorf("合法排序列 %q 被误拦：%v（enum 收窄不应连带放松成「只允许推荐列」）", col, err)
		}
	}
}

// TestOrderByEnumIsRecommendedSubset enum 必须是白名单的**真子集**，且恰为 5 个推荐列。
//
// 这条锁住 2026-10-05 的推荐集本身。**注意它不是锁token**：实测 enum
// 从 14 列缩到 5 列只省约 28 token，而 desc 为说清「推荐而非仅限」多用
// 71 字符，净收益仅 3 token。这里真正要挡的是「候选集退回去」——
// 铺开 14 列会让 AI 在 0 值常量列之间反复权衡（见 archiveOrderColumnList
// 的注释）。三个断言各挡一种回退：
//   - 精确等于推荐集 → 挡住「又加回几个列」；
//   - 真子集（非全量） → 挡住「干脆退回铺开白名单」；
//   - ⊆ 白名单 → 挡住「推荐了白名单里没有的列」，那会让 AI 照着 schema 填值却被闸门拒。
func TestOrderByEnumIsRecommendedSubset(t *testing.T) {
	spec := mustSpec(t, "content_article")
	got := spec.Params["order_by"].Enum
	want := []string{"created_time", "id", "sort", "updated_time", "views"} // 字典序
	if len(got) != len(want) {
		t.Fatalf("order_by enum 应为 %d 个推荐列 %v，实际 %d 个 %v", len(want), want, len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("order_by enum 第 %d 项应为 %q，实际 %q（完整 %v）", i, want[i], got[i], got)
		}
	}
	if len(got) == len(archiveOrderColumns) {
		t.Errorf("order_by enum 已等于白名单全量（%d 列），候选集收窄失效", len(got))
	}
	for _, c := range got {
		if !archiveOrderColumns[c] {
			t.Errorf("order_by enum 含白名单外的列 %q —— AI 照 schema 填值会被 normalizeListOrder 拒绝", c)
		}
	}
}

// TestOrderByEnumIsSorted enum 必须字典序稳定，否则每次 tools/list 响应顺序都不同。
func TestOrderByEnumIsSorted(t *testing.T) {
	for i := 1; i < len(archiveOrderColumnList()); i++ {
		if archiveOrderColumnList()[i-1] > archiveOrderColumnList()[i] {
			t.Fatalf("archiveOrderColumnList 未按字典序：%v", archiveOrderColumnList())
		}
	}
}

// TestOrderByErrorListsAllColumns 报错文案必须列**白名单全量**，不能只列推荐集。
//
// 这是 enum 收窄最容易漏掉的另一半：报错是调用方撞墙后唯一的纠错线索，
// 只列推荐值会让 AI 误以为推荐值即全部，撞到 title 这类真实但不推荐的列时
// 无从修正，只能反复试错或直接放弃排序。
func TestOrderByErrorListsAllColumns(t *testing.T) {
	inv := articleInvoker(t, `{"ok":true,"data":null}`, ``, nil)
	spec := mustSpec(t, "content_article")
	_, err := spec.Compose(context.Background(),
		map[string]any{"action": "list", "order_by": "nonexistent_col"}, inv)
	if err == nil {
		t.Fatal("非法 order_by 应返回 error")
	}
	msg := err.Error()
	// 全量白名单每一列都必须出现在报错里。
	for col := range archiveOrderColumns {
		if !strings.Contains(msg, col) {
			t.Errorf("order_by 报错文案漏列 %q（应列白名单全量 %d 列），实际=%v",
				col, len(archiveOrderColumns), msg)
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
//
// ⚠️ 这条测试同时是 2026-10-05 desc 压缩的**护栏**。压缩只允许删
// 「工具 desc 与参数 desc 之间的重复陈述」，不允许删任何独有语义——
// 因为丢语义不会报错，只会让 AI 悄悄退回到踩坑行为，而测试全绿。
// 下面的清单是压缩前逐条盘出来的，压缩后仍须全部命中。
func TestArticleDescDocumentsNewGuarantees(t *testing.T) {
	spec := mustSpec(t, "content_article")
	// 「不会静默返回空列表」这条承诺在压缩后**移到了 order_by 参数 desc**
	// （它只约束 order_by 一个参数，放在工具 desc 里属于越位陈述）。
	// 承诺必须仍在场，只是换了位置——所以这里查工具与参数的合集，
	// 而具体归属由 TestOrderByDescCarriesSilentDataGuard 单独钉住。
	combined := spec.Desc
	props := schemaPropsOf(spec)
	for name, p := range spec.Params {
		combined += "\n" + name + ":" + p.Desc
	}
	for _, want := range []string{
		"标签/标记",         // 未传的关联字段保真
		"不会静默返回空列表",   // order_by 白名单
		"不会假成功",         // publish/delete 存在性校验
		"moved_to_trash",   // 删除是入回收站而非物理删除
	} {
		if !strings.Contains(combined, want) {
			t.Errorf("工具+参数 desc 合起来缺少关键承诺 %q", want)
		}
	}
	ob, _ := props["order_by"].(map[string]any)
	if ob == nil {
		t.Fatal("order_by 参数定义缺失")
	}
	// enum 收窄成「推荐集」后，desc 措辞必须同步为「推荐」而非「只能是」，
	// 否则会把推荐集说成硬约束：AI 看到 title 不在 enum 里就不再用，
	// 而它其实是合法且有用的排序列（见 TestListAcceptsRealOrderColumns）。
	if d, _ := ob["description"].(string); !strings.Contains(d, "推荐") {
		t.Errorf("order_by 的 desc 应说明这是推荐值而非唯一合法值，实际=%q", d)
	}
	od, _ := props["order_dir"].(map[string]any)
	enum, _ := od["enum"].([]string)
	if len(enum) != 2 {
		t.Errorf("order_dir 应用 enum 约束 asc/desc，实际=%#v", od)
	}
}

// TestOrderByDescCarriesSilentDataGuard 钉住「不会静默返回空列表」这条承诺的归属。
//
// 它只约束 order_by 一个参数。压缩时若把它留在工具 desc，是越位陈述
// （模型会以为 list 整体都有这个保证）；若连它一起删掉，则 order_by 的
// 非法值行为就无人告知——而端点在这点上的行为是「MySQL 报 Unknown column、
// 控制器丢弃 Find 的 error、于是回 ok=true + 空列表」（2026-10-03 实测），
// AI 会据此误判「筛选条件太窄所以没数据」。这类静默错数据比报错危险得多。
func TestOrderByDescCarriesSilentDataGuard(t *testing.T) {
	spec := mustSpec(t, "content_article")
	p, ok := spec.Params["order_by"]
	if !ok {
		t.Fatal("order_by 参数缺失")
	}
	if !strings.Contains(p.Desc, "不会静默返回空列表") {
		t.Errorf("「不会静默返回空列表」这条承诺应在order_by 参数 desc 里，实际=%q", p.Desc)
	}
}

// TestArticleParamsKeepUniqueSemantics 锁住 6 个参数的**独有**语义。
//
// 为什么单独一条：2026-10-05 压缩 desc 时，最大的风险不是删少了（那是显眼的
// 信息缺失），而是删多了——把「status 在 list 与 publish 下语义不同」这类
// 只此一处存在的说明当成冗余删掉。它不会让任何测试变红，只会让 AI 在
// publish 时传错 status（实测 status="1" 会静默返回全部草稿）。
//
// 判定标准是「这句话是否只在本参数出现过一次」：跨层重复的（工具 desc
// 已说过「未传字段沿用原值」）不在此列，由 TestArticleDescNoCrossLayerDup
// 反向保证不重复。
func TestArticleParamsKeepUniqueSemantics(t *testing.T) {
	spec := mustSpec(t, "content_article")

	// 每项：参数名 → 必须出现在该参数 desc 里的语义锚点。
	// 选这些锚点是因为它们各自承载一个「删掉就会误用」的事实。
	required := map[string][]string{
		// status 是最危险的一个：同一字段在 list 是查哪张表、在 publish 是上/下架，
		// 且意图层会把 ok/draft 自动转成端点的 1/0。丢了这段 AI 必然传错。
		"status": {"过滤", "变更动作", "1/0"},
		// flag 的 8 个标记值直接决定前台展示位，模型得知道有哪些可选。
		"flag": {"h=头条", "j=跳转"},
		// 「传空数组则清空」与「不传则沿用」是两种相反语义，必须同时在场，
		// 否则 AI 会把「清空标签」写成「不传 tags」。
		"tags": {"清空", "沿用"},
		// draft 与publish 的关系：draft=false 即发布，省一次调用。
		"draft": {"false=发布", "preview=true"},
		// 排序参数的非法值行为：报错而非静默返回空列表。
		"order_by": {"推荐"},
		"order_dir": {"asc", "desc"},
	}

	for name, anchors := range required {
		p, ok := spec.Params[name]
		if !ok {
			t.Errorf("参数 %s 不应被压缩掉", name)
			continue
		}
		for _, a := range anchors {
			if !strings.Contains(p.Desc, a) {
				t.Errorf("参数 %s 的 desc 丢了独有语义 %q，实际=%q", name, a, p.Desc)
			}
		}
	}
}

// TestArticleDescNoCrossLayerDup 防止「省 token 改出重复劳动」。
//
// desc 压缩的目标之一就是去掉工具 desc 与参数 desc 的重复陈述，
// 但压缩很容易走过头：把工具 desc 里的语义整体复制到每个参数，
// 或者反过来在工具 desc 里堆参数级细节。两种都会让体积反弹，
// 且不会有任何测试报警——所以这里显式钉住「不重复」。
//
// 判据用「更新时不传则沿用原X」这类不变式的出现次数：
// 它只需要在**工具 desc** 说一次，各参数不必再说。
func TestArticleDescNoCrossLayerDup(t *testing.T) {
	spec := mustSpec(t, "content_article")

	// 「沿用原值」这条不变式：工具 desc 说一次即可。
	if n := strings.Count(spec.Desc, "沿用原值"); n != 1 {
		t.Errorf("工具 desc 里「沿用原值」应只出现 1 次（作为全局不变式），实际 %d 次", n)
	}
	// 各参数里不应再复述这条不变式。
	for _, name := range []string{"tags", "flag", "logo", "relation_ids", "content"} {
		p := spec.Params[name]
		if strings.Contains(p.Desc, "更新时不传则沿用") {
			t.Errorf("参数 %s 的 desc 仍在复述「更新时不传则沿用」这条全局不变式（工具 desc 已说过），参数=%q",
				name, p.Desc)
		}
	}
}

// TestArticleDescHasNoStaleWording 抓 desc 之间的措辞漂移。
//
// 2026-10-05 踩过：order_by 参数 desc 已改成「推荐用…也可填」（因为 enum 收窄成
// 推荐集），但工具 desc 里同一件事还写着「只能填 archives 表的真实列名」。
// 两处一矛盾，AI 到底该信哪个无从判断——而这种不一致不会触发任何测试。
//
// 所以凡是参数 desc 已经改用「推荐」口径的地方，工具 desc 必须同步。
func TestArticleDescHasNoStaleWording(t *testing.T) {
	spec := mustSpec(t, "content_article")
	p := spec.Params["order_by"]
	if !strings.Contains(p.Desc, "推荐") {
		t.Skip("order_by 参数 desc 尚未改为推荐口径，本用例暂不适用")
	}
	if strings.Contains(spec.Desc, "order_by 只能填") {
		t.Errorf("order_by 参数 desc 已改为「推荐用」口径，工具 desc 仍写「只能填」，两处矛盾会误导调用方。\n工具 desc=%s", spec.Desc)
	}
}
