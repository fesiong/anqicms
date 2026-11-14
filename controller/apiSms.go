package controller

import (
	"errors"
	"strings"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/provider/sms"
	"kandaoni.com/anqicms/request"
)

// respondSmsError 是前台短信端点的统一错误出口（R81/R82/R83）。
// code 恒为 config.StatusFailed；msg 只暴露可翻译键，绝不回显平台原始错误码/消息。
func respondSmsError(ctx iris.Context, site *provider.Website, err error) {
	// 平台原始错误码与消息只进 sms_logs，前台归约到通用翻译键（规格 §10.2 / R83）。
	if errors.Is(err, sms.ErrSmsSendFailed) {
		err = sms.ErrSmsSendFailed
	}
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
	default:
		// 其余错误一律不传参：对不含格式化指令的键传参会输出 %!(EXTRA int=...)（R82）。
		msg = translate(err.Error())
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
	// 发码：Ip 取 ctx.RemoteAddr()，不自己解析 X-Forwarded-For（R83）。
	result, err := currentSite.SendSmsCode(provider.SmsSendInput{
		PhoneCountry: req.PhoneCountry,
		Phone:        req.Phone,
		Scene:        req.Scene,
		Ip:           ctx.RemoteAddr(),
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
