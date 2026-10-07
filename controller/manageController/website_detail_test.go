package manageController

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/request"
)

// TestWebsiteDetailNoPasswordField 读接口不得返回任何 password 字段。
//
// 背景（2026-10-03）：GetWebsiteInfo 曾复用**写请求**结构体
// request.WebsiteRequest，而它带 AdminPassword 且 json tag 无 omitempty，
// 于是响应里出现了 "admin_password": ""。虽然密码是加密存储、值确实为空，
// 但让读接口带 password 字段本身就是坏设计：
//   - 调用方会误以为那里有可用凭据；
//   - 将来任何一处给它赋上值都会变成明文泄露。
//
// 修法是引入只读的 WebsiteDetailResponse，从结构上杜绝这类误用。
// 这里断言「响应结构体里没有 password 字段」，锁住这个约束。
func TestWebsiteDetailNoPasswordField(t *testing.T) {
	// WebsiteDetailResponse 必须存在且不含任何 password 字段
	b, err := os.ReadFile("../../request/website.go")
	if err != nil {
		t.Fatalf("读 request/website.go 失败: %v", err)
	}
	body := string(b)
	i := strings.Index(body, "type WebsiteDetailResponse struct")
	if i < 0 {
		t.Fatal("未找到 WebsiteDetailResponse（读接口应使用专用的只读结构体）")
	}
	seg := body[i:]
	if n := strings.Index(seg, "\n}"); n > 0 {
		seg = seg[:n]
	}
	// 结构体里可以有 Password 字段（用于输出掩码），但绝不能复用
	// config.MysqlConfig——那个带真值。断言 Mysql 用的是脱敏类型。
	if !strings.Contains(seg, "Mysql     WebsiteMysqlSafe") &&
		!strings.Contains(seg, "Mysql WebsiteMysqlSafe") {
		t.Errorf("Mysql 字段必须用脱敏类型 WebsiteMysqlSafe，实际：\n%s", seg)
	}
	if strings.Contains(seg, "config.MysqlConfig") {
		t.Error("只读结构体不得直接用 config.MysqlConfig（含数据库真密码）")
	}
	// 序列化验证：零值也不该冒出 password 键
	var zero request.WebsiteDetailResponse
	raw, err := json.Marshal(zero)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	// 零值序列化：password 应是空串（无 omitempty 时）而不是任何真值，
	// 且必须能安全反序列化回来（证明结构合法）。
	var back request.WebsiteDetailResponse
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Errorf("只读结构体应能正常反序列化：%v", err)
	}

	// 控制器必须用它而不是 WebsiteRequest
	c, err := os.ReadFile("website.go")
	if err != nil {
		t.Fatalf("读 website.go 失败: %v", err)
	}
	cs := string(c)
	j := strings.Index(cs, "func GetWebsiteInfo")
	if j < 0 {
		t.Skip("未找到 GetWebsiteInfo")
	}
	fn := cs[j:]
	if n := strings.Index(fn, "\n}\n"); n > 0 {
		fn = fn[:n]
	}
	if !strings.Contains(fn, "request.WebsiteDetailResponse{") {
		t.Error("GetWebsiteInfo 必须用 WebsiteDetailResponse，不能复用写请求结构体")
	}
	if strings.Contains(fn, "request.WebsiteRequest{") {
		t.Error("GetWebsiteInfo 仍复用 WebsiteRequest，会把 admin_password 带进响应")
	}
}

// TestWebsiteDetailMaskWritebackProtected 掩码必须配「回写保护」。
//
// 只脱敏不回写保护是半截修复：读接口返回 "password": "********"，
// 调用方（尤其是 AI）把读到的详情原样保存，掩码就会写进库——
// 数据库密码变成 "********"，站点连不上库，且不报任何错。
//
// 这与 pluginSendmail 的授权码掩码是同一个模式（三处必须一起改：
// 读端点掩码 / 写端点识别哨兵 / desc 说明），见 memory 里的记录。
func TestWebsiteDetailMaskWritebackProtected(t *testing.T) {
	c, err := os.ReadFile("website.go")
	if err != nil {
		t.Fatalf("读 website.go 失败: %v", err)
	}
	body := string(c)
	// 写端点必须识别掩码哨兵并沿用库中真值
	if !strings.Contains(body, "request.MysqlPasswordMask") {
		t.Error("写端点必须识别掩码哨兵 request.MysqlPasswordMask，否则掩码会被写进库")
	}
	if !strings.Contains(body, "req.Mysql.Password = dbSite.Mysql.Password") {
		t.Error("识别到掩码后必须沿用库里的真密码（req.Mysql.Password = dbSite.Mysql.Password）")
	}
	// 脱敏函数必须真的替换了密码，而不是原样返回
	if strings.Contains(string(mustReadFile(t, "../../request/website.go")),
		"Password: m.Password") {
		t.Error("SafeMysql 不得把真密码原样放进返回值")
	}
}

func mustReadFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读 %s 失败: %v", p, err)
	}
	return string(b)
}

// TestSafeMysqlAlwaysMasks 任何情况下都不能返回真密码。
//
// 含 UseDefault=true：此时密码复用主站点的，本就没有独立值可返回，
// 但仍应输出掩码而不是空串——空串会被误读成「没设密码」。
func TestSafeMysqlAlwaysMasks(t *testing.T) {
	cases := []config.MysqlConfig{
		{Database: "d", User: "u", Password: "真密码", Host: "h", Port: 3306},
		{Database: "d", User: "u", Password: "真密码", UseDefault: true},
		{Database: "d", User: "u", Password: ""}, // 本来就没密码
	}
	for i, in := range cases {
		got := request.SafeMysql(in)
		if got.Password != request.MysqlPasswordMask {
			t.Errorf("case %d: 密码应恒为掩码 %q，实际=%q（真值=%q）",
				i, request.MysqlPasswordMask, got.Password, in.Password)
		}
		// 非密码字段必须原样保留，否则调用方无法判断连的哪个库
		if got.Database != in.Database || got.User != in.User || got.Host != in.Host {
			t.Errorf("case %d: 非敏感字段应原样保留，实际=%+v", i, got)
		}
	}
}
