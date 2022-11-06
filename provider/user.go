package provider

import (
	"errors"
	"fmt"
	"github.com/medivhzhan/weapp/v3"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
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

func GetUserInfoByUserName(userName string) (*model.User, error) {
	var user model.User
	err := dao.DB.Where("`user_name` = ?", userName).Take(&user).Error

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func GetUserInfoByEmail(email string) (*model.User, error) {
	var user model.User
	err := dao.DB.Where("`email` = ?", email).Take(&user).Error

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func GetUserInfoByPhone(phone string) (*model.User, error) {
	var user model.User
	err := dao.DB.Where("`phone` = ?", phone).Take(&user).Error

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func CheckUserInviteCode(inviteCode string) (*model.User, error) {
	var user model.User
	err := dao.DB.Where("`invite_code` = ?", inviteCode).Take(&user).Error

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
		UserName:   req.UserName,
		RealName:   req.RealName,
		AvatarURL:  req.AvatarURL,
		Phone:      req.Phone,
		Email:      req.Email,
		IsRetailer: req.IsRetailer,
		ParentId:   req.ParentId,
		InviteCode: req.InviteCode,
		GroupId:    req.GroupId,
		ExpireTime: req.ExpireTime,
		Status:     req.Status,
	}
	req.Password = strings.TrimSpace(req.Password)
	if req.Password != "" {
		user.EncryptPassword(req.Password)
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

	dao.DB.Order("level asc,id asc").Find(&groups)

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
		Price:       req.Price,
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

func GetUserWechatByOpenid(openid string) (*model.UserWechat, error) {
	var userWechat model.UserWechat
	if err := dao.DB.Where("`openid` = ?", openid).First(&userWechat).Error; err != nil {
		return nil, err
	}

	return &userWechat, nil
}

func GetUserByUnionId(unionId string) (*model.User, error) {
	var userWechat model.UserWechat
	if err := dao.DB.Where("`union_id` = ? AND user_id > 0", unionId).First(&userWechat).Error; err != nil {
		return nil, err
	}

	user, err := GetUserInfoById(userWechat.UserId)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func GetUserWechatByUserId(userId uint) (*model.UserWechat, error) {
	var userWechat model.UserWechat
	if err := dao.DB.Where("`user_id` = ?", userId).First(&userWechat).Error; err != nil {
		return nil, err
	}

	return &userWechat, nil
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

func RegisterUser(req *request.ApiRegisterRequest) (*model.User, error) {
	if req.UserName == "" || req.Password == "" {
		return nil, errors.New(config.Lang("请正确填写用户名和密码"))
	}
	if len(req.Password) < 6 {
		return nil, errors.New(config.Lang("请输入6位以上的密码"))
	}
	_, err := GetUserInfoByUserName(req.UserName)
	if err == nil {
		return nil, errors.New(config.Lang("该用户名已被注册"))
	}
	if req.Phone != "" {
		if !VerifyCellphoneFormat(req.Phone) {
			return nil, errors.New(config.Lang("手机号不正确"))
		}
		_, err := GetUserInfoByPhone(req.Phone)
		if err == nil {
			return nil, errors.New(config.Lang("该手机号已被注册"))
		}
	}
	if req.Email != "" {
		if !VerifyEmailFormat(req.Email) {
			return nil, errors.New(config.Lang("邮箱不正确"))
		}
		_, err := GetUserInfoByEmail(req.Email)
		if err == nil {
			return nil, errors.New(config.Lang("该邮箱已被注册"))
		}
	}
	if req.Phone == "" && req.Email == "" {
		return nil, errors.New(config.Lang("邮箱和手机号至少填写一个"))
	}

	user := model.User{
		UserName:  req.UserName,
		RealName:  req.RealName,
		AvatarURL: req.AvatarURL,
		ParentId:  req.InviteId,
		Phone:     req.Phone,
		Email:     req.Email,
		GroupId:   0,
		Status:    1,
	}
	user.EncryptPassword(req.Password)
	dao.DB.Save(&user)

	_ = user.EncodeToken(dao.DB)

	return &user, nil
}

func LoginViaWeapp(req *request.ApiLoginRequest) (*model.User, error) {
	loginRs, err := GetWeappClient(false).Login(req.Code)
	if err != nil {
		return nil, err
	}
	log.Printf("%#v", loginRs)
	if loginRs.OpenID == "" {
		//openid 不在？
		return nil, errors.New("无法获取openid")
	}

	var wecahtUserInfo *weapp.UserInfo
	wecahtUserInfo, err = GetWeappClient(false).DecryptUserInfo(loginRs.SessionKey, req.RawData, req.EncryptedData, req.Signature, req.Iv)
	if err != nil {
		wecahtUserInfo = &weapp.UserInfo{
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
	userWechat, userErr := GetUserWechatByOpenid(loginRs.OpenID)
	var user *model.User
	if userErr != nil {
		//系统没记录，则插入一条记录
		user = &model.User{
			UserName:  wecahtUserInfo.Nickname,
			AvatarURL: wecahtUserInfo.Avatar,
			ParentId:  req.InviteId,
			GroupId:   0,
			Status:    1,
		}

		err = dao.DB.Save(user).Error
		if err != nil {
			return nil, err
		}

		userWechat = &model.UserWechat{
			UserId:    user.Id,
			Nickname:  wecahtUserInfo.Nickname,
			AvatarURL: wecahtUserInfo.Avatar,
			Gender:    wecahtUserInfo.Gender,
			Openid:    loginRs.OpenID,
			UnionId:   loginRs.UnionID,
			Platform:  config.PlatformWeapp,
			Status:    1,
		}

		err = dao.DB.Save(userWechat).Error
		if err != nil {
			//删掉
			dao.DB.Delete(user)
			return nil, err
		}

		go DownloadAvatar(userWechat.AvatarURL, user)
	} else {
		user, err = GetUserInfoById(userWechat.UserId)
		if err != nil {
			return nil, err
		}
		//更新信息
		if wecahtUserInfo.Nickname != "" && (userWechat.Nickname != wecahtUserInfo.Nickname || userWechat.AvatarURL != wecahtUserInfo.Avatar) {
			user.UserName = wecahtUserInfo.Nickname
			user.AvatarURL = wecahtUserInfo.Avatar
			err = dao.DB.Save(user).Error
			if err != nil {
				return nil, err
			}

			userWechat.Nickname = wecahtUserInfo.Nickname
			userWechat.AvatarURL = wecahtUserInfo.Avatar
			err = dao.DB.Save(userWechat).Error
			if err != nil {
				return nil, err
			}
		}
	}

	_ = user.EncodeToken(dao.DB)

	return user, nil
}

func LoginViaWechat(req *request.ApiLoginRequest) (*model.User, error) {
	openid := library.CodeCache.GetByCode(req.Code, false)
	if openid == "" {
		return nil, errors.New("验证码不正确")
	}
	// auto register
	userWechat, err := GetUserWechatByOpenid(openid)
	if err != nil {
		return nil, errors.New("用户信息不完整")
	}
	var user *model.User
	if userWechat.UserId == 0 {
		user = &model.User{
			UserName:  userWechat.Nickname,
			AvatarURL: userWechat.AvatarURL,
			GroupId:   0,
			Password:  "",
			Status:    1,
		}
		dao.DB.Save(user)
		userWechat.UserId = user.Id
		dao.DB.Save(userWechat)
	} else {
		user, err = GetUserInfoById(userWechat.UserId)
		if err != nil {
			return nil, errors.New("用户信息不完整")
		}
	}
	if req.InviteId > 0 && user.ParentId == 0 {
		user.ParentId = req.InviteId
		dao.DB.Save(user)
	}

	_ = user.EncodeToken(dao.DB)

	return user, nil
}

func LoginViaPassword(req *request.ApiLoginRequest) (*model.User, error) {
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

	_ = user.EncodeToken(dao.DB)

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
	filePath := fmt.Sprintf("/uploads/avatar/%s/%s/%s", tmpName[:3], tmpName[3:6], tmpName[6:])
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

func UpdateUserInfo(userId uint, req *request.UserRequest) error {
	user, err := GetUserInfoById(userId)
	if err != nil {
		return err
	}

	exist, err := GetUserInfoByUserName(req.UserName)
	if err == nil && exist.Id != user.Id {
		return errors.New(config.Lang("该用户名已被注册"))
	}

	if user.Phone != "" {
		req.Phone = ""
	}
	if user.Email != "" {
		req.Email = ""
	}
	if req.Phone != "" {
		if !VerifyCellphoneFormat(req.Phone) {
			return errors.New(config.Lang("手机号不正确"))
		}
		exist, err = GetUserInfoByPhone(req.Phone)
		if err == nil && exist.Id != user.Id {
			return errors.New(config.Lang("该手机号已被注册"))
		}
		user.Phone = req.Phone
	}
	if req.Email != "" {
		if !VerifyEmailFormat(req.Email) {
			return errors.New(config.Lang("邮箱不正确"))
		}
		exist, err = GetUserInfoByEmail(req.Email)
		if err == nil && exist.Id != user.Id {
			return errors.New(config.Lang("该邮箱已被注册"))
		}
		user.Email = req.Email
	}
	user.UserName = req.UserName
	user.RealName = req.RealName

	dao.DB.Save(user)

	return nil
}

func CleanUserVip() {
	if dao.DB == nil {
		return
	}
	var group model.UserGroup
	err := dao.DB.Where("`status` = 1").Order("level asc").Take(&group).Error
	if err != nil {
		return
	}
	dao.DB.Model(&model.User{}).Where("`status` = 1 and `group_id` != ? and `expire_time` < ?", group.Id, time.Now().Unix()).UpdateColumn("group_id", group.Id)
}

func GetUserDiscount(userId uint, user *model.User) int64 {
	if user == nil {
		user, _ = GetUserInfoById(userId)
	}
	if user != nil {
		if user.ParentId > 0 {
			parent, err := GetUserInfoById(user.ParentId)
			if err == nil {
				group, err := GetUserGroupInfo(parent.GroupId)
				if err == nil {
					if group.Setting.Discount > 0 {
						return group.Setting.Discount
					}
				}
			}
		}
	}

	return 0
}
