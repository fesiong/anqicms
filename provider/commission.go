package provider

import (
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
)

func GetCommissionList(page, pageSize int) ([]*model.Commission, int64) {
	var commissions []*model.Commission
	var total int64
	offset := (page - 1) * pageSize
	dao.DB.Model(&model.Commission{}).Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&commissions)
	if len(commissions) > 0 {
		var userIds = make([]uint, 0, len(commissions))
		for i := range commissions {
			userIds = append(userIds, commissions[i].UserId)
		}
		users := GetUsersInfoByIds(userIds)
		for i := range commissions {
			for u := range users {
				if commissions[i].UserId == users[u].Id {
					commissions[i].UserName = users[u].UserName
				}
			}
		}
	}
	return commissions, total
}

func GetCommissionById(id uint) (*model.Commission, error) {
	var commission model.Commission
	err := dao.DB.Where(&model.Commission{}).Where("`id` = ?", id).Take(&commission).Error
	if err != nil {
		return nil, err
	}

	return &commission, nil
}

