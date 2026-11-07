package config

import (
	"strings"
	"testing"
)

// 品牌参数化之后，官方版的对外表现必须一字不变，否则改造的成本会外溢到主站。
// 这里把官方版的关键值钉死：有人改 brand.json 时会先红一次，逼他确认是不是有意的。
func TestOfficialBrandUnchanged(t *testing.T) {
	b, ok := LookupOEM(DefaultOEMID)
	if !ok {
		t.Fatalf("缺少官方品牌 %q", DefaultOEMID)
	}
	cases := []struct {
		field string
		got   string
		want  string
	}{
		{"Name", b.Name, "AnQiCMS"},
		{"NameCN", b.NameCN, "安企CMS"},
		{"Vendor", b.Vendor, "AnQiCMS"},
		{"Website", b.Website, "https://www.anqicms.com"},
		{"AuthBase", b.AuthBase, "https://auth.anqicms.com/auth"},
		{"DownloadBase", b.DownloadBase, "https://www.anqicms.com/downloads"},
		// 升级包内主程序名：升级逻辑据此识别要替换哪个文件，
		// 与历史官方包保持一致，改了会让存量安装无法升级。
		{"Binary", b.Binary, "anqicms"},
		{"Database", b.Database, "anqicms"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("官方版 %s 变了：got %q want %q", c.field, c.got, c.want)
		}
	}
}

// 所有登记的品牌都必须能完整渲染。缺字段的配置不允许进入构建产物。
func TestAllBrandsComplete(t *testing.T) {
	if err := OEMLoadError(); err != nil {
		t.Fatalf("品牌数据加载失败：%v", err)
	}
	ids := OEMIDs()
	if len(ids) == 0 {
		t.Fatal("没有加载到任何品牌")
	}
	for _, id := range ids {
		b, _ := LookupOEM(id)
		for _, e := range b.Validate() {
			t.Errorf("config/oem/%s/brand.json: %s", id, e)
		}
		if b.ID != id {
			t.Errorf("品牌 %s 的 id 字段为 %q，与键不一致", id, b.ID)
		}
	}
}

// 未知标识必须回落到中性品牌，绝不能回落到官方品牌。
//
// 这是本次改造里唯一“错了会很难看”的地方：构建时 -X 注入拼错一个字母，
// 交付出去的包就会满屏官方 logo 与官网链接，而功能看起来一切正常。
// 反向对照：把 neutralBrand 改成返回官方品牌，本测试会立刻变红。
func TestUnknownOEMFallsBackToNeutral(t *testing.T) {
	official, _ := LookupOEM(DefaultOEMID)

	old := OEMID
	defer func() { OEMID = old }()

	OEMID = "acme-nonexistent"
	got := Brand()

	if got.ID != "acme-nonexistent" {
		t.Errorf("中性品牌应保留标识，got %q", got.ID)
	}
	for _, leaked := range []string{official.Name, official.NameCN, official.Vendor} {
		if leaked == "" {
			continue
		}
		if strings.Contains(got.Name, leaked) || strings.Contains(got.NameCN, leaked) || strings.Contains(got.Vendor, leaked) {
			t.Errorf("未知标识回落到官方品牌 %q：展示名 %q / %q / %q", leaked, got.Name, got.NameCN, got.Vendor)
		}
	}
	// 展示名必须由标识本身构成，否则界面上会出现空白 Logo 与空白标题。
	if got.Name != "acme-nonexistent" || got.NameCN != "acme-nonexistent" {
		t.Errorf("中性品牌展示名应等于标识，got %q / %q", got.Name, got.NameCN)
	}
}

// 空标识回落到官方版：未注入 -X 时 OEMID 就是默认值，这条路径必须正常工作。
func TestEmptyOEMIDIsOfficial(t *testing.T) {
	old := OEMID
	defer func() { OEMID = old }()

	OEMID = "   "
	if !IsOfficial() {
		t.Errorf("空白标识应视为官方版，CurrentOEMID() = %q", CurrentOEMID())
	}
	if Brand().Name != "AnQiCMS" {
		t.Errorf("空白标识应返回官方品牌，got %q", Brand().Name)
	}
}

