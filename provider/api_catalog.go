package provider

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// 本文件实现「L0 元数据层」：把后台 REST 端点沉淀成可供 AI 消费的 API 卡片。
//
// 设计要点：
//  1. 参数绑定信息用 go/ast 解析，不用正则。F 阶段与对接方案研究都用血泪验证过：
//     静态正则估算会系统性漏判（例如把 request.Archive、config.BannerItem 这类
//     不以 Request 结尾的具名类型漏掉，导致 schema 可得率被低估一半）。
//  2. 端点清单复用 ParseManageRoutes（已验证与运行时一致的 iris party 嵌套解析），
//     参数 schema 来自源码结构体定义，二者都与代码同步、不手写、不漂移。
//  3. 全部为只读派生，不触发任何真实调用，不涉及鉴权身份问题。
//
// 数据来源与「二进制部署」问题（重要）：
//
//	端点元数据本质上只能从源码派生——handler 内部 `var req request.Archive` 这类绑定关系
//	编译后就消失了，运行时反射只能拿到 handler 的函数指针，拿不到它读了哪个结构体。
//	所以参数绑定信息必须在**编译期**固化下来。
//
// 因此本层有两个来源，按优先级：
//  1. 嵌入表 api_catalog.json（go:embed）——由 cmd/apicatalog 生成，编译进二进制，
//     零源码依赖、零解析开销。格式是一行一条端点，**允许人工编辑**（改 risk/domain/补 desc）。
//  2. 源码解析（go/ast）——仅当嵌入表缺失或为空时回落。开发态可用，生产二进制旁没有
//     .go 源码会失败。
//
// 漂移风险：改了 handler 而没重新生成 → 嵌入表过期。由 api_catalog_gen_test.go 的
// TestCatalogFreshness 比对二者差异并打印明细（APICATALOG_STRICT=1 时转为失败）。
// 重新生成：go generate ./provider 或 go run ./cmd/apicatalog

