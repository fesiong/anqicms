package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

type ServerJson struct {
	Mysql  MysqlConfig  `json:"mysql"`
	Server ServerConfig `json:"server"`
}

type ConfigJson struct {
	//setting
	System  SystemConfig  `json:"system"`
	Content ContentConfig `json:"content"`
	Index   IndexConfig   `json:"index"`
	Contact ContactConfig `json:"contact"`
	Safe    SafeConfig    `json:"safe"`
	//plugin
	PluginPush        PluginPushConfig      `json:"plugin_push"`
	PluginSitemap     PluginSitemapConfig   `json:"plugin_sitemap"`
	PluginRewrite     PluginRewriteConfig   `json:"plugin_rewrite"`
	PluginAnchor      PluginAnchorConfig    `json:"plugin_anchor"`
	PluginGuestbook   PluginGuestbookConfig `json:"plugin_guestbook"`
	PluginUploadFiles []PluginUploadFile    `json:"plugin_upload_file"`
	PluginSendmail    PluginSendmail        `json:"plugin_sendmail"`
	PluginImportApi   PluginImportApiConfig `json:"plugin_import_api"`
	PluginStorage     PluginStorageConfig   `json:"plugin_storage"`
	PluginPay         PluginPayConfig       `json:"plugin_pay"`
	PluginWeapp       PluginWeappConfig     `json:"plugin_weapp"`
	PluginWechat      PluginWeappConfig     `json:"plugin_wechat"`
	PluginRetailer    PluginRetailerConfig  `json:"plugin_retailer"`
	PluginUser        PluginUserConfig      `json:"plugin_user"`
	PluginOrder       PluginOrderConfig     `json:"plugin_order"`
	PluginFulltext    PluginFulltextConfig  `json:"plugin_fulltext"`
}

func initPath() {
	sep := string(os.PathSeparator)
	//root := filepath.Dir(os.Args[0])
	//ExecPath, _ = filepath.Abs(root)
	ExecPath, _ = os.Executable()
	baseName := filepath.Base(ExecPath)
	ExecPath = filepath.Dir(ExecPath)
	if strings.Contains(baseName, "go_build") || strings.Contains(ExecPath, "go-build") || strings.Contains(baseName, "Test") {
		ExecPath, _ = os.Getwd()
	}
	if strings.Contains(baseName, "Test") {
		ExecPath = filepath.Dir(ExecPath)
	}
	length := utf8.RuneCountInString(ExecPath)
	lastChar := ExecPath[length-1:]
	if lastChar != sep {
		ExecPath = ExecPath + sep
	}
	ExecPath = strings.ReplaceAll(ExecPath, "\\", "/")
	log.Println(ExecPath)
}

func initJSON() {
	rawConfig, err := os.ReadFile(fmt.Sprintf("%sconfig.json", ExecPath))
	if err != nil {
		//未初始化
		rawConfig = []byte("{\"db\":{},\"server\":{\"env\": \"production\",\"port\": 8001,\"log_level\":\"error\"}}")
	}

	if err := json.Unmarshal(rawConfig, &Server); err != nil {
		fmt.Println("Invalid Config: ", err.Error())
		os.Exit(-1)
	}
}

var ExecPath string
var Server ServerJson
var JsonData ConfigJson
var CollectorConfig CollectorJson
var KeywordConfig KeywordJson
var AnqiUser AnqiUserConfig
var RestartChan = make(chan bool, 1)
var languages = map[string]string{}

func init() {
	initPath()
	initJSON()
	LoadCollectorConfig()
	LoadKeywordConfig()
	LoadLanguage()
}

func WriteConfig() error {
	//将现有配置写回文件
	configFile, err := os.OpenFile(fmt.Sprintf("%sconfig.json", ExecPath), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}

	defer configFile.Close()

	buff := &bytes.Buffer{}

	buf, err := json.MarshalIndent(Server, "", "\t")
	if err != nil {
		return err
	}
	buff.Write(buf)

	_, err = io.Copy(configFile, buff)
	if err != nil {
		return err
	}

	return nil
}

