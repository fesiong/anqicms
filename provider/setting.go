package provider

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/kataras/iris/v12/i18n"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
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
	// load setting from db
	w.LoadSystemSetting()
	w.LoadContentSetting()
	w.LoadIndexSetting()
	w.LoadContactSetting()
	w.LoadSafeSetting()
	w.LoadBannerSetting()
	w.LoadSensitiveWords()

	w.LoadPushSetting()
	w.LoadSitemapSetting()
	w.LoadRewriteSetting()
	w.LoadAnchorSetting()
	w.LoadGuestbookSetting()
	w.LoadUploadFilesSetting()
	w.LoadSendmailSetting()
	w.LoadImportApiSetting()
	w.LoadStorageSetting()
	w.LoadPaySetting()
	w.LoadWeappSetting()
	w.LoadWechatSetting()
	w.LoadRetailerSetting()
	w.LoadUserSetting()
	w.LoadOrderSetting()
	w.LoadFulltextSetting()
	w.LoadTitleImageSetting()
	w.LoadHtmlCacheSetting()
	w.LoadAnqiUser()
	w.LoadAiGenerateSetting()
	w.LoadCollectorSetting()
	w.LoadKeywordSetting()
	w.LoadInterferenceSetting()
	w.LoadWatermarkSetting()
	// 检查OpenAIAPI是否可用
	go w.CheckOpenAIAPIValid()
}

func (w *Website) LoadSystemSetting() {
	value := w.GetSettingValue(SystemSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.System)
	}
	//如果没有设置模板，则默认是default
	if w.System.TemplateName == "" {
		w.System.TemplateName = "default"
	}
	if w.System.Language == "" {
		w.System.Language = "zh"
	}
	// 默认站点
	if w.Id == 1 {
		w.System.DefaultSite = true
	}
}

func (w *Website) LoadContentSetting() {
	value := w.GetSettingValue(ContentSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.Content)
	}
}

func (w *Website) LoadIndexSetting() {
	value := w.GetSettingValue(IndexSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.Index)
	}
}

func (w *Website) LoadBannerSetting() {
	value := w.GetSettingValue(BannerSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.Banner)
	}
}

func (w *Website) LoadSensitiveWords() {
	value := w.GetSettingValue(SensitiveWordsKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.SensitiveWords)
	}
}

func (w *Website) LoadContactSetting() {
	value := w.GetSettingValue(ContactSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.Contact)
	}
}

func (w *Website) LoadSafeSetting() {
	value := w.GetSettingValue(SafeSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.Safe)
	}
}

func (w *Website) LoadPushSetting() {
	value := w.GetSettingValue(PushSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginPush)
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

func (w *Website) LoadSitemapSetting() {
	value := w.GetSettingValue(SitemapSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginSitemap)
	}
	// sitemap
	if w.PluginSitemap.Type != "xml" {
		w.PluginSitemap.Type = "txt"
	}
}

func (w *Website) LoadRewriteSetting() {
	value := w.GetSettingValue(RewriteSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginRewrite)
	}
}

func (w *Website) LoadAnchorSetting() {
	value := w.GetSettingValue(AnchorSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginAnchor)
	}
}

func (w *Website) LoadGuestbookSetting() {
	value := w.GetSettingValue(GuestbookSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginGuestbook)
	}
}

func (w *Website) LoadUploadFilesSetting() {
	value := w.GetSettingValue(UploadFilesSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginUploadFiles)
	}
}

func (w *Website) LoadSendmailSetting() {
	value := w.GetSettingValue(SendmailSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginSendmail)
	}
	if len(w.PluginSendmail.SendType) == 0 {
		w.PluginSendmail.SendType = []int{SendTypeGuestbook}
	}
}

