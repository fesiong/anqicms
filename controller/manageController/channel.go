package manageController

import (
	"errors"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// 渠道（企业 IM 接入）后台管理接口。
//
// 分层（R81）：本文件只做取参、调用 provider、统一 {code,msg,data} 输出。
// 密钥掩码、站点隔离、乐观锁与删除级联都在 provider/channelAdmin.go——
// 控制器里一旦出现 AppSecret 或模型行，就说明有一条响应绕过了投影（护栏见 channel_test.go）。
//
// 路由挂在 middleware.AdminPermissionRequired 下（route/manage.go）：
// 渠道端点不沿用后台「未知菜单默认放行」的历史行为。

// ChannelList 当前站点渠道连接列表（密钥掩码）。
func ChannelList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	list, err := currentSite.ChannelAdminList()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": list,
	})
}

// ChannelDetail 渠道详情与策略。
//
// 参数说明：
//   - 查询参数 "id": 渠道ID
func ChannelDetail(ctx iris.Context) {
	id := ctx.URLParamIntDefault("id", 0) // 渠道ID
	if !requirePositive(ctx, id, "id") {
		return
	}
	currentSite := provider.CurrentSite(ctx)
	view, err := currentSite.ChannelAdminDetail(uint(id))
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": view,
	})
}

// ChannelDetailForm 新建或更新渠道连接（id 为 0 时新建，更新需带 revision 乐观锁）。
func ChannelDetailForm(ctx iris.Context) {
	var req request.Channel
	if !readJSONPayload(ctx, &req) {
		return
	}
	currentSite := provider.CurrentSite(ctx)
	result, err := currentSite.ChannelAdminSave(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
		"data": result,
	})
}

// ChannelDelete 关闭并移除渠道，取消其未执行的工作（审计与投递历史保留）。
func ChannelDelete(ctx iris.Context) {
	var req request.ChannelDeleteRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	if !requirePositive(ctx, req.Id, "id") {
		return
	}
	currentSite := provider.CurrentSite(ctx)
	if err := currentSite.ChannelAdminDelete(req.Id); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已删除",
	})
}

// ChannelStatus 渠道连接与工作者状态。
//
// 参数说明：
//   - 查询参数 "id": 渠道ID
func ChannelStatus(ctx iris.Context) {
	id := ctx.URLParamIntDefault("id", 0) // 渠道ID
	if !requirePositive(ctx, id, "id") {
		return
	}
	currentSite := provider.CurrentSite(ctx)
	view, err := currentSite.ChannelAdminStatus(uint(id))
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": view,
	})
}

// ChannelCapabilities 能力、动作、权限映射、风险及当前可配置状态。
//
// 参数说明：
//   - 查询参数 "agent_id": 可选，接待智能体ID；传入时额外给出该智能体是否被授权（agent_allowed）
func ChannelCapabilities(ctx iris.Context) {
	agentId := ctx.URLParamIntDefault("agent_id", 0) // 接待智能体ID（可选）
	currentSite := provider.CurrentSite(ctx)
	view, err := currentSite.ChannelCapabilities(uint(agentId))
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": view,
	})
}

// ============================================================
// T9b：绑定、目标、规则、模板、记录、试发、投递重试
// ============================================================

// channelJSONError 统一失败响应：权限类错误映射为 1002，其余为 -1。
// 所有文案都来自 provider 的静态错误，控制器不拼接驱动原文。
func channelJSONError(ctx iris.Context, err error) {
	code := config.StatusFailed
	if errors.Is(err, provider.ErrChannelForbidden) {
		code = config.StatusNoAccess
	}
	ctx.JSON(iris.Map{
		"code": code,
		"msg":  err.Error(),
	})
}

// ChannelBindingToken 为当前登录管理员生成指定渠道的一次性绑定 token。
//
// 明文只在本响应里出现一次：响应显式声明 no-store，避免被中间层缓存（R89）。
func ChannelBindingToken(ctx iris.Context) {
	var req request.ChannelBindingTokenRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	if !requirePositive(ctx, req.ChannelId, "channel_id") {
		return
	}
	adminId := ctx.Values().GetUintDefault("adminId", 0)
	currentSite := provider.CurrentSite(ctx)
	plain, expiresAt, err := currentSite.CreateBindingToken(req.ChannelId, adminId)
	if err != nil {
		channelJSONError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"token":      plain,
			"expires_at": expiresAt,
		},
	})
}

// ChannelBindings 绑定列表：默认只返回本人；all=1 需要超级管理员（R89）。
func ChannelBindings(ctx iris.Context) {
	all := ctx.URLParamIntDefault("all", 0) == 1 // 1=查看全部（需超级管理员）
	adminId := ctx.Values().GetUintDefault("adminId", 0)
	currentSite := provider.CurrentSite(ctx)
	list, err := currentSite.ChannelBindings(adminId, all)
	if err != nil {
		channelJSONError(ctx, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": list,
	})
}

