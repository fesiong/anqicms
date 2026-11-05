package manageController

import (
	"strings"
	"testing"
)

// TestChannelControllerLeavesSecretsToProvider 是 T9a 的分层护栏（R81）。
//
// 渠道的密钥掩码、站点隔离、乐观锁与删除级联全部落在 provider/channelAdmin.go；
// 控制器一旦直接摸到 AppSecret / 模型行，就意味着有一条响应绕过了 provider 的投影——
// 那正是「列表里冒出密钥真值」最可能的形态。这里用源码断言把它钉死：
//   - 控制器源码不得出现 AppSecret / AppIdentityHash / model.MessageChannel / %+v；
//   - 每个 handler 必须委托给 provider 的对应方法；
//   - 两个写端点必须用 readJSONPayload（空载荷直接拒绝）。
func TestChannelControllerLeavesSecretsToProvider(t *testing.T) {
	src := loadControllerSources(t)

	for _, forbidden := range []string{"AppSecret", "AppIdentityHash", "model.MessageChannel", "%+v"} {
		for name, body := range src {
			if !strings.HasPrefix(name, "Channel") {
				continue
			}
			if strings.Contains(body, forbidden) {
				t.Errorf("%s 里出现了 %q：密钥掩码与响应投影必须留在 provider", name, forbidden)
			}
		}
	}

	delegations := map[string]string{
		"ChannelList":         "currentSite.ChannelAdminList(",
		"ChannelDetail":       "currentSite.ChannelAdminDetail(",
		"ChannelDetailForm":   "currentSite.ChannelAdminSave(",
		"ChannelDelete":       "currentSite.ChannelAdminDelete(",
		"ChannelStatus":       "currentSite.ChannelAdminStatus(",
		"ChannelCapabilities": "currentSite.ChannelCapabilities(",
	}
	for name, want := range delegations {
		body, ok := src[name]
		if !ok {
			t.Errorf("缺少 handler %s", name)
			continue
		}
		if !strings.Contains(body, want) {
			t.Errorf("%s 没有委托给 %s，控制器不该自己查库", name, want)
		}
	}

	for _, name := range []string{"ChannelDetailForm", "ChannelDelete"} {
		body, ok := src[name]
		if !ok {
			continue
		}
		if !strings.Contains(body, "readJSONPayload(ctx, &req)") {
			t.Errorf("%s 必须用 readJSONPayload 读取请求体（空载荷不能被当成保存成功）", name)
		}
	}
}
