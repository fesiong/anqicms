package provider

import (
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"strings"
	"time"
)

func InitAdmin(userName string, password string, force bool) error {
	if userName == "" || password == "" {
		return errors.New("请提供用户名和密码")
	}

	var exists int64
	db := dao.DB
	db.Model(&model.Admin{}).Count(&exists)
	if exists > 0 && !force {
		return errors.New("已有管理员不能再创建")
	}

	admin := &model.Admin{
		UserName: userName,
		Status:   1,
	}
	admin.Id = 1
	admin.EncryptPassword(password)
	err := admin.Save(db)
	if err != nil {
		return err
	}

	return nil
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

func GetAdminById(id uint) (*model.Admin, error) {
	var admin model.Admin
	db := dao.DB
	err := db.Where("`id` = ?", id).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func GetAdminInfoById(id uint) (*model.Admin, error) {
	var authUser model.Admin
	err := dao.DB.Where("id = ?", id).First(&authUser).Error
	if err != nil {
		return nil, err
	}

	return &authUser, nil
}

func GetAdminInfoByName(name string) (*model.Admin, error) {
	var authUser model.Admin
	err := dao.DB.Where("name = ?", name).First(&authUser).Error
	if err != nil {
		return nil, err
	}

	return &authUser, nil
}

func GetAdminAuthToken(userId uint, ip string, remember bool) string {
	t := time.Now().AddDate(0, 0, 1)
	// 记住会记住30天
	if remember {
		t = t.AddDate(0, 0, 29)
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"adminId": fmt.Sprintf("%d", userId),
		"ip":      ip,
		"t":       fmt.Sprintf("%d", t.Unix()),
	})
	// 获取签名字符串
	tokenString, err := jwtToken.SignedString([]byte(config.JsonData.Server.TokenSecret))
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
		adminLog.AdminId = uint(ctx.Values().GetIntDefault("adminId", 0))
		adminLog.Ip = ctx.RemoteAddr()
	}

	dao.DB.Create(&adminLog)
}
