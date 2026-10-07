package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parseTestFile 解析内联源码，返回 fset/file/首个函数体。
func parseTestFile(t *testing.T, src string) (*token.FileSet, *ast.File, *ast.BlockStmt) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析内联源码失败: %v", err)
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			return fset, file, fn.Body
		}
	}
	t.Fatalf("源码里没有函数: %s", src)
	return nil, nil, nil
}

func paramDesc(params []ParamMeta, name string) string {
	for _, p := range params {
		if p.Name == name {
			return p.Desc
		}
	}
	return ""
}

// TestFullwidthColonParamDoc 验证「参数说明」条目使用全角冒号（：）时不产生非法 UTF-8。
//
// 回归背景：strings.Index 返回字节偏移，"：" 占 3 字节，item[i+1:] 会从续字节中间
// 切开，json.Marshal 把非法字节转成 \ufffd，且 -check 永远报过期
// （pluginPay.go 的表单参数注释第一次踩爆）。
func TestFullwidthColonParamDoc(t *testing.T) {
	src := `package p

// Handler 上传证书。
//
// 参数说明：
//   - 表单参数 "name"：证书文件名，支持 .pem 后缀。
//   - 表单参数 "file"：证书文件
func Handler(ctx iris.Context) {
	name := ctx.PostValue("name")
	file, _, _ := ctx.FormFile("file")
	_, _ = name, file
}
`
	fset, file, body := parseTestFile(t, src)
	b := analyzeHandlerBody(fset, file, body)
	// paramDoc 由 collectHandlerBindings 统一填充，测试里手动补上
	fn, _ := file.Decls[0].(*ast.FuncDecl)
	b.paramDoc = parseParamDocSection(fn.Doc)
	params := mergeParams(b, nil)
	if got := paramDesc(params, "name"); got != "证书文件名，支持 .pem 后缀。" {
		t.Fatalf("全角冒号条目解析错误: %q", got)
	}
	if got := paramDesc(params, "file"); got != "证书文件" {
		t.Fatalf("file 说明解析错误: %q", got)
	}
}

// TestSliceOfStructBinding 验证 var req []pkg.Struct 的切片绑定：
// structType 保留 "[]" 前缀（告知消费方请求体是数组），mergeParams 剥前缀查 schema。
// 回归背景：namedType 原本不认 ArrayType，SettingDiyFieldForm 被判成 source=none。
func TestSliceOfStructBinding(t *testing.T) {
	src := `package p

func Handler(ctx iris.Context) {
	var req []config.CustomField
	if err := ctx.ReadJSON(&req); err != nil {
		return
	}
}
`
	fset, file, body := parseTestFile(t, src)
	b := analyzeHandlerBody(fset, file, body)
	if b.source != "struct" {
		t.Fatalf("source 期望 struct，实得 %s", b.source)
	}
	if b.structType != "[]config.CustomField" {
		t.Fatalf("structType 期望 []config.CustomField，实得 %s", b.structType)
	}
	params := mergeParams(b, map[string][]ParamMeta{
		"config.CustomField": {{Name: "name", Type: "string"}},
	})
	if len(params) != 1 || params[0].Name != "name" {
		t.Fatalf("切片元素结构体字段未展开: %+v", params)
	}
}

