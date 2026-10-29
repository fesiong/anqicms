package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"mime/multipart"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/medivhzhan/weapp/v3"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/image/webp"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

func (w *Website) GetUserList(ops func(tx *gorm.DB) *gorm.DB, page, pageSize int) ([]*model.User, int64) {
	var users []*model.User
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.User{})
	if ops != nil {
		tx = ops(tx)
	} else {
		tx = tx.Order("users.id desc")
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
	user.Link = w.GetUrl(PatternPeople, &user, 0)
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

func (w *Website) GetUserInfoByUrlToken(urlToken string) (*model.User, error) {
	return w.GetUserByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`url_token` = ?", urlToken)
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

func (w *Website) SaveUserInfo(req *request.UserRequest) (*model.User, error) {
	var user *model.User
	var err error
	// 用户名唯一性校验：user_name 是「按用户名登录」的凭据，重名会串号。
	// 创建时重名直接拒绝（调用方自己填的名字，改一下就行）；
	// 第三方自动注册不经过这里，那条路径用 uniqueUserName 加后缀而不是拒绝。
	req.UserName = strings.TrimSpace(req.UserName)
	if req.Id > 0 {
		user, err = w.GetUserInfoById(req.Id)
		if err != nil {
			// 用户不存在
			return nil, err
		}
		// 改名也不能撞上别人（与 UpdateUserInfo 同一口径）。
		if req.UserName != "" && w.userNameTaken(req.UserName, user.Id) {
			return nil, errors.New(w.Tr("TheUsernameHasBeenRegistered"))
		}
	} else {
		if req.UserName == "" {
			return nil, errors.New(w.Tr("PleaseEnterUsername"))
		}
		if w.userNameTaken(req.UserName, 0) {
			return nil, errors.New(w.Tr("TheUsernameHasBeenRegistered"))
		}
		user = &model.User{}
	}
	oldGroupId := user.GroupId

	// 指针字段的归一：nil 表示调用方没提这个字段，保持库里原值；
	// UpdateAll（表单全量提交）时才把「没提」当成「清空」。
	// 不能再用 `req.X != ""` / `req.X > 0` 判断——那会把「显式传空/传 0」也当成没传。
	if req.UpdateAll || req.UserName != "" {
		user.UserName = req.UserName
	}
	library.Assign(&user.RealName, req.RealName, req.UpdateAll)
	library.Assign(&user.FirstName, req.FirstName, req.UpdateAll)
	library.Assign(&user.LastName, req.LastName, req.UpdateAll)
	library.Assign(&user.Introduce, req.Introduce, req.UpdateAll)
	library.Assign(&user.Phone, req.Phone, req.UpdateAll)
	library.Assign(&user.Email, req.Email, req.UpdateAll)
	library.Assign(&user.InviteCode, req.InviteCode, req.UpdateAll)
	library.Assign(&user.Birthday, req.Birthday, req.UpdateAll)
	if req.AvatarURL != nil {
		user.AvatarURL = strings.TrimPrefix(*req.AvatarURL, w.PluginStorage.StorageUrl)
	} else if req.UpdateAll {
		user.AvatarURL = ""
	}
	library.Assign(&user.IsRetailer, req.IsRetailer, req.UpdateAll)
	library.Assign(&user.ParentId, req.ParentId, req.UpdateAll)
	library.Assign(&user.ExpireTime, req.ExpireTime, req.UpdateAll)
	// Status 的 0=待审核、-1=禁用 都是合法业务值，必须走指针才能写进去。
	library.Assign(&user.Status, req.Status, req.UpdateAll)
	library.Assign(&user.GroupId, req.GroupId, req.UpdateAll)
	// 用户组为空时落到默认组（原来是只挂在 GroupId>0 / Status>0 分支里，
	// 现在统一在末尾校正，语义不变但不会再漏）。
	if user.GroupId == 0 {
		user.GroupId = w.PluginUser.DefaultGroupId
	}

	req.Password = strings.TrimSpace(req.Password)
	if req.Password != "" {
		user.EncryptPassword(req.Password)
	}

	err = w.DB.Save(user).Error
	if req.Id == 0 {
		w.ProcessNewUser(user)
	}
	// 判断订阅
	// 用合并后的 user.Email 而不是 req.Email：PATCH 时 req.Email 是 nil。
	if user.Email != "" {
		subscriber, err2 := w.GetSubscriberByEmail(user.Email)
		if err2 == nil {
			// 添加订阅
			w.DB.Model(&model.Subscriber{}).Where("id = ?", subscriber.Id).UpdateColumn("user_id", int64(user.Id))
		}
	}
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
								v2s, _ := v2.(string)
								val2 = append(val2, v2s)
							}
							extraFields[v.FieldName] = strings.Join(val2, ",")
						} else if val, ok := extraValue["value"].([]string); ok {
							extraFields[v.FieldName] = strings.Join(val, ",")
						} else if extraValue["value"] != nil {
							extraFields[v.FieldName] = extraValue["value"]
						}
					} else if v.Type == config.CustomFieldTypeNumber || v.Type == config.CustomFieldTypeCategory {
						//只有这个类型的数据是数字，转成数字
						extraFields[v.FieldName], _ = strconv.ParseInt(fmt.Sprint(extraValue["value"]), 10, 64)
					} else if v.Type == config.CustomFieldTypeImages || v.Type == config.CustomFieldTypeTexts || v.Type == config.CustomFieldTypeArchive {
						// 存 json
						if val, ok := extraValue["value"].([]interface{}); ok {
							for j, v2 := range val {
								v2s, ok2 := v2.(string)
								if ok2 {
									val[j] = w.ReplaceContentUrl(v2s, false)
								}
							}
							buf, _ := json.Marshal(val)
							extraFields[v.FieldName] = string(buf)
						}
					} else if v.Type == config.CustomFieldTypeTimeline {
						// 存 json
						buf, _ := json.Marshal(extraValue["value"])
						extraFields[v.FieldName] = string(buf)
					} else {
						value, ok := extraValue["value"].(string)
						if ok {
							if v.Type == config.CustomFieldTypeImage || v.Type == config.CustomFieldTypeFile || v.Type == config.CustomFieldTypeEditor {
								extraFields[v.FieldName] = w.ReplaceContentUrl(value, false)
							} else {
								extraFields[v.FieldName] = extraValue["value"]
							}
						} else {
							extraFields[v.FieldName] = extraValue["value"]
						}
					}
				}
			} else {
				// 自定义字段没出现在请求里：只有全量提交才按空值补齐。
				// PATCH（AI/MCP 改一个手机号）不能顺手把其它自定义字段清空。
				if req.UpdateAll {
					if v.Type == config.CustomFieldTypeNumber {
						//只有这个类型的数据是数字，转成数字
						extraFields[v.FieldName] = 0
					} else {
						extraFields[v.FieldName] = ""
					}
				}
			}
		}
	}

	//extra
	if len(extraFields) > 0 {
		//入库
		w.DB.Model(model.User{}).Where("`id` = ?", user.Id).Updates(extraFields)
	}

	// 发送用户组变更邮件
	if oldGroupId > 0 && oldGroupId != user.GroupId {
		oldGroup, err := w.GetUserGroupInfo(oldGroupId)
		if err == nil {
			newGroup, _ := w.GetUserGroupInfo(user.GroupId)
			user.Group = newGroup
			w.SendUserGroupChangeEmail(user, oldGroup)
		}
	}

	return user, err
}

