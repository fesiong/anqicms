package provider

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

func GetNavList(typeId uint) ([]*model.Nav, error) {
	var tmpList []*model.Nav
	db := dao.DB
	//读取第一层
	if err := db.Where("type_id = ?", typeId).Order("sort asc").Find(&tmpList).Error; err != nil {
		//始终返回index
		return nil, err
	}

	var navList []*model.Nav
	for _, v := range tmpList {
		if v.ParentId == 0 {
			v.Link = GetUrl("nav", v, 0)
			//先获取顶层的
			//再获取是否有下一层的
			//平铺，后台使用
			navList = append(navList, v)
			for _, nv := range tmpList {
				if nv.ParentId == v.Id {
					nv.Link = GetUrl("nav", nv, 0)
					navList = append(navList, nv)
				}
			}
		}
	}

	return navList, nil
}

func GetNavById(id uint) (*model.Nav, error) {
	var nav model.Nav
	if err := dao.DB.Where("`id` = ?", id).First(&nav).Error; err != nil {
		return nil, err
	}

	return &nav, nil
}

func GetNavTypeList() ([]*model.NavType, error) {
	var navTypes []*model.NavType
	dao.DB.Order("id asc").Find(&navTypes)

	return navTypes, nil
}

func GetNavTypeById(id uint) (*model.NavType, error) {
	var navType model.NavType
	if err := dao.DB.Where("`id` = ?", id).First(&navType).Error; err != nil {
		return nil, err
	}

	return &navType, nil
}

func GetNavTypeByTitle(title string) (*model.NavType, error) {
	var navType model.NavType
	if err := dao.DB.Where("`title` = ?", title).First(&navType).Error; err != nil {
		return nil, err
	}

	return &navType, nil
}

func DeleteCacheNavs() {
	library.MemCache.Delete("navs")
}

func GetCacheNavs() []model.Nav {
	if dao.DB == nil {
		return nil
	}
	var navs []model.Nav

	result := library.MemCache.Get("navs")
	if result != nil {
		var ok bool
		navs, ok = result.([]model.Nav)
		if ok {
			return navs
		}
	}

	dao.DB.Where(model.Nav{}).Order("sort asc").Find(&navs)

	library.MemCache.Set("navs", navs, 0)

	return navs
}

func GetNavsFromCache(typeId uint) []*model.Nav {
	var tmpNavs []*model.Nav
	navs := GetCacheNavs()
	for i := range navs {
		if navs[i].ParentId == 0 && navs[i].TypeId == typeId {
			navs[i].Link = GetUrl("nav", &navs[i], 0)
			//先获取顶层的
			//再获取是否有下一层的
			//嵌套，前台使用
			navs[i].NavList = nil
			for j := range navs {
				if navs[j].ParentId == navs[i].Id {
					navs[j].Link = GetUrl("nav", &navs[j], 0)
					navs[i].NavList = append(navs[i].NavList, &navs[j])
				}
			}
			tmpNavs = append(tmpNavs, &navs[i])
		}
	}

	if len(tmpNavs) == 0 {
		return []*model.Nav{
			{
				Title:  config.Lang("首页"),
				Status: 1,
				NavType: model.NavTypeSystem,
				PageId: 0,
				Link: GetUrl("index", nil, 0),
			},
		}
	}

	return tmpNavs
}
