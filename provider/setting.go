package provider

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kataras/iris/v12/i18n"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

const (
	SystemSettingKey  = "system"
	ContentSettingKey = "content"
	IndexSettingKey   = "index"
	ContactSettingKey = "contact"
	SafeSettingKey    = "safe"
	BannerSettingKey  = "banner"
	SensitiveWordsKey = "sensitive_words"
	InstallTimeKey    = "install_time"
	CacheTypeKey      = "cache_type"
	DiyFieldsKey      = "diy_fields"

	PushSettingKey        = "push"
	SitemapSettingKey     = "sitemap"
	RewriteSettingKey     = "rewrite"
	AnchorSettingKey      = "anchor"
	GuestbookSettingKey   = "guestbook"
	UploadFilesSettingKey = "upload_file"
	SendmailSettingKey    = "sendmail"
	ImportApiSettingKey   = "import_api"
	StorageSettingKey     = "storage"
	PaySettingKey         = "pay"
	WeappSettingKey       = "weapp"
	WechatSettingKey      = "wechat"
	RetailerSettingKey    = "retailer"
	UserSettingKey        = "user"
	OrderSettingKey       = "order"
	FulltextSettingKey    = "fulltext"
	TitleImageSettingKey  = "title_image"
	WatermarkSettingKey   = "watermark"
	HtmlCacheSettingKey   = "html_cache"
	AnqiSettingKey        = "anqi"
	AiGenerateSettingKey  = "ai_generate"
	TimeFactorKey         = "time_factor"
	LimiterSettingKey     = "limiter"
	MultiLangSettingKey   = "multi_lang"
	TranslateSettingKey   = "translate"
	JsonLdSettingKey      = "json_ld"

	CollectorSettingKey = "collector"
	KeywordSettingKey   = "keyword"
	InterferenceKey     = "interference"
	LastRunVersionKey   = "last_run_version"
)

// I18n 来自运行中设置的I18n 对象
var I18n *i18n.I18n

func SetI18n(i *i18n.I18n) {
	I18n = i
}

func (w *Website) InitSetting() {
	if w.DB == nil {
		return
	}
	// load setting from db
	var settings []*model.Setting
	w.DB.Find(&settings)
	var settingMap = map[string]string{}
	for _, item := range settings {
		settingMap[item.Key] = item.Value
	}
	w.LoadSystemSetting(settingMap[SystemSettingKey])
	w.LoadContentSetting(settingMap[ContentSettingKey])
	w.LoadIndexSetting(settingMap[IndexSettingKey])
	w.LoadContactSetting(settingMap[ContactSettingKey])
	w.LoadSafeSetting(settingMap[SafeSettingKey])
	w.LoadBannerSetting(settingMap[BannerSettingKey])
	w.LoadSensitiveWords(settingMap[SensitiveWordsKey])

	w.LoadPushSetting(settingMap[PushSettingKey])
	w.LoadSitemapSetting(settingMap[SitemapSettingKey])
	w.LoadRewriteSetting(settingMap[RewriteSettingKey])
	w.LoadAnchorSetting(settingMap[AnchorSettingKey])
	w.LoadGuestbookSetting(settingMap[GuestbookSettingKey])
	w.LoadUploadFilesSetting(settingMap[UploadFilesSettingKey])
	w.LoadSendmailSetting(settingMap[SendmailSettingKey])
	w.LoadImportApiSetting(settingMap[ImportApiSettingKey])
	w.LoadStorageSetting(settingMap[StorageSettingKey])
	w.LoadPaySetting(settingMap[PaySettingKey])
	w.LoadWeappSetting(settingMap[WeappSettingKey])
	w.LoadWechatSetting(settingMap[WechatSettingKey])
	w.LoadRetailerSetting(settingMap[RetailerSettingKey])
	w.LoadUserSetting(settingMap[UserSettingKey])
	w.LoadOrderSetting(settingMap[OrderSettingKey])
	w.LoadFulltextSetting(settingMap[FulltextSettingKey])
	w.LoadTitleImageSetting(settingMap[TitleImageSettingKey])
	w.LoadHtmlCacheSetting(settingMap[HtmlCacheSettingKey])
	w.LoadAnqiUser(settingMap[AnqiSettingKey])
	w.LoadAiGenerateSetting(settingMap[AiGenerateSettingKey])
	w.LoadCollectorSetting(settingMap[CollectorSettingKey])
	w.LoadKeywordSetting(settingMap[KeywordSettingKey])
	w.LoadInterferenceSetting(settingMap[InterferenceKey])
	w.LoadWatermarkSetting(settingMap[WatermarkSettingKey])
	w.LoadTimeFactorSetting(settingMap[TimeFactorKey])
	w.LoadMultiLangSetting(settingMap[MultiLangSettingKey])
	w.LoadTranslateSetting(settingMap[TranslateSettingKey])
	w.LoadJsonLdSetting(settingMap[JsonLdSettingKey])
	// 检查OpenAIAPI是否可用
	go w.CheckOpenAIAPIValid()
}

