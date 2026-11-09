package provider

import (
	"os"
	"strings"
	"testing"
)

// 渠道执行链不得再出现写门或审批分支。
// 反向对照：在 channelChat.go 里临时加回 WriteGateMiddleware 或 applyApprovalGrant，本用例变红。
func TestChannelChainHasNoWriteGateOrApproval(t *testing.T) {
	raw, err := os.ReadFile("channelChat.go")
	if err != nil {
		t.Fatalf("读取 channelChat.go 失败：%v", err)
	}
	src := string(raw)
	for _, forbidden := range []string{
		"WriteGateMiddleware",
		"ChannelPendingConfirmation",
		"ChannelApprovalGrant",
		"applyApprovalGrant",
		"runApprovedCall",
		"channelApprovalBackend",
		"approvalDecision",
		"setApprovalErr",
	} {
		if strings.Contains(src, forbidden) {
			t.Errorf("渠道执行链仍包含审批实现：%s", forbidden)
		}
	}
}

// 审批模型不得再注册到 DAO；登记后启动 AutoMigrate 会重新建表并恢复读写。
func TestApprovalModelNotRegistered(t *testing.T) {
	raw, err := os.ReadFile("dao.go")
	if err != nil {
		t.Fatalf("读取 dao.go 失败：%v", err)
	}
	if strings.Contains(string(raw), "ChannelApproval") {
		t.Fatal("dao.go 仍注册了 ChannelApproval")
	}
}
