package provider

import (
	"context"
	"fmt"

	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// channelPermissions.go 是渠道聊天的授权层，只认三个来源：
//  1. 站点暴露面：ExposedIntents / ExposedTools，判定只走 intent.IntentAllowed；
//  2. api_exposure：在 api_invoke 能力内部按最终端点判定，不在本层重复实现；
//  3. 身份与生命周期：渠道启用/聊天启用、接待智能体、绑定有效性、管理员状态。
//
// 设计要点：
//   - 模型可见面与执行前拦截同时存在：只靠提示词约束等于没约束。
//   - 后台登录（middleware.AdminPermission）对「不在菜单里的路径」是放行的，
//     渠道没有后台菜单概念，也不再做任何菜单前缀映射。
//   - 判定不写回任何服务级状态：所有门禁按请求读库，执行主体由调用方逐层传入。

// checkChannelPrincipal 校验执行主体本身的完整性（不读库）。
func (w *Website) checkChannelPrincipal(p ExecutionPrincipal) error {
	if w == nil || w.Id == 0 {
		return fmt.Errorf("%w：站点未初始化", ErrChannelPrincipalInvalid)
	}
	if p.SiteId == 0 || p.SiteId != w.Id {
		return fmt.Errorf("%w：执行主体站点与当前站点不一致", ErrChannelPrincipalInvalid)
	}
	if p.ChannelId == 0 || p.BindingId == 0 || p.AdminId == 0 {
		return fmt.Errorf("%w：渠道/绑定/管理员标识缺失", ErrChannelPrincipalInvalid)
	}
	return nil
}

// channelForPrincipal 取执行主体对应的渠道（站点内），并校验渠道处于「可管理」状态。
//
// 三道判定：
//  1. 渠道存在且属于当前站点；
//  2. enabled = 1（连接启用）且 chat_enabled = 1（开启聊天管理）。仅通知渠道
//     （chat_enabled = 0）只投递通知，不该执行任何管理动作。Enabled 虽然由 T4 的
//     worker 生命周期兜住，但租约与 worker 是「连接层」，权限层不能把 fail-closed
//     寄托在另一层没漏判上——这是本层最合适的拒绝点；
//  3. 执行主体声明的智能体必须非 0 且与渠道配置的接待智能体一致。
func (w *Website) channelForPrincipal(ctx context.Context, p ExecutionPrincipal) (*model.MessageChannel, error) {
	var ch model.MessageChannel
	err := w.DB.WithContext(ctx).
		Where("`id` = ? AND `site_id` = ?", p.ChannelId, w.Id).
		Take(&ch).Error
	if err != nil {
		return nil, fmt.Errorf("%w：渠道不存在或不属于当前站点", ErrChannelPrincipalInvalid)
	}
	if ch.Enabled != 1 || ch.ChatEnabled != 1 {
		return nil, fmt.Errorf("%w", ErrChannelDisabled)
	}
	if p.AgentId == 0 || p.AgentId != ch.AgentId {
		return nil, fmt.Errorf("%w", ErrChannelAgentNotAllowed)
	}
	return &ch, nil
}

// channelExposureConfig 把站点自己的 MCP 配置转成内核可用的曝光配置。
//
// 必须用 w.LoadAiSetting("").Mcp，而不是 provider/mcp.go 的 GetMcpConfig()——
// 后者只返回主站点配置，多站点场景下等于让所有站点共用主站点的暴露面（R32）。
func (w *Website) channelExposureConfig() intent.Config {
	if w == nil {
		return intent.Config{}
	}
	mcp := w.LoadAiSetting("").Mcp
	return intent.Config{
		ExposedIntents: mcp.ExposedIntents,
		ExposedTools:   mcp.ExposedTools,
	}
}

// checkChannelIdentity 校验绑定仍有效、管理员仍存在且启用。
//
// 这是渠道唯一的「人」的判定：没有后台菜单权限匹配，也没有接待白名单。
// 错误文案不带管理员内部 ID。
func (w *Website) checkChannelIdentity(ctx context.Context, p ExecutionPrincipal) error {
	// 绑定是执行主体的身份凭据：撤销、换绑、换渠道后必须立刻失效（规格 §6）。
	// 放在这里而不是只信 principal 里的 AdminId——principal 是调用方造的，
	// 绑定可能在主体构造之后才被撤销。
	if err := w.checkChannelBinding(ctx, p); err != nil {
		return err
	}
	admin, err := w.GetAdminInfoById(p.AdminId)
	if err != nil || admin == nil || admin.Status != 1 {
		return fmt.Errorf("%w", ErrChannelAdminInvalid)
	}
	return nil
}

// checkChannelCall 是每次 cap 调用前的完整门禁：主体 → 渠道 → 身份。
func (w *Website) checkChannelCall(ctx context.Context, p ExecutionPrincipal) error {
	if err := w.checkChannelPrincipal(p); err != nil {
		return err
	}
	if _, err := w.channelForPrincipal(ctx, p); err != nil {
		return err
	}
	return w.checkChannelIdentity(ctx, p)
}

// checkChannelBinding 校验绑定仍有效且属于当前站点/渠道/管理员。
func (w *Website) checkChannelBinding(ctx context.Context, p ExecutionPrincipal) error {
	var binding model.ChannelBinding
	err := w.DB.WithContext(ctx).
		Where("`id` = ? AND `site_id` = ? AND `channel_id` = ? AND `admin_id` = ? AND `revoked_at` = 0",
			p.BindingId, w.Id, p.ChannelId, p.AdminId).
		Take(&binding).Error
	if err != nil {
		return fmt.Errorf("%w：绑定已撤销、已换绑或不属于该渠道", ErrChannelPrincipalInvalid)
	}
	return nil
}

// channelFindIntent 按名取意图声明（找不到返回 nil）。
func channelFindIntent(name string) *intent.IntentSpec {
	for _, spec := range intent.IntentCatalog {
		if spec != nil && spec.Name == name {
			return spec
		}
	}
	return nil
}

// assembleCapHandlers 装配「底层能力」表：端点/内置 handler + api_* 元能力。
//
// 与 NewAiChatService 用的是同一份装配逻辑（抽出本函数是为了让测试能遍历真实
// capHandlers，而不是在测试里手抄一份能力清单）。
func (svc *AiChatService) assembleCapHandlers() map[string]toolHandler {
	_, endpointHandlers := svc.getEinoTools()
	_, builtinHandlers := svc.getBuiltinEinoTools()
	caps := make(map[string]toolHandler, len(endpointHandlers)+len(builtinHandlers)+3)
	for name, handler := range endpointHandlers {
		caps[name] = handler
	}
	for name, handler := range builtinHandlers {
		caps[name] = handler
	}
	caps[capAPIList] = svc.capAPIList
	caps[capAPISchema] = svc.capAPISchema
	caps[capAPIInvoke] = svc.capAPIInvoke
	return caps
}