// ChannelBindingRevoke 解除绑定：本人可解自己的，撤销他人绑定需要超级管理员（R89）。
func ChannelBindingRevoke(ctx iris.Context) {
	var req request.ChannelBindingRevokeRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	if !requirePositive(ctx, req.Id, "id") {
		return
	}
	adminId := ctx.Values().GetUintDefault("adminId", 0)
	currentSite := provider.CurrentSite(ctx)
	if err := currentSite.ChannelBindingRevoke(req.Id, adminId); err != nil {
		channelJSONError(ctx, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已解绑",
	})
}

// ChannelTargets 渠道通知目标列表。
//
// 参数说明：
//   - 查询参数 "channel_id": 渠道ID
func ChannelTargets(ctx iris.Context) {
	channelId := ctx.URLParamIntDefault("channel_id", 0) // 渠道ID
	if !requirePositive(ctx, channelId, "channel_id") {
		return
	}
	currentSite := provider.CurrentSite(ctx)
	list, err := currentSite.ChannelTargets(uint(channelId))
	if err != nil {
		channelJSONError(ctx, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": list,
	})
}

// ChannelTargetSave 新建/更新群目标；私聊目标只能由绑定流程创建（R90）。
func ChannelTargetSave(ctx iris.Context) {
	var req request.ChannelTargetRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	if !requirePositive(ctx, req.ChannelId, "channel_id") {
		return
	}
	currentSite := provider.CurrentSite(ctx)
	view, err := currentSite.ChannelTargetSave(&req)
	if err != nil {
		channelJSONError(ctx, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
		"data": view,
	})
}

// ChannelRules 通知规则列表。
func ChannelRules(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	list, err := currentSite.ChannelRules()
	if err != nil {
		channelJSONError(ctx, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": list,
	})
}

// ChannelRuleSave 新建/更新通知规则（R91）。
func ChannelRuleSave(ctx iris.Context) {
	var req request.ChannelRuleRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	currentSite := provider.CurrentSite(ctx)
	view, err := currentSite.ChannelRuleSave(&req)
	if err != nil {
		channelJSONError(ctx, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
		"data": view,
	})
}

// ChannelRuleDelete 删除通知规则（R91）。
func ChannelRuleDelete(ctx iris.Context) {
	var req request.ChannelRuleDeleteRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	if !requirePositive(ctx, req.Id, "id") {
		return
	}
	currentSite := provider.CurrentSite(ctx)
	if err := currentSite.ChannelRuleDelete(req.Id); err != nil {
		channelJSONError(ctx, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已删除",
	})
}

// ChannelTemplates 通知模板列表。
func ChannelTemplates(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	list, err := currentSite.ChannelTemplates()
	if err != nil {
		channelJSONError(ctx, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": list,
	})
}

// ChannelTemplateSave 新建/更新通知模板（R92）。
func ChannelTemplateSave(ctx iris.Context) {
	var req request.ChannelTemplateRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	currentSite := provider.CurrentSite(ctx)
	view, err := currentSite.ChannelTemplateSave(&req)
	if err != nil {
		channelJSONError(ctx, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
		"data": view,
	})
}

// ChannelTemplatePreview 按目标安全级别预览模板；不落库（R92）。
func ChannelTemplatePreview(ctx iris.Context) {
	var req request.ChannelTemplatePreviewRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	currentSite := provider.CurrentSite(ctx)
	view, err := currentSite.ChannelTemplatePreview(&req)
	if err != nil {
		channelJSONError(ctx, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": view,
	})
}

// ChannelRecords 运行记录分页（投递/入站任务/审批/审计），秘密字段已投影剔除（R93）。
//
// 参数说明：
//   - 查询参数 "type": delivery / inbox / approval / audit（必填）
//   - 查询参数 "status": 可选状态筛选
//   - 查询参数 "page": 页码，默认 1
//   - 查询参数 "page_size": 每页条数，默认 20，上限 100
func ChannelRecords(ctx iris.Context) {
	recordType := ctx.URLParam("type") // 记录类型
	status := ctx.URLParam("status")   // 状态筛选
	page := ctx.URLParamIntDefault("page", 1)
	pageSize := ctx.URLParamIntDefault("page_size", 20)
	currentSite := provider.CurrentSite(ctx)
	result, err := currentSite.ChannelRecords(recordType, status, page, pageSize)
	if err != nil {
		channelJSONError(ctx, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// ChannelTestSend 验证渠道配置，或向已批准目标发送固定示例消息（R94）。
func ChannelTestSend(ctx iris.Context) {
	var req request.ChannelTestSendRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	if !requirePositive(ctx, req.Id, "id") {
		return
	}
	currentSite := provider.CurrentSite(ctx)
	result, err := currentSite.ChannelTestSend(req.Id, req.TargetId)
	if err != nil {
		channelJSONError(ctx, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// ChannelDeliveryRetry 对失败/结果未知的投递显式重试（R95）。
func ChannelDeliveryRetry(ctx iris.Context) {
	var req request.ChannelDeliveryRetryRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	if !requirePositive(ctx, req.Id, "id") {
		return
	}
	currentSite := provider.CurrentSite(ctx)
	if err := currentSite.ChannelDeliveryRetry(req.Id); err != nil {
		channelJSONError(ctx, err)
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已重新排队",
	})
}
