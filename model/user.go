package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Model
	ParentId    uint       `json:"parent_id" gorm:"column:parent_id;type:int(10);unsigned;not null;default:0"`
	UserName    string     `json:"user_name" gorm:"column:user_name;type:varchar(64) not null;default:''"`
	RealName    string     `json:"real_name" gorm:"column:real_name;type:varchar(64) not null;default:''"`
	AvatarURL   string     `json:"avatar_url" gorm:"column:avatar_url;type:varchar(255) not null;default:''"`
	Email       string     `json:"email" gorm:"column:email;type:varchar(100) not null;default:'';index:idx_email"`
	Phone       string     `json:"phone" gorm:"column:phone;type:varchar(20) not null;default:'';index"`
	GroupId     uint       `json:"group_id" gorm:"column:group_id;type:int(10) unsigned not null;default:0"`
	Password    string     `json:"-" gorm:"column:password;type:varchar(255) not null;default:''"`
	Status      int        `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`
	IsRetailer  int        `json:"is_retailer" gorm:"column:is_retailer;type:tinyint(1) not null;default:0"` // 是否是分销员
	Balance     int64      `json:"balance" gorm:"column:balance;type:bigint(20) not null;default:0;comment:'用户余额'"`
	TotalReward int64      `json:"total_reward" gorm:"column:total_reward;type:bigint(20) not null;default:0;comment:''"` // 分销员累计收益
	Token       string     `json:"token" gorm:"-"`
	Group       *UserGroup `json:"group" gorm:"-"`
}

type UserGroup struct {
	Model
	Title       string           `json:"title" gorm:"column:title;type:varchar(32) not null;default:''"`
	Description string           `json:"description" gorm:"column:description;type:varchar(250) not null;default:''"`
	Level       int              `json:"level" gorm:"column:level;type:int(10) not null;default:0"` // group level
	Status      int              `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`
	Setting     UserGroupSetting `json:"setting" gorm:"setting;type:text DEFAULT NULL; COMMENT '配置信息'"` //配置
}

type UserGroupSetting struct {
	//setting
	ParentReward string `json:"parent_reward"`
	SelfReward   string `json:"self_reward"`
}

// Value implements the driver.Valuer interface.
func (s UserGroupSetting) Value() (driver.Value, error) {
	return json.Marshal(s)
}

// Scan implements the sql.Scanner interface.
func (s *UserGroupSetting) Scan(src interface{}) error {
	switch src := src.(type) {
	case []byte:
		return json.Unmarshal(src, &s)
	case string:
		return json.Unmarshal([]byte(src), &s)
	case nil:
		*s = UserGroupSetting{}
		return nil
	}

	return fmt.Errorf("pq: cannot convert %T", src)
}

func (u *User) CheckPassword(password string) bool {
	if password == "" {
		return false
	}

	byteHash := []byte(u.Password)
	bytePass := []byte(password)
	err := bcrypt.CompareHashAndPassword(byteHash, bytePass)
	if err != nil {
		return false
	}

	return true
}

func (u *User) EncryptPassword(password string) error {
	if password == "" {
		return errors.New("密码为空")
	}
	pass := []byte(password)
	hash, err := bcrypt.GenerateFromPassword(pass, bcrypt.MinCost)
	if err != nil {
		return err
	}

	u.Password = string(hash)

	return nil
}