// ── 用户名唯一性（2026-10-27）────────────────────────────────────────────
//
// user_name 不只是展示名：LoginViaPassword 有一条「按 user_name 登录」的分支，
// 两个同名账号会让人登进别人那里。所以：
//   - 后台/AI 创建（SaveUserInfo，用户名由调用方自选）→ 重名直接拒绝，不改写；
//   - 第三方自动注册（Google/公众号/小程序，用户名取自平台昵称，用户无法自选）
//     → 先查重，重复时追加数字后缀：张三、张三2、张三3……
//
// 之所以两条路径策略不同：前者用户能改，报错让他换一个就行；后者他改不了昵称，
// 拒绝注册等于把第三方登录整个堵死。
const (
	// userNameMaxLen 与 model.User.UserName 的 varchar(64) 对齐。
	// 后缀最多 4 位（2..9999），所以 base 压到 60 个字符即可保证总长不越界。
	userNameMaxLen = 60
	// defaultThirdPartyUserName 是平台没给昵称时的兜底名。
	// 允许空用户名会让「按用户名登录」命中任意一条空名记录，必须堵掉。
	defaultThirdPartyUserName = "用户"
)

// truncateRunes 按字符（不是字节）截断，避免切裂多字节字符。
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// nextUniqueUserName 生成不冲突的用户名。taken 判定「该名字已被占用」。
// 抽成纯函数是为了让后缀规则能脱库测试。
func nextUniqueUserName(base string, taken func(string) bool) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = defaultThirdPartyUserName
	}
	base = truncateRunes(base, userNameMaxLen)
	if !taken(base) {
		return base
	}
	for i := 2; i <= 9999; i++ {
		candidate := base + strconv.Itoa(i)
		if !taken(candidate) {
			return candidate
		}
	}
	// 极端兜底：base 的 9999 个后缀都被占。纳秒时间戳定长 19 位，
	// 先把 base 压到 41 字符再拼，保证总长不超过 60。
	return truncateRunes(base, userNameMaxLen-19) + strconv.FormatInt(time.Now().UnixNano(), 10)
}

