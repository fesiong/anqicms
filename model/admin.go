package model

import (
	"errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Admin struct {
	Model
	UserName    string `json:"user_name" gorm:"column:user_name;type:varchar(16) not null;default:'';index:idx_user_name"`
	Password    string `json:"-" gorm:"column:password;type:varchar(128) not null;default:''"`
	Status      uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_status"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11) default 0;autoCreateTime"`
	LoginTime   int64  `json:"login_time" gorm:"column:login_time;type:int(11) default 0;index:idx_login_time"` //用户登录时间
	Token       string `json:"token" gorm:"-"`
}

func (admin *Admin) CheckPassword(password string) bool {
	if password == "" {
		return false
	}

	byteHash := []byte(admin.Password)
	bytePass := []byte(password)
	err := bcrypt.CompareHashAndPassword(byteHash, bytePass)
	if err != nil {
		return false
	}

	return true
}

func (admin *Admin) EncryptPassword(password string) error {
	if password == "" {
		return errors.New("密码为空")
	}
	pass := []byte(password)
	hash, err := bcrypt.GenerateFromPassword(pass, bcrypt.MinCost)
	if err != nil {
		return err
	}

	admin.Password = string(hash)

	return nil
}

func (admin *Admin) Save(db *gorm.DB) error {
	if err := db.Save(admin).Error; err != nil {
		return err
	}

	return nil
}
