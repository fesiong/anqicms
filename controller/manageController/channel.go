package manageController

import (
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
