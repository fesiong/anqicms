package provider

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
)

// 构造一个最小站点实例，仅带 TokenSecret 与站点域名，不触碰数据库。
func mockInvokeSite() *Website {
	return &Website{
		Id:          1,
		TokenSecret: "unit-test-secret",
		System: &config.SystemConfig{
			BaseUrl:  "http://localhost:8001",
			AdminUrl: "http://localhost:8001",
		},
	}
}

// mockAdminAPIApp 起一个最小 iris 应用，挂上与真实后台同构的中间件与端点，
// 用于端到端验证 InvokeAdminAPI 的投递链路。
func mockAdminAPIApp(t *testing.T, site *Website) *iris.Application {
	t.Helper()
	app := iris.New()

	var (
		seenXHost  string
		seenHost   string
		seenToken  string
		tokenValid bool
	)

	system := app.Party("/system")
	manage := system.Party("/api", func(ctx iris.Context) {
		seenXHost = ctx.GetHeader("X-Host")
		// 真实站点踩坑：只发 X-Host 不够，iris 自身按标准 HTTP Host 做站点/路由匹配。
		// 进程内请求没有网络栈，Request.URL 不含 host，必须由调用方显式设置 req.Host。
		seenHost = ctx.Host()
		seenToken = ctx.GetHeader("admin")
		// 用与 production 相同的密钥校验 token，证明签发链路有效
		if seenToken != "" {
			tok, err := jwt.Parse(seenToken, func(tok *jwt.Token) (interface{}, error) {
				return []byte(site.TokenSecret + "-admin-token"), nil
			})
			tokenValid = err == nil && tok.Valid
		}
		ctx.Next()
	})

	// GET：参数只来自 query
	manage.Get("/echo", func(ctx iris.Context) {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": iris.Map{
				"id":    ctx.URLParam("id"),
				"name":  ctx.URLParam("name"),
				"host":  seenXHost,
				"rhost": seenHost,
			},
		})
	})

	// POST：参数来自 JSON body
	manage.Post("/echo", func(ctx iris.Context) {
		var body map[string]any
		_ = ctx.ReadJSON(&body)
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": iris.Map{
				"title":       body["title"],
				"token_valid": tokenValid,
				"token_seen":  seenToken != "",
			},
		})
	})

	// POST upload：同构于真实的 AttachmentUpload（file → file1 兜底、category_id 走 PostValueInt），
	// 用于端到端验证 base64/data URI 能被还原成 multipart 并被 ctx.FormFile 正常读出。
	manage.Post("/upload", func(ctx iris.Context) {
		// 注意：iris 的 ctx.GetContentType() 读的是**响应**头，这里要的是请求头。
		// GetContentTypeRequested() 会把 ";" 之后的参数裁掉，boundary 也随之丢失，
		// 因此直接取原始 header，以便断言 boundary 确实存在。
		ctype := ctx.GetHeader("Content-Type")
		file, info, err := ctx.FormFile("file")
		if err != nil {
			file, info, err = ctx.FormFile("file1")
			if err != nil {
				ctx.JSON(iris.Map{"code": 1, "msg": "未收到文件: " + err.Error()})
				return
			}
		}
		defer file.Close()
		head := make([]byte, 8)
		n, _ := file.Read(head)
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": iris.Map{
				"filename":     info.Filename,
				"size":         info.Size,
				"head":         fmt.Sprintf("%x", head[:n]),
				"ctype_prefix": ctype,
				"category_id":  ctx.PostValueIntDefault("category_id", 0),
				"file_name":    ctx.PostValue("file_name"),
			},
		})
	})

	if err := app.Build(); err != nil {
		t.Fatalf("Build 失败: %v", err)
	}
	SetInvokerApplicationForTest(app)
	t.Cleanup(func() { SetInvokerApplicationForTest(nil) })
	return app
}

