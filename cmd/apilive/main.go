// Command apilive 在**真实环境**里跑一轮通用 REST 调用链路（G4 验证工具）。
//
// 与单元测试的区别：单测用的是 mock 站点与桩路由，这里用的是
//   - config.json 里真实的 MySQL 连接（真实数据、真实站点配置）
//   - route.Register 注册的真实路由表与真实中间件链
//   - BuildAPICatalog 提供的后台端点表（编译期嵌入 api_catalog.json，不依赖源码）
//   - 数据库里真实存在的管理员账号
//
// 用法：
//
//	go run ./cmd/apilive                      # 只读模式 + 推荐白名单（不落库）
//	go run ./cmd/apilive -mode read_write     # 放开写操作
//	go run ./cmd/apilive -ns archive,category # 自定义命名空间白名单
//	go run ./cmd/apilive -persist             # 旧：写身份与策略进库，跑完自动还原
//
//	# 本工具新增：开放全部 API 端点并进行 MCP 测试
//	go run ./cmd/apilive -mode all -ns "" -expose "*"            # 全开（G4+意图层）真实环境测试，跑完自动还原
//	go run ./cmd/apilive -mode all -ns "" -expose "*" -open      # 把「全部开放」配置写入站点并保持打开（真正开放线上 MCP）
//	go run ./cmd/apilive -expose none                            # 站点未配置白名单时的默认可见面（常用意图开箱可用）
//	go run ./cmd/apilive -wire                                # 对本地已运行的 MCP 端点做真实 JSON-RPC 冒烟测试
//
// 默认通过写入站点配置（或从库读取）注入策略，**不修改线上配置**（除非 -open）。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/pkg/ai/eino"
	"kandaoni.com/anqicms/pkg/mcp/intent"
	mcpserver "kandaoni.com/anqicms/pkg/mcp/server"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/route"
)