// paramNameRe 匹配合法参数名，用于从「- 查询参数 current: 说明」里挑出名字。
var paramNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ParamMeta 描述一个入参。
type ParamMeta struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`               // string / integer / number / boolean / array / object
	GoType   string   `json:"go_type,omitempty"`  // 原始 Go 类型，便于排查
	Desc     string   `json:"desc,omitempty"`     // 取自字段行尾注释或上方注释
	Required bool     `json:"required,omitempty"` // 仅当结构体显式标记 validate:"required"
	Enum     []string `json:"enum,omitempty"`
}

// EndpointMeta 是一张 API 卡片：单个端点的可执行元数据。
type EndpointMeta struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Handler  string `json:"handler"`
	NS       string `json:"ns"`       // 一级命名空间：archive / plugin / setting ...
	Resource string `json:"resource"` // plugin 取二级，其余取一级
	// Domain 是按 ns→域 映射表判定的能力域（见 pkg/mcp/intent/domain.go）。
	// 与 NS 的区别：NS 是路由结构，Domain 是语义分组——/plugin/* 一个 NS 装了 229 个端点，
	// 按 NS 无法裁剪，按 Domain 才能把备份/缓存这类高危能力单独关掉。
	Domain string `json:"domain"`
	// Risk 用于调用前的门禁判断：read 可直接放行，write 建议确认，destructive 默认拒绝。
	Risk string `json:"risk"`
	// Desc 是端点的一句话摘要，取自 handler 文档注释的首行（剥掉函数名前缀）。
	// api_list 阶段模型只看得到 method+path，没有摘要时它很难判断端点到底是干什么的。
	Desc string `json:"desc,omitempty"`
	// Doc 是 handler 文档注释的全文（已剔除「参数说明」段落，那部分进了各参数的 Desc）。
	// 只在 api_schema 给出，避免 api_list 体积膨胀。
	Doc string `json:"doc,omitempty"`
	// ParamSource 说明 params 的来路，便于判断 schema 可信度：
	//   struct    —— 结构体反射，字段完整可信
	//   urlparam  —— 逐个 URLParam 读取，仅有 key 与推断类型
	//   form      —— PostValue/FormValue 读取的表单字段
	//   multipart —— 含文件字段，文件内容以 base64 或 data URI 传入
	//   none      —— 无输入参数
	ParamSource string      `json:"param_source"`
	StructType  string      `json:"struct_type,omitempty"` // 如 request.Archive
	Params      []ParamMeta `json:"params"`
}

// APICatalog 是全部端点元数据的集合。
type APICatalog struct {
	Endpoints []EndpointMeta `json:"endpoints"`
	Source    string         `json:"source"`
	// GeneratedAt 仅在嵌入表里有值，标记该份数据的生成时间，便于排查是否过期。
	GeneratedAt string `json:"generated_at,omitempty"`
	// Embedded 标记本次结果来自编译期嵌入表（true）还是运行时源码解析（false）。
	Embedded bool `json:"embedded,omitempty"`
	// StructTouched 是被至少一个端点引用的结构体数量，用于自检 schema 命中情况。
	StructTouched int `json:"struct_touched"`
}

//go:generate go run kandaoni.com/anqicms/cmd/apicatalog

//go:embed api_catalog.json
var embeddedCatalogJSON string

// SourceStats 是分层的统计信息，用于校验与自检。
type SourceStats struct {
	Struct    int
	URLParam  int
	Form      int
	None      int
	Multipart int
}

var (
	catalogMu   sync.Mutex
	cachedCata  *APICatalog
	cachedErr   error
	catalogOnce bool
)

// BuildAPICatalog 返回全部后台端点的元数据。
//
// 优先级：编译期嵌入表（api_catalog.json） > 运行时源码解析。
// 嵌入表是二进制部署下唯一可用的来源，源码解析只是开发态的兜底与对照。
// 结果会被缓存，重复调用返回同一份。
func BuildAPICatalog() (*APICatalog, error) {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	if catalogOnce {
		return cachedCata, cachedErr
	}
	cachedCata, cachedErr = buildAPICatalog()
	catalogOnce = true
	return cachedCata, cachedErr
}

// ResetAPICatalogCache 清空缓存，仅用于测试与热重建场景。
func ResetAPICatalogCache() {
	catalogMu.Lock()
	cachedCata, cachedErr, catalogOnce = nil, nil, false
	catalogMu.Unlock()
}

// CatalogSource 描述本次目录数据的来源，用于诊断。
func (c *APICatalog) CatalogSource() string {
	if c == nil {
		return "nil"
	}
	if c.Embedded {
		return "embedded:" + c.GeneratedAt
	}
	return "source:" + c.Source
}

// buildAPICatalog 按「嵌入表优先、源码兜底」的顺序取目录。
func buildAPICatalog() (*APICatalog, error) {
	if c, err := loadEmbeddedCatalog(); err == nil {
		return c, nil
	}
	// 嵌入表不可用（被清空或格式损坏）才回落源码解析；
	// 生产二进制走到这里通常意味着源码不存在，最终返回 error。
	return buildAPICatalogFromSource()
}

// loadEmbeddedCatalog 解析编译进二进制的端点表。
func loadEmbeddedCatalog() (*APICatalog, error) {
	if strings.TrimSpace(embeddedCatalogJSON) == "" {
		return nil, errors.New("嵌入的端点表为空")
	}
	var c APICatalog
	if err := json.Unmarshal([]byte(embeddedCatalogJSON), &c); err != nil {
		return nil, fmt.Errorf("解析嵌入端点表失败: %w", err)
	}
	if len(c.Endpoints) == 0 {
		return nil, errors.New("嵌入的端点表无端点")
	}
	if c.Source == "" {
		c.Source = "embedded"
	}
	c.Embedded = true
	sortCatalog(c.Endpoints)
	return &c, nil
}

// BuildAPICatalogFromSource 强制从源码重新派生目录，忽略嵌入表。
//
// 用途：生成工具（cmd/apicatalog）产出嵌入表、漂移对比测试取样。
// 二进制部署环境下会因缺少 .go 源码而失败，调用方需要容忍该错误。
func BuildAPICatalogFromSource() (*APICatalog, error) {
	return buildAPICatalogFromSource()
}

// buildAPICatalogFromSource 从源码派生全部后台端点的元数据。
func buildAPICatalogFromSource() (*APICatalog, error) {
	managePath, err := FindManageRoute()
	if err != nil {
		return nil, err
	}
	endpoints, err := ParseManageRoutes(managePath)
	if err != nil {
		return nil, fmt.Errorf("解析路由失败: %w", err)
	}

	root := filepath.Dir(filepath.Dir(managePath)) // .../route/manage.go -> 仓库根
	ctrlDir := filepath.Join(root, "controller", "manageController")
	if _, statErr := os.Stat(ctrlDir); statErr != nil {
		return nil, fmt.Errorf("控制器源码目录不可用: %w", statErr)
	}

	bindings, err := collectHandlerBindings(ctrlDir)
	if err != nil {
		return nil, err
	}

	// 只解析真正被引用的结构体，避免把 request/config 整包遍历一遍。
	// 切片类型（[]config.CustomField）剥掉 "[]" 前缀后查表。
	want := map[string]bool{}
	for _, b := range bindings {
		if b.structType != "" {
			want[strings.TrimPrefix(b.structType, "[]")] = true
		}
	}
	schemas, arrayTypes, touched, err := collectStructSchemas(root, want)
	if err != nil {
		return nil, err
	}

	cat := &APICatalog{Source: managePath, StructTouched: touched}
	for _, ep := range endpoints {
		b := bindings[ep.Handler]
		desc, doc := splitHandlerDoc(b.doc, ep.Handler)
		params := mergeParams(b, schemas)
		psrc := b.source
		// source=struct 但查表无字段时按数组体处理，标成 array 才不会让模型
		// 去找对象字段。两类：type X []string（具名切片，arrayTypes 命中）、
		// []外部包.类型（如 []eino.Config，字段解析不到但"是数组"这一事实可标注）。
		// 注意 params 空也可能只是字段带 ast:"-" 被显式排除，这类保持 struct 不动。
		if psrc == "struct" && b.structType != "" && len(params) == 0 {
			if strings.HasPrefix(b.structType, "[]") {
				psrc = "array"
			} else if _, ok := arrayTypes[b.structType]; ok {
				psrc = "array"
			}
		}
		stType := b.structType
		if stType == "" && len(b.inlineFields) > 0 {
			stType = "inline" // 匿名结构体：定义就在 handler 里，没有包名.类型名可报
		}
		cat.Endpoints = append(cat.Endpoints, EndpointMeta{
			Method:      ep.Method,
			Path:        ep.Path,
			Handler:     ep.Handler,
			NS:          ep.NS,
			Resource:    ep.Resource,
			Domain:      string(intent.DomainOfPath(ep.Path)),
			Risk:        riskOf(ep.Method, ep.Path),
			Desc:        desc,
			Doc:         doc,
			ParamSource: psrc,
			StructType:  stType,
			Params:      params,
		})
	}
	sortCatalog(cat.Endpoints)
	return cat, nil
}

// splitHandlerDoc 把 handler 文档注释拆成「一句话摘要」与「剩余正文」。
//
// 摘要取自首行并剥掉函数名前缀——Go 的 godoc 惯例是注释以函数名开头
// （`GetWebsiteList 获取站点列表`），但这条信息对模型是冗余的：它已经在 handler 字段里了。
// 同时剔除「参数说明」段落，那部分按参数名分发给了各 ParamMeta.Desc，无需重复出现在正文。
func splitHandlerDoc(doc, handlerName string) (string, string) {
	if strings.TrimSpace(doc) == "" {
		return "", ""
	}
	lines := strings.Split(doc, "\n")
	summary := ""
	rest := make([]string, 0, len(lines))
	inParamSection := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "参数说明") {
			inParamSection = true
			continue
		}
		if inParamSection {
			// 参数段落以列表项形式延续，遇到非列表项即结束
			if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") {
				continue
			}
			inParamSection = false
		}
		if i == 0 {
			summary = trimmed
			summary = strings.TrimPrefix(summary, handlerName)
			summary = strings.TrimSpace(strings.TrimPrefix(summary, "："))
			summary = strings.TrimSpace(strings.TrimPrefix(summary, ":"))
			continue
		}
		rest = append(rest, line)
	}
	body := strings.TrimSpace(strings.Join(rest, "\n"))
	return summary, body
}

// CatalogDiff 是两份端点表的差异，用于漂移检查。
type CatalogDiff struct {
	Missing []string `json:"missing"` // 源码有、表中没有（新增路由未重新生成）
	Extra   []string `json:"extra"`   // 表中有、源码没有（已删除的路由，或手工补充的端点）
	Changed []string `json:"changed"` // 两端都有但元数据不一致
}

// Empty 判定两份表是否完全一致。
func (d CatalogDiff) Empty() bool {
	return len(d.Missing) == 0 && len(d.Extra) == 0 && len(d.Changed) == 0
}

// DiffCatalogs 比对两份端点表，key 为 "METHOD PATH"。
// base 通常是嵌入表，other 通常是源码解析结果。
func DiffCatalogs(base, other *APICatalog) CatalogDiff {
	var d CatalogDiff
	bm := map[string]EndpointMeta{}
	for _, ep := range base.Endpoints {
		bm[ep.Method+" "+ep.Path] = ep
	}
	om := map[string]EndpointMeta{}
	for _, ep := range other.Endpoints {
		om[ep.Method+" "+ep.Path] = ep
	}
	keys := make([]string, 0, len(bm))
	for k := range bm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		o, ok := om[k]
		if !ok {
			d.Extra = append(d.Extra, k)
			continue
		}
		if !sameEndpoint(bm[k], o) {
			d.Changed = append(d.Changed, k+" ["+changedFields(bm[k], o)+"]")
		}
	}
	okeys := make([]string, 0, len(om))
	for k := range om {
		okeys = append(okeys, k)
	}
	sort.Strings(okeys)
	for _, k := range okeys {
		if _, ok := bm[k]; !ok {
			d.Missing = append(d.Missing, k)
		}
	}
	return d
}

func sameEndpoint(a, b EndpointMeta) bool {
	return changedFields(a, b) == ""
}

// sameParam 逐字段比较入参。ParamMeta 含切片，不能直接用 ==。
func sameParam(a, b ParamMeta) bool {
	if a.Name != b.Name || a.Type != b.Type || a.GoType != b.GoType ||
		a.Desc != b.Desc || a.Required != b.Required || len(a.Enum) != len(b.Enum) {
		return false
	}
	for i := range a.Enum {
		if a.Enum[i] != b.Enum[i] {
			return false
		}
	}
	return true
}

// changedFields 列出两张卡片不一致的字段名，用于定位漂移点。
func changedFields(a, b EndpointMeta) string {
	var f []string
	if a.Handler != b.Handler {
		f = append(f, "handler")
	}
	if a.NS != b.NS {
		f = append(f, "ns")
	}
	if a.Resource != b.Resource {
		f = append(f, "resource")
	}
	if a.Domain != b.Domain {
		f = append(f, "domain")
	}
	if a.Risk != b.Risk {
		f = append(f, "risk")
	}
	if a.Desc != b.Desc {
		f = append(f, "desc")
	}
	if a.Doc != b.Doc {
		f = append(f, "doc")
	}
	if a.ParamSource != b.ParamSource {
		f = append(f, "param_source")
	}
	if a.StructType != b.StructType {
		f = append(f, "struct_type")
	}
	if len(a.Params) != len(b.Params) {
		f = append(f, fmt.Sprintf("params(%d→%d)", len(a.Params), len(b.Params)))
	} else {
		for i := range a.Params {
			if !sameParam(a.Params[i], b.Params[i]) {
				f = append(f, "params["+a.Params[i].Name+"]")
			}
		}
	}
	return strings.Join(f, ",")
}

// sortCatalog 保证端点顺序确定（路径优先，其次方法），使生成物可逐字节复现。
func sortCatalog(endpoints []EndpointMeta) {
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path != endpoints[j].Path {
			return endpoints[i].Path < endpoints[j].Path
		}
		return endpoints[i].Method < endpoints[j].Method
	})
}

// CatalogStats 统计数据来源分布，用于自检与回归测试。
func (c *APICatalog) CatalogStats() SourceStats {
	s := SourceStats{}
	for _, ep := range c.Endpoints {
		switch ep.ParamSource {
		case "struct":
			s.Struct++
		case "urlparam":
			s.URLParam++
		case "multipart":
			s.Multipart++
		case "form":
			s.Form++
		default:
			s.None++
		}
	}
	return s
}

// FindEndpoint 按路径与方法精确查找端点。
func (c *APICatalog) FindEndpoint(method, path string) (EndpointMeta, bool) {
	for _, ep := range c.Endpoints {
		if ep.Path == path && strings.EqualFold(ep.Method, method) {
			return ep, true
		}
	}
	return EndpointMeta{}, false
}

// handlerBinding 是某 handler 的参数绑定结论。
type handlerBinding struct {
	source     string
	structType string
	urlParams  []ParamMeta
	formParams []ParamMeta // PostValue/FormValue 读取的表单字段
	// inlineFields 是匿名结构体（var req struct{...}）的字段表。
	// 匿名结构体的定义就在 handler 里，不经过 request/config 包，
	// 所以字段在绑定分析时就能就地抽取，不必等 collectStructSchemas。
	inlineFields []ParamMeta
	// doc 是 handler 函数上方的文档注释原文（godoc 同款）。
	// 它是端点语义描述的**权威来源**：写在源码里离代码最近，改代码时最容易顺手更新，
	// 且不会像 api_catalog.json 那样被重新生成冲掉。
	doc string
	// paramDoc 是从 doc 的「参数说明」段落解析出的 参数名 → 说明。
	paramDoc map[string]string
	// fileParams 是 ctx.FormFile("xxx") 这类调用里的真实文件字段名。
	// 之所以单独存：它不是普通表单字段，schema 要给出"传 base64/data URI"的提示，
	// 且 mergeParams 只在源码没给出字段名时才补通用的 file。
	fileParams []ParamMeta
}

// parseGoFilesInDir 解析目录下所有非 _test.go 的 .go 文件，替代已弃用的 parser.ParseDir。
// ParseDir 不考虑 build tags，会把带 //go:build 限定的文件错误并入结果，这里逐文件解析以规避。
func parseGoFilesInDir(fset *token.FileSet, dir string, mode parser.Mode) ([]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, mode)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

// collectHandlerBindings 用 AST 解析控制器目录下每个 handler 的取参方式。
//
// 必须用 parser.ParseComments：URLParam/PostValue/FormFile 这类参数的说明只存在于
// 源码注释里，不带注释解析会让它们的 Desc 永远为空。
func collectHandlerBindings(dir string) (map[string]handlerBinding, error) {
	fset := token.NewFileSet()
	files, err := parseGoFilesInDir(fset, dir, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("解析控制器源码失败: %w", err)
	}

	out := map[string]handlerBinding{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			b := analyzeHandlerBody(fset, file, fn.Body)
			b.doc = docText(fn.Doc)
			b.paramDoc = parseParamDocSection(fn.Doc)
			out[fn.Name.Name] = b
		}
	}
	return out, nil
}

// analyzeHandlerBody 判定单个函数体的取参方式，优先级：结构体 > 文件 > 表单 > URLParam。
//
// 与结构体字段不同，URLParam/PostValue/FormFile 的参数名来自函数调用的字面量实参，
// 没有像 struct tag 那样的元数据位可放说明——**注释是唯一的信息来源**。
// 因此这里按"语句"而非"裸调用"遍历：每个调用都记住它所属的最内层语句，
// 才能定位到该行上方的注释或行尾注释（见 stmtComment）。
func analyzeHandlerBody(fset *token.FileSet, file *ast.File, body *ast.BlockStmt) handlerBinding {
	var (
		structType string
		readJSON   bool
		hasFile    bool
	)
	var inlineFields []ParamMeta
	var inline []ParamMeta
	urlSeen := map[string]bool{}
	var urlParams []ParamMeta
	formSeen := map[string]bool{}
	var formParams []ParamMeta
	fileSeen := map[string]bool{}
	var fileParams []ParamMeta

	// 预先收集所有语句的结束行：用于把"上一行的行尾注释"与"本行的上方注释"区分开。
	tails := nodeEndLines(fset, body)

	visit := func(stmt ast.Stmt, call *ast.CallExpr) {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "ctx" {
			return
		}
		desc := stmtComment(fset, file, stmt, tails)
		switch sel.Sel.Name {
		case "ReadJSON", "ReadForm", "ReadQuery", "ReadBody", "ReadPostForm":
			readJSON = true
			if len(call.Args) > 0 {
				structType, inline = resolveArgJSONTarget(fset, file, body, call.Args[0])
				if len(inline) > 0 {
					inlineFields = inline
				}
			}
		case "FormFile", "FormFiles", "UploadFormFiles", "MultipartForm":
			// 真正的文件字段：这类端点需要 multipart 才能投递二进制内容。
			// 能拿到字面量字段名就记下来，拿不到（多文件/无参）由 mergeParams 兜底。
			hasFile = true
			name := stringLitArg(call)
			if name != "" && !fileSeen[name] {
				fileSeen[name] = true
				fileParams = append(fileParams, ParamMeta{
					Name:   name,
					Type:   "string",
					GoType: "multipart.FileHeader",
					Desc:   fileParamDesc(desc),
				})
			}
		case "PostValue", "PostValueTrim", "PostValueDefault", "PostValueInt", "PostValueIntDefault",
			"PostValueInt64", "PostValueInt64Default", "PostValueFloat64", "PostValueFloat64Default",
			"PostValueBool", "FormValue", "FormValueDefault":
			name := stringLitArg(call)
			if name != "" && !formSeen[name] {
				formSeen[name] = true
				formParams = append(formParams, ParamMeta{
					Name: name,
					Type: urlParamTypeOf(sel.Sel.Name),
					Desc: desc,
				})
			}
		case "URLParam", "URLParamTrim", "URLParamDefault", "URLParamInt", "URLParamIntDefault",
			"URLParamInt64", "URLParamInt64Default", "URLParamUint", "URLParamUintDefault",
			"URLParamFloat64", "URLParamBool", "URLParamBoolDefault", "URLParamBoolE":
			name := stringLitArg(call)
			if name != "" && !urlSeen[name] {
				urlSeen[name] = true
				urlParams = append(urlParams, ParamMeta{
					Name: name,
					Type: urlParamTypeOf(sel.Sel.Name),
					Desc: desc,
				})
			}
		}
	}
	for _, stmt := range body.List {
		walkCallsInStmt(stmt, visit)
	}

	switch {
	case readJSON && (structType != "" || len(inlineFields) > 0):
		return handlerBinding{source: "struct", structType: structType, inlineFields: inlineFields, urlParams: urlParams, formParams: formParams, fileParams: fileParams}
	case readJSON:
		// 读到非具名结构体（如匿名 struct、iris.Map），无法给出字段级 schema，
		// 但仍可能有表单参数兜底信息，故不直接判为 none。
		if len(urlParams) > 0 {
			return handlerBinding{source: "urlparam", urlParams: urlParams, formParams: formParams, fileParams: fileParams}
		}
		if len(formParams) > 0 {
			return handlerBinding{source: "form", urlParams: urlParams, formParams: formParams, fileParams: fileParams}
		}
		return handlerBinding{source: "none", fileParams: fileParams}
	case hasFile:
		return handlerBinding{source: "multipart", urlParams: urlParams, formParams: formParams, fileParams: fileParams}
	case len(formParams) > 0:
		return handlerBinding{source: "form", urlParams: urlParams, formParams: formParams, fileParams: fileParams}
	case len(urlParams) > 0:
		return handlerBinding{source: "urlparam", urlParams: urlParams, fileParams: fileParams}
	default:
		return handlerBinding{source: "none", fileParams: fileParams}
	}
}

// walkCallsInStmt 遍历语句内的所有调用，并把每个调用归属到**最内层**包含它的语句。
//
// 为什么要归属到语句：注释挂在语句行上，不在调用表达式上。只有知道
// `id := ctx.URLParamInt("id") // 文档ID` 这一整行的范围，才能取到行尾注释。
// 遇到嵌套语句就下钻并把"当前语句"换成更内层的那个，避免把内层调用的注释
// 错取成外层 if/for 行的注释。
func walkCallsInStmt(stmt ast.Stmt, visit func(ast.Stmt, *ast.CallExpr)) {
	ast.Inspect(stmt, func(n ast.Node) bool {
		if n == ast.Node(stmt) {
			return true
		}
		switch v := n.(type) {
		case ast.Stmt:
			walkCallsInStmt(v, visit)
			return false // 子树已由递归处理，不再深入
		case *ast.CallExpr:
			visit(stmt, v)
			return true // 继续深入，处理嵌套调用（如 ctx.X(foo())）
		}
		return true
	})
}

