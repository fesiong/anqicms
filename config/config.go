package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"goblog/utils"
)

var jsonData map[string]interface{}

func initJSON() {
	bytes, err := ioutil.ReadFile("./config.json")
	if err != nil {
		fmt.Println("ReadFile: ", err.Error())
		os.Exit(-1)
	}

	configStr := string(bytes[:])
	reg := regexp.MustCompile(`/\*.*\*/`)

	configStr = reg.ReplaceAllString(configStr, "")
	bytes = []byte(configStr)

	if err := json.Unmarshal(bytes, &jsonData); err != nil {
		fmt.Println("invalid config: ", err.Error())
		os.Exit(-1)
	}
}

type dBConfig struct {
	Dialect      string
	Database     string
	User         string
	Password     string
	Host         string
	Port         int
	Charset      string
	URL          string
	MaxIdleConns int
	MaxOpenConns int
	TablePrefix  string
}

// DBConfig 数据库相关配置
var DBConfig dBConfig

func initDB() {
	utils.SetStructByJSON(&DBConfig, jsonData["database"].(map[string]interface{}))
	url := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		DBConfig.User, DBConfig.Password, DBConfig.Host, DBConfig.Port, DBConfig.Database, DBConfig.Charset)
	DBConfig.URL = url
}

type serverConfig struct {
	SiteName           string
	Host               string
	StaticHost         string
	Env                string
	LogDir             string
	LogFile            string
	APIPrefix          string
	UploadDir          string
	UploadPath         string
	Port               int
	TokenSecret        string
	TokenMaxAge        int
	PassSalt           string
	MaxMultipartMemory int
}

// ServerConfig 服务器相关配置
var ServerConfig serverConfig

func initServer() {
	utils.SetStructByJSON(&ServerConfig, jsonData["go"].(map[string]interface{}))
	sep := string(os.PathSeparator)
	execPath, _ := os.Getwd()
	length := utf8.RuneCountInString(execPath)
	lastChar := execPath[length-1:]
	if lastChar != sep {
		execPath = execPath + sep
	}
	if ServerConfig.UploadDir == "" {
		pathArr := []string{"website", "uploads"}
		uploadDir := execPath + strings.Join(pathArr, sep)
		ServerConfig.UploadDir = uploadDir
	}

	ymdStr := utils.GetTodayYMD("-")

	if ServerConfig.LogDir == "" {
		ServerConfig.LogDir = execPath + "logs" + sep
	} else {
		length := utf8.RuneCountInString(ServerConfig.LogDir)
		lastChar := ServerConfig.LogDir[length-1:]
		if lastChar != sep {
			ServerConfig.LogDir = ServerConfig.LogDir + sep
		}
	}
	ServerConfig.LogFile = ServerConfig.LogDir + ymdStr + ".log"
}

func init() {
	initJSON()
	initDB()
	initServer()
}
