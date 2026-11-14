package provider

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/sms"
)

// SmsSendInput 是发码请求（任务 9／10／11／13 依赖的接口）。
type SmsSendInput struct {
	PhoneCountry, Phone string
	Scene               string
	Ip                  string
	UserId              uint
}

// SmsSendResult 只有 ExpiresIn／ResendAfter 两个字段：绝不返回验证码本身（规格 §6.3）。
type SmsSendResult struct {
	ExpiresIn   int `json:"expires_in"`
	ResendAfter int `json:"resend_after"`
}

// 短信流水／验证码状态机取值，逐字来自规格 §4.2／§4.3 与 model/sms.go 注释：
//
//	0 pending（第 7e 步先落 pending）／1 success（第 9 步回写）／-1 failed（第 10 步作废）。
//
// config.StatusOK(=0) 承载 pending、config.StatusFailed(=-1) 承载 failed；
// success=1 在 config 中没有同值常量（StatusOK 是 0），以字面量表达，详见报告「疑虑」。
const (
	smsPending = config.StatusOK
	smsSuccess = 1
	smsFailed  = config.StatusFailed
)

// SendSmsCode 是生产入口：用 sms.NewSender 构造平台发送器后转调测试接缝（R62）。
func (w *Website) SendSmsCode(in SmsSendInput) (*SmsSendResult, error) {
	if w.PluginSms == nil || w.PluginSms.Provider == "" {
		return nil, sms.ErrSmsServiceNotEnabled
	}
	sender, err := sms.NewSender(w.PluginSms.Provider)
	if err != nil {
		// 未知服务商按未启用处理，fail closed（与 sms.Resolve 的 default 分支一致）。
		return nil, sms.ErrSmsServiceNotEnabled
	}
	return w.sendSmsCodeWithSender(in, sender)
}