// stmtComment 取语句（ctx.URLParam("x") 那一行）的注释。
//
// 是 docComment 在 ast.Stmt 上的专用薄封装，便于调用处自解释。
// 两种目标写法：
//
//	// 分类ID
//	categoryId := ctx.URLParamInt("category_id")
//	keyword := ctx.URLParam("keyword") // 搜索关键词
func stmtComment(fset *token.FileSet, file *ast.File, stmt ast.Stmt, tails map[int]bool) string {
	return docComment(fset, file, stmt, tails)
}

// fileParamDesc 组装文件字段的说明：优先源码注释，再补上传输格式提示。
// 只写"字段名"而不说清怎么传，模型会把它当成普通字符串。
func fileParamDesc(comment string) string {
	const how = "文件内容：data URI（data:image/png;base64,...）或裸 base64 字符串；可用 file_name/filename 指定文件名"
	if comment == "" {
		return how
	}
	return comment + "；" + how
}

// resolveArgJSONTarget 从 &req / &req.Field 回溯 req 的声明，返回两部分：
//   - typeStr：具名类型（"request.Archive"，切片为 "[]config.CustomField"），可能为空
//   - fields：匿名结构体（var req struct{...}）的就地字段表，非匿名时为 nil
//
// 两者互斥：匿名结构体不走 schemas 查表（它的定义不在 request/config 包里），
// 而是当场抽取字段——它的定义就在 handler 所在文件里，注释/tag 都拿得到。
func resolveArgJSONTarget(fset *token.FileSet, file *ast.File, body *ast.BlockStmt, arg ast.Expr) (string, []ParamMeta) {
	varName := ""
	switch e := arg.(type) {
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			if id, ok := e.X.(*ast.Ident); ok {
				varName = id.Name
			}
		}
	case *ast.Ident:
		varName = e.Name
	case *ast.SelectorExpr: // &req.Field
		if id, ok := e.X.(*ast.Ident); ok {
			varName = id.Name
		}
	}
	if varName == "" {
		return "", nil
	}
	typeStr, anon := lookupVarDecl(body, varName)
	if anon != nil {
		return "", structFields(fset, file, anon)
	}
	return typeStr, nil
}

