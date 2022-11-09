package provider

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"os"
	"time"
)

const (
	SystemSettingKey  = "system"
	ContentSettingKey = "content"
	IndexSettingKey   = "index"
	ContactSettingKey = "contact"
	SafeSettingKey    = "safe"

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

	CollectorSettingKey = "collector"
	KeywordSettingKey   = "keyword"
)

func InitSetting() {
	if dao.DB == nil {
		return
	}
	// 需要对 config.json数据进行迁移
	transferConfig()
	// load setting from db
	LoadSystemSetting()
	LoadContentSetting()
	LoadIndexSetting()
	LoadContactSetting()
	LoadSafeSetting()

	LoadPushSetting()
	LoadSitemapSetting()
	LoadRewriteSetting()
	LoadAnchorSetting()
	LoadGuestbookSetting()
	LoadUploadFilesSetting()
	LoadSendmailSetting()
	LoadImportApiSetting()
	LoadStorageSetting()
	LoadPaySetting()
	LoadWeappSetting()
	LoadWechatSetting()
	LoadRetailerSetting()
	LoadUserSetting()
	LoadOrderSetting()
	LoadFulltextSetting()

	LoadCollectorSetting()
	LoadKeywordSetting()
}

func transferConfig() {
	var existCount int64
	dao.DB.Model(&model.Setting{}).Count(&existCount)
	if existCount > 0 {
		return
	}
	// 表示没迁移过
	rawConfig, err := os.ReadFile(fmt.Sprintf("%sconfig.json", config.ExecPath))
	if err != nil {
		return
	}
	if err = json.Unmarshal(rawConfig, &config.JsonData); err != nil {
		return
	}
	// 逐个更新
	err = SaveSettingValue(SystemSettingKey, config.JsonData.System)
	_ = SaveSettingValue(ContentSettingKey, config.JsonData.Content)
	_ = SaveSettingValue(IndexSettingKey, config.JsonData.Index)
	_ = SaveSettingValue(ContactSettingKey, config.JsonData.Contact)
	_ = SaveSettingValue(SafeSettingKey, config.JsonData.Safe)

	_ = SaveSettingValue(PushSettingKey, config.JsonData.PluginPush)
	_ = SaveSettingValue(SitemapSettingKey, config.JsonData.PluginSitemap)
	_ = SaveSettingValue(RewriteSettingKey, config.JsonData.PluginRewrite)
	_ = SaveSettingValue(AnchorSettingKey, config.JsonData.PluginAnchor)
	_ = SaveSettingValue(GuestbookSettingKey, config.JsonData.PluginGuestbook)
	_ = SaveSettingValue(UploadFilesSettingKey, config.JsonData.PluginUploadFiles)
	_ = SaveSettingValue(SendmailSettingKey, config.JsonData.PluginSendmail)
	_ = SaveSettingValue(ImportApiSettingKey, config.JsonData.PluginImportApi)
	_ = SaveSettingValue(StorageSettingKey, config.JsonData.PluginStorage)
	_ = SaveSettingValue(PaySettingKey, config.JsonData.PluginPay)
	_ = SaveSettingValue(WeappSettingKey, config.JsonData.PluginWeapp)
	_ = SaveSettingValue(WechatSettingKey, config.JsonData.PluginWechat)
	_ = SaveSettingValue(RetailerSettingKey, config.JsonData.PluginRetailer)
	_ = SaveSettingValue(UserSettingKey, config.JsonData.PluginUser)
	_ = SaveSettingValue(OrderSettingKey, config.JsonData.PluginOrder)

	if err == nil {
		// update config.json
		config.WriteConfig()
	}
	// collector
	buf, err := os.ReadFile(fmt.Sprintf("%scollector.json", config.ExecPath))
	if err == nil {
		if err = json.Unmarshal(buf, &config.CollectorConfig); err == nil {
			_ = SaveSettingValue(CollectorSettingKey, config.CollectorConfig)
		}
	}

	// keyword
	buf, err = os.ReadFile(fmt.Sprintf("%skeyword.json", config.ExecPath))
	if err == nil {
		if err = json.Unmarshal(buf, &config.KeywordConfig); err == nil {
			_ = SaveSettingValue(KeywordSettingKey, config.KeywordConfig)
		}
	}

}

func LoadSystemSetting() {
	value := GetSettingValue(SystemSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.System)
	}
	//如果没有设置模板，则默认是default
	if config.JsonData.System.TemplateName == "" {
		config.JsonData.System.TemplateName = "default"
	}
	if config.JsonData.System.Language == "" {
		config.JsonData.System.Language = "zh"
	}
}

func LoadContentSetting() {
	value := GetSettingValue(ContentSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.Content)
	}
}