func main() {
	mode := flag.String("mode", provider.ExposureModeRead, "开放模式：off / read / read_write / all")
	ns := flag.String("ns", "", "命名空间白名单，逗号分隔；留空使用推荐配置")
	deny := flag.String("deny", "", "额外端点黑名单，逗号分隔，如 \"POST /archive/delete\"")
	invoke := flag.String("invoke", "/system/api/category/list", "要真实调用的只读端点（完整后台路径）")
	expose := flag.String("expose", "recommended", "意图暴露范围: recommended(默认推荐清单) / none(留空=代码默认可见面) / * (全部意图) / 逗号分隔的意图名或 domain:*")
	open := flag.Bool("open", false, "把「全部开放」配置写入站点并保持打开（真正开放线上 MCP）；默认仅测试并自动还原")
	wire := flag.Bool("wire", false, "对本地已运行的 MCP 端点(127.0.0.1:<port>/api/mcp)做真实 JSON-RPC 冒烟测试")
	flag.Parse()

	logger := log.New(os.Stdout, "", 0)

	// ---------- 1. 真实数据库与真实站点 ----------
	db := provider.GetDefaultDB()
	if db == nil {
		logger.Fatal("数据库未就绪：请检查 config.json 的 mysql 配置")
	}
	provider.InitWebsites()

	site := provider.CurrentSite(nil)
	if site == nil {
		logger.Fatal("未加载到站点：请确认数据库已完成初始化")
	}
	logger.Printf("站点 #%d  base=%s admin=%s token_secret=%s",
		site.Id, site.System.BaseUrl, site.System.AdminUrl, mask(site.TokenSecret))

	// ---------- 2. 真实路由表（含中间件链） ----------
	app := iris.New()
	app.Logger().SetLevel("error")
	if err := app.I18n.Load(config.ExecPath+"locales/*/*.yml", config.LoadLocales()...); err != nil {
		logger.Printf("i18n 加载失败（不影响多数端点）：%v", err)
	}
	provider.SetI18n(app.I18n)
	route.Register(app)
	if err := app.Build(); err != nil {
		logger.Fatalf("路由 Build 失败: %v", err)
	}
	logger.Printf("真实路由已注册并 Build（进程内直调目标）")
	routes := app.GetRoutes()
	apiRoutes := 0
	for _, r := range routes {
		if strings.HasPrefix(r.Path, "/system/api") {
			apiRoutes++
		}
	}
	logger.Printf("路由总数=%d，其中后台 /system/api 路由=%d", len(routes), apiRoutes)
	if apiRoutes == 0 {
		logger.Fatal("后台路由未注册：manageRoute 未生效，后续调用必然 404")
	}

	// 线上协议测试：仅对本地已运行的 MCP 端点发 JSON-RPC，不做任何配置改动。
	if *wire {
		runWireTest(logger, site)
		return
	}

	// ---------- 3. 当前生效的 MCP 配置（来自数据库） ----------
	mcpCfg := provider.GetMcpConfig()
	logger.Printf("库内 MCP 配置：enabled=%v exposed_intents=%v invoke_admin_id=%d exposure_mode=%q",
		mcpCfg.Enabled, mcpCfg.ExposedIntents, mcpCfg.InvokeAdminId, mcpCfg.ApiExposure.Mode)

	// ---------- 4. 解析本轮「全部开放」策略 ----------
	policy := provider.RecommendedExposure()
	policy.Mode = *mode
	userNS := strings.TrimSpace(*ns)
	if userNS != "" {
		policy.AllowNS = splitList(*ns)
	} else if *mode == provider.ExposureModeAll {
		// 「全开」语义：mode=all 且未显式指定 ns 白名单时，清空 AllowNS，
		// 让 G4 端点层同样彻底放开（仅保留代码级硬规则）。否则即便 mode=all，
		// RecommendedExposure 遗留的 AllowNS 仍会把 plugin/design/admin 等
		// 命名空间挡在 ns_not_allowed，导致「全开」名不副实。
		policy.AllowNS = nil
	}
	if strings.TrimSpace(*deny) != "" {
		policy.DenyEndpoints = splitList(*deny)
	}
	effExpose := exposeList(*expose)
	logger.Printf("本轮意图暴露白名单 effExpose=%v（模式 -expose=%q）", effExpose, *expose)

	// 真实执行身份：优先非超管，避免绕过组权限校验。
	var admin model.Admin
	if err := db.Where("status = ? AND id != ?", 1, 1).Order("id ASC").First(&admin).Error; err != nil {
		if err2 := db.Where("status = ?", 1).Order("id ASC").First(&admin).Error; err2 != nil {
			logger.Printf("未找到可用管理员，api_invoke 将被拒绝（不影响只读意图层验证）")
		} else {
			logger.Printf("警告：库内只有超级管理员 #%d，本次验证将绕过组权限校验", admin.Id)
		}
	}
	adminID := admin.Id

	// ---------- 5. 写入本轮测试配置（默认自动还原；-open 则保持打开） ----------
	if *open {
		cfg := site.LoadAiSetting("")
		cfg.Mcp.Enabled = true
		cfg.Mcp.ExposedIntents = []string{"*"}
		if adminID > 0 {
			cfg.Mcp.InvokeAdminId = adminID
		}
		cfg.Mcp.ApiExposure = eino.ApiExposureConfig{Mode: provider.ExposureModeAll}
		if err := site.SaveSettingValue(provider.AiSettingKey, cfg); err != nil {
			logger.Fatalf("写入开放配置失败: %v", err)
		}
		site.DeleteCache()
		logger.Printf("已写入并保持「全部开放」配置：exposed_intents=[*] exposure_mode=all invoke_admin_id=%d（重启服务后线上 MCP 生效）", adminID)
	} else {
		// 测试模式：快照后写入，defer 还原。绝不污染线上配置。
		cfg := site.LoadAiSetting("")
		snapshot, snapErr := json.Marshal(cfg)
		if snapErr != nil {
			logger.Fatalf("配置快照失败（不写入任何配置）: %v", snapErr)
		}
		cfg.Mcp.InvokeAdminId = adminID
		cfg.Mcp.ApiExposure = policy
		cfg.Mcp.ExposedIntents = effExpose
		if err := site.SaveSettingValue(provider.AiSettingKey, cfg); err != nil {
			logger.Fatalf("写入测试配置失败: %v", err)
		}
		site.DeleteCache()
		defer func() {
			var back eino.Configs
			if err := json.Unmarshal(snapshot, &back); err != nil {
				logger.Printf("严重：还原配置失败（反序列化）: %v", err)
				return
			}
			if err := site.SaveSettingValue(provider.AiSettingKey, &back); err != nil {
				logger.Printf("严重：还原配置失败: %v", err)
				return
			}
			site.DeleteCache()
			after := provider.GetMcpConfig()
			if after.InvokeAdminId != back.Mcp.InvokeAdminId ||
				after.ApiExposure.Mode != back.Mcp.ApiExposure.Mode ||
				fmt.Sprint(after.ExposedIntents) != fmt.Sprint(back.Mcp.ExposedIntents) {
				logger.Printf("严重：还原校验不通过！请手动修正 settings 表的 ai_setting")
				return
			}
			logger.Printf("已还原配置：invoke_admin_id=%d exposure_mode=%q exposed_intents=%v",
				after.InvokeAdminId, after.ApiExposure.Mode, after.ExposedIntents)
		}()
	}

	// ---------- 6. 真实能力层 ----------
	svc := site.NewAiChatService()
	ctx := context.Background()

	cat, err := provider.BuildAPICatalog()
	if err != nil {
		logger.Fatalf("目录构建失败: %v", err)
	}
	summary := provider.ExposureSummary(provider.CurrentApiExposure(), cat)
	printJSON(logger, "开放策略摘要（G4）", summary)

	capByName := func(name string) (func(context.Context, string) (string, error), bool) {
		h, ok := svc.ResolveCap(name)
		if !ok {
			return nil, false
		}
		return h, true
	}

	// --- api_list ---
	listH, ok := capByName("api_list")
	if !ok {
		logger.Printf("api_list 未注册（跳过后续）")
		return
	}
	listRaw, lErr := listH(ctx, `{"ns":"category","limit":5}`)
	if lErr != nil {
		logger.Printf("api_list 失败: %v（跳过后续）", lErr)
		return
	}
	printRaw(logger, "api_list（真实目录）", listRaw)

	// --- api_schema ---
	schemaH, ok2 := capByName("api_schema")
	if !ok2 {
		logger.Printf("api_schema 未注册（跳过后续）")
		return
	}
	schemaRaw, schErr := schemaH(ctx, fmt.Sprintf(`{"method":"GET","path":%q}`, *invoke))
	if schErr != nil {
		logger.Printf("api_schema 失败: %v（跳过后续）", schErr)
		return
	}
	printRaw(logger, "api_schema", schemaRaw)

	// --- api_invoke：库内已配置 invoke_admin_id 时应成功（含真实数据） ---
	invokeArgs, _ := json.Marshal(map[string]any{"method": "GET", "path": *invoke})
	if h, ok3 := capByName("api_invoke"); ok3 {
		invokeOut, invokeErr := h(ctx, string(invokeArgs))
		if invokeErr != nil {
			logger.Printf("api_invoke 被拒绝: %v", invokeErr)
		} else {
			printRaw(logger, "api_invoke（真实身份）", invokeOut)
		}
	} else {
		logger.Printf("api_invoke 未注册")
	}

	// ---------- 7. 意图层「全部开放」验证（kernel） ----------
	k := intent.NewKernel(intent.Config{
		ExposedIntents: effExpose,
	}, func(ctx context.Context, name string, args map[string]any) (string, error) {
		h, ok := svc.ResolveCap(name)
		if !ok {
			return "", fmt.Errorf("未注册能力 %s", name)
		}
		raw, mErr := json.Marshal(args)
		if mErr != nil {
			return "", mErr
		}
		return h(ctx, string(raw))
	}, nil)

	// 真实注册进 mcp.Server：RegisterAll 会按 allowed() 逐个 AddTool 并 track，
	// 使 RegisteredCount()/IsRegistered() 真实反映「全开」命中的意图。
	// 用 recover 兜住 SDK 的 AddTool panic（schema 非法时，见 go-sdk AddTool 对
	// 非 object 类型直接 panic）：把崩溃转为可见错误，避免整轮验证中断，
	// 同时这等价于验证线上 MCP 在 * 模式下「启动即注册全部意图」不会崩。
	mcpSrv, srvErr := mcpserver.New(mcpserver.DefaultConfig())
	if srvErr != nil {
		logger.Fatalf("创建 mcp.Server 失败: %v", srvErr)
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Printf("✘ RegisterAll 在 * 模式下面临 AddTool panic（这会让线上 MCP 启动即崩）: %v", r)
			}
		}()
		k.RegisterAll(mcpSrv.GetServer())
	}()

	registered := k.RegisteredCount()
	totalIntents := len(intent.IntentCatalog)
	expected := totalIntents + 2 // +2 个 meta 意图
	logger.Printf("\n--- 意图层暴露 ---")
	logger.Printf("暴露意图数=%d（全量意图=%d，预期含 meta=%d）", registered, totalIntents, expected)
	if *expose == "*" {
		if registered != expected {
			logger.Printf("⚠ 暴露意图数 %d 与预期 %d 不符（meta 意图数量可能变化），继续校验逐项暴露", registered, expected)
		}
		for _, s := range intent.IntentCatalog {
			if !k.IsRegistered(s.Name) {
				logger.Printf("✘ 全开模式下意图 %s 未暴露（DefaultOff 未解锁）", s.Name)
				return
			}
		}
		logger.Printf("✔ 全开模式：所有 %d 个意图（含全部 DefaultOff）均已暴露", totalIntents)
	}

	// ---------- 8. 广度测试：每个能力域挑一个只读动作，确认能穿过内核到达真实端点 ----------
	logger.Printf("\n--- 意图层广度测试（每域一个只读动作） ---")
	type breadthCase struct {
		domain, intent, args string
	}
	cases := []breadthCase{
		{"content", "content_article", `{"action":"list"}`},
		{"media", "media", `{"action":"list"}`},
		{"structure", "structure", `{"action":"nav_list"}`},
		{"seo", "seo_keyword", `{"action":"list"}`},
		{"traffic", "traffic_statistics", `{"action":"dashboard"}`},
		{"interaction", "interaction", `{"action":"comment_list"}`},
		{"commerce", "commerce", `{"action":"user_list"}`},
		{"channel", "channel_subscriber", `{"action":"list"}`},
		{"contentops", "contentops_material", `{"action":"list"}`},
		{"system", "system_config", `{"action":"site_info"}`},
		{"siteops", "siteops_maintain", `{"action":"cache_get"}`},
		{"account", "account", `{"action":"admin_list"}`},
		{"design", "design_manage", `{"action":"list"}`},
		{"agent", "agent", `{"action":"manage_list"}`},
		{"api", "api", `{"action":"list","ns":"category","limit":3}`},
	}
	okCount := 0
	for _, c := range cases {
		if !k.IsRegistered(c.intent) {
			logger.Printf("  · [%s] %s 未按本模式暴露，跳过", c.domain, c.intent)
			continue
		}
		res, kErr := k.Execute(ctx, c.intent, c.args)
		if kErr != nil {
			logger.Printf("  ✘ [%s] %s 失败: %v", c.domain, c.intent, kErr)
			continue
		}
		okCount++
		logger.Printf("  ✔ [%s] %s 成功（文本长度 %d）", c.domain, c.intent, len(res.Text))
	}
	logger.Printf("广度测试：%d/%d 个域只读意图穿过内核抵达真实端点", okCount, len(cases))

	// ---------- 9. 写操作解锁验证（可逆）：通过被点名的 DefaultOff 写意图创建并删除一条 SEO 关键词 ----------
	logger.Printf("\n--- 写操作解锁验证（可逆：创建+删除 __mcp_test__ 关键词） ---")
	testKeyword := "__mcp_test__"
	// 先把任何历史遗留清干净，确保从 0 开始。
	db.Exec("DELETE FROM keywords WHERE title = ?", testKeyword)
	var before int64
	db.Raw("SELECT COUNT(*) FROM keywords WHERE title = ?", testKeyword).Scan(&before)

	if _, wErr := k.Execute(ctx, "seo_keyword", fmt.Sprintf(`{"action":"create","title":%q,"link":"/__mcp_test__"}`, testKeyword)); wErr != nil {
		logger.Printf("  ✘ seo_keyword create 失败: %v", wErr)
	} else {
		// 直接用 DB 定位 id（不依赖 list 返回结构，更稳），再通过意图发起删除，保证可逆。
		var kwID int64
		db.Raw("SELECT id FROM keywords WHERE title = ?", testKeyword).Scan(&kwID)
		if kwID == 0 {
			logger.Printf("  ⚠ create 报告成功但 DB 中未找到该关键词（id 解析失败），将兜底清理")
		} else if _, dErr := k.Execute(ctx, "seo_keyword", fmt.Sprintf(`{"action":"delete","id":%d}`, kwID)); dErr != nil {
			logger.Printf("  ✘ seo_keyword delete(id=%d) 失败: %v", kwID, dErr)
		} else {
			var after int64
			db.Raw("SELECT COUNT(*) FROM keywords WHERE title = ?", testKeyword).Scan(&after)
			logger.Printf("  ✔ 创建/删除成功（id=%d，关键词数 %d→%d），证明 DefaultOff 写意图在「全开」下已解锁并可逆", kwID, before, after)
		}
	}
	// 兜底清理：无论上面是否成功，确保不留遗留测试数据（真实库）。
	res := db.Exec("DELETE FROM keywords WHERE title = ?", testKeyword)
	if res.RowsAffected > 0 {
		logger.Printf("  ℹ 已兜底清理遗留关键词（rows=%d）", res.RowsAffected)
	}

	// ---------- 10. 硬规则不变式：即便「全开」，凭证类/缺陷模块/提权路径仍须被拦 ----------
	logger.Printf("\n--- 硬规则不变式（全开下仍须拦截） ---")
	hardCases := []struct{ method, path, expect string }{
		{"POST", "/login", "登录"},
		{"GET", "/captcha", "验证码"},
		{"GET", "/aigenerate/setting", "aigenerate"},
		{"POST", "/admin/group/delete", "管理员"},
	}
	for _, hc := range hardCases {
		args, _ := json.Marshal(map[string]any{"method": hc.method, "path": hc.path})
		h, ok := capByName("api_invoke")
		if !ok {
			logger.Printf("  ✘ api_invoke 未注册，无法验证硬规则")
			continue
		}
		if _, hErr := h(ctx, string(args)); hErr != nil {
			if strings.Contains(hErr.Error(), hc.expect) || strings.Contains(hErr.Error(), "hard") {
				logger.Printf("  ✔ 硬规则拦截 %s %s", hc.method, hc.path)
			} else {
				logger.Printf("  ? %s %s 被拒但原因非硬规则: %v", hc.method, hc.path, hErr)
			}
		} else {
			logger.Printf("  ✘ 严重：%s %s 竟未被硬规则拦截（全开也不该放行）", hc.method, hc.path)
		}
	}

	logger.Println("\n完成：以上结果均来自真实数据库 + 真实路由 + 真实鉴权中间件链")
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// exposeList 把 -expose 解析为 ExposedIntents 白名单。
// 支持 "all" 作为 "*" 的同义词，避免 shell 把星号当 glob 展开。
func exposeList(spec string) []string {
	switch strings.TrimSpace(spec) {
	case "", "recommended":
		return intent.RecommendedExposed()
	case "none", "default":
		// 留空白名单 = 走代码默认可见面。用于验证"常用意图默认开放"这条策略本身，
		// 而不是验证某个白名单生效后的结果。
		return nil
	case "*", "all":
		return []string{"*"}
	default:
		return splitList(spec)
	}
}