// sendSmsCodeWithSender 是测试接缝：测试传假 sender，全程不触网、不发真实短信（R62）。
func (w *Website) sendSmsCodeWithSender(in SmsSendInput, sender sms.Sender) (*SmsSendResult, error) {
	cfg := w.PluginSms
	if cfg == nil {
		return nil, sms.ErrSmsServiceNotEnabled
	}
	// 站点内存里的全局对象必须先值拷贝再 ApplyDefaults：测试可能直接构造未填默认值的 cfg。
	snapshot := *cfg
	sms.ApplyDefaults(&snapshot)

	// 第 1 步：Provider 非空？
	if snapshot.Provider == "" {
		return nil, sms.ErrSmsServiceNotEnabled
	}

	// 第 2 步：scene 白名单（后台 test 场景跳过此步）。
	if in.Scene != sms.SceneTest {
		allowed := false
		for _, s := range snapshot.Scenes {
			if s == in.Scene {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, sms.ErrSmsSceneNotEnabled
		}
	}

	// 第 3 步：号码归一化 + 格式校验（失败 → ErrSmsPhoneInvalid）。
	phone, err := sms.NormalizePhone(in.PhoneCountry, in.Phone)
	if err != nil {
		return nil, err
	}
	region := phone.Region()

	// 第 4 步：Safe.Captcha 的图形验证码校验由 controller 层（任务 10）承担，
	// provider 不 import controller / middleware（前者成环，后者污染邮箱枚举探测预算）。

	// 第 5 步：IP 滑窗（小时 / 日），超限 → ErrSmsIpLimited。
	budget := smsIpBudgetFor(w.Id, snapshot.HourlyLimitPerIp, snapshot.DailyLimitPerIp)
	if !budget.Allow(in.Ip) {
		return nil, sms.ErrSmsIpLimited
	}

	// 第 6 步：区域解析与凭据继承（Twilio+86 / 缺凭据 / 缺签名 / 场景模板空 → 对应错误）。
	profile, err := sms.Resolve(&snapshot, region, in.Scene)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	nowUnix := now.Unix()
	dayStart := smsDayStart(now)

	// 事务落库后带出：明文码（只用于构造平台请求，绝不入库）、两条记录 Id。
	var plainCode string
	var codeId, logId uint

	// 第 7 步：DB 事务（不做任何网络 IO）。a→f 顺序不可换。
	err = w.DB.Transaction(func(tx *gorm.DB) error {
		// a. sms_logs 当日同号码计数（跨场景合计，不带 scene 条件）。
		var todayCount int64
		if e := tx.Model(&model.SmsLog{}).
			Where("site_id = ? AND phone_country = ? AND phone = ? AND created_time >= ?",
				w.Id, phone.Country, phone.Local, dayStart).
			Count(&todayCount).Error; e != nil {
			return e
		}
		if todayCount >= int64(snapshot.DailyLimitPerPhone) {
			return sms.ErrSmsPhoneLimited
		}

		// b. sms_codes 最新活跃行重发间隔（带 scene，按场景）。
		var latest model.SmsCode
		res := tx.Where("site_id = ? AND phone_country = ? AND phone = ? AND scene = ? AND consumed_at = 0 AND expires_at > ?",
			w.Id, phone.Country, phone.Local, in.Scene, nowUnix).
			Order("created_time DESC").First(&latest)
		if res.Error == nil {
			remain := latest.CreatedTime + int64(snapshot.ResendInterval) - nowUnix
			if remain > 0 {
				return &sms.WaitError{Seconds: int(remain)}
			}
		} else if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return res.Error
		}

		// c. 删除同 (site, country, phone, scene) 的已过期 / 已消费行。
		if e := tx.Where("site_id = ? AND phone_country = ? AND phone = ? AND scene = ? AND (expires_at <= ? OR consumed_at > 0)",
			w.Id, phone.Country, phone.Local, in.Scene, nowUnix).
			Delete(&model.SmsCode{}).Error; e != nil {
			return e
		}

		// d. crypto/rand 生成码（不用 math/rand）→ HMAC → 插 sms_codes(send_status=0)。
		code, e := smsGenerateCode(snapshot.CodeLength)
		if e != nil {
			return e
		}
		plainCode = code
		codeRow := &model.SmsCode{
			SiteId:       w.Id,
			PhoneCountry: phone.Country,
			Phone:        phone.Local,
			Scene:        in.Scene,
			CodeHash:     w.smsCodeHash(code),
			MaxAttempts:  snapshot.MaxVerifyAttempts,
			ExpiresAt:    nowUnix + int64(snapshot.CodeTTL),
			Provider:     snapshot.Provider,
			Region:       region,
			SendStatus:   smsPending,
			CreatedTime:  nowUnix,
		}
		if e := tx.Create(codeRow).Error; e != nil {
			return e
		}
		codeId = codeRow.Id

		// e. 插入 sms_logs(status=0 pending)：先落 pending 再调平台，进程崩了也有痕迹。
		logRow := &model.SmsLog{
			SiteId:       w.Id,
			PhoneCountry: phone.Country,
			Phone:        phone.Local,
			Scene:        in.Scene,
			Provider:     snapshot.Provider,
			Region:       region,
			TemplateId:   profile.TemplateId,
			SignName:     profile.SignName,
			Status:       smsPending,
			Ip:           in.Ip,
			UserId:       in.UserId,
			CreatedTime:  nowUnix,
		}
		if e := tx.Create(logRow).Error; e != nil {
			return e
		}
		logId = logRow.Id

		// f. 回写 sms_codes.log_id。
		if e := tx.Model(&model.SmsCode{}).Where("id = ?", codeRow.Id).
			Update("log_id", logRow.Id).Error; e != nil {
			return e
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 第 8 步：事务外调 Sender.Send（context.WithTimeout 10s，不重试）。
	// sender.Send 必须在 w.DB.Transaction 回调之外：持锁事务里做网络 IO 会按住
	// 行锁数秒，同一号码的并发请求全部串行化，平台一慢就雪崩（R62 / 规格 §6.3）。
	req := sms.SendRequest{
		To:    phone.E164(), // 复用 sms.NormalizePhone 的结果（R61）
		Scene: in.Scene,
		Code:  plainCode,                // 验证码，唯一模板变量
		OutId: fmt.Sprintf("%d", logId), // 关联 SmsLog.Id，透传平台做对账
	}
	// Twilio 走本地文案 Texts[scene] 渲染 {{code}} → SendRequest.Text（Body）；
	// 阿里云／腾讯云走模板 + Code 字段，不渲染正文。
	if snapshot.Provider == config.SmsProviderTwilio {
		req.Text = strings.ReplaceAll(profile.TemplateId, "{{code}}", plainCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sendStart := time.Now()
	result, sendErr := sender.Send(ctx, *profile, req)
	costMs := int(time.Since(sendStart) / time.Millisecond)
	if costMs < 1 {
		costMs = 1 // 瞬时假 sender 也留 1ms，避免成本记录恒为 0
	}

	// 第 9 步：回写两张表；第 10 步：平台失败置 send_status=-1（该码作废）。
	var finalErr error
	if sendErr != nil {
		var se *sms.SendError
		errCode := "SmsSendFailed"
		errMsg := smsErrSnippet("发送失败")
		retryable := 0
		// R61：必须用 errors.As 取 *SendError 的单行 Code／Message，绝不用 err.Error()
		//（errors.Join 的 Error() 是两行；err.Error() 还会混入哨兵键名）。
		// err_msg / biz_id 只写平台元数据，绝不写验证码明文或渲染后的正文。
		if errors.As(sendErr, &se) {
			errCode = se.Code
			errMsg = smsErrSnippet(se.Message)
			if se.Class == sms.ClassTransient {
				retryable = 1
			}
		}
		if e := w.DB.Model(&model.SmsLog{}).Where("id = ?", logId).Updates(map[string]interface{}{
			"status":    smsFailed,
			"err_code":  errCode,
			"err_msg":   errMsg,
			"retryable": retryable,
			"cost_ms":   costMs,
			"biz_id":    result.BizId,
		}).Error; e != nil {
			return nil, e
		}
		// 第 10 步：发送失败的码若仍可用，用户会拿着一个永远收不到的码干等；置 -1 作废。
		if e := w.DB.Model(&model.SmsCode{}).Where("id = ?", codeId).
			Update("send_status", smsFailed).Error; e != nil {
			return nil, e
		}
		finalErr = sendErr
	} else {
		if e := w.DB.Model(&model.SmsLog{}).Where("id = ?", logId).Updates(map[string]interface{}{
			"status":  smsSuccess,
			"biz_id":  result.BizId,
			"cost_ms": costMs,
		}).Error; e != nil {
			return nil, e
		}
		if e := w.DB.Model(&model.SmsCode{}).Where("id = ?", codeId).Updates(map[string]interface{}{
			"send_status": smsSuccess,
			"biz_id":      result.BizId,
		}).Error; e != nil {
			return nil, e
		}
	}

	// 第 11 步：惰性清理超期日志（单次 ≤ 1000 行，避免长事务）。
	// 清理失败不阻断本次发码结果（下次发码会再试），故忽略其 error，但仍接 .Error。
	cutoff := nowUnix - int64(snapshot.LogRetentionDays)*86400
	_ = w.DB.Where("site_id = ? AND created_time < ?", w.Id, cutoff).
		Limit(1000).Delete(&model.SmsLog{}).Error

	if finalErr != nil {
		return nil, finalErr
	}
	return &SmsSendResult{ExpiresIn: snapshot.CodeTTL, ResendAfter: snapshot.ResendInterval}, nil
}

// smsDayStart 返回当天零点（本地时区）的 Unix 秒，用于日限「当日」口径。
func smsDayStart(now time.Time) int64 {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
}

// smsGenerateCode 用 crypto/rand 生成定长数字验证码（不用 math/rand）。
func smsGenerateCode(length int) (string, error) {
	const digits = "0123456789"
	out := make([]byte, length)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		out[i] = digits[n.Int64()]
	}
	return string(out), nil
}
