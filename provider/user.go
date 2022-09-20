package provider

import (
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/medivhzhan/weapp/v3"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"log"
	"regexp"
	"strings"
	"time"
)

func GetUserList(ops func(tx *gorm.DB) *gorm.DB, page, pageSize int) ([]*model.User, int64) {
	var users []*model.User
	var total int64
	offset := (page - 1) * pageSize
	tx := dao.DB.Model(&model.User{})
	if ops != nil {
		tx = ops(tx)
	} else {
		tx = tx.Order("id desc")
	}
	tx.Count(&total).Limit(pageSize).Offset(offset).Find(&users)
	if len(users) > 0 {
		groups := GetUserGroups()
		for i := range users {
			for g := range groups {
				if users[i].GroupId == groups[g].Id {
					users[i].Group = groups[g]
				}
			}
		}
	}

	return users, total
}

func GetUserInfoById(userId uint) (*model.User, error) {
	var user model.User
	err := dao.DB.Where("`id` = ?", userId).Take(&user).Error

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func GetUsersInfoByIds(userIds []uint) []*model.User {
	var users []*model.User
	if len(userIds) == 0 {
		return users
	}
	dao.DB.Where("`id` IN(?)", userIds).Find(&users)

	return users
}

func SaveUserInfo(req *request.UserRequest) error {
	var user = model.User{
		UserName:  req.UserName,
		AvatarURL: req.AvatarURL,
		Phone:     req.Phone,
		GroupId:   req.GroupId,
		Status:    1,
		Balance:   0,
	}
	if req.Id > 0 {
		_, err := GetUserInfoById(req.Id)
		if err != nil {
			// 用户不存在
			return err
		}
		user.Id = req.Id
	}
	err := dao.DB.Save(&user).Error

	return err
}

func DeleteUserInfo(userId uint) error {
	var user model.User
	err := dao.DB.Where("`id` = ?", userId).Take(&user).Error

	if err != nil {
		return err
	}

	err = dao.DB.Delete(&user).Error

	return err
}

func GetUserGroups() []*model.UserGroup {
	var groups []*model.UserGroup

	dao.DB.Order("id asc").Find(&groups)

	return groups
}

func GetUserGroupInfo(groupId uint) (*model.UserGroup, error) {
	var group model.UserGroup

	err := dao.DB.Where("`id` = ?", groupId).Take(&group).Error

	if err != nil {
		return nil, err
	}

	return &group, nil
}

func SaveUserGroupInfo(req *request.UserGroupRequest) error {
	var group = model.UserGroup{
		Title:       req.Title,
		Description: req.Description,
		Level:       req.Level,
		Status:      1,
		Setting:     req.Setting,
	}
	if req.Id > 0 {
		_, err := GetUserGroupInfo(req.Id)
		if err != nil {
			// 不存在
			return err
		}
		group.Id = req.Id
	}
	err := dao.DB.Save(&group).Error

	return err
}

func GetUserWeixinByOpenid(openid string) (*model.UserWeixin, error) {
	var userWeixin model.UserWeixin
	if err := dao.DB.Where("`openid` = ?", openid).First(&userWeixin).Error; err != nil {
		return nil, err
	}

	return &userWeixin, nil
}

func GetUserWeixinByUserId(userId uint) (*model.UserWeixin, error) {
	var userWeixin model.UserWeixin
	if err := dao.DB.Where("`user_id` = ?", userId).First(&userWeixin).Error; err != nil {
		return nil, err
	}

	return &userWeixin, nil
}

func DeleteUserGroup(groupId uint) error {
	var group model.UserGroup
	err := dao.DB.Where("`id` = ?", groupId).Take(&group).Error

	if err != nil {
		return err
	}

	err = dao.DB.Delete(&group).Error

	return err
}

func LoginByWeixin(req *request.ApiLoginRequest) (*model.User, error) {
	loginRs, err := GetWeappClient(false).Login(req.Code)
	if err != nil {
		return nil, err
	}
	log.Printf("%#v", loginRs)
	if loginRs.OpenID == "" {
		//openid 不在？
		return nil, errors.New("无法获取openid")
	}

	var weixinUserInfo *weapp.UserInfo
	weixinUserInfo, err = GetWeappClient(false).DecryptUserInfo(loginRs.SessionKey, req.RawData, req.EncryptedData, req.Signature, req.Iv)
	if err != nil {
		weixinUserInfo = &weapp.UserInfo{
			Avatar:   req.Avatar,
			Gender:   int(req.Gender),
			Country:  req.County,
			City:     req.City,
			Language: "",
			Nickname: req.NickName,
			Province: req.Province,
		}
	}
	// 拿到openid
	userWeixin, userErr := GetUserWeixinByOpenid(loginRs.OpenID)
	var user *model.User
	if userErr != nil {
		//系统没记录，则插入一条记录
		user = &model.User{
			UserName:  weixinUserInfo.Nickname,
			AvatarURL: weixinUserInfo.Avatar,
			ParentId:  req.InviteId,
			GroupId:   0,
			Status:    1,
		}

		err = dao.DB.Save(user).Error
		if err != nil {
			return nil, err
		}

		userWeixin = &model.UserWeixin{
			UserId:    user.Id,
			Nickname:  weixinUserInfo.Nickname,
			AvatarURL: weixinUserInfo.Avatar,
			Gender:    weixinUserInfo.Gender,
			Openid:    loginRs.OpenID,
			UnionId:   loginRs.UnionID,
			Status:    1,
		}

		err = dao.DB.Save(userWeixin).Error
		if err != nil {
			//删掉
			dao.DB.Delete(user)
			return nil, err
		}

		go DownloadAvatar(userWeixin.AvatarURL, user)
	} else {
		user, err = GetUserInfoById(userWeixin.UserId)
		if err != nil {
			return nil, err
		}
		//更新信息
		if weixinUserInfo.Nickname != "" && (userWeixin.Nickname != weixinUserInfo.Nickname || userWeixin.AvatarURL != weixinUserInfo.Avatar) {
			user.UserName = weixinUserInfo.Nickname
			user.AvatarURL = weixinUserInfo.Avatar
			err = dao.DB.Save(user).Error
			if err != nil {
				return nil, err
			}

			userWeixin.Nickname = weixinUserInfo.Nickname
			userWeixin.AvatarURL = weixinUserInfo.Avatar
			err = dao.DB.Save(userWeixin).Error
			if err != nil {
				return nil, err
			}
		}
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": fmt.Sprintf("%d", user.Id),
		"t":      fmt.Sprintf("%d", time.Now().AddDate(0, 0, 30).Unix()),
	})
	// 获取签名字符串
	tokenString, err := jwtToken.SignedString([]byte(config.JsonData.Server.TokenSecret))
	if err != nil {
		return nil, err
	}
	user.Token = tokenString

	return user, nil
}