// lookupVarDecl 在函数体内查找变量的声明，返回（具名类型, 匿名结构体）。
//
// 具名覆盖：var x request.T、var x []config.T、x := request.T{}、x := &request.T{}。
// 匿名覆盖：var x struct{...} 与 x := struct{...}{}，此时具名类型为空、
// 直接返回 *ast.StructType 让调用方就地抽字段。
func lookupVarDecl(body *ast.BlockStmt, name string) (string, *ast.StructType) {
	var (
		foundType string
		foundAnon *ast.StructType
	)
	ast.Inspect(body, func(n ast.Node) bool {
		if foundType != "" || foundAnon != nil {
			return false
		}
		switch st := n.(type) {
		case *ast.ValueSpec: // var req request.Archive
			for _, id := range st.Names {
				if id.Name == name && st.Type != nil {
					foundType = namedType(st.Type)
					if foundType == "" {
						foundAnon, _ = st.Type.(*ast.StructType)
					}
					return false
				}
			}
		case *ast.AssignStmt: // req := request.Archive{} / req := &request.Archive{}
			for i, lhs := range st.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == name && i < len(st.Rhs) {
					foundType = namedTypeOfExpr(st.Rhs[i])
					if foundType == "" {
						if lit, ok := st.Rhs[i].(*ast.CompositeLit); ok {
							foundAnon, _ = lit.Type.(*ast.StructType)
						}
					}
					return false
				}
			}
		}
		return foundType == "" && foundAnon == nil
	})
	return foundType, foundAnon
}

