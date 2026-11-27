package graphql

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/i18n"
)

// buildFragmentFanout 构造「query { ...F } fragment F on Query { <field> × n }」
//
// 这正是安全通告里的绕过形状：旧守卫把 *ast.FragmentSpread 记作 1，
// 于是根级复杂度看起来只有 1，放行后由 graphql-go 的
// OverlappingFieldsCanBeMergedRule 对同名字段做 O(n²) 两两比较。
func buildFragmentFanout(field string, n int) string {
	return "query { ...F }\nfragment F on Query { " + strings.TrimSpace(strings.Repeat(field+" ", n)) + " }"
}

// buildInlineFanout 构造同样字段数的内联写法
func buildInlineFanout(field string, n int) string {
	return "{ " + strings.TrimSpace(strings.Repeat(field+" ", n)) + " }"
}

// isLimitError 判断是否属于规模超限类错误
func isLimitError(err error) bool {
	return errors.Is(err, ErrQueryTooDeep) ||
		errors.Is(err, ErrQueryTooComplex) ||
		errors.Is(err, ErrQueryTooManyFields) ||
		errors.Is(err, ErrQueryTooLong)
}

// TestAnalyzeQueryCountsFragmentSpread 锁定「片段引用必须展开计数」。
//
// 反向对照：旧实现（FragmentSpread 记作 1）会让根级字段数从 200 掉回 1，
// 本断言立刻变红 —— 证明这条护栏真的咬住了绕过点，而不是恒真断言。
func TestAnalyzeQueryCountsFragmentSpread(t *testing.T) {
	stats, err := analyzeQuery(buildFragmentFanout("__typename", 200))
	if !errors.Is(err, ErrQueryTooComplex) {
		t.Fatalf("200 个根级字段应判为 %v，实际 err=%v", ErrQueryTooComplex, err)
	}
	if stats == nil {
		t.Fatal("超限时仍应返回统计结果")
	}
	if stats.RootFields != 200 {
		t.Fatalf("片段展开后根级字段数应为 200，实际 %d（片段没有被展开计数？）", stats.RootFields)
	}
}

// TestAnalyzeQueryTreatsInlineAndFragmentAlike 片段形式不得比内联形式更宽松。
//
// 这是绕过的本质：同一个字段集，内联被拒、片段放行，就说明守卫有洞。
func TestAnalyzeQueryTreatsInlineAndFragmentAlike(t *testing.T) {
	_, inlineErr := analyzeQuery(buildInlineFanout("__typename", 200))
	_, fragErr := analyzeQuery(buildFragmentFanout("__typename", 200))

	if inlineErr == nil {
		t.Fatal("内联 200 字段应被拒绝")
	}
	if fragErr == nil {
		t.Fatal("片段形式 200 字段应被拒绝（绕过仍在）")
	}
	if inlineErr.Error() != fragErr.Error() {
		t.Fatalf("片段形式不应比内联形式更宽松：内联 %v，片段 %v", inlineErr, fragErr)
	}
}

// TestAnalyzeQueryRejectsFragmentFanoutQuickly 通告里的 PoC 形状必须被拒，
// 而且守卫自身不能变成新的耗时点。
//
// 字段数取 4000（约 48KB）：必须留在 maxQueryBytes 之内，
// 否则会被长度上限先拦掉，这条断言就成了一条恒真断言（反向对照失效）。
func TestAnalyzeQueryRejectsFragmentFanoutQuickly(t *testing.T) {
	start := time.Now()
	_, err := analyzeQuery(buildFragmentFanout("__typename", 4000))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("4000 字段的片段查询必须被拒绝")
	}
	if elapsed > 3*time.Second {
		t.Fatalf("守卫自身耗时过长：%v", elapsed)
	}
}

