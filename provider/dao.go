package provider

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"os"
	"strings"
)

var defaultDB *gorm.DB

func SetDefaultDB(db *gorm.DB) {
	defaultDB = db
}

func GetDefaultDB() *gorm.DB {
	if defaultDB == nil {
		if config.Server.Mysql.Database != "" {
			db, err := InitDB(&config.Server.Mysql)
			if err != nil {
				fmt.Println("Failed To Connect Database: ", err.Error())
				os.Exit(-1)
			}

			defaultDB = db
		}
	}

	return defaultDB
}

func InitDB(cfg *config.MysqlConfig) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	cfgUrl := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	db, err = gorm.Open(mysql.Open(cfgUrl), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "1049") {
			url2 := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local",
				cfg.User, cfg.Password, cfg.Host, cfg.Port)
			db, err = gorm.Open(mysql.Open(url2), &gorm.Config{
				DisableForeignKeyConstraintWhenMigrating: true,
			})
			if err != nil {
				return nil, err
			}
			err = db.Exec(fmt.Sprintf("CREATE DATABASE %s DEFAULT CHARACTER SET utf8mb4", cfg.Database)).Error
			if err != nil {
				return nil, err
			}
			//重新连接db
			db, err = gorm.Open(mysql.Open(cfgUrl), &gorm.Config{
				DisableForeignKeyConstraintWhenMigrating: true,
			})
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(1000)
	sqlDB.SetMaxOpenConns(10000)
	sqlDB.SetConnMaxLifetime(-1)

	return db, nil
}

func AutoMigrateDB(db *gorm.DB) error {
	//自动迁移数据库
	err := db.Set("gorm:table_options", "DEFAULT CHARSET=utf8mb4").AutoMigrate(
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
		&model.Website{},

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

func (w *Website) InitModelData() {
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
			Fields:    nil,
			IsSystem:  1,
			TitleName: "产品名称",
			Status:    1,
		},
	}
	for _, m := range modules {
		m.Database = w.Mysql.Database
		var exists int64
		w.DB.Model(&model.Module{}).Where("`id` = ?", m.Id).Count(&exists)
		if exists == 0 {
			w.DB.Create(&m)
			// 并生成表
			tplPath := fmt.Sprintf("%s/%s", w.GetTemplateDir(), m.TableName)
			m.Migrate(w.DB, tplPath, false)
		}
	}
	// 表字段重新检查
	w.DB.Model(&model.Module{}).Find(&modules)
	for _, m := range modules {
		m.Database = w.Mysql.Database
		tplPath := fmt.Sprintf("%s/%s", w.GetTemplateDir(), m.TableName)
		m.Migrate(w.DB, tplPath, false)
	}
	// 检查导航类别
	navType := model.NavType{Title: "默认导航"}
	navType.Id = 1
	w.DB.Model(&model.NavType{}).FirstOrCreate(&navType)
	// 检查分组
	adminGroup := model.AdminGroup{
		Model:       model.Model{Id: 1},
		Title:       "超级管理员",
		Description: "超级管理员分组",
		Status:      1,
		Setting:     model.GroupSetting{},
	}
	w.DB.Where("`id` = 1").FirstOrCreate(&adminGroup)

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
	w.DB.Model(&model.UserGroup{}).Count(&groupNum)
	if groupNum == 0 {
		w.DB.CreateInBatches(userGeroups, 10)
	}
}