func LoginByPassword(req *request.ApiLoginRequest) (*model.User, error) {
	var user model.User
	if VerifyEmailFormat(req.UserName) {
		//邮箱登录
		err := dao.DB.Where("email = ?", req.UserName).First(&user).Error
		if err != nil {
			return nil, err
		}
	} else if VerifyCellphoneFormat(req.UserName) {
		//手机号登录
		err := dao.DB.Where("phone = ?", req.UserName).First(&user).Error
		if err != nil {
			return nil, err
		}
	} else {
		//用户名登录
		err := dao.DB.Where("user_name = ?", req.UserName).First(&user).Error
		if err != nil {
			return nil, err
		}
	}
	//验证密码
	ok := user.CheckPassword(req.Password)
	if !ok {
		return nil, errors.New("密码错误")
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": fmt.Sprintf("%d", user.Id),
		"t":      fmt.Sprintf("%d", time.Now().AddDate(0, 0, 30).Unix()),
	})
	// 获取签名字符串
	tokenString, err := jwtToken.SignedString([]byte(config.JsonData.Server.TokenSecret))
	if err != nil {
		return nil, err
	}
	user.Token = tokenString

	return &user, nil
}

func VerifyEmailFormat(email string) bool {
	pattern := `\w+([-+.]\w+)*@\w+([-.]\w+)*\.\w+([-.]\w+)*` //匹配电子邮箱
	reg := regexp.MustCompile(pattern)
	return reg.MatchString(email)
}

func VerifyCellphoneFormat(cellphone string) bool {
	pattern := `1[3-9][0-9]{9}` //宽匹配手机号
	reg := regexp.MustCompile(pattern)
	return reg.MatchString(cellphone)
}

func DownloadAvatar(avatarUrl string, userInfo *model.User) {
	if avatarUrl == "" || !strings.HasPrefix(avatarUrl, "http") {
		return
	}

	//生成用户文件
	tmpName := fmt.Sprintf("%010d.jpg", userInfo.Id)
	filePath := fmt.Sprintf("uploads/avatar/%s/%s/%s", tmpName[:3], tmpName[3:6], tmpName[6:])
	attach, err := DownloadRemoteImage(avatarUrl, filePath)
	if err != nil {
		return
	}
	//写入完成，更新数据库
	userInfo.AvatarURL = attach.FileLocation
	dao.DB.Model(userInfo).UpdateColumn("avatar_url", userInfo.AvatarURL)
}

func GetRetailerMembers(retailerId uint, page, pageSize int) ([]*model.User, int64) {
	var users []*model.User
	var total int64
	offset := (page - 1) * pageSize
	tx := dao.DB.Model(&model.User{}).Where("`parent_id` = ?", retailerId)
	tx.Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&users)

	return users, total
}

func UpdateUserRealName(userId uint, realName string) error {
	err := dao.DB.Model(&model.User{}).Where("`id` = ?", userId).UpdateColumn("real_name", realName).Error

	return err
}

func SetRetailerInfo(userId uint, isRetailer int) error {
	err := dao.DB.Model(&model.User{}).Where("`id` = ?", userId).UpdateColumn("is_retailer", isRetailer).Error

	return err
}
