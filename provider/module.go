package provider

import (
	"errors"
	"fmt"
	"regexp"

	"gorm.io/gorm/clause"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

func (w *Website) GetModules() ([]model.Module, error) {
	var modules []model.Module
	err := w.DB.Order("id asc").Find(&modules).Error
	if err != nil {
		return nil, err
	}
	return modules, nil
}

func (w *Website) GetModuleById(id uint) (*model.Module, error) {
	var module model.Module
	db := w.DB
	err := db.Where("`id` = ?", id).First(&module).Error
	if err != nil {
		return nil, err
	}

	return &module, nil
}

func (w *Website) GetModuleByTableName(tableName string) (*model.Module, error) {
	var module model.Module
	db := w.DB
	err := db.Where("`table_name` = ?", tableName).First(&module).Error
	if err != nil {
		return nil, err
	}

	return &module, nil
}

func (w *Website) GetModuleByUrlToken(urlToken string) (*model.Module, error) {
	var module model.Module
	db := w.DB
	err := db.Where("`url_token` = ?", urlToken).First(&module).Error
	if err != nil {
		return nil, err
	}

	return &module, nil
}

func (w *Website) SaveModule(req *request.ModuleRequest) (module *model.Module, err error) {
	if req.Id > 0 {
		module, err = w.GetModuleById(req.Id)
		if err != nil {
			// 表示不存在，则新建一个
			module = &model.Module{
				Status: 1,
			}
			module.Id = req.Id
		}
	} else {
		module = &model.Module{
			Status: 1,
		}
	}
	// 检查tableName
	exists, err := w.GetModuleByTableName(req.TableName)
	if err == nil && exists.Id != req.Id {
		return nil, errors.New(w.Tr("ModelTableNameAlreadyExists"))
	}

	// 检查tableName
	exists, err = w.GetModuleByUrlToken(req.UrlToken)
	if err == nil && exists.Id != req.Id {
		return nil, errors.New(w.Tr("ModelUrlAliasAlreadyExists"))
	}

	oldTableName := module.TableName
	module.TableName = req.TableName

	if oldTableName != module.TableName {
		// 表示是新表
		if w.DB.Migrator().HasTable(module.TableName) {
			return nil, errors.New(w.Tr("ModelTableNameAlreadyExists"))
		}
	}
	// 检查fields
	for i := range req.Fields {
		// 不允许使用已存在的字段
		archiveFields, err := getColumns(w.DB, &model.Archive{})
		if err == nil {
			for _, val := range archiveFields {
				if val == req.Fields[i].FieldName {
					return nil, errors.New(req.Fields[i].FieldName + w.Tr("FieldAlreadyExists"))
				}
			}
		}
		match, err := regexp.MatchString(`^[a-z][0-9a-z_]+$`, req.Fields[i].FieldName)
		if err != nil || !match {
			return nil, errors.New(req.Fields[i].FieldName + w.Tr("IncorrectNaming"))
		}
	}
	// 检查 categoryFields
	// 检查fields
	for i := range req.CategoryFields {
		// 不允许使用已存在的字段
		categoryFields, err := getColumns(w.DB, &model.Category{})
		if err == nil {
			for _, val := range categoryFields {
				if val == req.CategoryFields[i].FieldName {
					return nil, errors.New(req.CategoryFields[i].FieldName + w.Tr("FieldAlreadyExists"))
				}
			}
		}
		match, err := regexp.MatchString(`^[a-z][0-9a-z_]+$`, req.CategoryFields[i].FieldName)
		if err != nil || !match {
			return nil, errors.New(req.CategoryFields[i].FieldName + w.Tr("IncorrectNaming"))
		}
	}

	module.Fields = req.Fields
	module.Title = req.Title
	module.Name = req.Name
	module.Fields = req.Fields
	module.CategoryFields = req.CategoryFields
	module.TitleName = req.TitleName
	module.UrlToken = req.UrlToken
	module.Status = req.Status
	module.Keywords = req.Keywords
	module.Description = req.Description

	err = w.DB.Save(module).Error
	if err != nil {
		return
	}
	// sync table
	if oldTableName != "" && oldTableName != module.TableName {
		if w.DB.Migrator().HasTable(oldTableName) {
			w.DB.Migrator().RenameTable(oldTableName, module.TableName)
		}
	}
	module.Database = w.Mysql.Database
	tplPath := fmt.Sprintf("%s/%s", w.GetTemplateDir(), module.TableName)
	module.Migrate(w.DB, tplPath, true)

	w.DeleteCacheModules()

	return
}

func (w *Website) DeleteModuleField(moduleId uint, fieldName string) error {
	module, err := w.GetModuleById(moduleId)
	if err != nil {
		return err
	}

	if !w.DB.Migrator().HasTable(module.TableName) {
		return nil
	}

	for i, val := range module.Fields {
		if val.FieldName == fieldName {
			if module.HasColumn(w.DB, val.FieldName) {
				w.DB.Exec("ALTER TABLE ? DROP COLUMN ?", clause.Table{Name: module.TableName}, clause.Column{Name: val.FieldName})
			}

			module.Fields = append(module.Fields[:i], module.Fields[i+1:]...)
			break
		}
	}
	// 回写
	err = w.DB.Save(module).Error
	return err
}

func (w *Website) DeleteModule(module *model.Module) error {
	// 删除该模型的所有内容
	// 删除 archive data
	var ids []uint
	for {
		w.DB.Model(&model.Archive{}).Unscoped().Where("module_id = ?", module.Id).Limit(1000).Pluck("id", &ids)
		if len(ids) == 0 {
			break
		}
		w.DB.Unscoped().Where("id IN(?)", ids).Delete(model.ArchiveData{})
		w.DB.Unscoped().Where("id IN(?)", ids).Delete(model.Archive{})
	}
	// 删除模型表
	if w.DB.Migrator().HasTable(module.TableName) {
		w.DB.Migrator().DropTable(module.TableName)
	}
	// 删除 module
	w.DB.Delete(module)

	return nil
}

func (w *Website) DeleteCacheModules() {
	w.Cache.Delete("modules")
}

func (w *Website) GetCacheModules() []model.Module {
	if w.DB == nil {
		return nil
	}
	var modules []model.Module

	err := w.Cache.Get("modules", &modules)
	if err == nil && len(modules) > 0 {
		return modules
	}

	err = w.DB.Model(model.Module{}).Where("`status` = ?", config.ContentStatusOK).Find(&modules).Error
	if err != nil {
		return nil
	}
	if len(modules) > 0 {
		_ = w.Cache.Set("modules", modules, 0)
	}

	return modules
}

func (w *Website) GetModuleFromCache(moduleId uint) *model.Module {
	modules := w.GetCacheModules()
	for i := range modules {
		if modules[i].Id == moduleId {
			return &modules[i]
		}
	}

	return nil
}

func (w *Website) GetModuleFromCacheByToken(urlToken string) *model.Module {
	modules := w.GetCacheModules()
	for i := range modules {
		if modules[i].UrlToken == urlToken {
			return &modules[i]
		}
	}

	return nil
}