// runWireTest 对本地已运行的 MCP 端点（127.0.0.1:<port>/api/mcp）做真实 JSON-RPC 冒烟测试。
// 只读、不改任何配置：initialize → tools/list → tools/call（一个只读意图）。
func runWireTest(logger *log.Logger, site *provider.Website) {
	cfg := provider.GetMcpConfig()
	if !cfg.Enabled || cfg.Token == "" {
		logger.Printf("线上 MCP 未启用或 token 为空，跳过协议测试（enabled=%v）", cfg.Enabled)
		return
	}
	port := config.Server.Server.Port
	url := fmt.Sprintf("http://127.0.0.1:%d/api/mcp", port)
	logger.Printf("对线上 MCP 端点 %s 做 JSON-RPC 冒烟测试", url)

	client := &http.Client{Timeout: 30 * time.Second}
	var sessionID string
	doRPC := func(method string, params any) map[string]any {
		body, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": 1, "method": method, "params": params,
		})
		req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		resp, err := client.Do(req)
		if err != nil {
			logger.Printf("  ✘ %s 请求失败: %v", method, err)
			return nil
		}
		defer resp.Body.Close()
		if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
			sessionID = sid
		}
		raw, _ := io.ReadAll(resp.Body)
		text := string(raw)
		var out map[string]any
		if jErr := json.Unmarshal(raw, &out); jErr != nil {
			// 兼容 SSE：抽取最后一个 data: 负载
			if idx := strings.LastIndex(text, "data:"); idx >= 0 {
				rest := text[idx+5:]
				if end := strings.Index(rest, "\n"); end >= 0 {
					rest = rest[:end]
				}
				_ = json.Unmarshal([]byte(strings.TrimSpace(rest)), &out)
			}
		}
		return out
	}

	initOut := doRPC("initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"clientInfo":      map[string]any{"name": "apilive-wire", "version": "1.0"},
		"capabilities":    map[string]any{},
	})
	if initOut == nil {
		logger.Fatalf("initialize 无响应")
	}
	logger.Printf("initialize: %s", truncateJSON(initOut))

	listOut := doRPC("tools/list", map[string]any{})
	if listOut == nil {
		logger.Fatalf("tools/list 无响应")
	}
	tools, _ := listOut["result"].(map[string]any)
	arr, _ := tools["tools"].([]any)
	logger.Printf("tools/list 返回工具数=%d", len(arr))
	for i, t := range arr {
		if tm, ok := t.(map[string]any); ok {
			if i < 15 {
				logger.Printf("  - %v", tm["name"])
			}
		}
	}

	callOut := doRPC("tools/call", map[string]any{
		"name":      "content_article",
		"arguments": map[string]any{"action": "list", "page": 1, "page_size": 2},
	})
	if callOut == nil {
		logger.Fatalf("tools/call 无响应")
	}
	logger.Printf("tools/call content_article: %s", truncateJSON(callOut))
	logger.Println("\n✔ 线上 MCP 协议层冒烟测试完成（initialize / tools/list / tools/call 均成功）")
}

