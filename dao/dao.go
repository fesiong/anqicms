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

func init() {
	if config.Server.Mysql.Database != "" {
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
		config.Server.Mysql.User, config.Server.Mysql.Password, config.Server.Mysql.Host, config.Server.Mysql.Port, config.Server.Mysql.Database)
	config.Server.Mysql.Url = url
	db, err = gorm.Open(mysql.Open(url), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "1049") {
			url2 := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local",
				config.Server.Mysql.User, config.Server.Mysql.Password, config.Server.Mysql.Host, config.Server.Mysql.Port)
			db, err = gorm.Open(mysql.Open(url2), &gorm.Config{
				DisableForeignKeyConstraintWhenMigrating: true,
			})
			if err != nil {
				return err
			}
			err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", config.Server.Mysql.Database)).Error
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
		&model.AdminGroup{},
		&model.AdminLoginLog{},
		&model.AdminLog{},
		&model.Attachment{},
		&model.AttachmentCategory{},
		&model.Category{},
		&model.Nav{},
		&model.NavType{},
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
		&model.Setting{},

		&model.User{},
		&model.UserGroup{},
		&model.UserWechat{},
		&model.UserWithdraw{},
		&model.WeappQrcode{},
		&model.Order{},
		&model.OrderDetail{},
		&model.OrderAddress{},
		&model.OrderRefund{},
		&model.Payment{},
		&model.Finance{},
		&model.Commission{},
		&model.WechatMenu{},
		&model.WechatMessage{},
		&model.WechatReplyRule{},
	)

	if err != nil {
		return err
	}

	return nil
}

func InitModelData(db *gorm.DB) {
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
			Fields: []config.CustomField{
				{
					Name:      "价格",
					FieldName: "price",
					Type:      "number",
					Required:  false,
					IsSystem:  true,
				},
				{
					Name:      "库存",
					FieldName: "stock",
					Type:      "number",
					Required:  false,
					IsSystem:  true,
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
			m.Migrate(db, false)
		}
	}
	// 表字段重新检查
	db.Model(&model.Module{}).Find(&modules)
	for _, m := range modules {
		m.Migrate(db, false)
	}
	// 检查导航类别
	navType := model.NavType{Title: "默认导航"}
	navType.Id = 1
	db.Model(&model.NavType{}).FirstOrCreate(&navType)
	// 检查分组
	adminGroup := model.AdminGroup{
		Model:       model.Model{Id: 1},
		Title:       "超级管理员",
		Description: "超级管理员分组",
		Status:      1,
		Setting:     model.GroupSetting{},
	}
	db.Where("`id` = 1").FirstOrCreate(&adminGroup)

	// set default user groups
	userGeroups := []model.UserGroup{
		{

			Title:  "普通用户",
			Level:  0,
			Status: 1,
		},
		{
			Model:  model.Model{Id: 2},
			Title:  "中级用户",
			Level:  1,
			Status: 1,
		},
		{
			Model:  model.Model{Id: 3},
			Title:  "高级用户",
			Level:  2,
			Status: 1,
		},
	}
	// check if groups not exist
	var groupNum int64
	db.Model(&model.UserGroup{}).Count(&groupNum)
	if groupNum == 0 {
		db.CreateInBatches(userGeroups, 10)
	}
}