func TestInvokeAdminAPIEndToEnd(t *testing.T) {
	site := mockInvokeSite()
	mockAdminAPIApp(t, site)

	// GET：走 query
	res, err := site.InvokeAdminAPI(2, "GET", "/system/api/echo", map[string]any{
		"id":   42,
		"name": "hello",
	})
	if err != nil {
		t.Fatalf("GET 调用失败: %v", err)
	}
	if !res.OK {
		t.Fatalf("GET 期望成功，实得 status=%d code=%d msg=%s", res.Status, res.Code, res.Msg)
	}
	data, _ := res.Data.(map[string]any)
	if data["id"] != "42" {
		t.Fatalf("query 参数 id 未正确送达: %#v", data)
	}
	if data["name"] != "hello" {
		t.Fatalf("query 参数 name 未正确送达: %#v", data)
	}
	if data["host"] != "localhost" {
		t.Fatalf("X-Host 未送达或不符合站点域名: %#v", data)
	}
	// 站点匹配依赖标准 HTTP Host，必须一并送达（真实环境缺此项会 404）
	if data["rhost"] != "localhost" {
		t.Fatalf("req.Host 未送达，真实站点会因此匹配失败并 404: %#v", data)
	}
	fmt.Printf("\nGET  送达 id=%v name=%v host=%v rhost=%v\n", data["id"], data["name"], data["host"], data["rhost"])

	// POST：走 JSON body，并校验凭证合法
	res2, err := site.InvokeAdminAPI(2, "POST", "/system/api/echo", map[string]any{
		"title": "测试标题",
	})
	if err != nil {
		t.Fatalf("POST 调用失败: %v", err)
	}
	if !res2.OK {
		t.Fatalf("POST 期望成功，实得 status=%d code=%d msg=%s", res2.Status, res2.Code, res2.Msg)
	}
	data2, _ := res2.Data.(map[string]any)
	if data2["title"] != "测试标题" {
		t.Fatalf("JSON body 参数 title 未正确送达: %#v", data2)
	}
	if data2["token_seen"] != true {
		t.Fatal("admin token 未注入")
	}
	if data2["token_valid"] != true {
		t.Fatal("admin token 未能通过校验，签发链路有问题")
	}
	fmt.Printf("POST 送达 title=%v token_seen=%v token_valid=%v\n",
		data2["title"], data2["token_seen"], data2["token_valid"])
}

// TestInvokeRejectsImplicitSuperAdmin 锁定安全边界：
// adminId==1 会被 AdminPermission 直接放行，因此绝不允许隐式回落。
func TestInvokeRejectsImplicitSuperAdmin(t *testing.T) {
	site := mockInvokeSite()
	mockAdminAPIApp(t, site)

	_, err := site.InvokeAdminAPI(0, "GET", "/system/api/echo", nil)
	if err == nil {
		t.Fatal("adminId=0 应当被拒绝，否则意味着存在隐式超级管理员路径")
	}
	fmt.Printf("\nadminId=0 被正确拒绝: %v\n", err)
}

// TestBuildQueryIsDeterministic 锁定 query 构造的稳定性：
// map 迭代顺序不确定，若不排序则同样入参会产出不同 URL，破坏可复现性。
func TestBuildQueryIsDeterministic(t *testing.T) {
	params := map[string]any{"z": "1", "a": "2", "m": 3, "bool": true}
	first := buildQuery(params)
	for i := 0; i < 20; i++ {
		if got := buildQuery(params); got != first {
			t.Fatalf("第 %d 次结果不一致: %q != %q", i, got, first)
		}
	}
	if first != "a=2&bool=true&m=3&z=1" {
		t.Fatalf("未按字典序构造 query: %q", first)
	}
	fmt.Printf("\nquery 构造稳定且有序: %s\n", first)
}

func TestAdminHostOf(t *testing.T) {
	cases := []struct {
		name, admin, base, want string
	}{
		{"优先取 AdminUrl 主机名", "https://admin.example.com/", "https://www.example.com/", "admin.example.com"},
		{"AdminUrl 缺失时回落 BaseUrl", "", "https://www.example.com/", "www.example.com"},
		{"带端口会被剥离", "http://localhost:8001/", "", "localhost"},
		{"非 http 前缀会被忽略", "example.com", "", "localhost"},
		{"站点未配置时回落 localhost", "", "", "localhost"},
		{"System 为 nil 时不 panic", "", "", "localhost"},
	}
	for _, c := range cases {
		var sys *config.SystemConfig
		if c.admin != "" || c.base != "" || c.name != "System 为 nil 时不 panic" {
			sys = &config.SystemConfig{AdminUrl: c.admin, BaseUrl: c.base}
		}
		w := &Website{Id: 1, System: sys}
		if got := adminHostOf(w); got != c.want {
			t.Fatalf("%s: 期望 %q，实得 %q", c.name, c.want, got)
		}
	}
	fmt.Println("\nadminHostOf 各分支校验通过")
}

