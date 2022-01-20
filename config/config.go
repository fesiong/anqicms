package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type configData struct {
	DB     mysqlConfig  `json:"mysql"`
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

	ArticleExtraFields []*CustomField `json:"article_extra_fields"`
	ProductExtraFields []*CustomField `json:"product_extra_fields"`
}

func initPath() {
	sep := string(os.PathSeparator)
	ExecPath, _ = os.Executable()
	ExecPath = filepath.Dir(ExecPath)
	pathArray := strings.Split(ExecPath, "/")
	if strings.Contains(ExecPath, "\\") {
		pathArray = strings.Split(ExecPath, "\\")
	}
	for i, v := range pathArray {
		if v == "irisweb" {
			ExecPath = strings.Join(pathArray[:i+1], "/")
			break
		}
	}

	if strings.Contains(ExecPath, "/GoLand") {
		//指定测试环境
		ExecPath = "/Users/fesion/data/gitpath/irisweb/"
	}

	length := utf8.RuneCountInString(ExecPath)
	lastChar := ExecPath[length-1:]
	if lastChar != sep {
		ExecPath = ExecPath + sep
	}
}

func initJSON() {
	rawConfig, err := ioutil.ReadFile(fmt.Sprintf("%sconfig.json", ExecPath))
	if err != nil {
		//未初始化
		rawConfig = []byte("{\"db\":{},\"server\":{\"site_name\":\"irisweb 博客\",\"env\": \"development\",\"port\": 8001,\"log_level\":\"debug\"}}")
	}

	if err := json.Unmarshal(rawConfig, &JsonData); err != nil {
		fmt.Println("Invalid Config: ", err.Error())
		os.Exit(-1)
	}
	//给后台添加默认值
	if JsonData.System.AdminUri == "" {
		JsonData.System.AdminUri = "/manage"
	}
	//如果没有设置模板，则默认是default
	if JsonData.System.TemplateName == "" {
		JsonData.System.TemplateName = "/default"
	}
}

func InitDB(setting *mysqlConfig) error {
	var db *gorm.DB
	var err error
	url := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		setting.User, setting.Password, setting.Host, setting.Port, setting.Database)
	setting.Url = url
	db, err = gorm.Open(mysql.Open(url), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "1049") {
			url2 := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local",
				setting.User, setting.Password, setting.Host, setting.Port)
			db, err = gorm.Open(mysql.Open(url2), &gorm.Config{
				DisableForeignKeyConstraintWhenMigrating: true,
			})
			if err != nil {
				return err
			}
			err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", setting.Database)).Error
			if err != nil {
				return err
			}
			//重新连接db
			db, err = gorm.Open(mysql.Open(url), &gorm.Config{
				DisableForeignKeyConstraintWhenMigrating: true,
			})
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(1000)
	sqlDB.SetMaxOpenConns(100000)
	sqlDB.SetConnMaxLifetime(-1)

	DB = db

	return nil
}

func initServer() {
	ServerConfig = JsonData.Server
	sep := string(os.PathSeparator)
	//root := filepath.Dir(os.Args[0])
	//ExecPath, _ = filepath.Abs(root)
	ExecPath, _ = os.Getwd()
	length := utf8.RuneCountInString(ExecPath)
	lastChar := ExecPath[length-1:]
	if lastChar != sep {
		ExecPath = ExecPath + sep
	}
}

var ExecPath string
var JsonData configData
var ServerConfig serverConfig
var DB *gorm.DB
var CollectorConfig CollectorJson

func init() {
	initPath()
	initJSON()
	initServer()
	if JsonData.DB.Database != "" {
		err := InitDB(&JsonData.DB)
		if err != nil {
			fmt.Println("Failed To Connect Database: ", err.Error())
			os.Exit(-1)
		}
	}

	LoadCollectorConfig()
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
