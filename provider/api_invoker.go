package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/kataras/iris/v12"
	irisctx "github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/config"
)

// 本文件实现「进程内直调后台 REST 端点」的执行层（G2）。
//
// 为什么可以这样做：anqicms 的前后台 API 与本站的 AI/MCP 服务运行在同一个进程里，
// 因此无需起端口、无需网络栈，直接把 http.Request 交给 iris 的路由即可得到响应。
// 这样既不用为 394 个端点重复实现业务逻辑，也不会绕过任何中间件。
//
// 为什么不是裸调 handler：本实现走的是完整的 iris 栈（中间件链 → 鉴权 → 路由 → handler）。
// 直接调用 manageController.XXX(ctx) 会跳过中间件与鉴权，是明确避免的做法。
//
// 为什么不用 bootstrap.Application：根包 anqicms 已 import 了 provider（见 bootstrap.go），
// provider 反向引用根包会造成循环依赖。因此改由 iris 自己的全局注册表获取实例。

// invokerAppOverride 允许测试注入固定的 Application，避免受同包其他用例注册顺序影响。
// 生产环境恒为 nil。
var invokerAppOverride *iris.Application

// SetInvokerApplicationForTest 仅供单元测试替换被测实例，请勿在生产调用。
func SetInvokerApplicationForTest(app *iris.Application) { invokerAppOverride = app }

// InvokeResult 是一次后台 API 调用的结果。
type InvokeResult struct {
	Status int    `json:"status"`        // HTTP 状态码
	Code   int    `json:"code"`          // 业务码，config.StatusOK(0) 为成功
	Msg    string `json:"msg"`           // 业务提示（多为 i18n 文案）
	Data   any    `json:"data"`          // 成功时的数据部分；解析失败则为 nil
	Raw    string `json:"raw,omitempty"` // 原始响应体，仅在 Data 无法解析时用于排查
	OK     bool   `json:"ok"`            // Status==200 且 Code==StatusOK
	// Extra 保留业务信封里除 code/msg/data 之外的全部顶层字段。
	//
	// 为什么必须有：后台大量 list 端点把分页信息放在信封顶层而不是 data 里，
	// 例如 ArchiveList 返回 {"code","msg","total","exact","data"}。
	// 只解 code/msg/data 会让 total 被静默丢弃，调用方拿到一个裸数组，
	// 既不知道总数也无法翻页——实测 content_article list 就是如此。
	Extra map[string]any `json:"extra,omitempty"`
}

// InvokeError 携带调用失败的原因分类，便于上层区分处理。
type InvokeError struct {
	Stage string // app / host / request / dispatch / response
	Msg   string
}

func (e *InvokeError) Error() string { return fmt.Sprintf("API 调用失败[%s]: %s", e.Stage, e.Msg) }

// getIrisApplication 从 iris 全局注册表获取 Application 实例。
// 返回 nil 表示当前进程尚未注册 iris 应用（例如单测或未启动服务的场景）。
func getIrisApplication() *iris.Application {
	if invokerAppOverride != nil {
		return invokerAppOverride
	}
	// LastApplication 返回最后注册的实例；注释明确允许转换为 *iris.Application。
	if app, ok := irisctx.LastApplication().(*iris.Application); ok && hasRoutes(app) {
		return app
	}
	// 兜底：进程内若存在多个实例（例如测试或嵌入式场景），最后注册的那个可能是空壳，
	// 此时退而选择第一张真正注册过路由的表，避免调用落到 NotFound。
	for _, app := range irisctx.GetApplications() {
		if a, ok := app.(*iris.Application); ok && hasRoutes(a) {
			return a
		}
	}
	if app, ok := irisctx.LastApplication().(*iris.Application); ok {
		return app
	}
	return nil
}

// hasRoutes 判断实例是否已有可用路由表（未 Build 的实例为空表）。
func hasRoutes(app *iris.Application) bool {
	return app != nil && len(app.GetRoutes()) > 0
}