func (w *Website) LoadSystemSetting(value string) {
	w.System = &config.SystemConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.System)
	}
	//如果没有设置模板，则默认是default
	if w.System.TemplateName == "" {
		w.System.TemplateName = "default"
	}
	if w.System.Language == "" {
		w.System.Language = "zh-CN"
	}
	// 默认站点
	if w.Id == 1 {
		w.System.DefaultSite = true
	}
}

func (w *Website) LoadContentSetting(value string) {
	w.Content = &config.ContentConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.Content)
	}
	if w.Content.MaxPage < 1 {
		w.Content.MaxPage = 1000
	}
	if w.Content.MaxLimit < 1 {
		w.Content.MaxLimit = 100
	}
}

func (w *Website) LoadIndexSetting(value string) {
	w.Index = &config.IndexConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.Index)
	}
}

func (w *Website) LoadBannerSetting(value string) {
	w.Banner = &config.BannerConfig{}
	if value != "" {
		err := json.Unmarshal([]byte(value), &w.Banner)
		if err != nil {
			// 旧版的，做一下兼容
			var banners []config.BannerItem
			if err = json.Unmarshal([]byte(value), &banners); err == nil {
				for i := range banners {
					if banners[i].Type == "" {
						banners[i].Type = "default"
					}
				}
				var mapBanners = map[string][]config.BannerItem{}
				for _, v := range banners {
					mapBanners[v.Type] = append(mapBanners[v.Type], v)
				}
				for i := range banners {
					if item, ok := mapBanners[banners[i].Type]; ok {
						banner := config.Banner{
							Type: banners[i].Type,
							List: item,
						}
						w.Banner.Banners = append(w.Banner.Banners, banner)
						delete(mapBanners, banners[i].Type)
					}
				}
			}
		}
	}
	if len(w.Banner.Banners) == 0 {
		w.Banner.Banners = append(w.Banner.Banners, config.Banner{
			Type: "default",
		})
	}
}

func (w *Website) LoadSensitiveWords(value string) {
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.SensitiveWords)
	}
}

func (w *Website) LoadContactSetting(value string) {
	w.Contact = &config.ContactConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.Contact)
	}
}

func (w *Website) LoadSafeSetting(value string) {
	w.Safe = &config.SafeConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.Safe)
	}
}

func (w *Website) LoadPushSetting(value string) {
	w.PluginPush = &config.PluginPushConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginPush)
	}
	// 兼容旧版 jscode
	if w.PluginPush.JsCode != "" {
		w.PluginPush.JsCodes = append(w.PluginPush.JsCodes, config.CodeItem{
			Name:  w.Tr("UnnamedJs"),
			Value: w.PluginPush.JsCode,
		})
		w.PluginPush.JsCode = ""

		_ = w.SaveSettingValue(PushSettingKey, w.PluginPush)
	}
}

func (w *Website) LoadSitemapSetting(value string) {
	w.PluginSitemap = &config.PluginSitemapConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginSitemap)
	}
	// sitemap
	if w.PluginSitemap.Type != "xml" {
		w.PluginSitemap.Type = "txt"
	}
}