// TestNamedSliceTypeIsArray 验证 type X []string 这类具名切片被标为 array。
// 回归背景：request.SensitiveWordsRequest 原本顶着 struct 标签却给不出字段（假 schema）。
func TestNamedSliceTypeIsArray(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "request"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "request", "req.go"), []byte(`package request

type SensitiveWordsRequest []string
`), 0o644); err != nil {
		t.Fatal(err)
	}
	schemas, arrays, _, err := collectStructSchemas(dir, map[string]bool{"request.SensitiveWordsRequest": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(schemas) != 0 {
		t.Fatalf("具名切片不应产出字段表: %+v", schemas)
	}
	if arrays["request.SensitiveWordsRequest"] != "string" {
		t.Fatalf("arrays 未记录元素类型: %+v", arrays)
	}
}

// TestAnonymousStructInlineFields 验证 var req struct{...} 匿名结构体的就地字段抽取。
// 回归背景：SettingAiForm 的匿名结构体（含 []*eino.Config 引用）原先被完全漏掉。
func TestAnonymousStructInlineFields(t *testing.T) {
	src := `package p

func Handler(ctx iris.Context) {
	var req struct {
		Write *config.AiGenerateConfig ` + "`json:\"write\"`" + ` // AI 写作配置
		Chat  []*eino.Config                        ` + "`json:\"chat\"`" + `
	}
	if err := ctx.ReadJSON(&req); err != nil {
		return
	}
}
`
	fset, file, body := parseTestFile(t, src)
	b := analyzeHandlerBody(fset, file, body)
	if b.source != "struct" {
		t.Fatalf("source 期望 struct，实得 %s", b.source)
	}
	if b.structType != "" {
		t.Fatalf("匿名结构体 structType 应为空，实得 %s", b.structType)
	}
	if len(b.inlineFields) != 2 {
		t.Fatalf("内联字段数期望 2，实得 %d: %+v", len(b.inlineFields), b.inlineFields)
	}
	if b.inlineFields[0].Name != "write" || b.inlineFields[0].Desc != "AI 写作配置" {
		t.Fatalf("write 字段解析错误: %+v", b.inlineFields[0])
	}
	if b.inlineFields[1].Name != "chat" || b.inlineFields[1].Type != "array" {
		t.Fatalf("chat 字段解析错误: %+v", b.inlineFields[1])
	}
	params := mergeParams(b, nil)
	if len(params) != 2 {
		t.Fatalf("mergeParams 未使用内联字段: %+v", params)
	}
}

// TestCallParamCommentPickup 验证 URLParam/PostValue/FormFile 的参数能取到源码注释。
//
// 这些参数名来自函数调用的字面量实参，没有 tag 之类的元数据位可放说明，
// **注释是唯一信息来源**。以前它们的 Desc 恒为空。
func TestCallParamCommentPickup(t *testing.T) {
	src := `package p

func Handler(ctx iris.Context) {
	// 分类ID
	categoryId := ctx.URLParamInt("category_id")
	keyword := ctx.URLParam("keyword") // 搜索关键词
	file, _, _ := ctx.FormFile("file")
	name := ctx.PostValue("name")
	if categoryId > 0 {
		// 嵌套语句里的上下方注释
		nested := ctx.URLParam("nested")
		_ = nested
	}
	_, _, _, _, _ = categoryId, keyword, file, name, 0
}
`
	fset, file, body := parseTestFile(t, src)
	b := analyzeHandlerBody(fset, file, body)

	if b.source != "multipart" {
		t.Fatalf("期望 multipart，实得 %s", b.source)
	}
	if got := paramDesc(b.urlParams, "category_id"); got != "分类ID" {
		t.Errorf("URLParamInt 未取到上方注释，实得 %q", got)
	}
	if got := paramDesc(b.urlParams, "keyword"); got != "搜索关键词" {
		t.Errorf("URLParam 未取到行尾注释，实得 %q", got)
	}
	if got := paramDesc(b.urlParams, "nested"); got != "嵌套语句里的上下方注释" {
		t.Errorf("嵌套语句内的参数未取到注释，实得 %q", got)
	}
	if got := paramDesc(b.formParams, "name"); got != "" {
		t.Errorf("无注释的参数应保持空 desc，实得 %q", got)
	}
	// 文件字段必须说清怎么传，否则模型会当成普通字符串
	if !strings.Contains(paramDesc(b.fileParams, "file"), "base64") {
		t.Errorf("文件字段缺少传输格式说明: %q", paramDesc(b.fileParams, "file"))
	}
	if len(b.fileParams) != 1 || b.fileParams[0].Name != "file" {
		t.Errorf("未从 ctx.FormFile 提取到字段名: %+v", b.fileParams)
	}
}

// TestCommentNotInheritedByNextParam 验证行尾注释不会被下一个参数错误继承。
//
// 这是实现"上方注释"匹配时最容易踩的坑：`A string // A 的说明` 紧贴下一行，
// 若不排除就会被下一行的字段当成自己的说明。
func TestCommentNotInheritedByNextParam(t *testing.T) {
	src := `package p

type Payload struct {
	A string ` + "`json:\"a\"`" + ` // A 的说明
	B string ` + "`json:\"b\"`" + `
	// C 的说明
	C string ` + "`json:\"c\"`" + `
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	var st *ast.StructType
	ast.Inspect(file, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if s, ok := ts.Type.(*ast.StructType); ok {
				st = s
				return false
			}
		}
		return true
	})
	if st == nil {
		t.Fatal("未找到结构体")
	}
	fields := structFields(fset, file, st)
	descs := map[string]string{}
	for _, f := range fields {
		descs[f.Name] = f.Desc
	}
	if descs["a"] != "A 的说明" {
		t.Errorf("a 应取到自己的行尾注释，实得 %q", descs["a"])
	}
	if descs["b"] != "" {
		t.Errorf("b 不应继承上一行的行尾注释，实得 %q", descs["b"])
	}
	if descs["c"] != "C 的说明" {
		t.Errorf("c 应取到上方注释，实得 %q", descs["c"])
	}
}

// TestTodoCommentNotUsedAsParamDesc 验证待办类注释不会被当成参数说明。
//
// 源码里 `//需要支持分页，还要支持搜索` 紧贴在第一条取参语句上方，位置上是合法的上方注释，
// 但它说的是"代码要做什么"，不是"参数是什么"。宁可漏掉说明，也不要把 TODO 推给模型。
func TestTodoCommentNotUsedAsParamDesc(t *testing.T) {
	src := `package p

func List(ctx iris.Context) {
	//需要支持分页，还要支持搜索
	current := ctx.URLParamIntDefault("current", 1)
	// 搜索关键词
	keyword := ctx.URLParam("keyword")
	_, _ = current, keyword
}
`
	fset, file, body := parseTestFile(t, src)
	b := analyzeHandlerBody(fset, file, body)
	if got := paramDesc(b.urlParams, "current"); got != "" {
		t.Errorf("待办注释不应成为参数说明，实得 %q", got)
	}
	if got := paramDesc(b.urlParams, "keyword"); got != "搜索关键词" {
		t.Errorf("正常注释应保留，实得 %q", got)
	}
}

// TestHandlerDocBecomesEndpointDesc 验证 handler 的 godoc 注释会变成端点摘要与参数说明。
//
// 这是"在 Go 源码里补注释"这条工作流的落点：注释写了就必须能在 api_catalog.json 里看到，
// 否则补注释的人无从确认成果。
func TestHandlerDocBecomesEndpointDesc(t *testing.T) {
	src := `package p

// GetWebsiteList 获取多站点的站点列表，支持分页和名称搜索。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - pageSize: 每页条数，默认为 20。
//   - 查询参数 "name": 站点名称模糊搜索。
func GetWebsiteList(ctx iris.Context) {
	current := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	name := ctx.URLParam("name")
	_, _, _ = current, pageSize, name
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok {
			fn = f
		}
	}
	b := analyzeHandlerBody(fset, file, fn.Body)
	b.doc = docText(fn.Doc)
	b.paramDoc = parseParamDocSection(fn.Doc)

	desc, body := splitHandlerDoc(b.doc, "GetWebsiteList")
	if desc != "获取多站点的站点列表，支持分页和名称搜索。" {
		t.Errorf("摘要应剥掉函数名前缀，实得 %q", desc)
	}
	if strings.Contains(body, "参数说明") || strings.Contains(body, "当前页码") {
		t.Errorf("正文不应残留参数说明段落，实得 %q", body)
	}
	params := mergeParams(b, nil)
	if got := paramDesc(params, "current"); got != "当前页码，默认为 1。" {
		t.Errorf("带引号的参数说明未生效，实得 %q", got)
	}
	if got := paramDesc(params, "pageSize"); got != "每页条数，默认为 20。" {
		t.Errorf("不带引号的参数说明未生效，实得 %q", got)
	}
	if got := paramDesc(params, "name"); got != "站点名称模糊搜索。" {
		t.Errorf("参数说明未生效，实得 %q", got)
	}
}

// TestMergeCatalogPreserveKeepsManualOnlyWhenMissing 验证 merge 只在源码缺失时补旧值。
// 正常生成流程默认不启用它（JSON 应忠实反映源码），但它必须行为正确。
func TestMergeCatalogPreserveKeepsManualOnlyWhenMissing(t *testing.T) {
	prev := &APICatalog{Endpoints: []EndpointMeta{{
		Method: "GET", Path: "/a", Desc: "手工补的",
		Params: []ParamMeta{{Name: "x", Desc: "手工参数说明"}},
	}}}
	next := &APICatalog{Endpoints: []EndpointMeta{{
		Method: "GET", Path: "/a", Desc: "源码里的",
		Params: []ParamMeta{{Name: "x"}},
	}}}
	got := MergeCatalogPreserve(next, prev)
	if got.Endpoints[0].Desc != "源码里的" {
		t.Errorf("源码有值时必须以源码为准，实得 %q", got.Endpoints[0].Desc)
	}
	if got.Endpoints[0].Params[0].Desc != "手工参数说明" {
		t.Errorf("源码缺参数说明时应沿用旧值，实得 %q", got.Endpoints[0].Params[0].Desc)
	}
}

// TestFileParamAliasHint 验证 file/file1 这类备选字段名会提示"二选一"。
// 控制器里 `先取 file 再取 file1` 的写法很常见，不说明会导致同一份内容被传两遍。
func TestFileParamAliasHint(t *testing.T) {
	src := `package p

func Upload(ctx iris.Context) {
	file, _, err := ctx.FormFile("file")
	if err != nil {
		file, _, err = ctx.FormFile("file1")
	}
	_, _ = file, err
}
`
	fset, file, body := parseTestFile(t, src)
	b := analyzeHandlerBody(fset, file, body)
	params := mergeParams(b, nil)
	first := paramDesc(params, "file1")
	if !strings.Contains(first, "二选一") {
		t.Errorf("备选文件字段缺少二选一提示: %q", first)
	}
	if !strings.Contains(paramDesc(params, "file"), "base64") {
		t.Errorf("主文件字段缺少传输格式说明")
	}
}

// TestFileParamFallbackWhenNoLiteral 验证拿不到字面量字段名时才补通用 file。
func TestFileParamFallbackWhenNoLiteral(t *testing.T) {
	src := `package p

func UploadMany(ctx iris.Context) {
	form, err := ctx.MultipartForm()
	_, _ = form, err
}
`
	fset, file, body := parseTestFile(t, src)
	b := analyzeHandlerBody(fset, file, body)
	if b.source != "multipart" {
		t.Fatalf("期望 multipart，实得 %s", b.source)
	}
	params := mergeParams(b, nil)
	if paramDesc(params, "file") == "" {
		t.Errorf("无字面量字段名时应补通用 file 参数，实得 %+v", params)
	}
}
