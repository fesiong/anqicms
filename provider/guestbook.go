package provider

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

func (w *Website) GetGuestbookList(keyword string, currentPage, pageSize int) ([]*model.Guestbook, int64, error) {
	var guestbooks []*model.Guestbook
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := w.DB.Model(&model.Guestbook{}).Order("id desc")
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

func (w *Website) GetAllGuestbooks() ([]*model.Guestbook, error) {
	var guestbooks []*model.Guestbook
	err := w.DB.Model(&model.Guestbook{}).Order("id desc").Find(&guestbooks).Error
	if err != nil {
		return nil, err
	}

	return guestbooks, nil
}

func (w *Website) GetGuestbookById(id uint) (*model.Guestbook, error) {
	var guestbook model.Guestbook

	err := w.DB.Where("`id` = ?", id).First(&guestbook).Error
	if err != nil {
		return nil, err
	}

	return &guestbook, nil
}

func (w *Website) DeleteGuestbook(guestbook *model.Guestbook) error {
	err := w.DB.Delete(guestbook).Error
	if err != nil {
		return err
	}

	return nil
}

func (w *Website) GetGuestbookFields() []*config.CustomField {
	//这里有默认的设置
	defaultFields := []*config.CustomField{
		{
			Name:      w.Tr("UserName"),
			FieldName: "user_name",
			Type:      "text",
			Required:  true,
			IsSystem:  true,
		},
		{
			Name:      w.Tr("ContactPhoneNumber"),
			FieldName: "contact",
			Type:      "text",
			Required:  false,
			IsSystem:  true,
		},
		{
			Name:      w.Tr("Email"),
			FieldName: "email",
			Type:      "text",
			Required:  false,
			IsSystem:  false,
		},
		{
			Name:      w.Tr("Qq"),
			FieldName: "qq",
			Type:      "text",
			Required:  false,
			IsSystem:  false,
		},
		{
			Name:      w.Tr("Whatsapp"),
			FieldName: "whatsapp",
			Type:      "text",
			Required:  false,
			IsSystem:  false,
		},
		{
			Name:      w.Tr("MessageContent"),
			FieldName: "content",
			Type:      "textarea",
			Required:  false,
			IsSystem:  true,
		},
	}

	exists := false
	for _, v := range w.PluginGuestbook.Fields {
		if v.IsSystem || v.FieldName == "user_name" {
			exists = true
			break
		}
	}
	var fields []*config.CustomField
	if exists {
		fields = w.PluginGuestbook.Fields
	} else {
		fields = append(defaultFields, w.PluginGuestbook.Fields...)
	}

	return fields
}