func (w *Website) LoadRewriteSetting(value string) {
	w.PluginRewrite = &config.PluginRewriteConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginRewrite)
	}
}

func (w *Website) LoadAnchorSetting(value string) {
	w.PluginAnchor = &config.PluginAnchorConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginAnchor)
	}
}

func (w *Website) LoadGuestbookSetting(value string) {
	w.PluginGuestbook = &config.PluginGuestbookConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginGuestbook)
	}
}

func (w *Website) LoadUploadFilesSetting(value string) {
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginUploadFiles)
	}
}

func (w *Website) LoadSendmailSetting(value string) {
	w.PluginSendmail = &config.PluginSendmail{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginSendmail)
	}
	if len(w.PluginSendmail.SendType) == 0 {
		w.PluginSendmail.SendType = []int{SendTypeGuestbook}
	}
}

func (w *Website) LoadImportApiSetting(value string) {
	w.PluginImportApi = &config.PluginImportApiConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginImportApi)
	}
	// 导入API生成
	if w.PluginImportApi.Token == "" || w.PluginImportApi.LinkToken == "" {
		h := md5.New()
		h.Write([]byte(fmt.Sprintf("%d", time.Now().Nanosecond())))
		if w.PluginImportApi.Token == "" {
			w.PluginImportApi.Token = hex.EncodeToString(h.Sum(nil))
		}
		if w.PluginImportApi.LinkToken == "" {
			w.PluginImportApi.LinkToken = w.PluginImportApi.Token
		}
		// 回写
		_ = w.SaveSettingValue(ImportApiSettingKey, w.PluginImportApi)
	}

}

func (w *Website) LoadStorageSetting(value string) {
	w.PluginStorage = &config.PluginStorageConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginStorage)
	}
	if w.PluginStorage.StorageType == "" {
		w.PluginStorage.StorageType = config.StorageTypeLocal
	}
	// 配置默认的storageUrl
	if w.PluginStorage.StorageUrl == "" || w.PluginStorage.StorageType == config.StorageTypeLocal {
		w.PluginStorage.StorageUrl = w.System.BaseUrl
	}
}

func (w *Website) LoadPaySetting(value string) {
	w.PluginPay = &config.PluginPayConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginPay)
	}
}

func (w *Website) LoadWeappSetting(value string) {
	w.PluginWeapp = &config.PluginWeappConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginWeapp)
	}
}

func (w *Website) LoadWechatSetting(value string) {
	w.PluginWechat = &config.PluginWeappConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginWechat)
	}
}

func (w *Website) LoadRetailerSetting(value string) {
	w.PluginRetailer = &config.PluginRetailerConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginRetailer)
	}
}

func (w *Website) LoadUserSetting(value string) {
	w.PluginUser = &config.PluginUserConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginUser)
	}
	if w.PluginUser.DefaultGroupId == 0 {
		w.PluginUser.DefaultGroupId = 1
	}
}

func (w *Website) LoadOrderSetting(value string) {
	w.PluginOrder = &config.PluginOrderConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginOrder)
	}
	if w.PluginOrder.AutoFinishDay <= 0 {
		// default auto finish day
		w.PluginOrder.AutoFinishDay = 10
	}
}

func (w *Website) LoadFulltextSetting(value string) {
	w.PluginFulltext = &config.PluginFulltextConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginFulltext)
	}
}

func (w *Website) LoadTitleImageSetting(value string) {
	w.PluginTitleImage = &config.PluginTitleImageConfig{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginTitleImage)
	}
	if w.PluginTitleImage.Width == 0 {
		w.PluginTitleImage.Width = 800
	}
	if w.PluginTitleImage.Height == 0 {
		w.PluginTitleImage.Height = 600
	}
	if w.PluginTitleImage.FontSize == 0 {
		w.PluginTitleImage.FontSize = 32
	}
	if w.PluginTitleImage.FontColor == "" {
		w.PluginTitleImage.FontColor = "#ffffff"
	}
}

