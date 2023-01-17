package provider

import (
	"errors"
	"regexp"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

func GetModules() ([]model.Module, error) {
	var modules []model.Module
	err := dao.DB.Order("id asc").Find(&modules).Error
	if err != nil {
		return nil, err
	}
	return modules, nil
}

func GetModuleById(id uint) (*model.Module, error) {
	var module model.Module
	db := dao.DB
	err := db.Where("`id` = ?", id).First(&module).Error
	if err != nil {
		return nil, err
	}

	return &module, nil
}

func GetModuleByTableName(tableName string) (*model.Module, error) {
	var module model.Module
	db := dao.DB
	err := db.Where("`table_name` = ?", tableName).First(&module).Error
	if err != nil {
		return nil, err
	}

	return &module, nil
}

func GetModuleByUrlToken(urlToken string) (*model.Module, error) {
	var module model.Module
	db := dao.DB
	err := db.Where("`url_token` = ?", urlToken).First(&module).Error
	if err != nil {
		return nil, err
	}

	return &module, nil
}

func SaveModule(req *request.ModuleRequest) (module *model.Module, err error) {
	if req.Id > 0 {
		module, err = GetModuleById(req.Id)
		if err != nil {
			return nil, err
		}
	} else {
		module = &model.Module{
			Status: 1,
		}
	}
	// 检查tableName
	exists, err := GetModuleByTableName(req.TableName)
	if err == nil && exists.Id != req.Id {
		return nil, errors.New(config.Lang("模型表名已存在，请更换一个"))
	}

	// 检查tableName
	exists, err = GetModuleByUrlToken(req.UrlToken)
	if err == nil && exists.Id != req.Id {
		return nil, errors.New(config.Lang("模型URL别名已存在，请更换一个"))
	}

	oldTableName := module.TableName
	module.TableName = req.TableName

	if oldTableName != module.TableName {
		// 表示是新表
		if dao.DB.Migrator().HasTable(module.TableName) {
			return nil, errors.New(config.Lang("模型表名已存在，请更换一个"))
		}
	}
	// 检查fields
	for i := range req.Fields {
		match, err := regexp.MatchString(`^[a-z][0-9a-z_]+$`, req.Fields[i].FieldName)
		if err != nil || !match {
			return nil, errors.New(req.Fields[i].FieldName + config.Lang("命名不正确"))
		}
	}

	module.Fields = req.Fields
	module.Title = req.Title
	module.Fields = req.Fields
	module.TitleName = req.TitleName
	module.UrlToken = req.UrlToken
	module.Status = req.Status

	err = dao.DB.Save(module).Error
	if err != nil {
		return
	}
	// sync table
	if oldTableName != "" && oldTableName != module.TableName {
		if dao.DB.Migrator().HasTable(oldTableName) {
			dao.DB.Migrator().RenameTable(oldTableName, module.TableName)
		}
	}

	module.Migrate(dao.DB, true)

	DeleteCacheModules()

	return
}

func DeleteModuleField(moduleId uint, fieldName string) error {
	module, err := GetModuleById(moduleId)
	if err != nil {
		return err
	}

	if !dao.DB.Migrator().HasTable(module.TableName) {
		return nil
	}

	for i, val := range module.Fields {
		if val.FieldName == fieldName {
			if module.HasColumn(dao.DB, val.FieldName) {
				dao.DB.Exec("ALTER TABLE ? DROP COLUMN ?", gorm.Expr(module.TableName), clause.Column{Name: val.FieldName})
			}

			module.Fields = append(module.Fields[:i], module.Fields[i+1:]...)
			break
		}
	}
	// 回写
	err = dao.DB.Save(module).Error
	return err
}

func DeleteModule(module *model.Module) error {
	// 删除该模型的所有内容
	// 删除 archive data
	var ids []uint
	for {
		dao.DB.Model(&model.Archive{}).Unscoped().Where("module_id = ?", module.Id).Limit(1000).Pluck("id", &ids)
		if len(ids) == 0 {
			break
		}
		dao.DB.Unscoped().Where("id IN(?)", ids).Delete(model.ArchiveData{})
		dao.DB.Unscoped().Where("id IN(?)", ids).Delete(model.Archive{})
	}
	// 删除模型表
	if dao.DB.Migrator().HasTable(module.TableName) {
		dao.DB.Migrator().DropTable(module.TableName)
	}
	// 删除 module
	dao.DB.Delete(module)

	return nil
}

func DeleteCacheModules() {
	library.MemCache.Delete("modules")
}

func GetCacheModules() []model.Module {
	if dao.DB == nil {
		return nil
	}
	var modules []model.Module

	result := library.MemCache.Get("modules")
	if result != nil {
		var ok bool
		modules, ok = result.([]model.Module)
		if ok {
			return modules
		}
	}

	dao.DB.Where(model.Module{}).Where("`status` = ?", config.ContentStatusOK).Find(&modules)

	library.MemCache.Set("modules", modules, 0)

	return modules
}

func GetModuleFromCache(moduleId uint) *model.Module {
	modules := GetCacheModules()
	for i := range modules {
		if modules[i].Id == moduleId {
			return &modules[i]
		}
	}

	return nil
}

func GetModuleFromCacheByToken(urlToken string) *model.Module {
	modules := GetCacheModules()
	for i := range modules {
		if modules[i].UrlToken == urlToken {
			return &modules[i]
		}
	}

	return nil
}