func namedTypeOfExpr(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.UnaryExpr:
		if v.Op == token.AND {
			return namedTypeOfExpr(v.X)
		}
	case *ast.CompositeLit:
		return namedType(v.Type)
	}
	return ""
}

// namedType 接受 pkg.Type 与 []pkg.Type 形式的具名类型，排除内置类型与指针。
//
// 切片要支持：`var req []config.CustomField` 是真实存在的写法（SettingDiyFieldForm），
// 返回值带 "[]" 前缀，消费方（want 集合 / schemas 查找）需先剥掉前缀再查表，
// 但 StructType 输出保留前缀——它同时承担"告诉模型请求体是数组"的职责。
func namedType(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.SelectorExpr:
		if pkg, ok := v.X.(*ast.Ident); ok {
			return pkg.Name + "." + v.Sel.Name
		}
	case *ast.StarExpr:
		return namedType(v.X)
	case *ast.ArrayType:
		// 元素不是具名类型（如 []string）时返回 ""，由调用方按"非结构体"处理
		if elt := namedType(v.Elt); elt != "" {
			return "[]" + elt
		}
	}
	return ""
}

func stringLitArg(call *ast.CallExpr) string {
	if len(call.Args) == 0 {
		return ""
	}
	if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
		if v, err := strconv.Unquote(lit.Value); err == nil {
			return v
		}
	}
	return ""
}