func LoadIndexSetting() {
	value := GetSettingValue(IndexSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.Index)
	}
}

func LoadContactSetting() {
	value := GetSettingValue(ContactSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.Contact)
	}
}

func LoadSafeSetting() {
	value := GetSettingValue(SafeSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.Safe)
	}
}

func LoadPushSetting() {
	value := GetSettingValue(PushSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginPush)
	}
	// 兼容旧版 jscode
	if config.JsonData.PluginPush.JsCode != "" {
		config.JsonData.PluginPush.JsCodes = append(config.JsonData.PluginPush.JsCodes, config.CodeItem{
			Name:  "未命名JS",
			Value: config.JsonData.PluginPush.JsCode,
		})
		config.JsonData.PluginPush.JsCode = ""

		_ = SaveSettingValue(PushSettingKey, config.JsonData.PluginPush)
	}
}

func LoadSitemapSetting() {
	value := GetSettingValue(SitemapSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginSitemap)
	}
	// sitemap
	if config.JsonData.PluginSitemap.Type != "xml" {
		config.JsonData.PluginSitemap.Type = "txt"
	}
}

func LoadRewriteSetting() {
	value := GetSettingValue(RewriteSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginRewrite)
	}
}

func LoadAnchorSetting() {
	value := GetSettingValue(AnchorSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginAnchor)
	}
}

func LoadGuestbookSetting() {
	value := GetSettingValue(GuestbookSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginGuestbook)
	}
}

func LoadUploadFilesSetting() {
	value := GetSettingValue(UploadFilesSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginUploadFiles)
	}
}

func LoadSendmailSetting() {
	value := GetSettingValue(SendmailSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginSendmail)
	}
}

func LoadImportApiSetting() {
	value := GetSettingValue(ImportApiSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginImportApi)
	}
	// 导入API生成
	if config.JsonData.PluginImportApi.Token == "" || config.JsonData.PluginImportApi.LinkToken == "" {
		h := md5.New()
		h.Write([]byte(fmt.Sprintf("%d", time.Now().Nanosecond())))
		if config.JsonData.PluginImportApi.Token == "" {
			config.JsonData.PluginImportApi.Token = hex.EncodeToString(h.Sum(nil))
		}
		if config.JsonData.PluginImportApi.LinkToken == "" {
			config.JsonData.PluginImportApi.LinkToken = config.JsonData.PluginImportApi.Token
		}
		// 回写
		_ = SaveSettingValue(ImportApiSettingKey, config.JsonData.PluginImportApi)
	}

}

func LoadStorageSetting() {
	value := GetSettingValue(StorageSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginStorage)
	}
	// 配置默认的storageUrl
	if config.JsonData.PluginStorage.StorageUrl == "" {
		config.JsonData.PluginStorage.StorageUrl = config.JsonData.System.BaseUrl
	}
	if config.JsonData.PluginStorage.StorageType == "" {
		config.JsonData.PluginStorage.StorageType = config.StorageTypeLocal
	}
}

func LoadPaySetting() {
	value := GetSettingValue(PaySettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginPay)
	}
}

func LoadWeappSetting() {
	value := GetSettingValue(WeappSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginWeapp)
	}
}

func LoadWechatSetting() {
	value := GetSettingValue(WechatSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginWechat)
	}
}

func LoadRetailerSetting() {
	value := GetSettingValue(RetailerSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginRetailer)
	}
}

func LoadUserSetting() {
	value := GetSettingValue(UserSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginUser)
	}
	if config.JsonData.PluginUser.DefaultGroupId == 0 {
		config.JsonData.PluginUser.DefaultGroupId = 1
	}
}

func LoadOrderSetting() {
	value := GetSettingValue(OrderSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginOrder)
	}
	if config.JsonData.PluginOrder.AutoFinishDay <= 0 {
		// default auto finish day
		config.JsonData.PluginOrder.AutoFinishDay = 10
	}
}

func LoadFulltextSetting() {
	value := GetSettingValue(FulltextSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.JsonData.PluginFulltext)
	}
}

