package provider

import (
	"irisweb/config"
	"irisweb/model"
)

func GetNavList(nest bool) ([]*model.Nav, error) {
	var tmpList []*model.Nav
	db := config.DB
	//读取第一层
	if err := db.Where("status = ?", 1).Order("sort asc").Find(&tmpList).Error; err != nil {
		//始终返回index
		return nil, err
	}

	if len(tmpList) == 0 {
		return []*model.Nav{
			{
				Id:     0,
				Title:  "首页",
				Status: 1,
				NavType: model.NavTypeSystem,
				PageId: 0,
				Link: "/",
			},
		}, nil
	}

	var navList []*model.Nav
	for _, v := range tmpList {
		if v.ParentId == 0 {
			v.Link = v.GetLink()
			//先获取顶层的
			//再获取是否有下一层的
			if nest {
				//嵌套，前台使用
				for _, nv := range tmpList {
					if nv.ParentId == v.Id {
						nv.Link = nv.GetLink()
						v.NavList = append(v.NavList, nv)
					}
				}
				navList = append(navList, v)
			} else {
				//平铺，后台使用
				navList = append(navList, v)
				for _, nv := range tmpList {
					if nv.ParentId == v.Id {
						nv.Link = nv.GetLink()
						navList = append(navList, nv)
					}
				}
			}
		}
	}


	return navList, nil
}

func GetNavById(id uint) (*model.Nav, error) {
	var nav model.Nav
	if err := config.DB.Where("id = ?", id).First(&nav).Error; err != nil {
		return nil, err
	}

	return &nav, nil
}