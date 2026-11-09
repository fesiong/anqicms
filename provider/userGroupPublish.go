package provider

import (
	"errors"
	"fmt"
	"sort"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// 用户组「可投稿栏目」授权：用户组 ↔ 栏目 的多对多关系。
//
// 三条硬约定（改动前先读）：
//
//  1. 关系存独立的 user_group_categories 表，不塞进 user_groups.setting 的 JSON。
//     栏目被删时要能清理授权，也要能按栏目反查用户组，JSON 列两件都做不到。
//  2. 授权集合为空 = 该用户组不能在前台投稿（fail closed）。产品要求「默认不选中」，
//     所以这里不能有「空即全部」的兜底 —— 那等于默认放行。
//  3. 匹配是严格 ID 相等，父子栏目不继承。父栏目授权不会自动覆盖子栏目，
//     前端「全选」一次性勾满所有节点，前后端对「全选」的理解保持一致。
const userGroupCategoryTable = "user_group_categories"

// PublishCategoryForbiddenError 表示用户组未获授权在某些栏目投稿。
// 用类型而不是裸 errors.New 是为了让 controller 能取回被拒的栏目 ID 去拼提示语。
type PublishCategoryForbiddenError struct {
	CategoryIds []uint
}

func (e *PublishCategoryForbiddenError) Error() string {
	return fmt.Sprintf("user group is not allowed to publish in categories %v", e.CategoryIds)
}

// ensureUserGroupCategoryTable 幂等建表。
//
// 不能只靠 AutoMigrateDB —— 那个只在 last_run_version != Version 时才跑，
// 升级包没改版本号（或本地直跑 go run）时表根本不存在，
// 之后每次读授权都会静默返回空 → 所有人都不能投稿。
func (w *Website) ensureUserGroupCategoryTable() error {
	if w.DB == nil || w.DB.Migrator().HasTable(userGroupCategoryTable) {
		return nil
	}

	return w.DB.Exec("CREATE TABLE IF NOT EXISTS `" + userGroupCategoryTable + "` (" +
		"`id` int(10) unsigned NOT NULL AUTO_INCREMENT," +
		"`group_id` int(10) unsigned NOT NULL DEFAULT 0," +
		"`category_id` int(10) unsigned NOT NULL DEFAULT 0," +
		"PRIMARY KEY (`id`)," +
		"UNIQUE KEY `uk_group_category` (`group_id`,`category_id`)," +
		"KEY `idx_category_id` (`category_id`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error
}

// GetUserGroupPublishCategoryIds 读取某个用户组被授权的投稿栏目 ID（升序）。
//
// 返回空切片而不是 nil：调用方（尤其是 JSON 响应）需要稳定拿到 `[]`。
func (w *Website) GetUserGroupPublishCategoryIds(groupId uint) []uint {
	ids := make([]uint, 0)
	if groupId == 0 || w.DB == nil {
		return ids
	}

	if err := w.DB.Model(&model.UserGroupCategory{}).
		Where("`group_id` = ?", groupId).
		Order("`category_id` asc").
		Pluck("category_id", &ids).Error; err != nil {
		return make([]uint, 0)
	}

	return ids
}

// AttachUserGroupPublishCategories 批量回填 UserGroup.PublishCategoryIds（一次查询）。
//
// 刻意不放进 GetUserGroupInfo：那个函数在 middleware.ParseUserToken 里
// 每个已登录请求都会跑一次，给它加一条查询等于给全站接口加一次 IO。
// 需要在响应里带授权的读接口自己显式调用本函数。
func (w *Website) AttachUserGroupPublishCategories(groups ...*model.UserGroup) {
	if len(groups) == 0 || w.DB == nil {
		return
	}

	groupIds := make([]uint, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		// 先全部置空切片，保证没有授权的用户组也序列化成 [] 而不是 null。
		group.PublishCategoryIds = make([]uint, 0)
		if group.Id > 0 {
			groupIds = append(groupIds, group.Id)
		}
	}
	if len(groupIds) == 0 {
		return
	}

	type groupCategoryRow struct {
		GroupId    uint `gorm:"column:group_id"`
		CategoryId uint `gorm:"column:category_id"`
	}
	var rows []groupCategoryRow
	if err := w.DB.Model(&model.UserGroupCategory{}).
		Select("`group_id`,`category_id`").
		Where("`group_id` IN (?)", groupIds).
		Order("`category_id` asc").
		Find(&rows).Error; err != nil {
		return
	}

	byGroup := make(map[uint][]uint, len(groupIds))
	for _, row := range rows {
		byGroup[row.GroupId] = append(byGroup[row.GroupId], row.CategoryId)
	}
	for _, group := range groups {
		if group == nil || group.Id == 0 {
			continue
		}
		if ids, ok := byGroup[group.Id]; ok {
			group.PublishCategoryIds = ids
		}
	}
}

// replaceUserGroupPublishCategories 全量替换某个用户组的授权（调用方负责事务）。
func (w *Website) replaceUserGroupPublishCategories(tx *gorm.DB, groupId uint, categoryIds []uint) error {
	if groupId == 0 {
		return errors.New("empty user group id")
	}
	if tx == nil {
		tx = w.DB
	}

	// 先删后插：授权列表通常只有几十条，比 diff 简单且不会漏删。
	if err := tx.Where("`group_id` = ?", groupId).Delete(&model.UserGroupCategory{}).Error; err != nil {
		return err
	}

	ids := NormalizePublishCategoryIds(categoryIds)
	if len(ids) == 0 {
		return nil
	}
	rows := make([]*model.UserGroupCategory, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, &model.UserGroupCategory{GroupId: groupId, CategoryId: id})
	}

	return tx.CreateInBatches(rows, 50).Error
}

// DeleteUserGroupPublishCategories 清空某个用户组的授权（删除用户组时调用）。
func (w *Website) DeleteUserGroupPublishCategories(groupId uint) error {
	if groupId == 0 || w.DB == nil {
		return nil
	}

	return w.DB.Where("`group_id` = ?", groupId).Delete(&model.UserGroupCategory{}).Error
}

// validatePublishCategories 校验提交的栏目 ID 都真实存在且是可投稿（archive）栏目。
//
// 不校验的话，前端传一个页面分类或已被删除的栏目 ID 也能存进关系表，
// 表面上「保存成功」，实际这条授权永远不会命中任何一次投稿。
func (w *Website) validatePublishCategories(categoryIds []uint) error {
	for _, id := range categoryIds {
		category := w.GetCategoryFromCache(id)
		if category == nil || category.Type != config.CategoryTypeArchive {
			return errors.New(w.Tr("InvalidPublishCategory"))
		}
	}

	return nil
}

// NormalizePublishCategoryIds 归一化授权列表：去掉 0、去重、升序。
func NormalizePublishCategoryIds(ids []uint) []uint {
	result := make([]uint, 0, len(ids))
	seen := make(map[uint]bool, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })

	return result
}

