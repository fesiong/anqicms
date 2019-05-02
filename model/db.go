package model

import (
	"fmt"
	"os"

	"goblog/config"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

// DB 数据库连接
var DB *gorm.DB

func initDB() {
	db, err := gorm.Open(config.DBConfig.Dialect, config.DBConfig.URL)
	if err != nil {
		fmt.Println("test db")
		fmt.Println(err.Error())
		os.Exit(-1)
	}
	if config.ServerConfig.Env == DevelopmentMode {
		db.LogMode(true)
	}
	db.DB().SetMaxIdleConns(config.DBConfig.MaxIdleConns)
	db.DB().SetMaxOpenConns(config.DBConfig.MaxOpenConns)

	//表前缀
	gorm.DefaultTableNameHandler = func(db *gorm.DB, defaultTableName string) string {
		return config.DBConfig.TablePrefix + defaultTableName
	}

	DB = db
}

func init() {
	initDB()
}
