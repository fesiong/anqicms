package provider

import (
	"os"
	"strings"
	"testing"
)

// 渠道执行链与渠道授权层不得再出现写门、审批分支或审批残留符号。
// 反向对照：在 channelChat.go 里临时加回 WriteGateMiddleware/applyApprovalGrant，
// 或在 channelPrincipal.go 里临时加回 ChannelArgsHash/ErrChannelApprovalInvalid，本用例变红。
func TestChannelChainHasNoWriteGateOrApproval(t *testing.T) {
	files := []string{
		"channelChat.go",
		// 授权层的纯数据文件：审批哨兵与参数摘要曾定义在这里，
		// 只扫 channelChat.go 时它们重新长回来不会变红。
		"channelPrincipal.go",
	}
	forbidden := []string{
		"WriteGateMiddleware",
		"ChannelPendingConfirmation",
		"ChannelApprovalGrant",
		"applyApprovalGrant",
		"runApprovedCall",
		"channelApprovalBackend",
		"approvalDecision",
		"setApprovalErr",
		// 审批子系统的导出残留：审批删掉后零调用，留着就是给后来者一个可用的旁路口子。
		"ChannelArgsHash",
		"ErrChannelApprovalInvalid",
	}
	for _, name := range files {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("读取 %s 失败：%v", name, err)
		}
		src := string(raw)
		for _, symbol := range forbidden {
			if strings.Contains(src, symbol) {
				t.Errorf("%s 仍包含审批实现：%s", name, symbol)
			}
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
