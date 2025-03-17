package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"kandaoni.com/anqicms/library"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

type ServerJson struct {
	Mysql  MysqlConfig  `json:"mysql"`
	Server ServerConfig `json:"server"`

	Sparks map[string]SparkSetting `json:"sparks"`
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
		Server.Server.Env = "production"
		Server.Server.Port = 8001
		Server.Server.LogLevel = "error"
	} else {
		if err = json.Unmarshal(rawConfig, &Server); err != nil {
			fmt.Println("Invalid Config: ", err.Error())
			logFile, err := os.OpenFile(ExecPath+"error.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0766)
			if nil == err {
				logFile.WriteString(fmt.Sprintln(time.Now().Format("2006-01-02 15:04:05"), "Invalid Config", err.Error()))
				defer logFile.Close()
			}
			os.Exit(-1)
		}
	}
}

var ExecPath string
var Server ServerJson
var AnqiUser AnqiUserConfig

var GoogleValid bool // can visit google or not

// RestartChan 1 to restart app, 0 to reload template, 2 to exit app
var RestartChan = make(chan int)

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

func GenerateRandString(length int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	buf := make([]byte, length)
	for i := 0; i < length; i++ {
		b := r.Intn(26) + 65
		buf[i] = byte(b)
	}
	return strings.ToLower(string(buf))
}

func LoadLocales() (languages []string) {
	// 读取language列表
	readerInfos, err := os.ReadDir(fmt.Sprintf("%slocales", ExecPath))
	var added = map[string]struct{}{}
	if err == nil {
		for _, info := range readerInfos {
			if info.IsDir() {
				added[info.Name()] = struct{}{}
				languages = append(languages, info.Name())
			}
		}
	}
	// 增加所有支持的语言
	for _, lang := range library.Languages {
		if _, ok := added[lang.Code]; !ok {
			languages = append(languages, lang.Code)
		}
	}

	return languages
}