// urlParamTypeOf 从 ctx.URLParamXxx 的方法名推断参数类型。
func urlParamTypeOf(method string) string {
	switch {
	case strings.Contains(method, "Int"):
		return "integer"
	case strings.Contains(method, "Float"):
		return "number"
	case strings.Contains(method, "Bool"):
		return "boolean"
	default:
		return "string"
	}
}

// collectStructSchemas 解析 request / config 包中被引用结构体的字段定义。
// 返回 "request.Archive" -> 字段表、类型名 -> 底层元素类型（具名切片，如
// "request.SensitiveWordsRequest" -> "string"），以及成功解析到的结构体数量。
// 只扫 request/config 会让这些端点顶着 struct 标签却给不出字段（假 schema）。
func collectStructSchemas(root string, want map[string]bool) (map[string][]ParamMeta, map[string]string, int, error) {
	schemas := map[string][]ParamMeta{}
	arrayTypes := map[string]string{}
	if len(want) == 0 {
		return schemas, arrayTypes, 0, nil
	}
	for _, dir := range []string{
		filepath.Join(root, "request"),
		filepath.Join(root, "config"),
	} {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		found, arrays, err := parseStructsInDir(dir, want)
		if err != nil {
			return nil, nil, 0, err
		}
		for k, v := range found {
			schemas[k] = v
		}
		for k, v := range arrays {
			arrayTypes[k] = v
		}
	}
	return schemas, arrayTypes, len(schemas), nil
}

func parseStructsInDir(dir string, want map[string]bool) (map[string][]ParamMeta, map[string]string, error) {
	fset := token.NewFileSet()
	files, err := parseGoFilesInDir(fset, dir, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("解析 %s 失败: %w", dir, err)
	}
	out := map[string][]ParamMeta{}
	arrays := map[string]string{}
	for _, file := range files {
		pkgName := file.Name.Name
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				full := pkgName + "." + ts.Name.Name
				if !want[full] {
					continue
				}
				switch st := ts.Type.(type) {
				case *ast.StructType:
					out[full] = structFields(fset, file, st)
				case *ast.ArrayType:
					// type X []string 这类具名切片：请求体是 JSON 数组而非对象，
					// 记下元素类型供 param_source=array 判定，不伪造字段
					arrays[full] = exprString(st.Elt)
				}
			}
		}
	}
	return out, arrays, nil
}

func structFields(fset *token.FileSet, file *ast.File, st *ast.StructType) []ParamMeta {
	var out []ParamMeta
	tails := nodeEndLines(fset, st) // 排除前一行字段的行尾注释被误当成上方注释
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			continue // 嵌入字段暂不展开
		}
		jsonName, skip, required := parseJSONTag(f.Tag)
		if skip {
			continue
		}
		goType := exprString(f.Type)
		if jsonName == "" {
			jsonName = snakeLite(f.Names[0].Name)
		}
		out = append(out, ParamMeta{
			Name:     jsonName,
			Type:     schemaTypeOf(goType),
			GoType:   goType,
			Desc:     docComment(fset, file, f, tails),
			Required: required,
		})
	}
	return out
}

// parseJSONTag 解析结构体 tag，返回 json 名 / 是否跳过 / 是否必填。
func parseJSONTag(tag *ast.BasicLit) (string, bool, bool) {
	if tag == nil {
		return "", false, false
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return "", false, false
	}
	value := reflectTagValue(raw, "json")
	// 支持 ast:"-"
	if b := reflectTagValue(raw, "ast"); b == "-" {
		value = b
	}
	// "-" 表示跳过字段
	if value == "-" {
		return "", true, false
	}
	required := false
	if b := reflectTagValue(raw, "validate"); b != "" {
		for _, part := range strings.Split(b, ",") {
			if strings.TrimSpace(part) == "required" {
				required = true
			}
		}
	}
	if i := strings.Index(value, ","); i >= 0 {
		value = value[:i]
	}
	return value, false, required
}

func reflectTagValue(tagValue, key string) string {
	rest := tagValue
	for len(rest) > 0 {
		i := strings.IndexByte(rest, ' ')
		head := rest
		if i >= 0 {
			head = rest[:i]
			rest = rest[i+1:]
		} else {
			rest = ""
		}
		head = strings.TrimSpace(head)
		if head == "" {
			continue
		}
		if j := strings.IndexByte(head, ':'); j >= 0 {
			if head[:j] != key {
				continue
			}
			val := head[j+1:]
			if strings.HasPrefix(val, "\"") {
				if v, err := strconv.Unquote(val); err == nil {
					return v
				}
			}
			return val
		}
	}
	return ""
}

// docComment 取一个 AST 节点的说明文字，是结构体字段与调用型参数的统一入口。
//
// 支持两类节点：
//  1. *ast.Field（结构体字段）——parser 已把注释关联到 Doc（上方）与 Comment（行尾）；
//  2. ast.Stmt（ctx.URLParam/PostValue/FormFile 所在的语句）——语句类型没有 Doc 字段，
//     只能按行号在 file.Comments 里定位。
//
// 匹配顺序：节点自带 Doc > 紧贴上一行的上方注释 > 同一行的行尾注释。
// 只认"紧贴"的注释（上方注释结束行 == 节点起始行-1），否则函数顶部的整段说明
// 会被误当成第一个参数的说明。
//
// tails 用于排除误配：它记录同组节点各自"结束在哪一行"。若某注释的起始行正好是
// 另一个节点的结束行，说明那是**别人的行尾注释**，不能当成当前节点的上方注释——
// `A string // A 的说明` 这种写法会让下一行的字段错误地继承 A 的说明。
// 调用方能拿到同组节点时请务必传入；传 nil 则退化为不做该排除。
func docComment(fset *token.FileSet, file *ast.File, n ast.Node, tails map[int]bool) string {
	if fset == nil || file == nil || n == nil {
		return ""
	}
	field, _ := n.(*ast.Field)
	if field != nil && field.Doc != nil {
		if s := commentText(field.Doc); s != "" {
			return s
		}
	}

	start := fset.Position(n.Pos()).Line
	end := fset.Position(n.End()).Line
	trailing := ""
	for _, cg := range file.Comments {
		first := fset.Position(cg.Pos()).Line
		last := fset.Position(cg.End()).Line
		if last == start-1 && !(tails != nil && tails[first]) {
			if s := commentText(cg); s != "" {
				return s
			}
		}
		if first == end && trailing == "" {
			trailing = commentText(cg)
		}
	}
	if trailing != "" {
		return trailing
	}
	if field != nil {
		return cleanComment(field.Comment.Text())
	}
	return ""
}