func (w *Website) LoadImportApiSetting() {
	value := w.GetSettingValue(ImportApiSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginImportApi)
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

func (w *Website) LoadStorageSetting() {
	value := w.GetSettingValue(StorageSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginStorage)
	}
	// 配置默认的storageUrl
	if w.PluginStorage.StorageUrl == "" {
		w.PluginStorage.StorageUrl = w.System.BaseUrl
	}
	if w.PluginStorage.StorageType == "" {
		w.PluginStorage.StorageType = config.StorageTypeLocal
	}
}

func (w *Website) LoadPaySetting() {
	value := w.GetSettingValue(PaySettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginPay)
	}
}

func (w *Website) LoadWeappSetting() {
	value := w.GetSettingValue(WeappSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginWeapp)
	}
}

func (w *Website) LoadWechatSetting() {
	value := w.GetSettingValue(WechatSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginWechat)
	}
}

func (w *Website) LoadRetailerSetting() {
	value := w.GetSettingValue(RetailerSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginRetailer)
	}
}

func (w *Website) LoadUserSetting() {
	value := w.GetSettingValue(UserSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginUser)
	}
	if w.PluginUser.DefaultGroupId == 0 {
		w.PluginUser.DefaultGroupId = 1
	}
}

func (w *Website) LoadOrderSetting() {
	value := w.GetSettingValue(OrderSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginOrder)
	}
	if w.PluginOrder.AutoFinishDay <= 0 {
		// default auto finish day
		w.PluginOrder.AutoFinishDay = 10
	}
}

func (w *Website) LoadFulltextSetting() {
	value := w.GetSettingValue(FulltextSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginFulltext)
	}
}

func (w *Website) LoadTitleImageSetting() {
	value := w.GetSettingValue(TitleImageSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginTitleImage)
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

func (w *Website) LoadWatermarkSetting() {
	value := w.GetSettingValue(WatermarkSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginWatermark)
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

func (w *Website) LoadHtmlCacheSetting() {
	value := w.GetSettingValue(HtmlCacheSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginHtmlCache)
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

func (w *Website) LoadAnqiUser() {
	value := w.GetSettingValue(AnqiSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.AnqiUser)
	}

	go w.AnqiCheckLogin(false)
}

func (w *Website) LoadAiGenerateSetting() {
	value := w.GetSettingValue(AiGenerateSettingKey)
	if value == "" {
		return
	}

	if err := json.Unmarshal([]byte(value), &w.AiGenerateConfig); err != nil {
		return
	}
}

func (w *Website) LoadCollectorSetting() {
	//先读取默认配置
	w.CollectorConfig = config.DefaultCollectorConfig
	//再根据用户配置来覆盖
	value := w.GetSettingValue(CollectorSettingKey)
	if value == "" {
		return
	}

	var collector config.CollectorJson
	if err := json.Unmarshal([]byte(value), &collector); err != nil {
		return
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

func (w *Website) LoadKeywordSetting() {
	//先读取默认配置
	w.KeywordConfig = config.DefaultKeywordConfig
	value := w.GetSettingValue(KeywordSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.KeywordConfig)
	}
	//再根据用户配置来覆盖
	if value == "" {
		return
	}

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

func (w *Website) GetTimeFactorSetting() (setting config.PluginTimeFactor) {
	value := w.GetSettingValue(TimeFactorKey)
	if value == "" {
		return
	}

	if err := json.Unmarshal([]byte(value), &setting); err != nil {
		return
	}

	return
}

// Tr as Translate, formats according to a format specifier and returns the resulting string after translate.
// 这是一个兼容函数，请使用 ctx.Tr
func (w *Website) Tr(str string, args ...interface{}) string {
	if I18n != nil {
		tmpStr := I18n.Tr(w.backLanguage, str, args...)
		if tmpStr != "" {
			return tmpStr
		}
	}

	return fmt.Sprintf(str, args...)
}

func (w *Website) TplTr(str string, args ...interface{}) string {
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
	library.DictClose()
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

func (w *Website) LoadInterferenceSetting() {
	value := w.GetSettingValue(InterferenceKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &w.PluginInterference)
	}
}
