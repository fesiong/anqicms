package provider

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/medivhzhan/weapp/v3"
	"golang.org/x/image/webp"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"image"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"mime/multipart"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (w *Website) GetUserList(ops func(tx *gorm.DB) *gorm.DB, page, pageSize int) ([]*model.User, int64) {
	var users []*model.User
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.User{})
	if ops != nil {
		tx = ops(tx)
	} else {
		tx = tx.Order("id desc")
	}
	tx.Count(&total).Limit(pageSize).Offset(offset).Find(&users)
	if len(users) > 0 {
		groups := w.GetUserGroups()
		for i := range users {
			users[i].GetThumb(w.PluginStorage.StorageUrl)
			for g := range groups {
				if users[i].GroupId == groups[g].Id {
					users[i].Group = groups[g]
				}
			}
		}
	}

	return users, total
}

func (w *Website) GetUserByFunc(ops func(tx *gorm.DB) *gorm.DB) (*model.User, error) {
	var user model.User
	err := ops(w.DB).Take(&user).Error
	if err != nil {
		return nil, err
	}
	user.GetThumb(w.PluginStorage.StorageUrl)
	user.Link = w.GetUrl("user", &user, 0)
	user.Extra = w.GetUserExtra(user.Id)
	return &user, nil
}

func (w *Website) GetUserInfoById(userId uint) (*model.User, error) {
	if userId == 0 {
		return nil, errors.New("no user")
	}
	return w.GetUserByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`id` = ?", userId)
	})
}

func (w *Website) GetUserInfoByUserName(userName string) (*model.User, error) {
	return w.GetUserByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`user_name` = ?", userName)
	})
}

func (w *Website) GetUserInfoByEmail(email string) (*model.User, error) {
	return w.GetUserByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`email` = ?", email)
	})
}

func (w *Website) GetUserInfoByPhone(phone string) (*model.User, error) {
	return w.GetUserByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`phone` = ?", phone)
	})
}

func (w *Website) CheckUserInviteCode(inviteCode string) (*model.User, error) {
	return w.GetUserByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`invite_code` = ?", inviteCode)
	})
}

func (w *Website) GetUsersInfoByIds(userIds []uint) []*model.User {
	var users []*model.User
	if len(userIds) == 0 {
		return users
	}
	w.DB.Where("`id` IN(?)", userIds).Find(&users)

	return users
}

func (w *Website) SaveUserInfo(req *request.UserRequest) error {
	var user *model.User
	var err error
	if req.Id > 0 {
		user, err = w.GetUserInfoById(req.Id)
		if err != nil {
			// 用户不存在
			return err
		}
	} else {
		user = &model.User{}
	}

	user.UserName = req.UserName
	user.RealName = req.RealName
	user.AvatarURL = req.AvatarURL
	user.Introduce = req.Introduce
	user.Phone = req.Phone
	user.Email = req.Email
	user.IsRetailer = req.IsRetailer
	user.ParentId = req.ParentId
	user.InviteCode = req.InviteCode
	user.GroupId = req.GroupId
	user.ExpireTime = req.ExpireTime
	user.Status = req.Status

	user.AvatarURL = strings.TrimPrefix(user.AvatarURL, w.PluginStorage.StorageUrl)
	req.Password = strings.TrimSpace(req.Password)
	if req.Password != "" {
		user.EncryptPassword(req.Password)
	}
	if user.GroupId == 0 {
		user.GroupId = w.PluginUser.DefaultGroupId
	}

	err = w.DB.Save(user).Error
	//extra
	extraFields := map[string]interface{}{}
	if len(w.PluginUser.Fields) > 0 {
		for _, v := range w.PluginUser.Fields {
			if req.Extra[v.FieldName] != nil {
				extraValue, ok := req.Extra[v.FieldName].(map[string]interface{})
				if ok {
					if v.Type == config.CustomFieldTypeCheckbox {
						//只有这个类型的数据是数组,数组转成,分隔字符串
						if val, ok := extraValue["value"].([]interface{}); ok {
							var val2 []string
							for _, v2 := range val {
								val2 = append(val2, v2.(string))
							}
							extraFields[v.FieldName] = strings.Join(val2, ",")
						}
					} else if v.Type == config.CustomFieldTypeNumber {
						//只有这个类型的数据是数字，转成数字
						extraFields[v.FieldName], _ = strconv.Atoi(fmt.Sprintf("%v", extraValue["value"]))
					} else {
						value, ok := extraValue["value"].(string)
						if ok {
							extraFields[v.FieldName] = strings.TrimPrefix(value, w.PluginStorage.StorageUrl)
						} else {
							extraFields[v.FieldName] = extraValue["value"]
						}
					}
				}
			} else {
				if v.Type == config.CustomFieldTypeNumber {
					//只有这个类型的数据是数字，转成数字
					extraFields[v.FieldName] = 0
				} else {
					extraFields[v.FieldName] = ""
				}
			}
		}
	}

	//extra
	if len(extraFields) > 0 {
		//入库
		w.DB.Model(model.User{}).Where("`id` = ?", user.Id).Updates(extraFields)
	}
	return err
}

