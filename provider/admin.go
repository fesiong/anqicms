package provider

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jinzhu/now"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"strings"
)

func InitAdmin(userName string, password string, force bool) error {
	if userName == "" || password == "" {
		return errors.New("请提供用户名和密码")
	}

	var exists model.Admin
	db := dao.DB
	err := db.Model(&model.Admin{}).Take(&exists).Error
	if err == nil && !force {
		if exists.GroupId == 0 {
			exists.GroupId = 1
			db.Model(&exists).UpdateColumn("group_id", exists.GroupId)
		}
		return errors.New("已有管理员不能再创建")
	}

	admin := &model.Admin{
		UserName: userName,
		Status:   1,
		GroupId:  1,
	}
	admin.Id = 1
	admin.EncryptPassword(password)
	err = admin.Save(db)
	if err != nil {
		return err
	}

	return nil
}

func GetAdminList(ops func(tx *gorm.DB) *gorm.DB, page, pageSize int) ([]*model.Admin, int64) {
	var admins []*model.Admin
	var total int64
	offset := (page - 1) * pageSize
	tx := dao.DB.Model(&model.Admin{})
	if ops != nil {
		tx = ops(tx)
	} else {
		tx = tx.Order("id desc")
	}
	tx.Count(&total).Limit(pageSize).Offset(offset).Find(&admins)
	if len(admins) > 0 {
		groups := GetAdminGroups()
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

func GetAdminGroups() []*model.AdminGroup {
	var groups []*model.AdminGroup

	dao.DB.Order("id asc").Find(&groups)

	return groups
}

func GetAdminGroupInfo(groupId uint) (*model.AdminGroup, error) {
	var group model.AdminGroup

	err := dao.DB.Where("`id` = ?", groupId).Take(&group).Error

	if err != nil {
		return nil, err
	}

	return &group, nil
}

func SaveAdminGroupInfo(req *request.GroupRequest) error {
	var group = model.AdminGroup{
		Title:       req.Title,
		Description: req.Description,
		Status:      1,
		Setting:     req.Setting,
	}
	if req.Id > 0 {
		_, err := GetAdminGroupInfo(req.Id)
		if err != nil {
			// 不存在
			return err
		}
		group.Id = req.Id
	}
	err := dao.DB.Save(&group).Error

	return err
}

func DeleteAdminGroup(groupId uint) error {
	var group model.AdminGroup
	err := dao.DB.Where("`id` = ?", groupId).Take(&group).Error

	if err != nil {
		return err
	}

	err = dao.DB.Delete(&group).Error

	return err
}

func DeleteAdminInfo(adminId uint) error {
	var admin model.Admin
	err := dao.DB.Where("`id` = ?", adminId).Take(&admin).Error

	if err != nil {
		return err
	}

	err = dao.DB.Delete(&admin).Error

	return err
}

func GetAdminByUserName(userName string) (*model.Admin, error) {
	var admin model.Admin
	db := dao.DB
	err := db.Where("`user_name` = ?", userName).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func GetAdminInfoById(id uint) (*model.Admin, error) {
	var admin model.Admin
	db := dao.DB
	err := db.Where("`id` = ?", id).First(&admin).Error
	if err != nil {
		return nil, err
	}
	admin.Group, _ = GetAdminGroupInfo(admin.GroupId)
	return &admin, nil
}

func GetAdminInfoByName(name string) (*model.Admin, error) {
	var admin model.Admin
	err := dao.DB.Where("name = ?", name).First(&admin).Error
	if err != nil {
		return nil, err
	}

	return &admin, nil
}

func GetAdminAuthToken(userId uint, remember bool) string {
	t := now.BeginningOfDay().AddDate(0, 0, 1)
	// 记住会记住30天
	if remember {
		t = t.AddDate(0, 0, 29)
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"adminId": fmt.Sprintf("%d", userId),
		"t":       fmt.Sprintf("%d", t.Unix()),
	})
	// 获取签名字符串
	tokenString, err := jwtToken.SignedString([]byte(config.Server.Server.TokenSecret))
	if err != nil {
		return ""
	}

	return tokenString
}

func UpdateAdminInfo(adminId uint, req request.AdminInfoRequest) (*model.Admin, error) {
	admin, err := GetAdminInfoById(adminId)
	if err != nil {
		return nil, err
	}
	//开始验证
	req.UserName = strings.TrimSpace(req.UserName)
	req.Password = strings.TrimSpace(req.Password)

	var exists *model.Admin

	if req.UserName != "" {
		exists, err = GetAdminInfoByName(req.UserName)
		if err == nil && exists.Id != admin.Id {
			return nil, errors.New("用户名已被占用，请更换一个")
		}
		admin.UserName = req.UserName
	}

	if req.Password != "" {
		if len(req.Password) < 6 {
			return nil, errors.New("请输入6位及以上长度的密码")
		}
		err = admin.EncryptPassword(req.Password)
		if err != nil {
			return nil, errors.New("密码设置失败")
		}
	}
	err = dao.DB.Save(admin).Error
	if err != nil {
		return nil, errors.New("用户信息更新失败")
	}

	return admin, nil
}

func AddAdminLog(ctx iris.Context, logData string) {
	adminLog := model.AdminLog{
		Log: logData,
	}
	if ctx != nil {
		adminLog.AdminId = ctx.Values().GetUintDefault("adminId", 0)
		admin, err := GetAdminInfoById(adminLog.AdminId)
		if err == nil {
			adminLog.UserName = admin.UserName
		}
		adminLog.Ip = ctx.RemoteAddr()
	}

	dao.DB.Create(&adminLog)
}
