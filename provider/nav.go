package provider

import (
	"errors"
	"fmt"
	"strings"

	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// GetNavList 获取导航列表，showType = list|children,默认children
func (w *Website) GetNavList(typeId uint, showType string) ([]*model.Nav, error) {
	var tmpList []*model.Nav
	db := w.DB
	if err := db.Where("type_id = ?", typeId).Order("sort asc").Find(&tmpList).Error; err != nil {
		return nil, err
	}
	for i := range tmpList {
		tmpList[i].Link = w.GetUrl("nav", tmpList[i], 0)
		tmpList[i].Thumb = tmpList[i].GetThumb(w.PluginStorage.StorageUrl)
	}

	return buildNavTree(tmpList, showType), nil
}

func buildNavTree(navs []*model.Nav, showType string) []*model.Nav {
	// 构建导航树的根节点
	var rootNodes []*model.Nav
	// 创建一个map用于快速查找节点
	nodeMap := make(map[uint]*model.Nav)

	// 初始化所有节点到nodeMap，并设置基本属性
	for _, node := range navs {
		node.NavList = []*model.Nav{}
		nodeMap[node.Id] = node
	}

	// 第一次遍历：构建树结构
	for _, node := range navs {
		if node.ParentId == 0 {
			rootNodes = append(rootNodes, node)
		} else if parentNode, ok := nodeMap[node.ParentId]; ok {
			node.Level = parentNode.Level + 1
			node.Parents = append(parentNode.Parents, model.ParentCategory{
				Id:    parentNode.Id,
				Title: parentNode.Title,
			})
			parentNode.NavList = append(parentNode.NavList, node)
		}
	}

	// 设置链接和缩进标识
	var setLinkAndSpacer func(node *model.Nav, level int)
	setLinkAndSpacer = func(node *model.Nav, level int) {
		if node == nil || level > 5 {
			return
		}
		// 设置缩进标识
		if level > 0 {
			var spacer strings.Builder
			for i := 0; i < level-1; i++ {
				spacer.WriteString("└－")
			}
			node.Spacer = spacer.String()
		}

		// 递归处理子节点
		if node != nil {
			for _, child := range node.NavList {
				if child != nil {
					setLinkAndSpacer(child, level+1)
				}
			}
		}
	}

	// 第二次遍历：设置链接和缩进标识
	for _, rootNode := range rootNodes {
		setLinkAndSpacer(rootNode, 0)
	}

	var result []*model.Nav
	if showType == "list" {
		// 将树结构扁平化为列表
		var flattenTree func(nodes []*model.Nav)
		flattenTree = func(nodes []*model.Nav) {
			for _, node := range nodes {
				result = append(result, node)
				flattenTree(node.NavList)
			}
		}

		flattenTree(rootNodes)
	} else {
		result = rootNodes
	}

	return result
}

func (w *Website) GetNavById(id uint) (*model.Nav, error) {
	var nav model.Nav
	if err := w.DB.Where("`id` = ?", id).First(&nav).Error; err != nil {
		return nil, err
	}
	nav.GetThumb(w.PluginStorage.StorageUrl)

	return &nav, nil
}

func (w *Website) GetNavTypeList() ([]*model.NavType, error) {
	var navTypes []*model.NavType
	w.DB.Order("id asc").Find(&navTypes)

	return navTypes, nil
}

func (w *Website) GetNavTypeById(id uint) (*model.NavType, error) {
	var navType model.NavType
	if err := w.DB.Where("`id` = ?", id).First(&navType).Error; err != nil {
		return nil, err
	}

	return &navType, nil
}

func (w *Website) GetNavTypeByTitle(title string) (*model.NavType, error) {
	var navType model.NavType
	if err := w.DB.Where("`title` = ?", title).First(&navType).Error; err != nil {
		return nil, err
	}

	return &navType, nil
}

func (w *Website) SaveNav(req *request.NavConfig) (*model.Nav, error) {
	var nav *model.Nav
	var err error
	isNew := false
	if req.Id > 0 {
		nav, err = w.GetNavById(req.Id)
		if err != nil {
			// 表示不存在，则新建一个
			isNew = true
			nav = &model.Nav{
				Status: 1,
			}
			nav.Id = req.Id
		}
	} else {
		isNew = true
		nav = &model.Nav{
			Status: 1,
		}
	}

	// 标题：新建时可以按 nav_type + page_id 推导（前端就是这么用的），
	// 更新时**不传标题 = 不改标题**——原来这里无条件要求标题非空，
	// 于是「把 5 号导航隐藏」（只传 id + status）会被判定成「请填写导航显示名称」，
	// PATCH 语义根本走不通。只有调用方显式改关联对象时才重新推导标题。
	if req.Title == "" && (isNew || req.NavType != nil || req.PageId != nil) {
		navType := library.DerefOr(req.NavType, nav.NavType)
		pageId := library.DerefOr(req.PageId, nav.PageId)
		if navType == model.NavTypeCategory {
			category := w.GetCategoryFromCache(uint(pageId))
			if category != nil {
				req.Title = category.Title
			}
		} else if navType == model.NavTypeArchive {
			archive, _ := w.GetArchiveById(pageId)
			if archive != nil {
				req.Title = archive.Title
			}
		}
	}
	if req.Title == "" && isNew {
		return nil, errors.New(w.Tr("PleaseFillInTheNavigationDisplayName"))
	}

	// 标题必填，空值永远不是合法写入值：UpdateAll 下漏传也不能把已有标题清空。
	if req.Title != "" {
		nav.Title = req.Title
	}
	if req.SubTitle != nil {
		nav.SubTitle = *req.SubTitle
	}
	if req.Description != nil {
		nav.Description = *req.Description
	}
	if req.ParentId != nil {
		nav.ParentId = *req.ParentId
	}
	if req.NavType != nil {
		nav.NavType = *req.NavType
	}
	if req.PageId != nil {
		nav.PageId = *req.PageId
	}
	if req.TypeId != nil {
		nav.TypeId = *req.TypeId
	}
	if req.Link != nil {
		nav.Link = *req.Link
	}
	if req.Sort != nil {
		nav.Sort = *req.Sort
	}
	if req.Logo != nil {
		nav.Logo = *req.Logo
	}
	if req.Style != nil {
		nav.Style = *req.Style
	}
	// 原来这里硬编码 nav.Status = 1，导致导航永远无法隐藏（前端传 status=0 也会被覆盖）。
	if req.Status != nil {
		nav.Status = *req.Status
	}

	err = nav.Save(w.DB)
	if err != nil {
		return nil, err
	}

	return nav, nil
}

func (w *Website) SaveNavType(req *request.NavTypeRequest) (*model.NavType, error) {
	var navType *model.NavType
	var err error
	if req.Id > 0 {
		navType, err = w.GetNavTypeById(req.Id)
		if err != nil {
			// 表示不存在，则新建一个
			navType = &model.NavType{}
			navType.Id = req.Id
		}
	} else {
		// 检查重复标题
		navType, err = w.GetNavTypeByTitle(req.Title)
		if err != nil {
			navType = &model.NavType{}
		}
	}

	navType.Title = req.Title

	err = w.DB.Save(navType).Error
	if err != nil {
		return nil, err
	}

	return navType, nil
}

func (w *Website) DeleteCacheNavs() {
	w.Cache.CleanAll("navs")
}

func (w *Website) GetCacheNavs(typeId uint, showType string) []*model.Nav {
	if w.DB == nil {
		return nil
	}
	var navs []*model.Nav
	cacheKey := fmt.Sprintf("navs-%d-%s", typeId, showType)
	err := w.Cache.Get(cacheKey, &navs)
	if err == nil {
		return navs
	}

	var tmpList []*model.Nav
	db := w.DB
	if err = db.Where("type_id = ?", typeId).Order("sort asc").Find(&tmpList).Error; err != nil {
		return nil
	}
	for i := range tmpList {
		tmpList[i].Link = w.GetUrl("nav", tmpList[i], 0)
		tmpList[i].Thumb = tmpList[i].GetThumb(w.PluginStorage.StorageUrl)
	}

	navs = buildNavTree(tmpList, showType)

	_ = w.Cache.Set(cacheKey, navs, 0)

	return navs
}

func (w *Website) GetNavsFromCache(typeId uint, showType string) []*model.Nav {
	navs := w.GetCacheNavs(typeId, showType)

	return navs
}
