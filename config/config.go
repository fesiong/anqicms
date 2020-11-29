package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"io"
	"io/ioutil"
	"irisweb/model"
	"os"
	"unicode/utf8"
)

type configData struct {
	DB     dbConfig     `json:"mysql"`
	Server serverConfig `json:"server"`
}

func initJSON() {
	rawConfig, err := ioutil.ReadFile("./config.json")
	if err != nil {
		//未初始化
		rawConfig = []byte("{\"db\":{},\"server\":{\"site_name\":\"irisweb 博客\",\"env\": \"development\",\"port\": 8001,\"log_level\":\"debug\"}}")
	}

	if err := json.Unmarshal(rawConfig, &JsonData); err != nil {
		fmt.Println("Invalid Config: ", err.Error())
		os.Exit(-1)
	}

	initServer()
	if JsonData.DB.Database != "" {
		err = InitDB(&JsonData.DB)
		if err != nil {
			fmt.Println("Failed To Connect Database: ", err.Error())
			os.Exit(-1)
		}
	}
}

func InitDB(setting *dbConfig) error {
	var db *gorm.DB
	var err error
	url := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		setting.User, setting.Password, setting.Host, setting.Port, setting.Database)
	setting.Url = url
	db, err = gorm.Open(mysql.Open(url), &gorm.Config{})
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		fmt.Println(setting, err.Error())
		os.Exit(-1)
	}

	sqlDB.SetMaxIdleConns(1000)
	sqlDB.SetMaxOpenConns(100000)
	sqlDB.SetConnMaxLifetime(-1)

	//创建表
	db.AutoMigrate(&model.Admin{}, &model.Article{}, &model.ArticleData{}, &model.Attachment{}, &model.Category{})

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