func TestValidOEMID(t *testing.T) {
	ok := []string{"anqicms", "acme", "acme-cms", "a1", "a_b-c"}
	bad := []string{"", "-acme", "_acme", "Acme", "acme cms", "安企", "acme/cms", "acme.cms"}
	for _, id := range ok {
		if !ValidOEMID(id) {
			t.Errorf("%q 应合法", id)
		}
	}
	for _, id := range bad {
		if ValidOEMID(id) {
			t.Errorf("%q 应非法", id)
		}
	}
}

// 库名会拼进 CREATE DATABASE，必须是标识符安全的形式。
func TestSQLDatabaseName(t *testing.T) {
	b := OEMBrand{ID: "acme", Database: "acme-cms.v2"}
	if got := b.SQLDatabaseName(); got != "acme_cms_v2" {
		t.Errorf("got %q want %q", got, "acme_cms_v2")
	}
	if got := (OEMBrand{ID: "acme"}).SQLDatabaseName(); got != "acme" {
		t.Errorf("Database 为空时应回退 ID，got %q", got)
	}
}

func TestNameForLocale(t *testing.T) {
	b := OEMBrand{Name: "AcmeCMS", NameCN: "艾克米CMS"}
	if got := b.NameForLocale("zh-CN"); got != "艾克米CMS" {
		t.Errorf("zh-CN got %q", got)
	}
	if got := b.NameForLocale("en-US"); got != "AcmeCMS" {
		t.Errorf("en-US got %q", got)
	}
	// 中文名为空时不能返回空串，否则界面上只剩一个 Logo。
	if got := (OEMBrand{Name: "AcmeCMS"}).NameForLocale("zh-CN"); got != "AcmeCMS" {
		t.Errorf("中文名缺失应回退主名，got %q", got)
	}
}

// Brandize 是品牌文案的唯一出口：locales 里 9 个语言的品牌名都在这里被替换。
// 官方版必须逐字节不变，OEM 版必须一个不剩地换掉。
func TestBrandize(t *testing.T) {
	official, _ := LookupOEM(DefaultOEMID)
	inputs := []string{
		"安企CMS(AnQiCMS)初始化安装",
		"欢迎使用安企内容管理系统",
		"请先登录 AnQiCMS 账号",
		"AnqiCMS 安装成功",
		"https://www.anqicms.com/help",
	}
	for _, in := range inputs {
		if got := BrandizeWith(official, in); got != in {
			t.Errorf("官方版被改动：\n in  %q\n out %q", in, got)
		}
	}

	oem := OEMBrand{
		ID: "acme", Name: "AcmeCMS", NameCN: "艾克米CMS",
		Website: "https://www.acme.test", AuthBase: "https://auth.acme.test/auth",
	}
	cases := map[string]string{
		"安企CMS(AnQiCMS)初始化安装":          "艾克米CMS(AcmeCMS)初始化安装",
		"欢迎使用安企内容管理系统":                 "欢迎使用艾克米CMS内容管理系统",
		"请先登录 AnQiCMS 账号":              "请先登录 AcmeCMS 账号",
		"AnqiCMS 安装成功":                 "AcmeCMS 安装成功",
		"https://www.anqicms.com/help": "https://www.acme.test/help",
		// 子域示例（locales 里有 m. 与 admin. 几十处）：换主机不换前缀，
		// 用「anqicms.com 整段换掉」而不是逐个枚举，才补得齐。
		"https://admin.anqicms.com": "https://admin.acme.test",
		"https://m.anqicms.com":     "https://m.acme.test",
		// 授权端点整段换成品牌自己的网关，不能只是换域名。
		"https://auth.anqicms.com/auth/v1": "https://auth.acme.test/auth/v1",
	}
	for in, want := range cases {
		if got := BrandizeWith(oem, in); got != want {
			t.Errorf("OEM 替换不对：\n in   %q\n got  %q\n want %q", in, got, want)
		}
	}

	// 中文名缺失时用主名兜底，不能替换成空串（界面上会剩下断句）。
	noCN := OEMBrand{ID: "acme", Name: "AcmeCMS"}
	if got := BrandizeWith(noCN, "欢迎使用安企CMS"); got != "欢迎使用AcmeCMS" {
		t.Errorf("中文名缺失兜底不对：%q", got)
	}

	if got := BrandizeWith(oem, ""); got != "" {
		t.Errorf("空串应原样返回，got %q", got)
	}
}