func (w *Website) DeleteUserInfo(userId uint) error {
	var user model.User
	err := w.DB.Where("`id` = ?", userId).Take(&user).Error

	if err != nil {
		return err
	}

	err = w.DB.Delete(&user).Error

	return err
}

func (w *Website) GetUserGroups() []*model.UserGroup {
	var groups []*model.UserGroup

	w.DB.Order("level asc,id asc").Find(&groups)

	return groups
}

func (w *Website) GetUserGroupInfo(groupId uint) (*model.UserGroup, error) {
	var group model.UserGroup

	err := w.DB.Where("`id` = ?", groupId).Take(&group).Error

	if err != nil {
		return nil, err
	}

	return &group, nil
}

func (w *Website) GetUserGroupInfoByLevel(level int) (*model.UserGroup, error) {
	var group model.UserGroup

	err := w.DB.Where("`level` = ?", level).Take(&group).Error

	if err != nil {
		return nil, err
	}

	return &group, nil
}

func (w *Website) SaveUserGroupInfo(req *request.UserGroupRequest) error {
	var group = model.UserGroup{
		Title:       req.Title,
		Description: req.Description,
		Level:       req.Level,
		Price:       req.Price,
		Status:      1,
		Setting:     req.Setting,
	}
	if req.Id > 0 {
		_, err := w.GetUserGroupInfo(req.Id)
		if err != nil {
			// 不存在
			return err
		}
		group.Id = req.Id
	}
	err := w.DB.Save(&group).Error

	return err
}

func (w *Website) GetUserWechatByOpenid(openid string) (*model.UserWechat, error) {
	var userWechat model.UserWechat
	if err := w.DB.Where("`openid` = ?", openid).First(&userWechat).Error; err != nil {
		return nil, err
	}

	return &userWechat, nil
}