func LoadCollectorConfig() {
	//先读取默认配置
	CollectorConfig = DefaultCollectorConfig
	//再根据用户配置来覆盖
	buf, err := os.ReadFile(fmt.Sprintf("%scollector.json", ExecPath))
	configStr := ""
	if err != nil {
		//文件不存在
		return
	}
	configStr = string(buf[:])
	reg := regexp.MustCompile(`/\*.*\*/`)

	configStr = reg.ReplaceAllString(configStr, "")
	buf = []byte(configStr)

	var collector CollectorJson
	if err = json.Unmarshal(buf, &collector); err != nil {
		return
	}

	//开始处理
	if collector.ErrorTimes != 0 {
		CollectorConfig.ErrorTimes = collector.ErrorTimes
	}
	if collector.Channels != 0 {
		CollectorConfig.Channels = collector.Channels
	}
	if collector.TitleMinLength != 0 {
		CollectorConfig.TitleMinLength = collector.TitleMinLength
	}
	if collector.ContentMinLength != 0 {
		CollectorConfig.ContentMinLength = collector.ContentMinLength
	}

	CollectorConfig.AutoCollect = collector.AutoCollect
	CollectorConfig.AutoPseudo = collector.AutoPseudo
	CollectorConfig.CategoryId = collector.CategoryId
	CollectorConfig.StartHour = collector.StartHour
	CollectorConfig.EndHour = collector.EndHour
	CollectorConfig.FromWebsite = collector.FromWebsite
	CollectorConfig.CollectMode = collector.CollectMode
	CollectorConfig.SaveType = collector.SaveType
	CollectorConfig.FromEngine = collector.FromEngine
	CollectorConfig.Language = collector.Language
	CollectorConfig.InsertImage = collector.InsertImage
	CollectorConfig.Images = collector.Images

	if CollectorConfig.Language == "" {
		CollectorConfig.Language = LanguageZh
	}

	if collector.DailyLimit > 0 {
		CollectorConfig.DailyLimit = collector.DailyLimit
	}
	if CollectorConfig.DailyLimit > 10000 {
		//最大1万，否则发布不完
		CollectorConfig.DailyLimit = 10000
	}

	for _, v := range collector.TitleExclude {
		exists := false
		for _, vv := range CollectorConfig.TitleExclude {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			CollectorConfig.TitleExclude = append(CollectorConfig.TitleExclude, v)
		}
	}
	for _, v := range collector.TitleExcludePrefix {
		exists := false
		for _, vv := range CollectorConfig.TitleExcludePrefix {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			CollectorConfig.TitleExcludePrefix = append(CollectorConfig.TitleExcludePrefix, v)
		}
	}
	for _, v := range collector.TitleExcludeSuffix {
		exists := false
		for _, vv := range CollectorConfig.TitleExcludeSuffix {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			CollectorConfig.TitleExcludeSuffix = append(CollectorConfig.TitleExcludeSuffix, v)
		}
	}
	for _, v := range collector.ContentExcludeLine {
		exists := false
		for _, vv := range CollectorConfig.ContentExcludeLine {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			CollectorConfig.ContentExcludeLine = append(CollectorConfig.ContentExcludeLine, v)
		}
	}
	for _, v := range collector.ContentExclude {
		exists := false
		for _, vv := range CollectorConfig.ContentExclude {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			CollectorConfig.ContentExclude = append(CollectorConfig.ContentExclude, v)
		}
	}
	for _, v := range collector.ContentReplace {
		exists := false
		for _, vv := range CollectorConfig.ContentReplace {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			CollectorConfig.ContentReplace = append(CollectorConfig.ContentReplace, v)
		}
	}
}

func LoadKeywordConfig() {
	//先读取默认配置
	KeywordConfig = DefaultKeywordConfig
	//再根据用户配置来覆盖
	buf, err := os.ReadFile(fmt.Sprintf("%skeyword.json", ExecPath))
	configStr := ""
	if err != nil {
		//文件不存在
		return
	}
	configStr = string(buf[:])
	reg := regexp.MustCompile(`/\*.*\*/`)

	configStr = reg.ReplaceAllString(configStr, "")
	buf = []byte(configStr)

	var keyword KeywordJson
	if err = json.Unmarshal(buf, &keyword); err != nil {
		return
	}

	KeywordConfig.AutoDig = keyword.AutoDig
	KeywordConfig.FromEngine = keyword.FromEngine
	KeywordConfig.FromWebsite = keyword.FromWebsite
	KeywordConfig.Language = keyword.Language

	for _, v := range keyword.TitleExclude {
		exists := false
		for _, vv := range KeywordConfig.TitleExclude {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			KeywordConfig.TitleExclude = append(KeywordConfig.TitleExclude, v)
		}
	}
	for _, v := range keyword.TitleReplace {
		exists := false
		for _, vv := range KeywordConfig.TitleReplace {
			if vv == v {
				exists = true
			}
		}
		if !exists {
			KeywordConfig.TitleReplace = append(KeywordConfig.TitleReplace, v)
		}
	}
}

func LoadLanguage() {
	// 重置
	languages = map[string]string{}
	if JsonData.System.Language == "" {
		// 默认中文
		JsonData.System.Language = "zh"
	}
	languagePath := fmt.Sprintf("%slanguage/%s.yml", ExecPath, JsonData.System.Language)

	yamlFile, err := os.ReadFile(languagePath)
	if err == nil {
		strSlice := strings.Split(strings.ReplaceAll(strings.ReplaceAll(string(yamlFile), "\r\n", "\n"), "\r", "\n"), "\n")
		for _, v := range strSlice {
			vSplit := strings.SplitN(strings.TrimSpace(v), ":", 2)
			if len(vSplit) == 2 {
				languages[strings.Trim(vSplit[0], "\" ")] = strings.Trim(vSplit[1], "\" ")
			}
		}
	}
	//ended
}

func Lang(str string) string {
	if newStr, ok := languages[str]; ok {
		return newStr
	}

	return str
}
