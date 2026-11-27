package manageController

import (
	"runtime"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
)

// 升级链路是品牌化里最容易“看起来对、实际没换掉”的一段：
// URL 拼错 → 下载 404；主程序名对不上 → 包里找不到二进制 → 回滚旧文件
// 但界面仍报成功。所以这里把两段拼接与识别都钉死。

func TestVersionCheckURLCarriesOEM(t *testing.T) {
	official, ok := config.LookupOEM(config.DefaultOEMID)
	if !ok {
		t.Fatal("缺少官方品牌")
	}
	got := versionCheckURL(official)
	if !strings.HasPrefix(got, official.DownloadBase+"/version.json?") {
		t.Errorf("地址前缀不对：%s", got)
	}
	if !strings.Contains(got, "&oem=anqicms") {
		t.Errorf("缺少 oem 参数，官网无法按品牌下发版本线：%s", got)
	}

	// 换成 OEM 品牌后，必须用该品牌自己的下载源与标识。
	// 反向对照：若把 brand 换成硬编码官方地址，这里立刻变红。
	oem := config.OEMBrand{ID: "acme", Name: "AcmeCMS", DownloadBase: "https://dl.acme.test/releases"}
	got = versionCheckURL(oem)
	if !strings.HasPrefix(got, "https://dl.acme.test/releases/version.json?") {
		t.Errorf("OEM 地址前缀不对：%s", got)
	}
	if !strings.Contains(got, "&oem=acme") {
		t.Errorf("OEM 标识未带上：%s", got)
	}
	if strings.Contains(got, "anqicms") {
		t.Errorf("OEM 地址里出现官方域名：%s", got)
	}
}

func TestUpgradePackageURLUsesBrandBinary(t *testing.T) {
	official, _ := config.LookupOEM(config.DefaultOEMID)
	want := official.DownloadBase + "/anqicms-" + runtime.GOOS + "-" + runtime.GOARCH + "-v3.9.0.zip?oem=anqicms"
	if got := upgradePackageURL(official, "3.9.0"); got != want {
		t.Errorf("官方包地址变了：\n got %s\nwant %s", got, want)
	}

	oem := config.OEMBrand{ID: "acme", Binary: "acmecms", DownloadBase: "https://dl.acme.test/releases"}
	want = "https://dl.acme.test/releases/acmecms-" + runtime.GOOS + "-" + runtime.GOARCH + "-v3.9.0.zip?oem=acme"
	if got := upgradePackageURL(oem, "3.9.0"); got != want {
		t.Errorf("OEM 包地址不对：\n got %s\nwant %s", got, want)
	}
}

func TestIsMainBinaryFile(t *testing.T) {
	official, _ := config.LookupOEM(config.DefaultOEMID)

	cases := []struct {
		name    string
		entry   string
		brand   config.OEMBrand
		running string
		want    bool
	}{
		{"官方主程序", "anqicms", official, "anqicms", true},
		{"官方主程序带目录", "dist/anqicms", official, "anqicms", true},
		{"windows 主程序", "anqicms.exe", official, "anqicms", true},
		{"OEM 主程序", "acmecms", config.OEMBrand{Binary: "acmecms"}, "acmecms", true},
		{"客户改过名", "renamed", config.OEMBrand{Binary: "acmecms"}, "renamed", true},
		{"非主程序", "CHANGELOG.md", official, "anqicms", false},
		{"同前缀但不是", "anqicms.bak", official, "anqicms", false},
		{"cwebp 不是主程序", "cwebp_linux_amd64", official, "anqicms", false},
		// 空 binary 不能匹配空条目名，否则会把根目录下同名的空串误判成主程序。
		{"空品牌名不匹配", "", config.OEMBrand{}, "anqicms", false},
	}

	for _, c := range cases {
		if got := isMainBinaryFile(c.entry, c.brand, c.running); got != c.want {
			t.Errorf("%s: isMainBinaryFile(%q) = %v, want %v", c.name, c.entry, got, c.want)
		}
	}
}
