package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"hash/crc32"
	"kandaoni.com/anqicms/library"
	"strconv"
	"strings"
	"time"
)

type User struct {
	Model
	ParentId    uint   `json:"parent_id" gorm:"column:parent_id;type:int(10);unsigned;not null;default:0"`
	UserName    string `json:"user_name" gorm:"column:user_name;type:varchar(64) not null;default:''"`
	RealName    string `json:"real_name" gorm:"column:real_name;type:varchar(64) not null;default:''"`
	AvatarURL   string `json:"avatar_url" gorm:"column:avatar_url;type:varchar(255) not null;default:''"`
	Introduce   string `json:"introduce" gorm:"column:introduce;type:varchar(1000) not null;default:''"` // 介绍
	Email       string `json:"email" gorm:"column:email;type:varchar(100) not null;default:'';index:idx_email"`
	Phone       string `json:"phone" gorm:"column:phone;type:varchar(20) not null;default:'';index"`
	GroupId     uint   `json:"group_id" gorm:"column:group_id;type:int(10) unsigned not null;default:0"`
	Password    string `json:"-" gorm:"column:password;type:varchar(255) not null;default:''"`
	Status      int    `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`
	IsRetailer  int    `json:"is_retailer" gorm:"column:is_retailer;type:tinyint(1) not null;default:0"` // 是否是分销员
	Balance     int64  `json:"balance" gorm:"column:balance;type:bigint(20) not null;default:0;comment:'用户余额'"`
	TotalReward int64  `json:"total_reward" gorm:"column:total_reward;type:bigint(20) not null;default:0;comment:''"` // 分销员累计收益
	InviteCode  string `json:"invite_code" gorm:"column:invite_code;type:varchar(100) not null;default:'';index:idx_invite_code"`
	LastLogin   int64  `json:"last_login" gorm:"column:last_login;type:int(11);default:0"`
	ExpireTime  int64  `json:"expire_time" gorm:"column:expire_time;type:int(11);default:0"`

	Extra         map[string]*CustomField `json:"extra" gorm:"-"`
	Token         string                  `json:"token" gorm:"-"`
	Group         *UserGroup              `json:"group" gorm:"-"`
	FullAvatarURL string                  `json:"full_avatar_url" gorm:"-"`
	Link          string                  `json:"link" gorm:"-"`
}

type UserGroup struct {
	Model
	Title          string           `json:"title" gorm:"column:title;type:varchar(32) not null;default:''"`
	Description    string           `json:"description" gorm:"column:description;type:varchar(1000) not null;default:''"`
	Level          int              `json:"level" gorm:"column:level;type:int(10) not null;default:0"` // group level
	Status         int              `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`
	Price          int64            `json:"price" gorm:"column:price;type:bigint(20) not null;default:0"`
	Setting        UserGroupSetting `json:"setting" gorm:"setting;type:text DEFAULT NULL; COMMENT '配置信息'"` //配置
	FavorablePrice int64            `json:"favorable_price" gorm:"-"`
}

type UserGroupSetting struct {
	//setting
	ShareReward  int64 `json:"share_reward"`
	ParentReward int64 `json:"parent_reward"`
	Discount     int64 `json:"discount"`
	ExpireDay    int   `json:"expire_day"`

	ContentNoVerify  bool `json:"content_no_verify"`  // 评论/内容发布是否不需要审核
	ContentNoCaptcha bool `json:"content_no_captcha"` // 评论/内容发布是否不需要验证码
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

func (u *User) AfterCreate(tx *gorm.DB) (err error) {
	if u.InviteCode == "" {
		id := crc32.ChecksumIEEE([]byte(strconv.Itoa(int(u.Id))))
		result := library.DecimalToAny(int64(id), 36)
		u.InviteCode = result
		tx.Save(u)
	}

	return
}

func (u *User) GetThumb(storageUrl string) string {
	u.FullAvatarURL = u.AvatarURL
	//取第一张
	if u.FullAvatarURL != "" {
		if !strings.HasPrefix(u.FullAvatarURL, "http") && !strings.HasPrefix(u.FullAvatarURL, "//") {
			u.FullAvatarURL = storageUrl + "/" + strings.TrimPrefix(u.FullAvatarURL, "/")
		}
	}

	return u.FullAvatarURL
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

func (u *User) LogLogin(tx *gorm.DB) error {
	u.LastLogin = time.Now().Unix()
	tx.Model(u).UpdateColumn("last_login", u.LastLogin)

	return nil
}