func (w *Website) GetUserByUnionId(unionId string) (*model.User, error) {
	var userWechat model.UserWechat
	if err := w.DB.Where("`union_id` = ? AND user_id > 0", unionId).First(&userWechat).Error; err != nil {
		return nil, err
	}

	user, err := w.GetUserInfoById(userWechat.UserId)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (w *Website) GetUserWechatByUserId(userId uint) (*model.UserWechat, error) {
	var userWechat model.UserWechat
	if err := w.DB.Where("`user_id` = ?", userId).First(&userWechat).Error; err != nil {
		return nil, err
	}

	return &userWechat, nil
}

func (w *Website) DeleteUserGroup(groupId uint) error {
	var group model.UserGroup
	err := w.DB.Where("`id` = ?", groupId).Take(&group).Error

	if err != nil {
		return err
	}

	err = w.DB.Delete(&group).Error

	return err
}

func (w *Website) RegisterUser(req *request.ApiRegisterRequest) (*model.User, error) {
	if req.UserName == "" || req.Password == "" {
		return nil, errors.New(w.Tr("PleaseFillInTheUsernameAndPasswordCorrectly"))
	}
	if len(req.Password) < 6 {
		return nil, errors.New(w.Tr("PleaseEnterAPasswordOfMoreThan6Digits"))
	}
	_, err := w.GetUserInfoByUserName(req.UserName)
	if err == nil {
		return nil, errors.New(w.Tr("TheUsernameHasBeenRegistered"))
	}
	if req.Phone != "" {
		if !w.VerifyCellphoneFormat(req.Phone) {
			return nil, errors.New(w.Tr("IncorrectMobilePhoneNumber"))
		}
		_, err := w.GetUserInfoByPhone(req.Phone)
		if err == nil {
			return nil, errors.New(w.Tr("TheMobilePhoneNumberHasBeenRegistered"))
		}
	}
	if req.Email != "" {
		if !w.VerifyEmailFormat(req.Email) {
			return nil, errors.New(w.Tr("IncorrectEmail"))
		}
		_, err := w.GetUserInfoByEmail(req.Email)
		if err == nil {
			return nil, errors.New(w.Tr("TheEmailHasBeenRegistered"))
		}
	}
	if req.Phone == "" && req.Email == "" {
		return nil, errors.New(w.Tr("FillInAtLeastOneOfTheEmailAndMobilePhoneNumber"))
	}

	user := model.User{
		UserName:  req.UserName,
		RealName:  req.RealName,
		AvatarURL: req.AvatarURL,
		ParentId:  req.InviteId,
		Phone:     req.Phone,
		Email:     req.Email,
		GroupId:   w.PluginUser.DefaultGroupId,
		Status:    1,
	}
	user.EncryptPassword(req.Password)
	w.DB.Save(&user)

	user.Token = w.GetUserAuthToken(user.Id, true)
	_ = user.LogLogin(w.DB)

	return &user, nil
}

func (w *Website) LoginViaWeapp(req *request.ApiLoginRequest) (*model.User, error) {
	loginRs, err := w.GetWeappClient(false).Login(req.Code)
	if err != nil {
		return nil, err
	}
	//log.Printf("%#v", loginRs)
	if loginRs.OpenID == "" {
		//openid 不在？
		return nil, errors.New(w.Tr("UnableToObtainOpenid"))
	}

	var wecahtUserInfo *weapp.UserInfo
	wecahtUserInfo, err = w.GetWeappClient(false).DecryptUserInfo(loginRs.SessionKey, req.RawData, req.EncryptedData, req.Signature, req.Iv)
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
	userWechat, userErr := w.GetUserWechatByOpenid(loginRs.OpenID)
	var user *model.User
	if userErr != nil {
		//系统没记录，则插入一条记录
		user = &model.User{
			UserName:  wecahtUserInfo.Nickname,
			AvatarURL: wecahtUserInfo.Avatar,
			ParentId:  req.InviteId,
			GroupId:   w.PluginUser.DefaultGroupId,
			Status:    1,
		}

		err = w.DB.Save(user).Error
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

		err = w.DB.Save(userWechat).Error
		if err != nil {
			//删掉
			w.DB.Delete(user)
			return nil, err
		}

		go w.DownloadAvatar(userWechat.AvatarURL, user)
	} else {
		user, err = w.GetUserInfoById(userWechat.UserId)
		if err != nil {
			return nil, err
		}
		//更新信息
		if wecahtUserInfo.Nickname != "" && (userWechat.Nickname != wecahtUserInfo.Nickname || userWechat.AvatarURL != wecahtUserInfo.Avatar) {
			user.UserName = wecahtUserInfo.Nickname
			user.AvatarURL = wecahtUserInfo.Avatar
			err = w.DB.Save(user).Error
			if err != nil {
				return nil, err
			}

			userWechat.Nickname = wecahtUserInfo.Nickname
			userWechat.AvatarURL = wecahtUserInfo.Avatar
			err = w.DB.Save(userWechat).Error
			if err != nil {
				return nil, err
			}
		}
	}

	user.Token = w.GetUserAuthToken(user.Id, true)
	_ = user.LogLogin(w.DB)

	return user, nil
}

func (w *Website) LoginViaWechat(req *request.ApiLoginRequest) (*model.User, error) {
	openid := library.CodeCache.GetByCode(req.Code, false)
	if openid == "" {
		return nil, errors.New(w.Tr("VerificationCodeIsIncorrect"))
	}
	// auto register
	userWechat, err := w.GetUserWechatByOpenid(openid)
	if err != nil {
		return nil, errors.New(w.Tr("IncompleteUserInfo"))
	}
	var user *model.User
	if userWechat.UserId == 0 {
		user = &model.User{
			UserName:  userWechat.Nickname,
			AvatarURL: userWechat.AvatarURL,
			GroupId:   w.PluginUser.DefaultGroupId,
			Password:  "",
			Status:    1,
		}
		w.DB.Save(user)
		userWechat.UserId = user.Id
		w.DB.Save(userWechat)
	} else {
		user, err = w.GetUserInfoById(userWechat.UserId)
		if err != nil {
			return nil, errors.New(w.Tr("IncompleteUserInfo"))
		}
	}
	if req.InviteId > 0 && user.ParentId == 0 {
		user.ParentId = req.InviteId
		w.DB.Save(user)
	}

	user.Token = w.GetUserAuthToken(user.Id, true)
	_ = user.LogLogin(w.DB)

	return user, nil
}

func (w *Website) LoginViaPassword(req *request.ApiLoginRequest) (*model.User, error) {
	var user model.User
	if w.VerifyEmailFormat(req.UserName) {
		//邮箱登录
		err := w.DB.Where("email = ?", req.UserName).First(&user).Error
		if err != nil {
			return nil, err
		}
	} else if w.VerifyCellphoneFormat(req.UserName) {
		//手机号登录
		err := w.DB.Where("phone = ?", req.UserName).First(&user).Error
		if err != nil {
			return nil, err
		}
	} else {
		//用户名登录
		err := w.DB.Where("user_name = ?", req.UserName).First(&user).Error
		if err != nil {
			return nil, err
		}
	}
	//验证密码
	ok := user.CheckPassword(req.Password)
	if !ok {
		return nil, errors.New(w.Tr("WrongPassword"))
	}

	user.Token = w.GetUserAuthToken(user.Id, true)
	_ = user.LogLogin(w.DB)

	return &user, nil
}

func (w *Website) VerifyEmailFormat(email string) bool {
	pattern := `\w+([-+.]\w+)*@\w+([-.]\w+)*\.\w+([-.]\w+)*` //匹配电子邮箱
	reg := regexp.MustCompile(pattern)
	return reg.MatchString(email)
}

func (w *Website) VerifyCellphoneFormat(cellphone string) bool {
	pattern := `1[3-9][0-9]{9}` //宽匹配手机号
	reg := regexp.MustCompile(pattern)
	return reg.MatchString(cellphone)
}

func (w *Website) DownloadAvatar(avatarUrl string, userInfo *model.User) {
	if avatarUrl == "" || !strings.HasPrefix(avatarUrl, "http") {
		return
	}

	//生成用户文件
	tmpName := fmt.Sprintf("%010d.jpg", userInfo.Id)
	filePath := fmt.Sprintf("/uploads/avatar/%s/%s/%s", tmpName[:3], tmpName[3:6], tmpName[6:])
	attach, err := w.DownloadRemoteImage(avatarUrl, filePath)
	if err != nil {
		return
	}
	//写入完成，更新数据库
	userInfo.AvatarURL = attach.FileLocation
	w.DB.Model(userInfo).UpdateColumn("avatar_url", userInfo.AvatarURL)
}

func (w *Website) GetRetailerMembers(retailerId uint, page, pageSize int) ([]*model.User, int64) {
	var users []*model.User
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.User{}).Where("`parent_id` = ?", retailerId)
	tx.Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&users)

	return users, total
}

