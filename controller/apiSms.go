package controller

import (
	"errors"
	"log"
	"strings"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/provider/sms"
	"kandaoni.com/anqicms/request"
)

// respondSmsError 是前台短信端点的统一错误出口（R81/R82/R83/R144）。
// code 恒为 config.StatusFailed；msg 只暴露可翻译文案，绝不回显解析器/DB 原始措辞。
func respondSmsError(ctx iris.Context, site *provider.Website, err error) {
	var wait *sms.WaitError
	var attempts *sms.AttemptsError
	// 方法值：site.TplTr 内部兜底是 fmt.Sprintf(str, args...)，被 Go 1.24+ printf
	// vet 识别为 printf wrapper；对「无可变参数的非常量格式串」会报
	// non-constant format string。经方法值调用可规避，翻译语义与直接调用完全一致。
	translate := site.TplTr
	var msg string
	switch {
	case errors.As(err, &wait):
		// 仅 *WaitError 携带 %d 参数：把 Seconds 作为 TplTr 可变参数传入（R82）。
		msg = translate(err.Error(), wait.Seconds)
	case errors.As(err, &attempts):
		// 仅 *AttemptsError 携带 %d 参数：把 Remaining 传入（R82）。
		msg = translate(err.Error(), attempts.Remaining)
	case errors.Is(err, sms.ErrSmsSendFailed):
		// 平台原始错误码与消息只进 sms_logs，前台归约到通用翻译键（规格 §10.2 / R83）。
		msg = translate(sms.ErrSmsSendFailed.Error())
	default:
		// R144：字典即白名单。判据统一收敛到 provider.SmsBusinessMessage：
		// 内部固定用 TplTr（不做品牌替换），避免 OEM 构建下 Brandize 改写普通
		// 错误文本、被误判成语言键而把原文送出门（前台/后台判据必须一致）。
		// 查不到键的（ctx.ReadJSON 解析错误、DB 驱动错误）才是内部措辞：
		// 只进服务端日志，对外归约到 SmsSendFailed。
		if translated, ok := provider.SmsBusinessMessage(site, err); ok {
			msg = translated
		} else {
			log.Println("短信端点内部错误：", err)
			msg = translate(sms.ErrSmsSendFailed.Error())
		}
	}
	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  msg,
	})
}

