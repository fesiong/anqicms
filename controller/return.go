package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/provider"
	"strings"
)

func PaypalReturnResult(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	terraceId := strings.TrimSpace(ctx.URLParam("token"))
	if terraceId == "" {
		ShowMessage(ctx, ctx.Tr("paymentParameterError"), nil)
		return
	}

	payment, err := currentSite.GetPaymentInfoByTerraceId(terraceId)
	if err != nil {
		ShowMessage(ctx, ctx.Tr("paymentParameterError")+" "+err.Error(), nil)
		return
	}

	if payment.PaidTime > 0 {
		// 已经支付过了
		ShowMessage(ctx, ctx.Tr("paymentSuccessful"), nil)
		return
	}

	// query order detail first
	err = currentSite.TraceQuery(payment)
	if payment.PaidTime > 0 {
		// 支付成功
		ShowMessage(ctx, ctx.Tr("paymentSuccessful"), nil)
	} else {
		ShowMessage(ctx, ctx.Tr("paymentResultError")+" "+err.Error(), nil)
		return
	}
}

func PaypalCancelResult(ctx iris.Context) {

	ShowMessage(ctx, ctx.Tr("paymentCanceled"), nil)
}