// TestAnalyzeQueryCountsEveryOperation 多个 operation 必须全部计入。
//
// 反向对照：旧实现遍历 definitions 时直接覆盖，只看最后一个 operation，
// 把大查询放在前面就能绕过。本断言要求根级字段数取各 operation 的最大值。
func TestAnalyzeQueryCountsEveryOperation(t *testing.T) {
	query := "query Big { " + strings.TrimSpace(strings.Repeat("__typename ", 200)) +
		" }\nquery Small { __typename }"

	stats, err := analyzeQuery(query)
	if !errors.Is(err, ErrQueryTooComplex) {
		t.Fatalf("大 operation 不是最后一个时也必须被拒绝，实际 err=%v", err)
	}
	if stats.RootFields != 200 {
		t.Fatalf("根级字段数应取各 operation 的最大值 200，实际 %d", stats.RootFields)
	}
}

// TestAnalyzeQueryCountsUnusedFragmentDefinition 未被引用的片段定义同样计入。
//
// graphql-go 的校验会遍历文档里的每一个 selection set，没被引用的片段
// 也会进入两两比较，因此不能只统计被引用的片段。
func TestAnalyzeQueryCountsUnusedFragmentDefinition(t *testing.T) {
	query := "query { __typename }\nfragment F on Query { " +
		strings.TrimSpace(strings.Repeat("__typename ", 200)) + " }"

	stats, err := analyzeQuery(query)
	if err == nil || !isLimitError(err) {
		t.Fatalf("未被引用的片段定义同样会被 graphql 校验，必须被拒绝，实际 err=%v", err)
	}
	if stats.MaxSetFields != 200 {
		t.Fatalf("片段定义内的字段数应计入单 set 上限，实际 %d", stats.MaxSetFields)
	}
}

// TestAnalyzeQueryRejectsNestedFragmentFanout 片段放大发生在子选择集里同样要拦。
//
// 根级只有一个字段，复杂度检查看不出来，靠「单个 selection set 展开后的
// 字段数」这一层兜住 —— 这正是 graphql-go 挨打的那个 set。
func TestAnalyzeQueryRejectsNestedFragmentFanout(t *testing.T) {
	query := "query { site { ...F } }\nfragment F on Query { " +
		strings.TrimSpace(strings.Repeat("__typename ", 200)) + " }"

	stats, err := analyzeQuery(query)
	if err == nil || !isLimitError(err) {
		t.Fatalf("嵌套片段放大必须被拒绝，实际 err=%v", err)
	}
	if stats.MaxSetFields != 200 {
		t.Fatalf("子选择集展开后的字段数应为 200，实际 %d", stats.MaxSetFields)
	}
}

// TestAnalyzeQueryAllowsNormalQueries 正常查询必须放行，防止把接口改死。
func TestAnalyzeQueryAllowsNormalQueries(t *testing.T) {
	cases := []string{
		"{ __typename }",
		"query { site { title } }",
		"query { ...F } fragment F on Query { __typename }",
		"query { ... on Query { __typename } }",
		"query { site { title } archives { total items { id title } } }",
		"query A { __typename } query B { __typename }",
	}
	for _, q := range cases {
		if _, err := analyzeQuery(q); err != nil {
			t.Errorf("正常查询不应被拒绝 %q: %v", q, err)
		}
	}
}

// TestAnalyzeQueryRejectsTooDeepQuery 深度限制仍然生效。
func TestAnalyzeQueryRejectsTooDeepQuery(t *testing.T) {
	if _, err := analyzeQuery("{ a { b { c { d { e { f } } } } } }"); !errors.Is(err, ErrQueryTooDeep) {
		t.Fatalf("6 层查询应判为 %v，实际 err=%v", ErrQueryTooDeep, err)
	}
	if _, err := analyzeQuery("{ a { b { c { d { e } } } } }"); err != nil {
		t.Fatalf("5 层查询应放行，实际 err=%v", err)
	}
	// 片段内的深度要算进引用处，深度绕过同样要拦
	deepFragment := "query { site { ...F } }\nfragment F on Query { a { b { c { d { e { f } } } } } }"
	if _, err := analyzeQuery(deepFragment); !errors.Is(err, ErrQueryTooDeep) {
		t.Fatalf("片段内 6 层应判为 %v，实际 err=%v", ErrQueryTooDeep, err)
	}
}

