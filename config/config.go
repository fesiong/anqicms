package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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
}

func initPath() {
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