func LoadCollectorSetting() {
	//先读取默认配置
	config.CollectorConfig = config.DefaultCollectorConfig
	//再根据用户配置来覆盖
	value := GetSettingValue(CollectorSettingKey)
	if value == "" {
		return
	}

	var collector config.CollectorJson
	if err := json.Unmarshal([]byte(value), &collector); err != nil {
		return
	}

	//开始处理
	if collector.ErrorTimes != 0 {
		config.CollectorConfig.ErrorTimes = collector.ErrorTimes
	}
	if collector.Channels != 0 {
		config.CollectorConfig.Channels = collector.Channels
	}
	if collector.TitleMinLength != 0 {
		config.CollectorConfig.TitleMinLength = collector.TitleMinLength
	}
	if collector.ContentMinLength != 0 {
		config.CollectorConfig.ContentMinLength = collector.ContentMinLength
	}

	config.CollectorConfig.AutoCollect = collector.AutoCollect
	config.CollectorConfig.AutoPseudo = collector.AutoPseudo
	config.CollectorConfig.CategoryId = collector.CategoryId
	config.CollectorConfig.StartHour = collector.StartHour
	config.CollectorConfig.EndHour = collector.EndHour
	config.CollectorConfig.FromWebsite = collector.FromWebsite
	config.CollectorConfig.CollectMode = collector.CollectMode
	config.CollectorConfig.SaveType = collector.SaveType
	config.CollectorConfig.FromEngine = collector.FromEngine
	config.CollectorConfig.Language = collector.Language
	config.CollectorConfig.InsertImage = collector.InsertImage
	config.CollectorConfig.Images = collector.Images

	if config.CollectorConfig.Language == "" {
		config.CollectorConfig.Language = config.LanguageZh
	}

	if collector.DailyLimit > 0 {
		config.CollectorConfig.DailyLimit = collector.DailyLimit
	}
	if config.CollectorConfig.DailyLimit > 10000 {
		//最大1万，否则发布不完
		config.CollectorConfig.DailyLimit = 10000
	}

	for _, v := range collector.TitleExclude {
		exists := false
		for _, vv := range config.CollectorConfig.TitleExclude {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			config.CollectorConfig.TitleExclude = append(config.CollectorConfig.TitleExclude, v)
		}
	}
	for _, v := range collector.TitleExcludePrefix {
		exists := false
		for _, vv := range config.CollectorConfig.TitleExcludePrefix {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			config.CollectorConfig.TitleExcludePrefix = append(config.CollectorConfig.TitleExcludePrefix, v)
		}
	}
	for _, v := range collector.TitleExcludeSuffix {
		exists := false
		for _, vv := range config.CollectorConfig.TitleExcludeSuffix {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			config.CollectorConfig.TitleExcludeSuffix = append(config.CollectorConfig.TitleExcludeSuffix, v)
		}
	}
	for _, v := range collector.ContentExcludeLine {
		exists := false
		for _, vv := range config.CollectorConfig.ContentExcludeLine {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			config.CollectorConfig.ContentExcludeLine = append(config.CollectorConfig.ContentExcludeLine, v)
		}
	}
	for _, v := range collector.ContentExclude {
		exists := false
		for _, vv := range config.CollectorConfig.ContentExclude {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			config.CollectorConfig.ContentExclude = append(config.CollectorConfig.ContentExclude, v)
		}
	}
	for _, v := range collector.ContentReplace {
		exists := false
		for _, vv := range config.CollectorConfig.ContentReplace {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			config.CollectorConfig.ContentReplace = append(config.CollectorConfig.ContentReplace, v)
		}
	}
}

func LoadKeywordSetting() {
	//先读取默认配置
	config.KeywordConfig = config.DefaultKeywordConfig
	value := GetSettingValue(KeywordSettingKey)
	if value != "" {
		_ = json.Unmarshal([]byte(value), &config.KeywordConfig)
	}
	//再根据用户配置来覆盖
	if value == "" {
		return
	}

	var keyword config.KeywordJson
	if err := json.Unmarshal([]byte(value), &keyword); err != nil {
		return
	}

	config.KeywordConfig.AutoDig = keyword.AutoDig
	config.KeywordConfig.FromEngine = keyword.FromEngine
	config.KeywordConfig.FromWebsite = keyword.FromWebsite
	config.KeywordConfig.Language = keyword.Language

	for _, v := range keyword.TitleExclude {
		exists := false
		for _, vv := range config.KeywordConfig.TitleExclude {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			config.KeywordConfig.TitleExclude = append(config.KeywordConfig.TitleExclude, v)
		}
	}
	for _, v := range keyword.TitleReplace {
		exists := false
		for _, vv := range config.KeywordConfig.TitleReplace {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			config.KeywordConfig.TitleReplace = append(config.KeywordConfig.TitleReplace, v)
		}
	}
}

func GetSettingValue(key string) string {
	var value string
	dao.DB.Model(&model.Setting{}).Where("`key` = ?", key).Pluck("value", &value)
	return value
}

func SaveSettingValue(key string, value interface{}) error {
	setting := model.Setting{
		Key: key,
	}

	buf, err := json.Marshal(value)
	if err != nil {
		return err
	}
	setting.Value = string(buf)

	return dao.DB.Save(&setting).Error
}