func (w *Website) LoadWatermarkSetting(value string) {
	w.PluginWatermark = &config.PluginWatermark{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginWatermark)
	}
	if w.PluginWatermark.Size == 0 {
		w.PluginWatermark.Size = 20
	}
	if w.PluginWatermark.Position == 0 {
		w.PluginWatermark.Position = 9
	}
	if w.PluginWatermark.Opacity == 0 {
		w.PluginWatermark.Opacity = 100
	}
	if w.PluginWatermark.MinSize == 0 {
		w.PluginWatermark.MinSize = 400
	}
	if w.PluginWatermark.Color == "" {
		w.PluginWatermark.Color = "#ffffff"
	}
}

func (w *Website) LoadHtmlCacheSetting(value string) {
	w.PluginHtmlCache = &config.PluginHtmlCache{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginHtmlCache)
	}
	// if no item, set to default
	// index default cache 5 minutes
	if w.PluginHtmlCache.Open == false {
		w.PluginHtmlCache.IndexCache = 300
		// list default cache 60 minutes
		w.PluginHtmlCache.ListCache = 3600
		// detail default cache 24 hours
		w.PluginHtmlCache.DetailCache = 86400
	}
}

func (w *Website) LoadAnqiUser(value string) {
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.AnqiUser)
	}

	go w.AnqiCheckLogin(false)
}

func (w *Website) LoadAiGenerateSetting(value string) {
	w.AiGenerateConfig = &config.AiGenerateConfig{}
	if value == "" {
		return
	}

	if err := json.Unmarshal([]byte(value), w.AiGenerateConfig); err != nil {
		return
	}
}

func (w *Website) LoadCollectorSetting(value string) {
	//先读取默认配置
	w.CollectorConfig = &config.DefaultCollectorConfig
	//再根据用户配置来覆盖
	if value == "" {
		return
	}

	var collector config.CollectorJson
	if err := json.Unmarshal([]byte(value), &collector); err != nil {
		return
	}

	// 配置代理
	if collector.ProxyConfig.Open && collector.ProxyConfig.ApiUrl != "" {
		w.CollectorConfig.ProxyConfig = collector.ProxyConfig
		// 启用代理
		w.Proxy = NewProxyIPs(&w.CollectorConfig.ProxyConfig)
	} else {
		// 释放代理
		w.Proxy = nil
	}
	//开始处理
	if collector.ErrorTimes != 0 {
		w.CollectorConfig.ErrorTimes = collector.ErrorTimes
	}
	if collector.Channels != 0 {
		w.CollectorConfig.Channels = collector.Channels
	}
	if collector.TitleMinLength != 0 {
		w.CollectorConfig.TitleMinLength = collector.TitleMinLength
	}
	if collector.ContentMinLength != 0 {
		w.CollectorConfig.ContentMinLength = collector.ContentMinLength
	}

	w.CollectorConfig.AutoCollect = collector.AutoCollect
	w.CollectorConfig.AutoPseudo = collector.AutoPseudo
	w.CollectorConfig.AutoTranslate = collector.AutoTranslate
	w.CollectorConfig.ToLanguage = collector.ToLanguage
	w.CollectorConfig.CategoryId = collector.CategoryId
	w.CollectorConfig.CategoryIds = collector.CategoryIds
	w.CollectorConfig.StartHour = collector.StartHour
	w.CollectorConfig.EndHour = collector.EndHour
	w.CollectorConfig.FromWebsite = collector.FromWebsite
	w.CollectorConfig.CollectMode = collector.CollectMode
	w.CollectorConfig.SaveType = collector.SaveType
	w.CollectorConfig.Language = collector.Language
	w.CollectorConfig.InsertImage = collector.InsertImage
	w.CollectorConfig.Images = collector.Images
	w.CollectorConfig.ImageCategoryId = collector.ImageCategoryId

	if w.CollectorConfig.Language == "" {
		w.CollectorConfig.Language = config.LanguageZh
	}

	if collector.DailyLimit > 0 {
		w.CollectorConfig.DailyLimit = collector.DailyLimit
	}
	if w.CollectorConfig.DailyLimit > 10000 {
		//最大1万，否则发布不完
		w.CollectorConfig.DailyLimit = 10000
	}

	for _, v := range collector.TitleExclude {
		exists := false
		for _, vv := range w.CollectorConfig.TitleExclude {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			w.CollectorConfig.TitleExclude = append(w.CollectorConfig.TitleExclude, v)
		}
	}
	for _, v := range collector.TitleExcludePrefix {
		exists := false
		for _, vv := range w.CollectorConfig.TitleExcludePrefix {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			w.CollectorConfig.TitleExcludePrefix = append(w.CollectorConfig.TitleExcludePrefix, v)
		}
	}
	for _, v := range collector.TitleExcludeSuffix {
		exists := false
		for _, vv := range w.CollectorConfig.TitleExcludeSuffix {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			w.CollectorConfig.TitleExcludeSuffix = append(w.CollectorConfig.TitleExcludeSuffix, v)
		}
	}
	for _, v := range collector.ContentExcludeLine {
		exists := false
		for _, vv := range w.CollectorConfig.ContentExcludeLine {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			w.CollectorConfig.ContentExcludeLine = append(w.CollectorConfig.ContentExcludeLine, v)
		}
	}
	for _, v := range collector.ContentExclude {
		exists := false
		for _, vv := range w.CollectorConfig.ContentExclude {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			w.CollectorConfig.ContentExclude = append(w.CollectorConfig.ContentExclude, v)
		}
	}
	for _, v := range collector.ContentReplace {
		exists := false
		for _, vv := range w.CollectorConfig.ContentReplace {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			w.CollectorConfig.ContentReplace = append(w.CollectorConfig.ContentReplace, v)
		}
	}
}