// fileSeen 判断已收集的文件字段里是否含指定名字。
func fileSeen(params []ParamMeta, name string) (ParamMeta, bool) {
	for _, p := range params {
		if p.Name == name {
			return p, true
		}
	}
	return ParamMeta{}, false
}

// MergeCatalogPreserve 用旧表补齐新表里缺失的描述，返回新表。
//
// ⚠️ 默认不启用（见 cmd/apicatalog 的 -merge）。
// 正常流程是「观察 api_catalog.json → 回源码补注释 → 重新生成」，
// 此时 JSON 必须是源码的**忠实镜像**：在 Go 里删掉注释后，JSON 里就该同步消失；
// 若沿用了旧值，被删的说明会变成幽灵残留，反而掩盖"该端点其实没有描述"这个事实。
//
// 只有确实要在 JSON 里手工补写、且接受"重新生成可能被源码覆盖"的例外场景才用它。
// 启用时策略仍是源码优先：新表已有值的字段一律以新表为准，只补齐新表为空的字段。
func MergeCatalogPreserve(next, prev *APICatalog) *APICatalog {
	if next == nil {
		return prev
	}
	if prev == nil {
		return next
	}
	old := map[string]EndpointMeta{}
	for _, ep := range prev.Endpoints {
		old[ep.Method+" "+ep.Path] = ep
	}
	for i := range next.Endpoints {
		o, ok := old[next.Endpoints[i].Method+" "+next.Endpoints[i].Path]
		if !ok {
			continue
		}
		if next.Endpoints[i].Desc == "" {
			next.Endpoints[i].Desc = o.Desc
		}
		if next.Endpoints[i].Doc == "" {
			next.Endpoints[i].Doc = o.Doc
		}
		oldParams := map[string]ParamMeta{}
		for _, p := range o.Params {
			oldParams[p.Name] = p
		}
		for j := range next.Endpoints[i].Params {
			if next.Endpoints[i].Params[j].Desc != "" {
				continue
			}
			if p, ok := oldParams[next.Endpoints[i].Params[j].Name]; ok {
				next.Endpoints[i].Params[j].Desc = p.Desc
			}
		}
	}
	return next
}

// nodeEndLines 收集一组节点各自的结束行号，供 docComment 排除行尾注释误配。
func nodeEndLines(fset *token.FileSet, root ast.Node) map[int]bool {
	out := map[int]bool{}
	if fset == nil || root == nil {
		return out
	}
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		out[fset.Position(n.End()).Line] = true
		return true
	})
	return out
}

// todoMarkers 是"待办/备注"类注释的特征词。
//
// 源码里有大量 `//需要支持分页，还要支持搜索` 这种紧贴在第一条取参语句上方的
// 功能待办备注——它在位置上完全符合"上方注释"，但描述的是**代码要做什么**，
// 不是**参数是什么**。把它当成参数说明推给模型只会造成困惑，故过滤掉。
// 这是启发式：宁可漏掉个别说明，也不要把 TODO 当成字段语义。
var todoMarkers = []string{
	"todo", "fixme", "xxx", "hack", "note:",
	"需要支持", "待实现", "待完成", "待补充", "待补", "未完成",
	"暂不", "暂时", "待测", "待优化",
}

// docText 取注释组全文（保留换行），用于 handler 的文档注释。
func docText(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	return strings.TrimSpace(cg.Text())
}