func (w *Website) UpdateUserRealName(userId uint, realName string) error {
	err := w.DB.Model(&model.User{}).Where("`id` = ?", userId).UpdateColumn("real_name", realName).Error

	return err
}

func (w *Website) SetRetailerInfo(userId uint, isRetailer int) error {
	err := w.DB.Model(&model.User{}).Where("`id` = ?", userId).UpdateColumn("is_retailer", isRetailer).Error

	return err
}

func (w *Website) UpdateUserInfo(userId uint, req *request.UserRequest) error {
	user, err := w.GetUserInfoById(userId)
	if err != nil {
		return err
	}

	exist, err := w.GetUserInfoByUserName(req.UserName)
	if err == nil && exist.Id != user.Id {
		return errors.New(w.Tr("TheUsernameHasBeenRegistered"))
	}

	if user.Phone != "" {
		req.Phone = ""
	}
	if user.Email != "" {
		req.Email = ""
	}
	if req.Phone != "" {
		if !w.VerifyCellphoneFormat(req.Phone) {
			return errors.New(w.Tr("IncorrectMobilePhoneNumber"))
		}
		exist, err = w.GetUserInfoByPhone(req.Phone)
		if err == nil && exist.Id != user.Id {
			return errors.New(w.Tr("TheMobilePhoneNumberHasBeenRegistered"))
		}
		user.Phone = req.Phone
	}
	if req.Email != "" {
		if !w.VerifyEmailFormat(req.Email) {
			return errors.New(w.Tr("IncorrectEmail"))
		}
		exist, err = w.GetUserInfoByEmail(req.Email)
		if err == nil && exist.Id != user.Id {
			return errors.New(w.Tr("TheEmailHasBeenRegistered"))
		}
		user.Email = req.Email
	}
	user.UserName = req.UserName
	user.RealName = req.RealName
	user.Introduce = req.Introduce
	if user.GroupId == 0 {
		user.GroupId = w.PluginUser.DefaultGroupId
	}

	w.DB.Save(user)

	return nil
}

