package provider

import (
	"errors"
	"irisweb/config"
	"irisweb/model"
)

func InitAdmin(userName string, password string) error {
	if userName == "" || password == "" {
		return errors.New("请提供用户名和密码")
	}

	var exists int64
	db := config.DB
	db.Model(&model.Admin{}).Count(&exists)
	if exists > 0 {
		return errors.New("已有管理员不能再创建")
	}

	admin := &model.Admin{
		UserName: userName,
		Status:   1,
	}
	admin.Password = admin.EncryptPassword(password)
	err := admin.Save(db)
	if err !=  nil {
		return err
	}

	return nil
}

func GetAdminByUserName(userName string)(*model.Admin, error) {
	var admin model.Admin
	db := config.DB
	err := db.Where("`user_name` = ?", userName).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func GetAdminById(id uint)(*model.Admin, error) {
	var admin model.Admin
	db := config.DB
	err := db.Where("`id` = ?", id).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}