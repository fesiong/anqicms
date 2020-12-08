package model

import (
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"time"
)

type Admin struct {
	Model
	Id          uint `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey;"`
	UserName    string `json:"user_name" gorm:"column:user_name;type:varchar(16) not null;default:'';index:idx_user_name"`
	Password    string `json:"-" gorm:"column:password;type:varchar(128) not null;default:''"`
	Status      uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_status"`
	CreatedTime int64   `json:"created_time" gorm:"column:created_time;type:int(11) not null;default:0;index:idx_created_time"`
	UpdatedTime int64   `json:"updated_time" gorm:"column:updated_time;type:int(11) not null;default:0;index:idx_updated_time"`
	DeletedTime int64   `json:"-" gorm:"column:deleted_time;type:int(11) not null;default:0"`
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

func (admin *Admin) EncryptPassword(password string) string {
	if password == "" {
		return ""
	}
	pass := []byte(password)
	hash, err := bcrypt.GenerateFromPassword(pass, bcrypt.MinCost)
	if err != nil {
		return ""
	}

	return string(hash)
}

func (admin *Admin) Save(db *gorm.DB) error {
	if admin.Id == 0 {
		admin.CreatedTime = time.Now().Unix()
	}
	admin.UpdatedTime = time.Now().Unix()

	if err := db.Save(admin).Error; err != nil {
		return err
	}

	return nil
}