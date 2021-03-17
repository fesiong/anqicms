package provider

import (
	"irisweb/config"
	"irisweb/model"
)

func GetGuestbookList(keyword string, currentPage, pageSize int) ([]*model.Guestbook, int64, error) {
	var guestbooks []*model.Guestbook
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := config.DB.Model(&model.Guestbook{}).Order("id desc")
	if keyword != "" {
		//模糊搜索
		builder = builder.Where("(`user_name` like ? OR `contact` like ?)", "%"+keyword+"%", "%"+keyword+"%")
	}

	err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&guestbooks).Error
	if err != nil {
		return nil, 0, err
	}

	return guestbooks, total, nil
}

func GetAllGuestbooks() ([]*model.Guestbook, error) {
	var guestbooks []*model.Guestbook
	err := config.DB.Model(&model.Guestbook{}).Order("id desc").Find(&guestbooks).Error
	if err != nil {
		return nil, err
	}

	return guestbooks, nil
}

func GetGuestbookById(id uint) (*model.Guestbook, error) {
	var guestbook model.Guestbook

	err := config.DB.Where("`id` = ?", id).First(&guestbook).Error
	if err != nil {
		return nil, err
	}

	return &guestbook, nil
}

func DeleteGuestbook(guestbook *model.Guestbook) error {
	err := config.DB.Delete(guestbook).Error
	if err != nil {
		return err
	}

	return nil
}