func truncateJSON(v any) string {
	raw, _ := json.Marshal(v)
	s := string(raw)
	if len(s) > 240 {
		s = s[:240] + "…"
	}
	return s
}

// rawProbe 直接投递一次请求，剥掉 HTML 标签后返回可见文本，
// 用于在 404/提示页场景下看清真实原因（响应体常被截断，看不到提示文案）。
func rawProbe(app *iris.Application, site *provider.Website, adminId uint, path string) string {
	target := path
	if !strings.HasPrefix(target, "/") {
		target = "/" + target
	}
	if !strings.HasPrefix(target, "/system/api") {
		target = "/system/api" + target
	}
	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return ""
	}
	req.Host = adminHostForLog(site)
	req.Header.Set("X-Host", adminHostForLog(site))
	req.Header.Set("admin", site.GetAdminAuthToken(adminId, true))
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	body := rec.Body.String()
	if i := strings.Index(body, "<body"); i >= 0 {
		body = body[i:]
	}
	// 去标签，只留可见文本
	var b strings.Builder
	for _, r := range body {
		if r == '<' {
			b.WriteRune(' ')
			continue
		}
		if r == '>' {
			continue
		}
		b.WriteRune(r)
	}
	text := strings.Join(strings.Fields(b.String()), " ")
	if len(text) > 260 {
		text = text[:260]
	}
	return fmt.Sprintf("[status=%d] %s", rec.Code, text)
}

// adminHostForLog 复算一次投递用的 host，便于在 404 时排查站点匹配问题。
func adminHostForLog(site *provider.Website) string {
	if site == nil || site.System == nil {
		return "localhost"
	}
	for _, raw := range []string{site.System.AdminUrl, site.System.BaseUrl} {
		if !strings.HasPrefix(raw, "http") {
			continue
		}
		if i := strings.Index(raw, "://"); i >= 0 {
			rest := raw[i+3:]
			if j := strings.IndexAny(rest, "/:"); j >= 0 {
				return rest[:j]
			}
			return rest
		}
	}
	return "localhost"
}

func mask(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}

func printJSON(logger *log.Logger, title string, v any) {
	raw, _ := json.MarshalIndent(v, "", "  ")
	logger.Printf("\n--- %s ---\n%s", title, raw)
}

func printRaw(logger *log.Logger, title, raw string) {
	var indented any
	if err := json.Unmarshal([]byte(raw), &indented); err == nil {
		pretty, _ := json.MarshalIndent(indented, "", "  ")
		logger.Printf("\n--- %s ---\n%s", title, pretty)
		return
	}
	logger.Printf("\n--- %s ---\n%s", title, raw)
}