// parseParamDocSection 从 handler 文档注释里解析「参数说明」段落。
//
// 约定格式（宽容解析，匹配不上就忽略，绝不报错）：
//
//	// GetWebsiteList 获取多站点的站点列表。
//	//
//	// 参数说明：
//	//   - 查询参数 "current": 当前页码，默认为 1。
//	//   - name: 站点名称模糊搜索。
//
// 为什么值得单独约定一段：行内注释能说明单个参数，但**默认值、取值范围、枚举含义**
// 这些信息没有别的地方可写，而它们恰恰是模型最容易填错的部分。
func parseParamDocSection(cg *ast.CommentGroup) map[string]string {
	raw := docText(cg)
	if raw == "" {
		return nil
	}
	out := map[string]string{}
	inSection := false
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "参数说明") {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		// 段落结束：遇到不以 - 开头的正文行
		if !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "*") {
			inSection = false
			continue
		}
		item := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "-"), "*"))
		if name, desc, ok := splitParamDocItem(item); ok {
			out[name] = desc
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// splitParamDocItem 拆分 `- 查询参数 "current": 当前页码，默认为 1。` 这类条目。
// 支持两种写法：带引号取引号内名字；不带引号取冒号前的最后一个标识符。
func splitParamDocItem(item string) (string, string, bool) {
	// 优先：引号内的名字
	if i := strings.Index(item, ":"); i >= 0 {
		head, desc := item[:i], strings.TrimSpace(item[i+1:])
		if desc == "" {
			return "", "", false
		}
		if l, r := strings.Index(head, "\""), strings.LastIndex(head, "\""); l >= 0 && r > l {
			return strings.TrimSpace(head[l+1 : r]), desc, true
		}
		// 退化：冒号前的最后一个标识符（跳过"查询参数"这类中文导语）
		fields := strings.Fields(head)
		for i := len(fields) - 1; i >= 0; i-- {
			if paramNameRe.MatchString(fields[i]) {
				return fields[i], desc, true
			}
		}
	}
	// 中文冒号
	//
	// ⚠️ 必须用 len("：") 而不是 i+1 跳过冒号：strings.Index 返回字节偏移，
	// 而 "：" 占 3 个字节，item[i+1:] 会从冒号的续字节中间切开，
	// 产出非法 UTF-8 → json.Marshal 把它们转成 \ufffd，且源码与文件永远对不上
	// （-check 永远报过期）。pluginPay.go 的全角冒号注释第一次踩爆了这个分支。
	const fullwidthColon = "："
	if i := strings.Index(item, fullwidthColon); i >= 0 {
		head, desc := item[:i], strings.TrimSpace(item[i+len(fullwidthColon):])
		if desc == "" {
			return "", "", false
		}
		if l, r := strings.Index(head, "\""), strings.LastIndex(head, "\""); l >= 0 && r > l {
			return strings.TrimSpace(head[l+1 : r]), desc, true
		}
		fields := strings.Fields(head)
		for i := len(fields) - 1; i >= 0; i-- {
			if paramNameRe.MatchString(fields[i]) {
				return fields[i], desc, true
			}
		}
	}
	return "", "", false
}

// commentText 取注释组的正文，待办/备注类一律返回空（见 isTodoComment）。
func commentText(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	s := cleanComment(cg.Text())
	if isTodoComment(s) {
		return ""
	}
	return s
}

// isTodoComment 判断注释是否是待办/备注而非参数说明。
func isTodoComment(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	if lower == "" {
		return false
	}
	for _, m := range todoMarkers {
		if strings.HasPrefix(lower, m) || strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func cleanComment(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "//")
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok {
			return x.Name + "." + v.Sel.Name
		}
	case *ast.StarExpr:
		return "*" + exprString(v.X)
	case *ast.ArrayType:
		return "[]" + exprString(v.Elt)
	case *ast.MapType:
		return "map[" + exprString(v.Key) + "]" + exprString(v.Value)
	case *ast.InterfaceType:
		return "interface{}"
	}
	return fmt.Sprintf("%T", e)
}

// schemaTypeOf 把 Go 类型映射到 JSON schema 类型，与 intent.ParamSpec.Type 口径一致。
func schemaTypeOf(goType string) string {
	switch {
	case strings.HasPrefix(goType, "[]"):
		return "array"
	case strings.HasPrefix(goType, "map["):
		return "object"
	}
	switch goType {
	case "string":
		return "string"
	case "bool":
		return "boolean"
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "byte", "rune":
		return "integer"
	case "float32", "float64":
		return "number"
	case "interface{}", "any":
		return "object"
	default:
		// time.Time、具名结构体、别名等按 object 处理，避免过度乐观地标注为 string
		return "object"
	}
}

// snakeLite 在缺少 json tag 时给一个稳定的默认字段名。
func snakeLite(name string) string {
	var b strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

// riskOf 判定端点风险等级：read 可直接放行，write 建议确认，destructive 默认拒绝。
func riskOf(method, path string) string {
	lower := strings.ToLower(path)
	for _, kw := range []string{"delete", "remove", "reset", "destroy", "upgrade", "clear"} {
		if strings.Contains(lower, kw) {
			return "destructive"
		}
	}
	if strings.EqualFold(method, "GET") {
		return "read"
	}
	return "write"
}

// mergeParams 汇总某端点最终可见的参数列表：
// 结构体字段为主，表单字段次之，URLParam 补充两者都没有的键。
//
// multipart 来源的端点额外补一个 file 参数说明：原本 17 个上传端点使用 ctx.FormFile
// 直接取文件，AST 里没有任何名称信息，不补充的话调用方无从知道该传什么。
func mergeParams(b handlerBinding, schemas map[string][]ParamMeta) []ParamMeta {
	var out []ParamMeta
	seen := map[string]bool{}
	// 文件字段放最前：它是这类端点的必填项，模型第一眼就该看到。
	//
	// 控制器里常见写法是先取 "file"、取不到再取 "file1"（attachment/upload 就是），
	// 这两个是**同一个文件的备选名**。不写明的话模型会两个都传，同一份内容被上传两遍。
	_, hasMainFile := fileSeen(b.fileParams, "file")
	multiFile := hasMainFile && len(b.fileParams) > 1
	for _, p := range b.fileParams {
		if seen[p.Name] {
			continue
		}
		seen[p.Name] = true
		if multiFile && p.Name != "file" {
			p.Desc += "；备选字段名，与 file 二选一，通常只传 file"
		}
		out = append(out, p)
	}
	if len(b.inlineFields) > 0 {
		// 匿名结构体：字段已在绑定分析时就地抽取，不查 schemas
		for _, p := range b.inlineFields {
			if seen[p.Name] {
				continue
			}
			seen[p.Name] = true
			out = append(out, p)
		}
	} else if b.structType != "" {
		// 切片类型（[]config.CustomField）查表要去掉前缀；前缀本身保留在
		// structType 输出里，用于告知消费方"请求体是元素数组"
		for _, p := range schemas[strings.TrimPrefix(b.structType, "[]")] {
			if seen[p.Name] {
				continue
			}
			seen[p.Name] = true
			out = append(out, p)
		}
	}
	for _, p := range b.formParams {
		if seen[p.Name] {
			continue
		}
		seen[p.Name] = true
		out = append(out, p)
	}
	for _, p := range b.urlParams {
		if seen[p.Name] {
			continue
		}
		seen[p.Name] = true
		out = append(out, p)
	}
	// 兜底：确实用了 FormFile 但源码里没给出字面量字段名（多文件上传等），
	// 才补一个通用的 file。已从源码拿到字段名时不再重复补，避免同一个文件传两遍。
	if b.source == "multipart" && len(b.fileParams) == 0 && !seen["file"] {
		out = append([]ParamMeta{{
			Name: "file",
			Type: "string",
			Desc: fileParamDesc(""),
		}}, out...)
	}
	// 文档注释里的「参数说明」优先级最高：它通常含默认值、取值范围这类行内注释写不下的信息。
	// 只对注释里点名的参数生效，其余保留从代码注释提取到的说明。
	for i := range out {
		if d, ok := b.paramDoc[out[i].Name]; ok && d != "" {
			out[i].Desc = d
		}
	}
	return out
}

// ensure ExecPath 与覆盖审计一致的兜底：若 FindManageRoute 依赖 ExecPath 未设置，
// 尝试从当前工作目录上溯。此变量仅用于保证本文件的行为不依赖外部初始化顺序。
var _ = config.ExecPath
