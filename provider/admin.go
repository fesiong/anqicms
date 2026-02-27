package provider

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"strings"
	"time"
	"unicode/utf8"
)

func (w *Website) InitAdmin(userName string, password string, force bool) error {
	if userName == "" || password == "" {
		return errors.New(w.Tr("PleaseProvideUsernameAndPassword"))
	}

	var exists model.Admin
	db := w.DB
	err := db.Model(&model.Admin{}).Take(&exists).Error
	if err == nil && !force {
		if exists.GroupId == 0 {
			exists.GroupId = 1
			db.Model(&exists).UpdateColumn("group_id", exists.GroupId)
		}
		return errors.New(w.Tr("ExistingAdministratorsCannotCreateAnymore"))
	}

	admin := &model.Admin{
		UserName: userName,
		Status:   1,
		GroupId:  1,
	}
	admin.Id = 1
	admin.EncryptPassword(password)
	err = w.DB.Save(admin).Error
	if err != nil {
		return err
	}

	return nil
}

func (w *Website) GetAdminList(ops func(tx *gorm.DB) *gorm.DB, page, pageSize int) ([]*model.Admin, int64) {
	var admins []*model.Admin
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.Admin{})
	if ops != nil {
		tx = ops(tx)
	} else {
		tx = tx.Order("id desc")
	}
	tx.Count(&total).Limit(pageSize).Offset(offset).Find(&admins)
	if len(admins) > 0 {
		groups := w.GetAdminGroups()
		for i := range admins {
			for g := range groups {
				if admins[i].GroupId == groups[g].Id {
					admins[i].Group = groups[g]
				}
			}
		}
	}

	return admins, total
}

func (w *Website) GetAdminGroups() []*model.AdminGroup {
	var groups []*model.AdminGroup

	w.DB.Order("id asc").Find(&groups)

	return groups
}

func (w *Website) GetAdminGroupInfo(groupId uint) (*model.AdminGroup, error) {
	var group model.AdminGroup

	err := w.DB.Where("`id` = ?", groupId).Take(&group).Error

	if err != nil {
		return nil, err
	}
	if group.Id == 1 {
		// 1 为超级管理员，不能被修改
		group.Setting.Permissions = nil
	}

	return &group, nil
}

func (w *Website) SaveAdminGroupInfo(req *request.GroupRequest) error {
	var group = model.AdminGroup{
		Title:       req.Title,
		Description: req.Description,
		Status:      1,
		Setting:     req.Setting,
	}
	if req.Id > 0 {
		_, err := w.GetAdminGroupInfo(req.Id)
		if err != nil {
			// 不存在
			return err
		}
		group.Id = req.Id
	}
	err := w.DB.Save(&group).Error

	return err
}

func (w *Website) DeleteAdminGroup(groupId uint) error {
	var group model.AdminGroup
	err := w.DB.Where("`id` = ?", groupId).Take(&group).Error

	if err != nil {
		return err
	}
	// 不能删除超级管理员
	if group.Id == 1 {
		return errors.New("permission denied")
	}

	err = w.DB.Delete(&group).Error

	return err
}

func (w *Website) DeleteAdminInfo(adminId uint) error {
	var admin model.Admin
	err := w.DB.Where("`id` = ?", adminId).Take(&admin).Error

	if err != nil {
		return err
	}
	// 不能删除超级管理员
	if admin.Id == 1 {
		return errors.New("permission denied")
	}

	err = w.DB.Delete(&admin).Error

	return err
}

func (w *Website) GetAdminByUserName(userName string) (*model.Admin, error) {
	var admin model.Admin
	db := w.DB
	err := db.Where("`user_name` = ?", userName).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func (w *Website) GetAdminInfoById(id uint) (*model.Admin, error) {
	var admin model.Admin
	if w.DB == nil {
		return nil, errors.New("database not ready")
	}
	db := w.DB
	err := db.Where("`id` = ?", id).First(&admin).Error
	if err != nil {
		return nil, err
	}
	admin.Group, _ = w.GetAdminGroupInfo(admin.GroupId)
	return &admin, nil
}

func (w *Website) GetAdminInfoByName(name string) (*model.Admin, error) {
	var admin model.Admin
	err := w.DB.Where("name = ?", name).First(&admin).Error
	if err != nil {
		return nil, err
	}

	return &admin, nil
}

func (w *Website) GetAdminAuthToken(userId uint, remember bool) string {
	// 默认24小时
	t := time.Now().Add(24 * time.Hour)
	// 记住会记住30天
	if remember {
		t = t.AddDate(0, 0, 29)
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"adminId": fmt.Sprintf("%d", userId),
		"t":       fmt.Sprintf("%d", t.Unix()),
	})
	// 获取签名字符串
	tokenString, err := jwtToken.SignedString([]byte(w.TokenSecret + "-admin-token"))
	if err != nil {
		return ""
	}

	return tokenString
}

func (w *Website) UpdateAdminInfo(adminId uint, req request.AdminInfoRequest) (*model.Admin, error) {
	admin, err := w.GetAdminInfoById(adminId)
	if err != nil {
		return nil, err
	}
	//开始验证
	req.UserName = strings.TrimSpace(req.UserName)
	req.Password = strings.TrimSpace(req.Password)

	var exists *model.Admin

	if req.UserName != "" {
		exists, err = w.GetAdminInfoByName(req.UserName)
		if err == nil && exists.Id != admin.Id {
			return nil, errors.New(w.Tr("UsernameIsAlreadyInUse"))
		}
		admin.UserName = req.UserName
	}

	if req.Password != "" {
		if len(req.Password) < 6 {
			return nil, errors.New(w.Tr("PleaseEnterAPasswordOf6CharactersOrMore"))
		}
		err = admin.EncryptPassword(req.Password)
		if err != nil {
			return nil, errors.New(w.Tr("PasswordSettingFailed"))
		}
	}
	err = w.DB.Save(admin).Error
	if err != nil {
		return nil, errors.New(w.Tr("UserUpdateFailed"))
	}

	return admin, nil
}

func (w *Website) AddAdminLog(ctx iris.Context, logData string) {
	if utf8.RuneCountInString(logData) > 250 {
		logData = string([]rune(logData)[:250])
	}
	adminLog := model.AdminLog{
		Log: logData,
	}
	if ctx != nil {
		adminLog.AdminId = ctx.Values().GetUintDefault("adminId", 0)
		admin, err := w.GetAdminInfoById(adminLog.AdminId)
		if err == nil {
			adminLog.UserName = admin.UserName
		}
		adminLog.Ip = ctx.RemoteAddr()
	}

	w.DB.Create(&adminLog)
}