func (w *Website) CleanUserVip() {
	if w.DB == nil {
		return
	}
	var group model.UserGroup
	err := w.DB.Where("`status` = 1").Order("level asc").Take(&group).Error
	if err != nil {
		return
	}
	w.DB.Model(&model.User{}).Where("`status` = 1 and `group_id` != ? and `expire_time` < ?", group.Id, time.Now().Unix()).UpdateColumn("group_id", group.Id)
}

func (w *Website) GetUserDiscount(userId uint, user *model.User) int64 {
	if user == nil {
		user, _ = w.GetUserInfoById(userId)
	}
	if user != nil {
		if user.ParentId > 0 {
			parent, err := w.GetUserInfoById(user.ParentId)
			if err == nil {
				group, err := w.GetUserGroupInfo(parent.GroupId)
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

func (w *Website) GetUserFields() []*config.CustomField {
	//这里有默认的设置
	fields := w.PluginUser.Fields

	return fields
}

func (w *Website) MigrateUserTable(fields []*config.CustomField, focus bool) {
	tableName := "users"
	// 根据表单字段，生成数据
	for _, field := range fields {
		field.CheckSetFilter()
		column := field.GetFieldColumn()
		if !w.DB.Migrator().HasColumn(model.User{}, field.FieldName) {
			//创建语句
			w.DB.Exec("ALTER TABLE ? ADD COLUMN ?", clause.Table{Name: tableName}, gorm.Expr(column))
		} else if focus {
			//更新语句
			w.DB.Exec("ALTER TABLE ? MODIFY COLUMN ?", clause.Table{Name: tableName}, gorm.Expr(column))
		}
	}
}

func (w *Website) DeleteUserField(fieldName string) error {
	for i, val := range w.PluginUser.Fields {
		if val.FieldName == fieldName {
			if w.DB.Migrator().HasColumn(model.User{}, val.FieldName) {
				w.DB.Migrator().DropColumn(model.User{}, val.FieldName)
			}

			w.PluginUser.Fields = append(w.PluginUser.Fields[:i], w.PluginUser.Fields[i+1:]...)
			break
		}
	}
	// 回写
	err := w.SaveSettingValue(UserSettingKey, w.PluginUser)
	return err
}

func (w *Website) GetUserExtra(id uint) map[string]*model.CustomField {
	//读取extra
	result := map[string]interface{}{}
	extraFields := map[string]*model.CustomField{}
	var fields []string
	for _, v := range w.PluginUser.Fields {
		fields = append(fields, "`"+v.FieldName+"`")
	}
	//从数据库中取出来
	if len(fields) > 0 {
		w.DB.Model(model.User{}).Where("`id` = ?", id).Select(strings.Join(fields, ",")).Scan(&result)
		//extra的CheckBox的值
		for _, v := range w.PluginUser.Fields {
			value, ok := result[v.FieldName].(string)
			if ok {
				if v.Type == config.CustomFieldTypeImage || v.Type == config.CustomFieldTypeFile || v.Type == config.CustomFieldTypeEditor {
					result[v.FieldName] = w.ReplaceContentUrl(value, true)
				}
			}
			extraFields[v.FieldName] = &model.CustomField{
				Name:        v.Name,
				Value:       result[v.FieldName],
				Default:     v.Content,
				FollowLevel: v.FollowLevel,
			}
		}
	}

	return extraFields
}

func (w *Website) GetUserAuthToken(userId uint, remember bool) string {
	// 默认24小时
	t := time.Now().Add(24 * time.Hour)
	// 记住会记住30天
	if remember {
		t = t.AddDate(0, 0, 29)
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": fmt.Sprintf("%d", userId),
		"t":      fmt.Sprintf("%d", t.Unix()),
	})
	// 获取签名字符串
	tokenString, err := jwtToken.SignedString([]byte(w.TokenSecret + "-user-token"))
	if err != nil {
		return ""
	}

	return tokenString
}

func (w *Website) UploadUserAvatar(userId uint, file multipart.File) (avatarUrl string, err error) {
	var fileName string
	img, imgType, err := image.Decode(file)
	if err != nil {
		file.Seek(0, 0)
		img, err = webp.Decode(file)
		imgType = "webp"
		if err != nil {
			return "", errors.New(w.Tr("UnsupportedImageFormat"))
		}
	}
	if imgType == "jpeg" {
		imgType = "jpg"
	}
	if imgType != "jpg" && imgType != "gif" && imgType != "webp" {
		imgType = "png"
	}
	file.Seek(0, 0)
	fileName = "uploads/user/" + strconv.Itoa(int(userId)) + "." + imgType
	// 头像统一处理裁剪成正方形，256*256，并且不加水印
	newImg := library.ThumbnailCrop(256, 256, img, 2)
	buf, _, _ := encodeImage(newImg, imgType, w.Content.Quality)
	// 上传图片
	_, err = w.Storage.UploadFile(fileName, buf)
	if err != nil {
		return "", err
	}

	// 更新用户头像地址
	w.DB.Model(&model.User{}).Where("`id` = ?", userId).UpdateColumn("avatar_url", fileName)

	// 返回头像地址
	return w.PluginStorage.StorageUrl + "/" + fileName, nil
}