func (w *Website) LoadKeywordSetting(value string) {
	//先读取默认配置
	w.KeywordConfig = &config.DefaultKeywordConfig
	//再根据用户配置来覆盖
	if value == "" {
		return
	}

	_ = json.Unmarshal([]byte(value), w.KeywordConfig)

	var keyword config.KeywordJson
	if err := json.Unmarshal([]byte(value), &keyword); err != nil {
		return
	}

	w.KeywordConfig.AutoDig = keyword.AutoDig
	w.KeywordConfig.Language = keyword.Language
	w.KeywordConfig.MaxCount = keyword.MaxCount

	for _, v := range keyword.TitleExclude {
		exists := false
		for _, vv := range w.KeywordConfig.TitleExclude {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			w.KeywordConfig.TitleExclude = append(w.KeywordConfig.TitleExclude, v)
		}
	}
	for _, v := range keyword.TitleReplace {
		exists := false
		for _, vv := range w.KeywordConfig.TitleReplace {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			w.KeywordConfig.TitleReplace = append(w.KeywordConfig.TitleReplace, v)
		}
	}
}

func (w *Website) LoadTimeFactorSetting(value string) {
	w.PluginTimeFactor = &config.PluginTimeFactor{}
	if value == "" {
		return
	}

	if err := json.Unmarshal([]byte(value), w.PluginTimeFactor); err != nil {
		return
	}

	return
}

func (w *Website) LoadMultiLangSetting(value string) {
	w.MultiLanguage = &config.PluginMultiLangConfig{}
	if value == "" {
		return
	}

	if err := json.Unmarshal([]byte(value), w.MultiLanguage); err != nil {
		return
	}
	if w.MultiLanguage.SiteType == "" {
		w.MultiLanguage.SiteType = config.MultiLangSiteTypeMulti
	}
	w.MultiLanguage.DefaultLanguage = w.System.Language
	return
}

func (w *Website) LoadTranslateSetting(value string) {
	w.PluginTranslate = &config.PluginTranslateConfig{}
	if value == "" {
		return
	}

	if err := json.Unmarshal([]byte(value), w.PluginTranslate); err != nil {
		return
	}

	return
}

