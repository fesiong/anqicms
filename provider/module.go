package provider

import (
	"errors"
	"fmt"
	"regexp"
	"time"

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

// ensureModuleKindColumn 给 modules 表补 kind 列并回填历史数据。
//
// 为什么不能只靠 AutoMigrateDB：它只在 last_run_version != config.Version
// 时执行（升级才跑），新增列在老站库上不会自动出现——而 kind 是每次读模块
// 都要用的字段，缺列会让所有模块查询直接 Unknown column（整站 500）。
// 所以放在 InitModelData 里（每次站点初始化都跑）。
//
// 回填用 setting 标记只做一次：否则每次开机都会把 table_name=product 的模型
// 刷成 product，用户在后台手工改过的类型永远改不回来。
func (w *Website) ensureModuleKindColumn() error {
	if w.DB == nil || !w.DB.Migrator().HasTable(&model.Module{}) {
		return nil
	}
	if !w.DB.Migrator().HasColumn(&model.Module{}, "kind") {
		if err := w.DB.Exec("ALTER TABLE `modules` ADD COLUMN `kind` varchar(20) NOT NULL DEFAULT 'article'").Error; err != nil {
			return err
		}
		w.DeleteCacheModules()
	}
	const flagKey = "upgrade_module_kind"
	if w.GetSettingValue(flagKey) != "" {
		return nil
	}
	// 历史数据：表名是 product 的就是产品模型，其余（含用户自建表名）按文章处理。
	if err := w.DB.Model(&model.Module{}).Where("`table_name` = ?", "product").
		Update("kind", model.ModuleKindProduct).Error; err != nil {
		return err
	}
	_ = w.DB.Model(&model.Module{}).Where("`kind` = ''").
		Update("kind", model.ModuleKindArticle).Error
	_ = w.SaveSettingValue(flagKey, time.Now().Format("2006-01-02 15:04:05"))
	w.DeleteCacheModules()
	return nil
}

// normalizeModuleKind 把库里的值收敛到两个合法枚举。
//
// 空串/未知值一律按 article 处理：旧行为就是「不是 2 就当文章」，
// 这样即使某行的 kind 是脏数据，页面也不至于整块空白。
func normalizeModuleKind(kind string) string {
	if kind == model.ModuleKindProduct {
		return model.ModuleKindProduct
	}
	return model.ModuleKindArticle
}

// GetModuleKind 返回模型类型（article / product）。
//
// 不复用 GetModuleFromCache：那个只返回 status=1 的模型，产品模块一停用
// 这里就退化成 article，商品页被按文章渲染（价格区块与 Product 结构化
// 数据一起消失）——这是旧硬编码（id==2 不看 status）没有的退化路径，
// 不能借改类型判断重新引入。因此用独立的缓存只存 id→kind。
func (w *Website) GetModuleKind(moduleId uint) string {
	if w.DB == nil {
		return model.ModuleKindArticle
	}
	if w.Cache != nil {
		var cached map[uint]string
		if err := w.Cache.Get("module_kinds", &cached); err == nil && cached != nil {
			return normalizeModuleKind(cached[moduleId])
		}
	}
	var modules []model.Module
	if err := w.DB.Model(&model.Module{}).Select("`id`", "`kind`").Find(&modules).Error; err != nil {
		return model.ModuleKindArticle
	}
	kinds := make(map[uint]string, len(modules))
	for _, m := range modules {
		kinds[m.Id] = m.Kind
	}
	if w.Cache != nil {
		_ = w.Cache.Set("module_kinds", kinds, 0)
	}
	return normalizeModuleKind(kinds[moduleId])
}

// IsProductModule 判断给定模型是不是「产品」类型。
//
// 替代散落各处的 `ModuleId == 2`。模块 id 是自增的，删了重建、导入站点后
// 未必还是 2；把 1 号模型改名成「产品」也不会生效。
func (w *Website) IsProductModule(moduleId uint) bool {
	return w.GetModuleKind(moduleId) == model.ModuleKindProduct
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
			// 同 SaveCategory：传 id 即更新，不存在必须报错。
			// 曾"不存在则按 req.Id 新建"，会造出幽灵模型（模型是内容表结构的定义，
			// 多一个空模型会出现在建文档的选项里）。2026-10-27 由全接口测试发现。
			return nil, fmt.Errorf("模型不存在: id=%d", req.Id)
		}
	} else {
		module = &model.Module{
			Status: 1,
		}
	}
	oldTableName := ""
	if req.UpdateAll || req.TableName != "" {
		// 检查tableName
		exists, err := w.GetModuleByTableName(req.TableName)
		if err == nil && exists.Id != req.Id {
			return nil, errors.New(w.Tr("ModelTableNameAlreadyExists"))
		}

		oldTableName = module.TableName
		module.TableName = req.TableName

		if oldTableName != module.TableName {
			// 表示是新表
			if w.DB.Migrator().HasTable(module.TableName) {
				return nil, errors.New(w.Tr("ModelTableNameAlreadyExists"))
			}
		}
	}

	if req.UpdateAll || req.UrlToken != "" {
		// 检查tableName
		exists, err := w.GetModuleByUrlToken(req.UrlToken)
		if err == nil && exists.Id != req.Id {
			return nil, errors.New(w.Tr("ModelUrlAliasAlreadyExists"))
		}
	}

	if req.UpdateAll || req.Fields != nil {
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
		module.Fields = req.Fields
	}
	if req.UpdateAll || req.CategoryFields != nil {
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
		module.CategoryFields = req.CategoryFields
	}

	if req.UpdateAll || req.Title != "" {
		module.Title = req.Title
	}
	if req.UpdateAll || req.Name != "" {
		module.Name = req.Name
	}
	if req.UpdateAll || req.UrlToken != "" {
		module.UrlToken = req.UrlToken
	}
	if req.Status != nil {
		module.Status = *req.Status
	}
	// kind 只接受两个枚举值。传错要报错而不是静默忽略——前端选了「产品」但
	// 后端不认识时，模型会落成文章，商品页静默退化成文章渲染。
	if req.Kind != nil {
		switch *req.Kind {
		case model.ModuleKindArticle, model.ModuleKindProduct:
			module.Kind = *req.Kind
		default:
			return nil, fmt.Errorf("未知的模型类型: %q（只支持 %s / %s）", *req.Kind, model.ModuleKindArticle, model.ModuleKindProduct)
		}
	} else if req.UpdateAll && module.Kind == "" {
		// 新建且没指定类型：默认文章。GORM 的 Save 写全列，DB 的 DEFAULT 不生效，
		// 漏了这一步库里就是空串（读的时候虽会归一成 article，但脏数据会传染给导出/外部读库）。
		module.Kind = model.ModuleKindArticle
	}
	// 原来这里判的是 req.TitleName（已废弃字段），导致「改关键词」永远不生效：
	// 只有传了 title_name 才会把 keywords 写进去。改用 keywords 自己的指针判断。
	if req.Keywords != nil {
		module.Keywords = *req.Keywords
	}
	if req.Description != nil {
		module.Description = *req.Description
	}

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
	tplPath := w.GetTemplateDir() + "/" + module.TableName
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
	if w.Cache == nil {
		return
	}
	w.Cache.Delete("modules")
	// id→kind 是独立缓存（见 GetModuleKind：不能复用 modules 列表，
	// 那份只含 status=1 的模型）。改模型/改类型时两张都要失效。
	w.Cache.Delete("module_kinds")
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