// TestAnalyzeQueryDetectsFragmentCycles 循环片段必须被识别且不能死循环。
func TestAnalyzeQueryDetectsFragmentCycles(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		_, err := analyzeQuery("query { ...F } fragment F on Query { ...F }")
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrFragmentCycle) {
			t.Fatalf("循环片段应判为 %v，实际 err=%v", ErrFragmentCycle, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("循环片段导致分析不收敛（死循环）")
	}
}

// TestAnalyzeQueryRejectsOversizedQuery 查询语句长度上限。
func TestAnalyzeQueryRejectsOversizedQuery(t *testing.T) {
	query := "{ " + strings.Repeat("a ", maxQueryBytes) + " }"
	if _, err := analyzeQuery(query); !errors.Is(err, ErrQueryTooLong) {
		t.Fatalf("超长查询应判为 %v，实际 err=%v", ErrQueryTooLong, err)
	}
}

// TestAnalyzeQueryRejectsInvalidQuery 语法错误必须 fail closed，不能放行给 graphql.Do。
func TestAnalyzeQueryRejectsInvalidQuery(t *testing.T) {
	if _, err := analyzeQuery("{ "); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("语法错误应判为 %v，实际 err=%v", ErrInvalidQuery, err)
	}
}

// TestGraphQLHandlerRejectsFragmentFanout 端到端：未认证的片段放大请求被快速拒绝，
// 同时正常查询照常返回（防止把接口改死）。
func TestGraphQLHandlerRejectsFragmentFanout(t *testing.T) {
	app := iris.New()
	// provider.CurrentSite 在没有注册站点时会造默认站点并读 ctx.GetLocale()，
	// 测试环境没有 i18n 全局模块，这里补一个最小语言包。
	tr := i18n.New()
	if err := tr.LoadKV(i18n.LangMap{"en-US": map[string]interface{}{"anqicms": "anqicms"}}, "en-US"); err != nil {
		t.Fatalf("装载测试语言包失败：%v", err)
	}
	app.I18n = tr
	RegisterRoutes(app.Party("/api"))
	app.Build()

	srv := httptest.NewServer(app)
	defer srv.Close()

	post := func(query string) (string, time.Duration) {
		body, err := json.Marshal(map[string]string{"query": query})
		if err != nil {
			t.Fatalf("构造请求体失败：%v", err)
		}
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/graphql", strings.NewReader(string(body)))
		if err != nil {
			t.Fatalf("构造请求失败：%v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		start := time.Now()
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("请求失败：%v", err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("读取响应失败：%v", err)
		}
		return string(data), time.Since(start)
	}

	// 对照组：正常查询必须照常返回
	data, _ := post("{ __typename }")
	if !strings.Contains(data, `"code":0`) {
		t.Fatalf("正常查询应返回 code 0，实际 %s", data)
	}

	// 攻击组：片段放大（查询语句与请求体都在限额内，由规模守卫拦下）
	data, elapsed := post(buildFragmentFanout("__typename", 2000))
	if !strings.Contains(data, `"code":-1`) {
		t.Fatalf("片段放大查询必须被拒绝，实际 %s", data)
	}
	if !strings.Contains(data, "Query too complex") && !strings.Contains(data, "Query has too many fields") {
		t.Fatalf("拒绝提示应是规模超限，实际 %s", data)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("拒绝耗时过长：%v", elapsed)
	}

	// 攻击组：超大请求体（由请求体限额拦下，同样必须快速拒绝）
	data, elapsed = post(buildFragmentFanout("__typename", 64000))
	if !strings.Contains(data, `"code":-1`) {
		t.Fatalf("超大请求体必须被拒绝，实际 %s", data)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("拒绝耗时过长：%v", elapsed)
	}
}