// TestInvokeResultRequireOK 验证业务码非 0 时的错误包装。
func TestInvokeResultRequireOK(t *testing.T) {
	ok := &InvokeResult{Status: http.StatusOK, Code: config.StatusOK, OK: true}
	if _, err := ok.RequireOK(); err != nil {
		t.Fatalf("code=0 不应报错: %v", err)
	}
	notLogin := &InvokeResult{Status: http.StatusOK, Code: config.StatusNoLogin, Msg: "请先登录"}
	if _, err := notLogin.RequireOK(); err == nil {
		t.Fatal("未登录业务码应被 RequireOK 捕获")
	} else {
		fmt.Printf("\nRequireOK 正确捕获业务错误: %v\n", err)
	}
}

// TestInvokeUploadBase64EndToEnd 是 17 个 multipart 上传端点的端到端回归。
//
// 链路：调用方只给一段字符串 → partitionFileParams 判定为文件 → buildMultipartBody 还原成
// multipart → ServeHTTP 交给 iris → handler 里的 ctx.FormFile 读出真实文件。
// 任何一环退化（比如把 base64 当普通字段塞进 JSON body）都会在 FormFile 处失败，
// 因此这条断言能真正兜住「函数全覆盖但调用方拿不到文件」的风险。
func TestInvokeUploadBase64EndToEnd(t *testing.T) {
	// 1x1 透明 PNG，带真实文件头，长度已过 looksLikeFileContent 的判定门槛
	const pngB64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/x8AAwMCAO+ip1sAAAAASUVORK5CYII="
	pngBytes, err := base64.StdEncoding.DecodeString(pngB64)
	if err != nil {
		t.Fatalf("测试素材非法: %v", err)
	}

	cases := []struct {
		name     string
		params   map[string]any
		wantName string
	}{
		{
			"裸 base64 自动嗅探扩展名",
			map[string]any{"file": pngB64, "category_id": 7},
			"upload.png",
		},
		{
			"data URI 通过 name 参数指定文件名",
			map[string]any{"file": "data:image/png;name=logo hello.png;base64," + pngB64, "category_id": 7},
			"logo hello.png",
		},
		{
			"file_name 显式指定时优先生效",
			map[string]any{"file": pngB64, "file_name": "cover.png", "category_id": 7},
			"cover.png",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			site := mockInvokeSite()
			mockAdminAPIApp(t, site)

			res, err := site.InvokeAdminAPI(2, "POST", "/system/api/upload", c.params)
			if err != nil {
				t.Fatalf("调用失败: %v", err)
			}
			if !res.OK {
				t.Fatalf("期望成功，实得 status=%d code=%d msg=%s raw=%s", res.Status, res.Code, res.Msg, res.Raw)
			}
			data, _ := res.Data.(map[string]any)

			if got := data["filename"]; got != c.wantName {
				t.Fatalf("文件名不符：期望 %q，实得 %q", c.wantName, got)
			}
			if got := data["head"]; got != "89504e470d0a1a0a" {
				t.Fatalf("文件内容未完整送达，首 8 字节为 %q", got)
			}
			if size, _ := data["size"].(float64); int64(size) != int64(len(pngBytes)) {
				t.Fatalf("文件大小不符：期望 %d，实得 %v", len(pngBytes), size)
			}
			if ct, _ := data["ctype_prefix"].(string); !strings.HasPrefix(ct, "multipart/form-data; boundary=") {
				t.Fatalf("Content-Type 不是 multipart: %q", ct)
			}
			// 普通表单字段必须与文件共存于同一 multipart 中（真实 handler 用 PostValueInt 读取）
			if got := data["category_id"]; got != float64(7) {
				t.Fatalf("表单字段 category_id 未送达: %#v", got)
			}
			fmt.Printf("\n上传链路打通：filename=%v size=%v head=%v\n",
				data["filename"], data["size"], data["head"])
		})
	}
}

// TestInvokeUploadRejectsNonFileString 兜住误判：普通长文本不应被当成文件，
// 否则一次普通的「内容是 base64 样例」提交会被悄悄改写 semantics。
func TestInvokeUploadRejectsNonFileString(t *testing.T) {
	site := mockInvokeSite()
	mockAdminAPIApp(t, site)

	// "hello world" 的裸 base64，解出来只有 11 字节，远低于文件门槛
	res, err := site.InvokeAdminAPI(2, "POST", "/system/api/upload", map[string]any{
		"file": "aGVsbG8gd29ybGQ=",
	})
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if res.OK {
		t.Fatalf("短字符串不应被识别为文件，却成功了: %#v", res.Data)
	}
	if res.Code != 1 {
		t.Fatalf("期望 handler 报「未收到文件」，实得 code=%d msg=%s", res.Code, res.Msg)
	}
	fmt.Printf("\n短字符串未被误判为文件：%s\n", res.Msg)
}