// adminHostOf 推导应当投递给站点的 Host 标识。
//
// 必须与目标站点匹配，否则有两个后果：
//  1. middleware.ParseAdminUrl 会校验 AdminUrl 的主机名是否等于请求的 host，不等即报未登录；
//  2. provider.CurrentSite 按 host 匹配站点，不匹配会回落到默认站点（多站点下打到错的站）。
//
// library.GetHost 会优先读取 X-Host 并自动剥离端口，因此这里只需给出主机名本身。
func adminHostOf(w *Website) string {
	cands := []string{}
	if w != nil && w.System != nil {
		cands = append(cands, w.System.AdminUrl, w.System.BaseUrl)
	}
	for _, raw := range cands {
		if !strings.HasPrefix(raw, "http") {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if h := parsed.Hostname(); h != "" {
			return h
		}
	}
	// 站点未配置域名时回落到 localhost；library.GetHost 会把回环地址归一化为 localhost，
	// 与单站点部署下的默认行为一致。
	return "localhost"
}

// buildQuery 构造 query string。对 key 排序，保证同样的参数得到同样的 URL
// —— 沿用覆盖审计的教训：map 迭代顺序不确定会破坏结果的可复现性。
func buildQuery(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	q := url.Values{}
	for _, k := range keys {
		q.Set(k, scalarToString(params[k]))
	}
	return q.Encode()
}

// scalarToString 把常见入参值转成字符串；复合类型序列化为 JSON，
// 这样数组与对象也能随 query 传递。
func scalarToString(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case json.Number:
		return val.String()
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case int:
		return strconv.Itoa(val)
	case int64:
		return fmt.Sprintf("%d", val)
	case uint:
		return fmt.Sprintf("%d", val)
	default:
		if raw, err := json.Marshal(v); err == nil {
			return string(raw)
		}
		return fmt.Sprintf("%v", v)
	}
}