// userNameTaken 判断用户名是否已被占用；excludeId > 0 时排除自身（更新场景）。
func (w *Website) userNameTaken(userName string, excludeId uint) bool {
	tx := w.DB.Model(&model.User{}).Where("`user_name` = ?", userName)
	if excludeId > 0 {
		tx = tx.Where("`id` <> ?", excludeId)
	}
	var count int64
	tx.Count(&count)
	return count > 0
}

// uniqueUserName 供第三方自动注册使用：重名时加数字后缀而不是拒绝。
func (w *Website) uniqueUserName(base string, excludeId uint) string {
	return nextUniqueUserName(base, func(candidate string) bool {
		return w.userNameTaken(candidate, excludeId)
	})
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

func (w *Website) UpdateUserBalance(req *request.ApiUserBalanceRequest) error {
	var user model.User
	err := w.DB.Where("`id` = ?", req.UserId).Take(&user).Error

	if err != nil {
		return err
	}

	tx := w.DB.Begin()
	//生成用户支付记录
	err = tx.Model(user).Where("id = ?", user.Id).UpdateColumn("balance", gorm.Expr("`balance` + ?", req.Amount)).Error
	if err != nil {
		tx.Rollback()
		return err
	}
	var userBalance int64
	tx.Model(&model.User{}).Where("`id` = ?", req.UserId).Pluck("balance", &userBalance)
	//状态更改了，增加一条记录到用户
	finance := model.Finance{
		UserId:      req.UserId,
		Direction:   config.FinanceIncome,
		Amount:      req.Amount,
		AfterAmount: userBalance,
		Action:      config.FinanceActionCharge,
		Remark:      req.Remark,
		Status:      1,
	}
	err = tx.Create(&finance).Error
	if err != nil {
		tx.Rollback()

		return err
	}
	tx.Commit()

	return nil
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

func (w *Website) SaveUserGroupInfo(req *request.UserGroupRequest) (*model.UserGroup, error) {
	// 这里原来是「用请求字段拼一个新对象再 Save」，GORM 的 Save 会写全列，
	// 于是没传的 Level/Price/Description 被清成零值，Status 更是硬编码成 1
	// ——用户组只要保存过一次就再也停不掉。改成先读库再按指针合并。
	var group *model.UserGroup
	if req.Id > 0 {
		exist, err := w.GetUserGroupInfo(req.Id)
		if err != nil {
			// 不存在
			return nil, err
		}
		group = exist
	} else {
		group = &model.UserGroup{Status: 1}
	}
	library.Assign(&group.Title, req.Title, req.UpdateAll)
	library.Assign(&group.Description, req.Description, req.UpdateAll)
	if req.Level != nil {
		group.Level = *req.Level
	} else if req.UpdateAll {
		group.Level = 0
	}
	if req.Price != nil {
		group.Price = *req.Price
	} else if req.UpdateAll {
		group.Price = 0
	}
	// Status 的 0 是「停用」，是合法值，不能用 >0 判断。
	if req.Status != nil {
		group.Status = *req.Status
	}
	if req.Setting != nil {
		group.Setting = *req.Setting
	}

	if err := w.DB.Save(group).Error; err != nil {
		return nil, err
	}

	return group, nil
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
	template, tplExist := w.GetEmailTemplateInfo("register")
	exist, err := w.GetUserInfoByUserName(req.UserName)
	if err == nil {
		// 邮箱已存在，如果还没验证，则发送验证邮件
		if !exist.EmailVerified && tplExist && template.Open {
			_ = w.SendVerifyEmail(exist, "verify")
			return exist, nil
		}
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
		exist, err := w.GetUserInfoByEmail(req.Email)
		if err == nil {
			// 邮箱已存在，如果还没验证，则发送验证邮件
			if !exist.EmailVerified && tplExist && template.Open {
				_ = w.SendVerifyEmail(exist, "verify")
				return exist, nil
			}
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
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Birthday:  req.Birthday,
		Introduce: req.Introduce,
		Phone:     req.Phone,
		Email:     req.Email,
		GroupId:   w.PluginUser.DefaultGroupId,
		Status:    w.PluginUser.GetDefaultStatus(),
	}
	user.AvatarURL = strings.TrimPrefix(user.AvatarURL, w.PluginStorage.StorageUrl)
	req.Password = strings.TrimSpace(req.Password)
	user.EncryptPassword(req.Password)
	w.DB.Save(&user)

	//extra
	extraFields := map[string]interface{}{}
	if len(w.PluginUser.Fields) > 0 {
		for _, v := range w.PluginUser.Fields {
			if req.Extra[v.FieldName] != nil {
				extraValue, ok := req.Extra[v.FieldName]
				if ok {
					if v.Type == config.CustomFieldTypeCheckbox {
						//只有这个类型的数据是数组,数组转成,分隔字符串
						if val, ok := extraValue.([]interface{}); ok {
							var val2 []string
							for _, v2 := range val {
								v2s, _ := v2.(string)
								val2 = append(val2, v2s)
							}
							extraFields[v.FieldName] = strings.Join(val2, ",")
						} else if val, ok := extraValue.([]string); ok {
							extraFields[v.FieldName] = strings.Join(val, ",")
						} else if extraValue != nil {
							extraFields[v.FieldName] = extraValue
						}
					} else if v.Type == config.CustomFieldTypeNumber || v.Type == config.CustomFieldTypeCategory {
						//只有这个类型的数据是数字，转成数字
						extraFields[v.FieldName], _ = strconv.ParseInt(fmt.Sprint(extraValue), 10, 64)
					} else if v.Type == config.CustomFieldTypeImages || v.Type == config.CustomFieldTypeTexts || v.Type == config.CustomFieldTypeArchive {
						// 存 json
						if val, ok := extraValue.([]interface{}); ok {
							for j, v2 := range val {
								v2s, ok2 := v2.(string)
								if ok2 {
									val[j] = w.ReplaceContentUrl(v2s, false)
								}
							}
							buf, _ := json.Marshal(val)
							extraFields[v.FieldName] = string(buf)
						}
					} else if v.Type == config.CustomFieldTypeTimeline {
						// 存 json
						buf, _ := json.Marshal(extraValue)
						extraFields[v.FieldName] = string(buf)
					} else {
						value, ok := extraValue.(string)
						if ok {
							if v.Type == config.CustomFieldTypeImage || v.Type == config.CustomFieldTypeFile || v.Type == config.CustomFieldTypeEditor {
								extraFields[v.FieldName] = w.ReplaceContentUrl(value, false)
							} else {
								extraFields[v.FieldName] = extraValue
							}
						} else {
							extraFields[v.FieldName] = extraValue
						}
					}
				}
			} else {
				if v.Type == config.CustomFieldTypeNumber || v.Type == config.CustomFieldTypeCategory {
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

	w.ProcessNewUser(&user)

	if tplExist && template.Open && user.Email != "" {
		_ = w.SendVerifyEmail(&user, "verify")
		return &user, nil
	}

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
			// 昵称来自微信，用户改不了；撞名时加数字后缀而不是拒绝注册。
			UserName:      w.uniqueUserName(wecahtUserInfo.Nickname, 0),
			AvatarURL:     wecahtUserInfo.Avatar,
			ParentId:      req.InviteId,
			GroupId:       w.PluginUser.DefaultGroupId,
			ResetPassword: true,
			Status:        w.PluginUser.GetDefaultStatus(),
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

		w.ProcessNewUser(user)

		go w.DownloadAvatar(userWechat.AvatarURL, user)
	} else {
		user, err = w.GetUserInfoById(userWechat.UserId)
		if err != nil {
			return nil, err
		}
		//更新信息
		if wecahtUserInfo.Nickname != "" && (userWechat.Nickname != wecahtUserInfo.Nickname || userWechat.AvatarURL != wecahtUserInfo.Avatar) {
			user.UserName = w.uniqueUserName(wecahtUserInfo.Nickname, user.Id)
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
			// 昵称来自公众号，用户改不了；撞名时加数字后缀而不是拒绝注册。
			UserName:      w.uniqueUserName(userWechat.Nickname, 0),
			AvatarURL:     userWechat.AvatarURL,
			GroupId:       w.PluginUser.DefaultGroupId,
			Password:      "",
			ResetPassword: true,
			Status:        w.PluginUser.GetDefaultStatus(),
		}
		w.DB.Save(user)
		userWechat.UserId = user.Id
		w.DB.Save(userWechat)

		w.ProcessNewUser(user)
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

func (w *Website) LoginViaGoogle(req *request.ApiLoginRequest) (*model.User, error) {
	googleCfg := w.GetGoogleAuthConfig(false)
	if googleCfg == nil {
		return nil, errors.New(w.Tr("GoogleAuthDisable"))
	}
	ctx, done := context.WithTimeout(context.Background(), 15*time.Second)
	defer done()
	tok, err := googleCfg.Exchange(ctx, req.Code)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	// 用 REST API 获取用户信息，替代 google.golang.org/api/oauth2/v2
	client := googleCfg.Client(ctx, tok)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	var googleUser GoogleUser
	if err := json.Unmarshal(body, &googleUser); err != nil {
		log.Println(err)
		return nil, err
	}
	// 直接对接 users 表的 email
	user, err := w.GetUserInfoByEmail(googleUser.Email)
	if err != nil {
		// 用户不存在，新建
		user = &model.User{
			Email:         googleUser.Email,
			EmailVerified: googleUser.EmailVerified,
			UserName:      w.uniqueUserName(googleUser.Name, 0),
			AvatarURL:     googleUser.Picture,
			GoogleId:      googleUser.Sub,
			GroupId:       w.PluginUser.DefaultGroupId,
			Password:      "",
			ResetPassword: true,
			Status:        w.PluginUser.GetDefaultStatus(),
		}
		if googleUser.GivenName != "" {
			user.RealName = googleUser.GivenName + " " + googleUser.FamilyName
		}
		w.DB.Save(user)
		w.ProcessNewUser(user)
	} else {
		user.GoogleId = googleUser.Sub
		if googleUser.Name != "" {
			// 老用户同步昵称时同样可能撞上别人的名字，排除自身后加后缀。
			user.UserName = w.uniqueUserName(googleUser.Name, user.Id)
		}
		if googleUser.GivenName != "" {
			user.RealName = googleUser.GivenName + " " + googleUser.FamilyName
		}
		if user.AvatarURL == "" {
			user.AvatarURL = googleUser.Picture
		}
		w.DB.Save(user)
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

	// 密码成本因子升级：旧密码使用低成本因子时自动升级
	cost, _ := bcrypt.Cost([]byte(user.Password))
	if cost < model.BcryptCost {
		user.EncryptPassword(req.Password)
		w.DB.Model(&user).UpdateColumn("password", user.Password)
	}

	user.Token = w.GetUserAuthToken(user.Id, true)
	_ = user.LogLogin(w.DB)
	// 如果要验证邮箱，并且没完成验证，则发送验证邮件
	template, tplExist := w.GetEmailTemplateInfo("register")
	if user.Email != "" && !user.EmailVerified && tplExist && template.Open {
		_ = w.SendVerifyEmail(&user, "verify")
	}
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
	attach, err := w.DownloadRemoteImage(avatarUrl, filePath, 0)
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

	if req.UserName != "" {
		exist, err := w.GetUserInfoByUserName(req.UserName)
		if err == nil && exist.Id != user.Id {
			return errors.New(w.Tr("TheUsernameHasBeenRegistered"))
		}
	}

	// if user.Phone != "" {
	// 	req.Phone = ""
	// }
	// if user.Email != "" {
	// 	req.Email = ""
	// }
	if req.Phone != nil {
		// if !w.VerifyCellphoneFormat(*req.Phone) {
		// 	return errors.New(w.Tr("IncorrectMobilePhoneNumber"))
		// }
		if *req.Phone != "" {
			exist, err := w.GetUserInfoByPhone(*req.Phone)
			if err == nil && exist.Id != user.Id {
				return errors.New(w.Tr("TheMobilePhoneNumberHasBeenRegistered"))
			}
		}
		user.Phone = *req.Phone
	}
	if req.Email != nil {
		if *req.Email != "" {
			if !w.VerifyEmailFormat(*req.Email) {
				return errors.New(w.Tr("IncorrectEmail"))
			}
			exist, err := w.GetUserInfoByEmail(*req.Email)
			if err == nil && exist.Id != user.Id {
				return errors.New(w.Tr("TheEmailHasBeenRegistered"))
			}
		}
		user.Email = *req.Email
	}
	if req.UserName != "" {
		user.UserName = req.UserName
	}
	if req.RealName != nil {
		user.RealName = *req.RealName
	}
	if req.FirstName != nil {
		user.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		user.LastName = *req.LastName
	}
	// 姓名是「姓 + 名」拼出来的展示名：只有当调用方真的传了这两项、
	// 且拼完后真实姓名仍为空时才补，不能因为 PATCH 没传就把已有的 RealName 冲掉。
	if req.FirstName != nil && req.LastName != nil && user.RealName == "" {
		user.RealName = *req.FirstName + " " + *req.LastName
	}
	if req.Introduce != nil {
		user.Introduce = *req.Introduce
	}
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
	// todo
}

// GetUserDiscount 获取用户优惠比例， 优先级：用户组优惠比例 > 通过分享链接下单优惠比例
func (w *Website) GetUserDiscount(userId uint, user *model.User) (discount, shippingDiscount int64) {
	if user == nil && userId > 0 {
		user, _ = w.GetUserInfoById(userId)
	}
	if user != nil {
		group, err := w.GetUserGroupInfo(user.GroupId)
		if err == nil {
			if group.Setting.Discount > 0 {
				discount = group.Setting.Discount
			}
			if group.Setting.ShippingDiscount > 0 {
				shippingDiscount = group.Setting.ShippingDiscount
			}
		}
		if user.ParentId > 0 {
			parent, err := w.GetUserInfoById(user.ParentId)
			if err == nil {
				group, err := w.GetUserGroupInfo(parent.GroupId)
				if err == nil {
					if group.Setting.ShareDiscount > 0 && group.Setting.ShareDiscount > discount {
						discount = group.Setting.ShareDiscount
					}
				}
			}
		}
	}

	return discount, shippingDiscount
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

func (w *Website) GetUserExtra(id uint) map[string]*config.CustomField {
	//读取extra
	result := map[string]interface{}{}
	// 从数据库中取出来
	extraFields := map[string]*config.CustomField{}
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
				} else if v.Type == config.CustomFieldTypeImages {
					// json 还原
					var images []string
					err := json.Unmarshal([]byte(value), &images)
					if err == nil {
						for i := range images {
							images[i] = w.ReplaceContentUrl(images[i], true)
						}
						result[v.FieldName] = images
					}
				} else if v.Type == config.CustomFieldTypeTexts {
					var texts []config.CustomFieldTexts
					err := json.Unmarshal([]byte(value), &texts)
					if err == nil {
						result[v.FieldName] = texts
					}
				} else if v.Type == config.CustomFieldTypeTimeline {
					var val config.TimelineField
					_ = json.Unmarshal([]byte(value), &val)
					result[v.FieldName] = val
				} else if v.Type == config.CustomFieldTypeArchive {
					var arcIds []int64
					err := json.Unmarshal([]byte(value), &arcIds)
					if err == nil {
						result[v.FieldName] = arcIds
					}
				} else if v.Type == config.CustomFieldTypeCategory || v.Type == config.CustomFieldTypeNumber {
					result[v.FieldName], _ = strconv.ParseInt(value, 10, 64)
				}
			}
			extraFields[v.FieldName] = &config.CustomField{
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
		"userId": fmt.Sprint(userId),
		"t":      fmt.Sprint(t.Unix()),
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
	_, err = w.UploadFile(fileName, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}

	// 更新用户头像地址
	w.DB.Model(&model.User{}).Where("`id` = ?", userId).UpdateColumn("avatar_url", fileName)

	// 返回头像地址
	return w.PluginStorage.StorageUrl + "/" + fileName, nil
}

// 处理新用户
func (w *Website) ProcessNewUser(user *model.User) {
	// 记录日志
	_ = w.ShopCountLog.Write(&request.LogStatisticRequest{Action: request.LogActionUser, Type: request.LogTypeUserCreate, Id: int64(user.Id)})

	// 同步到brevo
	client := w.BrevoClient("")
	if client != nil {
		_ = client.CreateContact(user)
	}
	// 新用户欢迎邮件
	w.SendWelcomeEmail(user)
}