// ApiSmsSend 处理 POST /api/sms/send（规格 §7.1）。
func ApiSmsSend(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ApiSmsSendRequest
	if err := ctx.ReadJSON(&req); err != nil {
		respondSmsError(ctx, currentSite, err)
		return
	}
	// 前台场景白名单：后台 test 场景与 notify. 通知命名空间一律拒（R83）。
	if req.Scene == sms.SceneTest || strings.HasPrefix(req.Scene, "notify.") {
		respondSmsError(ctx, currentSite, sms.ErrSmsSceneNotEnabled)
		return
	}
	// 图形验证码：Safe.Captcha == 1 时必须在 SendSmsCode 之前校验（简报步骤 1）。
	if currentSite.Safe.Captcha == 1 {
		if !Store.Verify(req.CaptchaId, req.Captcha, true) {
			respondSmsError(ctx, currentSite, errors.New("GraphicCodeIncorrect"))
			return
		}
	}
	// bind_old：要求登录态，且号码必须等于库内当前用户绑定号码，
	// 否则可以给任意号码发码，变成免费短信轰炸跳板（规格 §7.1 / R83）。
	// 拒绝原因不细分，避免被用来越权探测。
	if req.Scene == sms.SceneBindOld {
		userId := ctx.Values().GetUintDefault("userId", 0)
		if userId == 0 {
			respondSmsError(ctx, currentSite, sms.ErrSmsSceneNotEnabled)
			return
		}
		bound, err := currentSite.GetUserInfoById(userId)
		if err != nil {
			respondSmsError(ctx, currentSite, sms.ErrSmsSceneNotEnabled)
			return
		}
		reqPhone, err := sms.NormalizePhone(req.PhoneCountry, req.Phone)
		if err != nil {
			respondSmsError(ctx, currentSite, err)
			return
		}
		boundPhone, err := sms.NormalizePhone(bound.PhoneCountry, bound.Phone)
		if err != nil || boundPhone.Country != reqPhone.Country || boundPhone.Local != reqPhone.Local {
			respondSmsError(ctx, currentSite, sms.ErrSmsSceneNotEnabled)
			return
		}
	}
	// 发码：Ip 取 SmsClientIP（公网直连忽略伪造代理头，私有对端才采信 iris 解析），
	// 详见 provider.SmsClientIP 的注释。
	result, err := currentSite.SendSmsCode(provider.SmsSendInput{
		PhoneCountry: req.PhoneCountry,
		Phone:        req.Phone,
		Scene:        req.Scene,
		Ip:           provider.SmsClientIP(ctx),
		UserId:       ctx.Values().GetUintDefault("userId", 0),
	})
	if err != nil {
		respondSmsError(ctx, currentSite, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// ApiSmsPasswordReset 处理 POST /api/sms/password/reset（规格 §7.1 / 裁决 R93）。
// 独立端点，不搅动既有 POST /api/password/reset 的 email-token 逻辑。
// 顺序：ReadJSON → 密码 ≥6 位 → 归一化 → 校验 reset 场景码 → 按 (区号,号码) 查用户
// → EncryptPassword + Save（接 .Error）。
func ApiSmsPasswordReset(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ApiSmsPasswordResetRequest
	if err := ctx.ReadJSON(&req); err != nil {
		respondSmsError(ctx, currentSite, err)
		return
	}
	req.Password = strings.TrimSpace(req.Password)
	if len(req.Password) < 6 {
		// 沿用既有 ApiResetUserPassword 的键（R93）。
		respondSmsError(ctx, currentSite, errors.New("PleaseFillInAPasswordOfMoreThan6Digits"))
		return
	}
	p, err := sms.NormalizePhone(req.PhoneCountry, req.Phone)
	if err != nil {
		respondSmsError(ctx, currentSite, err)
		return
	}
	if err = currentSite.VerifySmsCode(p.Country, p.Local, sms.SceneReset, req.Code); err != nil {
		respondSmsError(ctx, currentSite, err)
		return
	}
	user, err := currentSite.GetUserInfoByPhoneCountry(p.Country, p.Local)
	if err != nil {
		// R93：查不到用户时与「错码第 5 次」经 respondSmsError 得到逐字相同的文案，
		// 且 errors.As 命中才会把 0 作为 %d 传入；不泄露注册状态，也绝不返回
		// 用户不存在类区分提示，更不能返回裸的错码哨兵（该键文案含 %d，无参会
		// 输出 %!(NOVERB)，违反 R82）。
		respondSmsError(ctx, currentSite, &sms.AttemptsError{Remaining: 0})
		return
	}
	if err = user.EncryptPassword(req.Password); err != nil {
		// 此刻验证码已在上面被消费、密码还没改：这不是「短信发送失败」。
		// 归约成 SmsSendFailed 会让用户去重发验证码，排查方向完全错。
		log.Println("短信重置密码加密失败：", err)
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.Tr("SaveFailed"),
		})
		return
	}
	if err = currentSite.DB.Save(user).Error; err != nil {
		log.Println("短信重置密码保存失败：", err)
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.Tr("SaveFailed"),
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
	})
}

// ApiSmsPhoneBind 处理 POST /api/sms/phone/bind（规格 §7.1 / 裁决 R94）。
// 由 route/base.go 的 middleware.UserAuth 保护；userId 取登录态，为 0 直接拒。
// 错误统一走 respondSmsError，成功只回 {code, msg}，不回显号码。
func ApiSmsPhoneBind(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ApiSmsPhoneBindRequest
	if err := ctx.ReadJSON(&req); err != nil {
		respondSmsError(ctx, currentSite, err)
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)
	if userId == 0 {
		respondSmsError(ctx, currentSite, sms.ErrSmsSceneNotEnabled)
		return
	}
	if err := currentSite.BindUserPhone(userId, req.OldCode, req.NewPhoneCountry, req.NewPhone, req.NewCode); err != nil {
		respondSmsError(ctx, currentSite, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
	})
}
