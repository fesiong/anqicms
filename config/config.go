package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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
