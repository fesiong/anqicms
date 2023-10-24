package provider

import (
	"kandaoni.com/anqicms/model"
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

func (w *Website) DeleteCacheNavs() {
	w.MemCache.Delete("navs")
}

func (w *Website) GetCacheNavs() []model.Nav {
	if w.DB == nil {
		return nil
	}
	var navs []model.Nav

	result := w.MemCache.Get("navs")
	if result != nil {
		var ok bool
		navs, ok = result.([]model.Nav)
		if ok {
			return navs
		}
	}

	w.DB.Where(model.Nav{}).Order("sort asc,id asc").Find(&navs)

	w.MemCache.Set("navs", navs, 0)

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
			navs[i].NavList = nil
			for j := range navs {
				if navs[j].ParentId == navs[i].Id {
					navs[j].Spacer = "└  "
					navs[j].Link = w.GetUrl("nav", &navs[j], 0)
					// 增加三级
					navs[j].NavList = nil
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
				Title:   w.Lang("首页"),
				Status:  1,
				NavType: model.NavTypeSystem,
				PageId:  0,
				Link:    w.GetUrl("index", nil, 0),
			},
		}
	}

	return tmpNavs
}