func (w *Website) LoadJsonLdSetting(value string) {
	w.PluginJsonLd = &config.PluginJsonLdConfig{}
	if value == "" {
		return
	}

	if err := json.Unmarshal([]byte(value), w.PluginJsonLd); err != nil {
		return
	}

	return
}

func (w *Website) GetDiyFieldSetting() []config.ExtraField {
	var fields []config.ExtraField
	err := w.Cache.Get(DiyFieldsKey, &fields)
	if err != nil {
		value := w.GetSettingValue(DiyFieldsKey)
		if value != "" {
			err = json.Unmarshal([]byte(value), &fields)
			if err == nil {
				_ = w.Cache.Set(DiyFieldsKey, fields, 86400)
			}
		}
	}

	return fields
}

// Tr as Translate
// 这是一个兼容函数，请使用 ctx.Tr
func (w *Website) Tr(str string, args ...interface{}) string {
	if I18n == nil {
		I18n = i18n.New()
		_ = I18n.Load(config.ExecPath+"locales/*/*.yml", config.LoadLocales()...)
		// default to chinese
		lang, exists := os.LookupEnv("LANG")
		if !exists {
			lang = "zh-CN"
		} else {
			lang = strings.ReplaceAll(strings.Split(lang, ".")[0], "_", "-")
		}
		I18n.SetDefault(lang)
	}
	if I18n != nil && w != nil {
		tmpStr := I18n.Tr(w.backLanguage, str, args...)
		if tmpStr != "" {
			return tmpStr
		}
	}

	//return fmt.Sprintf(str, args...)
	if len(args) > 0 {
		return str + fmt.Sprintf("%v", args)
	}
	return str
}

func (w *Website) TplTr(str string, args ...interface{}) string {
	if w.TplI18n != nil {
		tmpStr := w.TplI18n.Tr(w.System.Language, str, args...)
		if tmpStr != "" {
			return tmpStr
		}
	}
	if I18n != nil {
		tmpStr := I18n.Tr(w.System.Language, str, args...)
		if tmpStr != "" {
			return tmpStr
		}
	}
	return fmt.Sprintf(str, args...)
}

func (w *Website) GetSettingValue(key string) string {
	var value string
	if w.DB == nil {
		return value
	}
	w.DB.Model(&model.Setting{}).Where("`key` = ?", key).Pluck("value", &value)
	return value
}

func (w *Website) SaveSettingValue(key string, value interface{}) error {
	if w.DB == nil {
		return nil
	}
	setting := model.Setting{
		Key: key,
	}

	buf, err := json.Marshal(value)
	if err != nil {
		return err
	}
	setting.Value = string(buf)

	return w.DB.Save(&setting).Error
}

func (w *Website) SaveSettingValueRaw(key string, value interface{}) error {
	if w.DB == nil {
		return nil
	}
	setting := model.Setting{
		Key:   key,
		Value: fmt.Sprintf("%v", value),
	}

	return w.DB.Save(&setting).Error
}

func (w *Website) DeleteCache() {
	// todo, 清理缓存
	w.Cache.CleanAll()
	// 释放词典
	DictClose()
	// 记录
	filePath := w.CachePath + "cache_clear.log"
	os.WriteFile(filePath, []byte(fmt.Sprintf("%d", time.Now().Unix())), os.ModePerm)
}

func (w *Website) MatchSensitiveWords(content string) (matches []string) {
	if len(w.SensitiveWords) == 0 || len(content) == 0 {
		return
	}
	// content 需要移除代码
	content = library.StripTags(content)
	for _, word := range w.SensitiveWords {
		if len(word) == 0 {
			continue
		}
		if strings.Contains(content, word) {
			matches = append(matches, word)
		}
	}

	return
}

