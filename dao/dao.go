package dao

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"os"
	"strings"
)

// DB connection
var DB *gorm.DB
var OriginDB *gorm.DB
var err error

func init() {
	if config.JsonData.Mysql.Database != "" {
		err := InitDB()
		if err != nil {
			fmt.Println("Failed To Connect Database: ", err.Error())
			os.Exit(-1)
		}
	}
}

func InitDB() error {
	var db *gorm.DB
	var err error
	url := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.JsonData.Mysql.User, config.JsonData.Mysql.Password, config.JsonData.Mysql.Host, config.JsonData.Mysql.Port, config.JsonData.Mysql.Database)
	config.JsonData.Mysql.Url = url
	db, err = gorm.Open(mysql.Open(url), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "1049") {
			url2 := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local",
				config.JsonData.Mysql.User, config.JsonData.Mysql.Password, config.JsonData.Mysql.Host, config.JsonData.Mysql.Port)
			db, err = gorm.Open(mysql.Open(url2), &gorm.Config{
				DisableForeignKeyConstraintWhenMigrating: true,
			})
			if err != nil {
				return err
			}
			err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", config.JsonData.Mysql.Database)).Error
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

func AutoMigrateDB(db *gorm.DB) error {
	//自动迁移数据库
	err := db.AutoMigrate(
		&model.Admin{},
		&model.AdminLoginLog{},
		&model.AdminLog{},
		&model.Attachment{},
		&model.AttachmentCategory{},
		&model.Category{},
		&model.Nav{},
		&model.Link{},
		&model.Comment{},
		&model.Anchor{},
		&model.AnchorData{},
		&model.Guestbook{},
		&model.Keyword{},
		&model.Material{},
		&model.MaterialCategory{},
		&model.MaterialData{},
		&model.Statistic{},
		&model.Tag{},
		&model.TagData{},
		&model.Redirect{},
		&model.Module{},
		&model.Archive{},
		&model.ArchiveData{},
		&model.SpiderInclude{},
	)

	if err != nil {
		return err
	}

	// 检查默认模型，如果没有，则添加, 默认的模型：1 文章，2 产品
	var modules = []model.Module{
		{
			Model:     model.Model{Id: 1},
			TableName: "article",
			UrlToken:  "news",
			Title:     "文章中心",
			Fields:    nil,
			IsSystem:  1,
			TitleName: "标题",
			Status:    1,
		},
		{
			Model:     model.Model{Id: 2},
			TableName: "product",
			UrlToken:  "product",
			Title:     "产品中心",
			Fields:    []config.CustomField{
				{
					Name: "价格",
					FieldName: "price",
					Type: "number",
					Required: false,
					IsSystem: true,
				},
				{
					Name: "库存",
					FieldName: "stock",
					Type: "number",
					Required: false,
					IsSystem: true,
				},
			},
			IsSystem:  1,
			TitleName: "产品名称",
			Status:    1,
		},
	}
	for _, m := range modules {
		var exists int64
		db.Model(&model.Module{}).Where("`id` = ?", m.Id).Count(&exists)
		if exists == 0 {
			db.Create(&m)
			// 并生成表
			m.Migrate(db)
		}
	}

	return nil
}