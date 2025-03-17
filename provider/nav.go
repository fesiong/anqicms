package provider

import (
	"errors"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

func (w *Website) GetNavList(typeId uint) ([]*model.Nav, error) {
	var tmpList []*model.Nav
	db := w.DB
	//读取第一层
	if err := db.Where("type_id = ?", typeId).Order("sort asc").Find(&tmpList).Error; err != nil {
		//始终返回index
		return nil, err
	}

	var navList []*model.Nav
	for _, v := range tmpList {
		if v.ParentId == 0 {
			v.Link = w.GetUrl("nav", v, 0)
			//先获取顶层的
			//再获取是否有下一层的
			//平铺，后台使用
			navList = append(navList, v)
			for _, nv := range tmpList {
				if nv.ParentId == v.Id {
					nv.Spacer = "└  "
					nv.Link = w.GetUrl("nav", nv, 0)
					navList = append(navList, nv)
					// 增加三级
					for _, nv3 := range tmpList {
						if nv3.ParentId == nv.Id {
							nv3.Spacer = "└  └  "
							nv3.Link = w.GetUrl("nav", nv3, 0)
							navList = append(navList, nv3)
						}
					}
				}
			}
		}
	}

	return navList, nil
}

func (w *Website) GetNavById(id uint) (*model.Nav, error) {
	var nav model.Nav
	if err := w.DB.Where("`id` = ?", id).First(&nav).Error; err != nil {
		return nil, err
	}

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

func (w *Website) GetCacheNavs() []model.Nav {
	if w.DB == nil {
		return nil
	}
	var navs []model.Nav

	err := w.Cache.Get("navs", &navs)
	if err == nil {
		return navs
	}

	err = w.DB.Where(model.Nav{}).Order("sort asc,id asc").Find(&navs).Error
	if err != nil {
		return nil
	}

	if len(navs) > 0 {
		_ = w.Cache.Set("navs", navs, 0)
	}

	return navs
}

func (w *Website) GetNavsFromCache(typeId uint) []*model.Nav {
	var tmpNavs []*model.Nav
	navs := w.GetCacheNavs()
	for i := range navs {
		if navs[i].ParentId == 0 && navs[i].TypeId == typeId {
			navs[i].Link = w.GetUrl("nav", &navs[i], 0)
			//先获取顶层的
			//再获取是否有下一层的
			//嵌套，前台使用
			navs[i].NavList = make([]*model.Nav, 0)
			for j := range navs {
				if navs[j].ParentId == navs[i].Id {
					navs[j].Spacer = "└  "
					navs[j].Link = w.GetUrl("nav", &navs[j], 0)
					// 增加三级
					navs[j].NavList = make([]*model.Nav, 0)
					for k := range navs {
						if navs[k].ParentId == navs[j].Id {
							navs[k].Spacer = "└  └  "
							navs[k].Link = w.GetUrl("nav", &navs[k], 0)
							navs[j].NavList = append(navs[j].NavList, &navs[k])
						}
					}
					navs[i].NavList = append(navs[i].NavList, &navs[j])
				}
			}
			tmpNavs = append(tmpNavs, &navs[i])
		}
	}

	if len(tmpNavs) == 0 {
		return []*model.Nav{
			{
				Title:   w.Tr("Home"),
				Status:  1,
				NavType: model.NavTypeSystem,
				PageId:  0,
				Link:    w.GetUrl("index", nil, 0),
			},
		}
	}

	return tmpNavs
}