func (w *Website) ReplaceSensitiveWords(content []byte) []byte {
	if len(w.SensitiveWords) == 0 || len(content) == 0 {
		return content
	}

	type replaceType struct {
		Key   []byte
		Value []byte
	}
	var replacedMatch []*replaceType
	numCount := 0
	//过滤所有属性
	reg, _ := regexp.Compile("(?i)<!?/?[a-z0-9-]+(\\s+[^>]+)?>")
	content = reg.ReplaceAllFunc(content, func(s []byte) []byte {
		key := []byte(fmt.Sprintf("{$%d}", numCount))
		replacedMatch = append(replacedMatch, &replaceType{
			Key:   key,
			Value: s,
		})
		numCount++

		return key
	})
	// 替换所有敏感词为星号
	for _, word := range w.SensitiveWords {
		if len(word) == 0 {
			continue
		}
		if bytes.Contains(content, []byte(word)) {
			content = bytes.ReplaceAll(content, []byte(word), bytes.Repeat([]byte("*"), utf8.RuneCountInString(word)))
		} else {
			// 增加支持正则表达式替换，定义正则表达式以{开头}结束，如：{[1-9]\d{4,10}}
			if strings.HasPrefix(word, "{") && strings.HasSuffix(word, "}") && len(word) > 2 {
				// 移除首尾花括号
				newWord := word[1 : len(word)-1]
				re, err := regexp.Compile(newWord)
				if err == nil {
					content = re.ReplaceAll(content, bytes.Repeat([]byte("*"), utf8.RuneCountInString(word)))
				}
				continue
			}
		}
	}
	// 替换回来
	for i := len(replacedMatch) - 1; i >= 0; i-- {
		content = bytes.Replace(content, replacedMatch[i].Key, replacedMatch[i].Value, 1)
	}

	return content
}

func (w *Website) LoadInterferenceSetting(value string) {
	w.PluginInterference = &config.PluginInterference{}
	if value != "" {
		_ = json.Unmarshal([]byte(value), w.PluginInterference)
	}
}

func (w *Website) GetLimiterSetting() *config.PluginLimiter {
	var limiter config.PluginLimiter
	value := w.GetSettingValue(LimiterSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &limiter)
	}

	return &limiter
}

func (w *Website) ReplaceContentUrl(content string, reverse bool) string {
	if len(content) == 0 {
		return content
	}
	// todo，替换的时候，还需要考虑 Markdown
	// 匹配Markdown ![新的图片](http://xxx/xxx.webp)
	mdRe, _ := regexp.Compile(`!\[([^]]*)\]\(([^)]+)\)`)
	if mdRe.MatchString(content) {
		content = mdRe.ReplaceAllStringFunc(content, func(s string) string {
			match := mdRe.FindStringSubmatch(s)
			if len(match) < 3 {
				return s
			}
			if reverse {
				// 恢复
				if strings.HasPrefix(match[2], "/uploads") {
					s = strings.Replace(s, match[2], w.PluginStorage.StorageUrl+match[2], 1)
				}
			} else {
				s = strings.Replace(s, match[2], strings.TrimPrefix(match[2], w.PluginStorage.StorageUrl), 1)
			}

			return s
		})
	}

	if reverse {
		// 支持替换url
		if strings.HasPrefix(content, "/uploads") {
			content = w.PluginStorage.StorageUrl + content
		} else {
			// 恢复
			content = strings.ReplaceAll(content, "\"/uploads", "\""+w.PluginStorage.StorageUrl+"/uploads")
		}
		return content
	} else {
		// todo 应该只替换 src,href 中的 baseUrl
		// 图片都上传到 uploads
		if strings.HasPrefix(content, w.PluginStorage.StorageUrl) {
			content = strings.TrimPrefix(content, w.PluginStorage.StorageUrl)
		} else {
			content = strings.ReplaceAll(content, "\""+w.PluginStorage.StorageUrl+"/uploads", "\"/uploads")
			// 如果baseUrl 是 127.0.0.1 或者localhost，则把它们全部替换
			if strings.Contains(w.System.BaseUrl, "127.0.0.1") || strings.Contains(w.System.BaseUrl, "localhost") {
				content = strings.ReplaceAll(content, w.System.BaseUrl, "")
			}
		}
	}

	return content
}