// DeniedPublishCategories 返回 requested 中未被 allowed 覆盖的栏目 ID（去重、保持原顺序）。
//
// 纯函数，不碰 DB —— 权限判定的正反例都靠它锁住（见 userGroupPublish_test.go）。
func DeniedPublishCategories(allowed, requested []uint) []uint {
	if len(requested) == 0 {
		return nil
	}
	allowedSet := make(map[uint]bool, len(allowed))
	for _, id := range allowed {
		allowedSet[id] = true
	}
	denied := make([]uint, 0, len(requested))
	seen := make(map[uint]bool, len(requested))
	for _, id := range requested {
		if id == 0 || allowedSet[id] || seen[id] {
			continue
		}
		seen[id] = true
		denied = append(denied, id)
	}

	return denied
}

// CollectPublishCategoryIds 取本次请求要落到的全部栏目，语义与 SaveArchive 的多分类归一保持一致：
// 传了 category_ids 以它为准，否则用 category_id。
func CollectPublishCategoryIds(req *request.Archive) []uint {
	if req == nil {
		return nil
	}
	if req.CategoryIds != nil {
		return NormalizePublishCategoryIds(*req.CategoryIds)
	}
	if req.CategoryId > 0 {
		return []uint{req.CategoryId}
	}

	return nil
}

// GetArchiveCategoryIds 文档当前所属的全部栏目。
// 多分类表为空（老数据只写了 archives.category_id）时回落到主分类。
func (w *Website) GetArchiveCategoryIds(archiveId int64) []uint {
	if archiveId <= 0 || w.DB == nil {
		return nil
	}

	var categoryIds []uint
	w.DB.Model(&model.ArchiveCategory{}).
		Where("`archive_id` = ?", archiveId).
		Order("`category_id` asc").
		Pluck("category_id", &categoryIds)
	if len(categoryIds) > 0 {
		return categoryIds
	}

	var mainCategoryId uint
	w.DB.Model(&model.Archive{}).Where("`id` = ?", archiveId).Pluck("category_id", &mainCategoryId)
	if mainCategoryId > 0 {
		return []uint{mainCategoryId}
	}

	return nil
}

// checkUserPublishCategories 判定用户组能否把内容落到 categoryIds。
//
// group 为 nil（用户没有用户组 / 用户组已删除）一律按「无授权」处理：
// 不能因为查不到用户组就退回放行。
func (w *Website) checkUserPublishCategories(group *model.UserGroup, categoryIds []uint) error {
	if len(categoryIds) == 0 {
		// 没指定栏目时交给 SaveArchive 报「请选择一个栏目」，这里不抢它的错误。
		return nil
	}

	var allowed []uint
	if group != nil && group.Id > 0 {
		allowed = w.GetUserGroupPublishCategoryIds(group.Id)
	}
	if denied := DeniedPublishCategories(allowed, categoryIds); len(denied) > 0 {
		return &PublishCategoryForbiddenError{CategoryIds: denied}
	}

	return nil
}

// PublishArchiveAsUser 前台用户投稿的唯一入口：先做栏目授权校验，再落库。
//
// 所有「以普通用户身份投稿」的路径都必须走这里，不要直接调 SaveArchive ——
// 后台 / MCP / 采集器走 SaveArchive（它们本来就该能写任意栏目）。
//
// 更新已有文档时如果请求没带栏目，按库里的现有栏目校验：
// 否则「先投到授权栏目、再原地改内容」就能绕开后来被撤销的授权。
func (w *Website) PublishArchiveAsUser(req *request.Archive, group *model.UserGroup) (*model.Archive, error) {
	targets := CollectPublishCategoryIds(req)
	if len(targets) == 0 && req != nil && req.Id > 0 {
		targets = w.GetArchiveCategoryIds(req.Id)
	}

	if err := w.checkUserPublishCategories(group, targets); err != nil {
		return nil, err
	}

	return w.SaveArchive(req)
}
