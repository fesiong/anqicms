package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

type Admin struct {
	Model
	UserName  string      `json:"user_name" gorm:"column:user_name;type:varchar(32) not null;default:'';index:idx_user_name"`
	Password  string      `json:"-" gorm:"column:password;type:varchar(128) not null;default:''"`
	Status    uint        `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`
	LoginTime int64       `json:"login_time" gorm:"column:login_time;type:int(11);default:0"` //用户登录时间
	GroupId   uint        `json:"group_id" gorm:"column:group_id;type:int(10) unsigned not null;default:0"`
	Token     string      `json:"token" gorm:"-"`
	Group     *AdminGroup `json:"group" gorm:"-"`
	SiteId    uint        `json:"site_id" gorm:"-"`
	IsSuper   bool        `json:"is_super" gorm:"-"` // 是否是主站点的管理员
}

type AdminGroup struct {
	Model
	Title       string       `json:"title" gorm:"column:title;type:varchar(32) not null;default:''"`
	Description string       `json:"description" gorm:"column:description;type:varchar(1000) not null;default:''"`
	Status      int          `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`
	Setting     GroupSetting `json:"setting" gorm:"setting;type:text DEFAULT NULL; COMMENT '配置信息'"` //配置
}

type GroupSetting struct {
	// 权限控制部分
	Permissions []string `json:"permissions"`
}

// Value implements the driver.Valuer interface.
func (s GroupSetting) Value() (driver.Value, error) {
	return json.Marshal(s)
}

// Scan implements the sql.Scanner interface.
func (s *GroupSetting) Scan(src interface{}) error {
	switch src := src.(type) {
	case []byte:
		return json.Unmarshal(src, &s)
	case string:
		return json.Unmarshal([]byte(src), &s)
	case nil:
		*s = GroupSetting{}
		return nil
	}

	return fmt.Errorf("pq: cannot convert %T", src)
}

type AdminLoginLog struct {
	Model
	AdminId  uint   `json:"admin_id" gorm:"column:admin_id;type:int(10) unsigned not null;default:0;index:idx_admin_id"`
	Ip       string `json:"ip" gorm:"column:ip;type:varchar(32) not null;default:''"`
	Status   uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`
	UserName string `json:"user_name" gorm:"column:user_name;type:varchar(32) not null;default:''"`
	Password string `json:"password" gorm:"column:password;type:varchar(128) not null;default:''"`
}

type AdminLog struct {
	Model
	AdminId  uint   `json:"admin_id" gorm:"column:admin_id;type:int(10) unsigned not null;default:0;index:idx_admin_id"`
	Ip       string `json:"ip" gorm:"column:ip;type:varchar(32) not null;default:''"`
	Log      string `json:"log" gorm:"column:log;type:varchar(250) not null;default:''"`
	UserName string `json:"user_name" gorm:"column:user_name;type:varchar(32) not null;default:''"`
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
