package provider

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
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
		"adminId": fmt.Sprint(userId),
		"t":       fmt.Sprint(t.Unix()),
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

// 后台免密跳转登录的票据。
//
// 旧实现把 admins.password 里的 bcrypt 哈希当共享密钥（sign = sha256(hash + nonce)），
// nonce 由调用方随意填写、永不过期。于是哈希一旦从任何途径泄露——例如排序参数被逐字符
// 拖出来——攻击者就得到永久免密登录，并且这条路径不经过验证码、错误锁定和 status 检查。
// 现在密钥是站点自己的 TokenSecret，票据短时效、一次性，且不可延长有效期。
const (
	AdminSSOTTL = 2 * time.Minute

	adminSSOPrefix  = "admin-sso"
	adminSSOSkew    = time.Minute
	adminSSOUsePref = "admin-sso-used-"
)

var (
	ErrAdminSSOExpired = errors.New("admin sso ticket expired")
	ErrAdminSSOInvalid = errors.New("admin sso ticket invalid")
	ErrAdminSSOUsed    = errors.New("admin sso ticket already used")
)

// MintAdminSSONonce 生成 "<过期时间戳>:<随机串>"，有效期由服务端决定。
func MintAdminSSONonce(ttl time.Duration) string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return fmt.Sprintf("%d:%s", time.Now().Add(ttl).Unix(), hex.EncodeToString(buf))
}

// SignAdminSSO 用目标站点的 TokenSecret 对票据签名。
func SignAdminSSO(tokenSecret, userName, nonce string) string {
	mac := hmac.New(sha256.New, []byte(tokenSecret))
	mac.Write([]byte(strings.Join([]string{adminSSOPrefix, userName, nonce}, "|")))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyAdminSSO 校验票据：签名必须来自本站 TokenSecret，nonce 未过期、未被拉长过
// 有效期，且在本进程内只用过一次。
func (w *Website) VerifyAdminSSO(userName, nonce, sign string) error {
	colon := strings.IndexByte(nonce, ':')
	if colon <= 0 || w.TokenSecret == "" {
		return ErrAdminSSOInvalid
	}
	expireAt, err := strconv.ParseInt(nonce[:colon], 10, 64)
	if err != nil {
		return ErrAdminSSOInvalid
	}

	now := time.Now()
	expire := time.Unix(expireAt, 0)
	if !expire.After(now) {
		return ErrAdminSSOExpired
	}
	// 只承认「刚签发」的票据，拿到密钥也不能签出长期有效的登录凭据
	if expire.After(now.Add(AdminSSOTTL + adminSSOSkew)) {
		return ErrAdminSSOInvalid
	}

	expected := SignAdminSSO(w.TokenSecret, userName, nonce)
	if !hmac.Equal([]byte(expected), []byte(sign)) {
		return ErrAdminSSOInvalid
	}

	// 用后即焚。Cache 没有原子写入，这里的重复提交窗口只有毫秒级，
	// 而提交者本来就必须持有本站密钥，因此只作为纵深防御。
	// key 取 nonce 的摘要：file 缓存会把 key 当文件名。
	marker := sha256.Sum256([]byte(nonce))
	usedKey := adminSSOUsePref + hex.EncodeToString(marker[:])
	var used string
	if w.Cache.Get(usedKey, &used) == nil {
		return ErrAdminSSOUsed
	}
	ttl := int64(time.Until(expire).Seconds())
	if ttl < 5 {
		ttl = 5
	}
	_ = w.Cache.Set(usedKey, "1", ttl)

	return nil
}
