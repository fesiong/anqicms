package config

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type configData struct {
	Mysql  mysqlConfig  `json:"mysql"`
	Server serverConfig `json:"server"`
	//setting
	System  systemConfig  `json:"system"`
	Content contentConfig `json:"content"`
	Index   indexConfig   `json:"index"`
	Contact contactConfig `json:"contact"`
	//plugin
	PluginPush        pluginPushConfig      `json:"plugin_push"`
	PluginSitemap     pluginSitemapConfig   `json:"plugin_sitemap"`
	PluginRewrite     PluginRewriteConfig   `json:"plugin_rewrite"`
	PluginAnchor      pluginAnchorConfig    `json:"plugin_anchor"`
	PluginGuestbook   pluginGuestbookConfig `json:"plugin_guestbook"`
	PluginUploadFiles []PluginUploadFile    `json:"plugin_upload_file"`
	PluginSendmail    pluginSendmail        `json:"plugin_sendmail"`
	PluginImportApi   pluginImportApiConfig `json:"plugin_import_api"`
}

func initPath() {
	sep := string(os.PathSeparator)
	//root := filepath.Dir(os.Args[0])
	//ExecPath, _ = filepath.Abs(root)
	ExecPath, _ = os.Executable()
	baseName := filepath.Base(ExecPath)
	ExecPath = filepath.Dir(ExecPath)
	if strings.Contains(baseName, "go_build") || strings.Contains(baseName, "Test") {
		ExecPath, _ = os.Getwd()
	}
	pathArray := strings.Split(ExecPath, "/")
	//如果是测试目录，则保留到根目录。这定义根目录为：anqicms
	if strings.Contains(ExecPath, "\\") {
		pathArray = strings.Split(ExecPath, "\\")
	}

	for i, v := range pathArray {
		if v == "anqicms" {
			ExecPath = strings.Join(pathArray[:i+1], "/")
			break
		}
	}
	length := utf8.RuneCountInString(ExecPath)
	lastChar := ExecPath[length-1:]
	if lastChar != sep {
		ExecPath = ExecPath + sep
	}
	log.Println(ExecPath)
}

func initJSON() {
	rawConfig, err := ioutil.ReadFile(fmt.Sprintf("%sconfig.json", ExecPath))
	if err != nil {
		//未初始化
		rawConfig = []byte("{\"db\":{},\"server\":{\"site_name\":\"安企内容管理系统(AnqiCMS)\",\"env\": \"development\",\"port\": 8001,\"log_level\":\"debug\"}}")
	}

	if err := json.Unmarshal(rawConfig, &JsonData); err != nil {
		fmt.Println("Invalid Config: ", err.Error())
		os.Exit(-1)
	}

	//如果没有设置模板，则默认是default
	if JsonData.System.TemplateName == "" {
		JsonData.System.TemplateName = "default"
	}
	if JsonData.System.Language == "" {
		JsonData.System.Language = "zh"
	}
	// 兼容旧版 jscode
	if JsonData.PluginPush.JsCode != "" {
		JsonData.PluginPush.JsCodes = append(JsonData.PluginPush.JsCodes, CodeItem{
			Name:  "未命名JS",
			Value: JsonData.PluginPush.JsCode,
		})
		JsonData.PluginPush.JsCode = ""
		_ = WriteConfig()
	}
	// sitemap
	if JsonData.PluginSitemap.Type != "xml" {
		JsonData.PluginSitemap.Type = "txt"
	}
	// 导入API生成
	if JsonData.PluginImportApi.Token == "" {
		h := md5.New()
		h.Write([]byte(fmt.Sprintf("%d", time.Now().Nanosecond())))
		JsonData.PluginImportApi.Token = hex.EncodeToString(h.Sum(nil))
		// 回写
		_ = WriteConfig()
	}
}

func initServer() {
	ServerConfig = JsonData.Server
}

var ExecPath string
var JsonData configData
var ServerConfig serverConfig
var CollectorConfig CollectorJson
var RestartChan = make(chan bool, 1)
var languages = map[string]string{}

func init() {
	initPath()
	initJSON()
	initServer()
	LoadCollectorConfig()
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

	buf, err := json.MarshalIndent(JsonData, "", "\t")
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
	CollectorConfig = defaultCollectorConfig
	//再根据用户配置来覆盖
	buf, err := ioutil.ReadFile(fmt.Sprintf("%scollector.json", ExecPath))
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

func LoadLanguage() {
	// 重置
	languages = map[string]string{}
	if JsonData.System.Language == "" {
		// 默认中文
		JsonData.System.Language = "zh"
	}
	languagePath := fmt.Sprintf("%slanguage/%s.yml", ExecPath, JsonData.System.Language)

	yamlFile, err := ioutil.ReadFile(languagePath)
	if err == nil {
		strSlice := strings.Split(string(yamlFile), "\n")
		for _, v := range strSlice {
			vSplit := strings.SplitN(v, ":", 2)
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