// InvokeAdminAPI 以指定管理员身份，在进程内调用目标站点的后台 REST 端点。
//
//   - adminId 必须显式传入且大于 0：身份是有意模糊不起来的安全边界，
//     绝不在此处隐式回落到超级管理员（adminId==1 会被 AdminPermission 直接放行）。
//   - path 传完整的后台路径，如 /system/api/archive/detail。
//   - params 的投递策略：GET/DELETE 走 query；其余方法走 JSON body，
//     并同时在 query 放一份，因为不少 handler 同时混用 URLParam 与 ReadJSON。
func (w *Website) InvokeAdminAPI(adminId uint, method, path string, params map[string]any) (*InvokeResult, error) {
	app := getIrisApplication()
	if app == nil {
		return nil, &InvokeError{Stage: "app", Msg: "当前进程未注册 iris 应用，无法进程内调用"}
	}
	if adminId == 0 {
		return nil, &InvokeError{Stage: "request", Msg: "必须显式指定 adminId，不允许隐式的超级管理员身份"}
	}
	if path == "" {
		return nil, &InvokeError{Stage: "request", Msg: "path 不能为空"}
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	method = strings.ToUpper(method)
	if params == nil {
		params = map[string]any{}
	}

	// 上传类端点：先剥离出文件内容（base64 / data URI），剩下的才是普通字段。
	// base64 体积远大于普通字段，必须从 query 中剔除，避免撑爆 URL。
	fileParts, rest := []filePart{}, params
	if len(params) > 0 {
		hint := filenameHintOf(params)
		var err error
		fileParts, rest, err = partitionFileParams(params, hint)
		if err != nil {
			return nil, &InvokeError{Stage: "request", Msg: err.Error()}
		}
	}

	// query：GET/DELETE 只走 query；写方法也附带一份，兼容混用 URLParam 的 handler
	target := path
	query := buildQuery(rest)
	if query != "" {
		target += "?" + query
	}

	body, contentType, err := buildRequestBody(method, fileParts, rest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, target, body)
	if err != nil {
		return nil, &InvokeError{Stage: "request", Msg: err.Error()}
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// host 标识：决定 CurrentSite 命中哪个站点，以及能否通过 ParseAdminUrl 的入口校验。
	//
	// 必须**同时**设置 req.Host 与 X-Host，缺一不可（真实站点实测，勿删其一）：
	//   - X-Host 供应用代码消费：library.GetHost 优先读它并自动去端口；
	//   - req.Host 供 iris 自身消费：路由匹配与部分中间件走标准 HTTP Host，
	//     而进程内请求没有真实网络栈、Request.URL 里不含 host，不设置会导致
	//     站点匹配失败并落到 NotFound —— 表现为 404 提示页，极易被误判成鉴权未通过。
	if host := adminHostOf(w); host != "" {
		req.Host = host
		req.Header.Set("X-Host", host)
	}
	// 站点身份：由目标站点签发短期 JWT（密钥含该站独有的 TokenSecret）
	req.Header.Set("admin", w.GetAdminAuthToken(adminId, true))

	rec := httptest.NewRecorder()
	// Router 必须已 Build（生产由 app.Run 完成），否则 mainHandler 为 nil 会 panic。
	app.ServeHTTP(rec, req)

	res := &InvokeResult{Status: rec.Code}
	rawBody := strings.TrimSpace(rec.Body.String())

	var payload struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if rawBody != "" {
		if err := json.Unmarshal([]byte(rawBody), &payload); err == nil {
			res.Code = payload.Code
			res.Msg = payload.Msg
			if len(payload.Data) > 0 {
				var decoded any
				if e := json.Unmarshal(payload.Data, &decoded); e == nil {
					res.Data = decoded
				}
			}
			res.Extra = envelopeExtras(rawBody)
		} else {
			// 非 JSON 响应（如文件下载、HTML 错误页）不强行解释，保留原文便于排查
			res.Raw = truncateForLog(rawBody, 512)
		}
	}
	res.OK = res.Status == http.StatusOK && res.Code == config.StatusOK
	return res, nil
}

// envelopeExtras 取出业务信封里除 code/msg/data 之外的全部顶层字段。
//
// 背景见 InvokeResult.Extra：ArchiveList 等 list 端点把 total/exact 放在信封顶层，
// 只解 code/msg/data 会让分页信息凭空消失。这里做一次全量解码再剔除已知键，
// 而不是逐个字段硬编码——端点很多，写死必然漏。
func envelopeExtras(rawBody string) map[string]any {
	var all map[string]any
	if err := json.Unmarshal([]byte(rawBody), &all); err != nil {
		return nil
	}
	extra := map[string]any{}
	for k, v := range all {
		switch k {
		case "code", "msg", "data":
			continue
		}
		extra[k] = v
	}
	if len(extra) == 0 {
		return nil
	}
	return extra
}

// buildRequestBody 按场景构造请求体：
//   - 存在文件时必须走 multipart，JSON 无法承载二进制；
//   - 其余写方法走 JSON body；
//   - GET/DELETE 不带 body。
//
// 返回 content type 为空表示不设置该 header。
func buildRequestBody(method string, files []filePart, fields map[string]any) (io.Reader, string, error) {
	if method == http.MethodGet || method == http.MethodDelete {
		return nil, "", nil
	}
	if len(files) > 0 {
		buf, ct, err := buildMultipartBody(fields, files)
		if err != nil {
			return nil, "", &InvokeError{Stage: "request", Msg: err.Error()}
		}
		return buf, ct, nil
	}
	if len(fields) == 0 {
		// POST/PUT 即使一个字段都没有，也必须发一个**合法的空 JSON 对象**。
		//
		// 原因（2026-10-04 实测）：返回 nil body 时 http.NewRequest 会把
		// req.Body 置为 nil，iris 在 ctx.ReadJSON 里对 nil Body 做 io.ReadAll
		// 直接 panic（runtime error: invalid memory address or nil pointer
		// dereference），最后被 recover 兜成 500「服务器内部错误」。
		//
		// 那个提示是误导的：真实原因就是「没传参数」，而 recover 的文案写着
		// 「通常不是参数问题」，AI 会被带偏去查别的地方。实测所有 POST 端点
		// 空 body 调用都这样（anqi/template/download、anqi/skill/edit、
		// website/save …），并非个别端点的问题。
		//
		// 发 `{}` 后，端点走正常的 ReadJSON 成功 → 参数校验分支，
		// 返回「XX 不能为空」这类**可读**的 400 语义错误。
		return bytes.NewReader([]byte("{}")), "application/json", nil
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return nil, "", &InvokeError{Stage: "request", Msg: "参数序列化失败: " + err.Error()}
	}
	return bytes.NewReader(raw), "application/json", nil
}

// filenameHintOf 取出入参里显式声明的文件名，用于补足缺少文件名的上传。
func filenameHintOf(params map[string]any) map[string]string {
	hint := map[string]string{}
	for _, key := range []string{"filename", "file_name"} {
		if v, ok := params[key].(string); ok && strings.TrimSpace(v) != "" {
			hint[key] = v
		}
	}
	return hint
}

// RequireOK 在业务码非 0 时返回错误，便于上层直接返回给用户。
func (r *InvokeResult) RequireOK() (*InvokeResult, error) {
	if r == nil {
		return nil, &InvokeError{Stage: "response", Msg: "无响应"}
	}
	if r.OK {
		return r, nil
	}
	msg := r.Msg
	if msg == "" {
		msg = fmt.Sprintf("HTTP %d / code %d", r.Status, r.Code)
	}
	return r, &InvokeError{Stage: "response", Msg: msg}
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
