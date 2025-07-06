package provider

import (
	"errors"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"strings"
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
			parentNode.NavList = append(parentNode.NavList, node)
		}
	}

	// 设置链接和缩进标识
	var setLinkAndSpacer func(node *model.Nav, level int)
	setLinkAndSpacer = func(node *model.Nav, level int) {
		// 设置缩进标识
		if level > 0 {
			var spacer strings.Builder
			for i := 0; i < level-1; i++ {
				spacer.WriteString("└  ")
			}
			node.Spacer = spacer.String()
		}

		// 递归处理子节点
		for _, child := range node.NavList {
			setLinkAndSpacer(child, level+1)
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
	if req.Title == "" {
		if req.NavType == model.NavTypeCategory {
			category := w.GetCategoryFromCache(uint(req.PageId))
			if category != nil {
				req.Title = category.Title
			}
		} else if req.NavType == model.NavTypeArchive {
			archive, _ := w.GetArchiveById(req.PageId)
			if archive != nil {
				req.Title = archive.Title
			}
		}
	}
	if req.Title == "" {
		return nil, errors.New(w.Tr("PleaseFillInTheNavigationDisplayName"))
	}

	var nav *model.Nav
	var err error
	if req.Id > 0 {
		nav, err = w.GetNavById(req.Id)
		if err != nil {
			// 表示不存在，则新建一个
			nav = &model.Nav{
				Status: 1,
			}
			nav.Id = req.Id
		}
	} else {
		nav = &model.Nav{
			Status: 1,
		}
	}

	nav.Title = req.Title
	nav.SubTitle = req.SubTitle
	nav.Description = req.Description
	nav.ParentId = req.ParentId
	nav.NavType = req.NavType
	nav.PageId = req.PageId
	nav.TypeId = req.TypeId
	nav.Link = req.Link
	nav.Sort = req.Sort
	nav.Logo = req.Logo
	nav.Status = 1

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
	w.Cache.Delete("navs")
}

func (w *Website) GetCacheNavs() []*model.Nav {
	if w.DB == nil {
		return nil
	}
	var navs []*model.Nav

	err := w.Cache.Get("navs", &navs)
	if err == nil {
		return navs
	}

	err = w.DB.Where(model.Nav{}).Order("sort asc,id asc").Find(&navs).Error
	if err != nil {
		_ = w.Cache.Set("navs", navs, 60)
		return nil
	}

	if len(navs) > 0 {
		for i := range navs {
			navs[i].Link = w.GetUrl("nav", navs[i], 0)
			navs[i].GetThumb(w.PluginStorage.StorageUrl)
		}
		_ = w.Cache.Set("navs", navs, 0)
	}

	return navs
}

func (w *Website) GetNavsFromCache(typeId uint, showType string) []*model.Nav {
	navs := w.GetCacheNavs()
	var tmpNavs = make([]*model.Nav, 0, len(navs))

	for i := range navs {
		if navs[i].TypeId == typeId {
			tmpNavs = append(tmpNavs, navs[i])
		}
	}
	result := buildNavTree(tmpNavs, showType)

	return result
}
