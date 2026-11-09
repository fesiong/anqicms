package provider_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
	"golang.org/x/text/language"

	"kandaoni.com/anqicms/controller/manageController"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

// 本文件从 HTTP 层驱动真实控制器，锁住绑定 token 的两个对外承诺：
//  1. 响应带 Cache-Control: no-store（明文 token 不得被任何中间层缓存）；
//  2. 明文只在这一次响应里出现一次，数据库里只有它的 SHA-256。
//
// 为什么放在 provider 目录的 external test 包里：controller/manageController 的
// 测试工作目录拿不到 config.json（config.initPath 不会往上找），整条链路连不上库；
// provider 目录可以，而 provider_test 导入 manageController 不构成循环依赖。
func TestBindingTokenResponseIsNoStoreAndSinglePlaintext(t *testing.T) {
	site := bindingHTTPTestSite(t)
	admin := bindingHTTPTestAdmin(t, site)
	channel := bindingHTTPTestChannel(t, site)

	app := iris.New()
	localeKey := app.ConfigurationReadOnly().GetLocaleContextKey()
	app.Post("/system/api/channels/binding/token", func(ctx iris.Context) {
		// 复刻真实链路的注入：站点与登录管理员都由中间件写入 ctx.Values。
		// 最小测试应用没有加载 i18n；CurrentSite 会读 locale，先塞一个空实现避免空指针。
		ctx.Values().Set(localeKey, bindingHTTPLocale{})
		ctx.Values().Set("siteId", site.Id)
		ctx.Values().Set("adminId", strconv.FormatUint(uint64(admin.Id), 10))
		manageController.ChannelBindingToken(ctx)
	})
	if err := app.Build(); err != nil {
		t.Fatalf("构建测试应用失败：%v", err)
	}

	body := fmt.Sprintf(`{"channel_id":%d}`, channel.Id)
	req := httptest.NewRequest("POST", "/system/api/channels/binding/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	raw := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("期望 HTTP 200，实得 %d / body=%s", rec.Code, raw)
	}
	if cacheControl := rec.Header().Get("Cache-Control"); !strings.Contains(cacheControl, "no-store") {
		t.Fatalf("绑定 token 响应必须带 Cache-Control: no-store，实得 %q", cacheControl)
	}

	var envelope struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Token     string `json:"token"`
			ExpiresAt int64  `json:"expires_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("解析响应失败：%v / body=%s", err, raw)
	}
	if envelope.Code != 0 {
		t.Fatalf("生成绑定 token 失败：code=%d msg=%s", envelope.Code, envelope.Msg)
	}
	if envelope.Data.Token == "" {
		t.Fatalf("响应缺少 token：%s", raw)
	}
	if count := strings.Count(raw, envelope.Data.Token); count != 1 {
		t.Fatalf("token 明文只允许出现一次，实际 %d 次：%s", count, raw)
	}
	if envelope.Data.ExpiresAt <= time.Now().Unix() || envelope.Data.ExpiresAt > time.Now().Add(2*time.Hour).Unix() {
		t.Fatalf("过期时间不合理：%d", envelope.Data.ExpiresAt)
	}

	var row model.ChannelBindingToken
	if err := site.DB.Where("site_id = ? AND channel_id = ? AND admin_id = ?", site.Id, channel.Id, admin.Id).
		Order("id desc").Take(&row).Error; err != nil {
		t.Fatalf("读取落库的绑定 token 失败：%v", err)
	}
	// 独立算摘要：复用被测实现的哈希函数会让「两边一起写错」永远通过。
	sum := sha256.Sum256([]byte(envelope.Data.Token))
	if want := hex.EncodeToString(sum[:]); row.TokenHash != want {
		t.Fatalf("库里必须只存 token 的 SHA-256：want=%s got=%s", want, row.TokenHash)
	}
	if row.TokenHash == envelope.Data.Token {
		t.Fatal("token_hash 列出现了明文")
	}
	if strings.Contains(fmt.Sprintf("%+v", row), envelope.Data.Token) {
		t.Fatal("token 行里出现了明文")
	}
}

// bindingHTTPLocale 是 iris.Locale 的最小实现：本用例只关心响应头与 token，
// 不需要任何翻译文案。
type bindingHTTPLocale struct{}

func (bindingHTTPLocale) Index() int                                        { return 0 }
func (bindingHTTPLocale) Tag() *language.Tag                                { tag := language.English; return &tag }
func (bindingHTTPLocale) Language() string                                  { return "en-US" }
func (bindingHTTPLocale) GetMessage(key string, args ...interface{}) string { return key }

// bindingHTTPTestSite 初始化站点 1 并停掉生产渠道管理器/通知工作者（避免真实联网）。
func bindingHTTPTestSite(t *testing.T) *provider.Website {
	t.Helper()
	dbSite, err := provider.GetDBWebsiteInfo(1)
	if err != nil {
		t.Fatalf("读取站点 1 失败：%v", err)
	}
	dbSite.Status = 1
	provider.InitWebsite(dbSite)
	site := provider.GetWebsite(1)
	if site == nil || site.DB == nil {
		t.Fatal("站点 1 未初始化，无法连接开发库")
	}
	site.StopChannelManager()
	site.StopNotifyWorker()
	t.Cleanup(func() {
		site.StopChannelManager()
		site.StopNotifyWorker()
	})
	if err := site.DB.AutoMigrate(
		&model.MessageChannel{},
		&model.ChannelBindingToken{},
	); err != nil {
		t.Fatalf("迁移渠道相关表失败：%v", err)
	}
	return site
}

func bindingHTTPTestAdmin(t *testing.T, site *provider.Website) *model.Admin {
	t.Helper()
	suffix := strconv.FormatInt(time.Now().UnixNano()%1e12, 10)
	admin := &model.Admin{
		UserName: "binding_http_admin_" + suffix,
		Status:   1,
		GroupId:  2,
	}
	if err := site.DB.Create(admin).Error; err != nil {
		t.Fatalf("创建测试管理员失败：%v", err)
	}
	t.Cleanup(func() {
		if err := site.DB.Where("id = ?", admin.Id).Delete(&model.Admin{}).Error; err != nil {
			t.Logf("清理测试管理员 %d 失败：%v", admin.Id, err)
		}
	})
	return admin
}

func bindingHTTPTestChannel(t *testing.T, site *provider.Website) *model.MessageChannel {
	t.Helper()
	suffix := strconv.FormatInt(time.Now().UnixNano()%1e12, 10)
	channel := &model.MessageChannel{
		SiteId:    site.Id,
		Platform:  model.ChannelPlatformFeishu,
		Name:      "绑定HTTP测试-" + suffix,
		AppId:     "binding_http_app_" + suffix,
		OrgId:     "binding_http_org_" + suffix,
		AppSecret: "binding_http_secret_" + suffix,
		Revision:  1,
	}
	if err := site.DB.Create(channel).Error; err != nil {
		t.Fatalf("创建测试渠道失败：%v", err)
	}
	t.Cleanup(func() {
		if err := site.DB.Where("channel_id = ?", channel.Id).Delete(&model.ChannelBindingToken{}).Error; err != nil {
			t.Logf("清理渠道 %d 的绑定 token 失败：%v", channel.Id, err)
		}
		if err := site.DB.Where("id = ? AND site_id = ?", channel.Id, site.Id).Delete(&model.MessageChannel{}).Error; err != nil {
			t.Logf("清理渠道 %d 失败：%v", channel.Id, err)
		}
	})
	return channel
}
