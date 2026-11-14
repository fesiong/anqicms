package provider

import (
	"crypto/rand"
	"errors"
	"math/big"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/sms"
	"kandaoni.com/anqicms/request"
)

// 自动注册合成字段口径（规格 §6.5 / 裁决 R84）：
// UserName 随机 12 位（不用手机号，避免号码在部分模板里被公开显示），
// Password 随机 32 位（用户不可知，后续通过短信找回密码设置真实密码），
// 撞名重试至多 3 次。随机串一律用 crypto/rand，不用 math/rand。
const (
	smsLoginUserNameLen = 12
	smsLoginPasswordLen = 32
	smsLoginMaxRetry    = 3
)

const smsLoginAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// smsLoginRandom 用 crypto/rand 生成定长字母数字随机串（R84）。
func smsLoginRandom(n int) (string, error) {
	out := make([]byte, n)
	for i := range out {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(smsLoginAlphabet))))
		if err != nil {
			return "", err
		}
		out[i] = smsLoginAlphabet[idx.Int64()]
	}
	return string(out), nil
}

// LoginViaSms 手机号 + 短信验证码登录：未注册则自动注册（规格 §6.5 / §7.1）。
//
// 判定顺序不可换：先归一化 → 再校验验证码（在建号之前，错码不得触发自动注册）
// → 再按 (区号, 号码) 查号 → 已存在则过审核/封禁门 → 不存在才合成请求调
// RegisterUser。成功路径与密码登录同形（GetUserAuthToken + LogLogin）。
func (w *Website) LoginViaSms(req *request.ApiLoginRequest) (*model.User, error) {
	// 1. 号码归一化（失败原样返回 ErrSmsPhoneInvalid）。
	phone, err := sms.NormalizePhone(req.PhoneCountry, req.Phone)
	if err != nil {
		return nil, err
	}
	// 2. 校验验证码（scene=login，一次性消费）；必须在建号之前。
	if err := w.VerifySmsCode(phone.Country, phone.Local, sms.SceneLogin, req.Code); err != nil {
		return nil, err
	}
	// 3. 按 (区号, 号码) 查号。
	user, err := w.GetUserInfoByPhoneCountry(phone.Country, phone.Local)
	if err == nil {
		// 已存在：不得绕过站点审核/封禁策略。返回可翻译键错误（R81），
		// 与 controller/apiUser.go 既有 user.Status 兜底同键，前台 TplTr 可翻译。
		if user.Status != config.UserStatusActive {
			return nil, errors.New("UserNotActive")
		}
		user.Token = w.GetUserAuthToken(user.Id, true)
		_ = user.LogLogin(w.DB)
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		// DB 故障不能当作「用户不存在」，否则会误建号。
		return nil, err
	}

	// 4. 不存在 → 自动注册（复用 RegisterUser，不另写建号逻辑）。
	for i := 0; i < smsLoginMaxRetry; i++ {
		userName, e := smsLoginRandom(smsLoginUserNameLen)
		if e != nil {
			return nil, e
		}
		password, e := smsLoginRandom(smsLoginPasswordLen)
		if e != nil {
			return nil, e
		}
		regReq := &request.ApiRegisterRequest{
			UserName:        userName,
			Password:        password,
			Phone:           phone.Local,
			PhoneCountry:    phone.Country,
			PhoneNormalized: true, // R85：sms.NormalizePhone 已严格校验，RegisterUser 据此跳过老正则
		}
		newUser, e := w.RegisterUser(regReq)
		if e != nil {
			// R88：只有该随机 UserName 确实撞名才换名重试；其余失败
			// （DB 故障、号码非法、guard 查重命中、审核策略）立即原样返回 e，
			// 绝不吞成 LoginFailed 后再重试 3 次。
			if !w.smsLoginIsNameCollision(userName) {
				return nil, e
			}
			continue
		}
		// RegisterUser 的撞名分支可能返回一个已存在的同名用户；只接受号码与区号
		// 都与本次入参完全一致的新用户，否则视为撞名重试，绝不把别人的账号签发 token。
		// R86：区号由 RegisterUser 直接落库（PhoneCountry: req.PhoneCountry），不再
		// 事后 Update，故此处必须同时校验区号；漏掉区号错配会签发错区号账号。
		if newUser == nil || newUser.Phone != phone.Local || newUser.PhoneCountry != phone.Country {
			continue
		}
		// 审核门：注册后仍要过 GetDefaultStatus()。站点默认待审核/禁止时，
		// 自动注册后必须拒绝登录（规格 §6.5），不得绕过站点审核策略。
		if newUser.Status != config.UserStatusActive ||
			w.PluginUser.GetDefaultStatus() != config.UserStatusActive {
			return nil, errors.New("UserNotActive")
		}
		// 成功路径与密码登录同形。
		newUser.Token = w.GetUserAuthToken(newUser.Id, true)
		_ = newUser.LogLogin(w.DB)
		return newUser, nil
	}
	return nil, errors.New("LoginFailed")
}

// smsLoginIsNameCollision 判定一次注册失败是否属于「随机 UserName 撞名」。
// R88：只有 GetUserInfoByUserName 确实查得到该用户名才算撞名、才换名重试；
// 其余失败（DB 故障、号码非法、guard 查重命中、审核策略）一律由调用方原样
// 返回 e，不得被吞成 LoginFailed 后再重试 3 次。
func (w *Website) smsLoginIsNameCollision(userName string) bool {
	_, err := w.GetUserInfoByUserName(userName)
	return err == nil
}
