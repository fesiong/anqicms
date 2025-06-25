package provider

import (
	"errors"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"log"
	"os"
	"strings"
	"time"
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
				library.DebugLog(config.ExecPath, "error.log", time.Now().Format("2006-01-02 15:04:05"), "连接数据库失败", err.Error())
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
	// 连接池设置
	sqlDB.SetMaxIdleConns(500)
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	err = db.Use(&model.NextArchiveIdPlugin{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func AutoMigrateDB(db *gorm.DB, focus bool) error {
	var lastVersion string
	db.Model(&model.Setting{}).Where("`key` = ?", LastRunVersionKey).Pluck("value", &lastVersion)
	if focus || lastVersion != config.Version {
		// 强制转换archive表的title字段
		forceChangeArchiveTitle(db)

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
			&model.CommentPraise{},
			&model.Anchor{},
			&model.AnchorData{},
			&model.Guestbook{},
			&model.Keyword{},
			&model.Material{},
			&model.MaterialCategory{},
			&model.MaterialData{},
			&model.StatisticLog{},
			&model.Tag{},
			&model.TagData{},
			&model.Redirect{},
			&model.Module{},
			&model.ArchiveData{},
			&model.SpiderInclude{},
			&model.Setting{},
			&model.Website{},
			&model.AiArticlePlan{},
			&model.ArchiveCategory{},
			&model.ArchiveRelation{},
			&model.ArchiveFlag{},
			&model.HtmlPushLog{},
			&model.Archive{},
			&model.ArchiveDraft{},
			&model.TranslateLog{},
			&model.TranslateHtmlLog{},

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
			&model.TagContent{},
		)

		if err != nil {
			log.Println("migrate table error ", err)
			return err
		}
		// 取消使用 MyISAM 引擎
		//engine, _ := getTableEngine(db, "archives")
		//if engine == "MyISAM" {
		//	db.Exec("ALTER TABLE archives ENGINE=InnoDB")
		//}
		//engine, _ = getTableEngine(db, "archive_drafts")
		//if engine == "MyISAM" {
		//	db.Exec("ALTER TABLE archive_drafts ENGINE=InnoDB")
		//}
		// 先删除deleteAt
		if db.Migrator().HasColumn(&model.Archive{}, "deleted_at") {
			db.Unscoped().Where("`deleted_at` is not null").Delete(model.Archive{})
			_ = db.Migrator().DropColumn(&model.Archive{}, "deleted_at")
		}
		// 转换archives的草稿部分数据到archive_drafts
		if db.Migrator().HasColumn(&model.Archive{}, "status") {
			archiveColumns, err1 := getColumns(db, &model.Archive{})
			draftColumns, err2 := getColumns(db, &model.ArchiveDraft{})
			// 取得交集
			var columns []string
			if err1 == nil && err2 == nil {
				seen := make(map[string]bool)
				for _, column := range archiveColumns {
					seen[column] = true
				}
				for _, column := range draftColumns {
					if seen[column] {
						columns = append(columns, column)
					}
				}
			}
			columnString := "`" + strings.Join(columns, "`,`") + "`"
			db.Exec("INSERT INTO `archive_drafts` (?) SELECT ? FROM `archives` WHERE `status` != 1", gorm.Expr(columnString), gorm.Expr(columnString))
			db.Where("`status` != 1").Delete(model.Archive{})
			_ = db.Migrator().DropColumn(&model.Archive{}, "status")
		}
		if db.Migrator().HasColumn(&model.Archive{}, "flag") {
			var tinyArcs []struct {
				Id   int64  `json:"id"`
				Flag string `json:"flag"`
			}
			db.Model(&model.Archive{}).Where("flag IS NOT NULL AND flag != ''").Scan(&tinyArcs)
			for _, tinyArc := range tinyArcs {
				if len(tinyArc.Flag) == 0 {
					continue
				}
				flags := strings.Split(tinyArc.Flag, ",")
				for _, flag := range flags {
					if len(flag) == 0 {
						continue
					}
					arcFlag := model.ArchiveFlag{
						Flag:      flag,
						ArchiveId: tinyArc.Id,
					}
					db.Model(&model.ArchiveFlag{}).Where("`archive_id` = ? AND `flag` = ?", arcFlag.ArchiveId, arcFlag.Flag).FirstOrCreate(&arcFlag)
				}
			}
			// 移除字段
			_ = db.Migrator().DropColumn(&model.Archive{}, "flag")
		}
		// end 升级转换部分

		setting := model.Setting{
			Key:   LastRunVersionKey,
			Value: config.Version,
		}

		return db.Save(&setting).Error
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
			Name:      w.Tr("articleModule"),
			Title:     w.Tr("ArticleCenter"),
			Fields:    nil,
			IsSystem:  1,
			TitleName: w.Tr("Title"),
			Status:    1,
		},
		{
			Model:     model.Model{Id: 2},
			TableName: "product",
			UrlToken:  "product",
			Name:      w.Tr("productModule"),
			Title:     w.Tr("ProductCenter"),
			Fields:    nil,
			IsSystem:  1,
			TitleName: w.Tr("ProductName"),
			Status:    1,
		},
	}
	for _, m := range modules {
		m.Database = w.Mysql.Database
		var dbModule model.Module
		w.DB.Model(&model.Module{}).Where("`id` = ?", m.Id).Take(&dbModule)
		if dbModule.Id == 0 {
			w.DB.Create(&m)
			// 并生成表
			tplPath := fmt.Sprintf("%s/%s", w.GetTemplateDir(), m.TableName)
			m.Migrate(w.DB, tplPath, false)
		} else if dbModule.Name == "" {
			// 修复name
			w.DB.Model(&dbModule).UpdateColumn("name", m.Name)
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
	navType := model.NavType{Title: w.Tr("DefaultNavigation")}
	navType.Id = 1
	w.DB.Model(&model.NavType{}).FirstOrCreate(&navType)
	// 默认管理员
	_ = w.InitAdmin("admin", "123456", false)
	// 检查分组
	adminGroup := model.AdminGroup{
		Model:       model.Model{Id: 1},
		Title:       w.Tr("SuperAdministrator"),
		Description: w.Tr("SuperAdministratorGroup"),
		Status:      1,
		Setting:     model.GroupSetting{},
	}
	w.DB.Where("`id` = 1").FirstOrCreate(&adminGroup)
	// user table
	w.MigrateUserTable(w.PluginUser.Fields, false)
	// set default user groups
	userGroups := []model.UserGroup{
		{

			Title:  w.Tr("OrdinaryUser"),
			Level:  0,
			Status: 1,
		},
		{
			Model:  model.Model{Id: 2},
			Title:  w.Tr("IntermediateUser"),
			Level:  1,
			Status: 1,
		},
		{
			Model:  model.Model{Id: 3},
			Title:  w.Tr("AdvancedUser"),
			Level:  2,
			Status: 1,
		},
	}
	// check if groups not exist
	var groupNum int64
	w.DB.Model(&model.UserGroup{}).Count(&groupNum)
	if groupNum == 0 {
		w.DB.CreateInBatches(userGroups, 10)
	}
	// 升级多分类
	upgradeTime := w.GetSettingValue("upgrade_archive_category")
	if len(upgradeTime) == 0 {
		// 没升级
		go func() {
			defer func() {
				_ = w.SaveSettingValue("upgrade_archive_category", time.Now().Format("2006-01-02 15:04:05"))
			}()
			w.UpgradeMultiCategory()
		}()
	}
}

func getTableEngine(db *gorm.DB, tableName string) (string, error) {
	var tableStatus = make(map[string]interface{})
	db.Raw(fmt.Sprintf("SHOW TABLE STATUS LIKE '%s'", tableName)).Scan(&tableStatus)
	if engine, ok := tableStatus["Engine"].(string); ok {
		return engine, nil
	}
	return "", errors.New("not found engine")
}

func getColumns(db *gorm.DB, dst interface{}) ([]string, error) {
	columnsTypes, err := db.Migrator().ColumnTypes(dst)
	if err != nil {
		return nil, err
	}

	var columns []string
	for _, column := range columnsTypes {
		columns = append(columns, column.Name())
	}

	return columns, nil
}

// forceChangeArchiveTitle
// 强制修改标题，使其符合mysql 5.6的190长度
func forceChangeArchiveTitle(db *gorm.DB) {
	columnsTypes, err := db.Migrator().ColumnTypes(&model.Archive{})
	if err != nil {
		return
	}

	for _, column := range columnsTypes {
		if column.Name() == "title" {
			if colLen, _ := column.Length(); colLen > 190 {
				// 修改title的长度
				db.Exec("UPDATE `archives` SET `title` = LEFT(`title`, 190) WHERE CHAR_LENGTH(`title`) > 190")
			}
			break
		}
	}
}
