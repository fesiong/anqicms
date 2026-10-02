package provider

import (
	"strings"
	"testing"
)

// TestRecentAIErrors 验证近期错误环形缓冲的写入与有序读取。
func TestRecentAIErrors(t *testing.T) {
	// 清空缓冲 (测试隔离)：通过写入满容量来覆盖旧数据
	oldBuf, oldHead, oldSize := recentAIErrorsBuf, recentAIErrorsHead, recentAIErrorsSize
	defer func() {
		recentAIErrorsBuf, recentAIErrorsHead, recentAIErrorsSize = oldBuf, oldHead, oldSize
	}()
	recentAIErrorsBuf = make([]RecentAIError, recentAIErrorCap)
	recentAIErrorsHead = 0
	recentAIErrorsSize = 0

	RecordAIError("err-one")
	RecordAIError("err-two")
	RecordAIError("err-three")

	got := GetRecentAIErrors()
	if len(got) != 3 {
		t.Fatalf("期望 3 条，实际 %d", len(got))
	}
	if got[0].Msg != "err-one" || got[2].Msg != "err-three" {
		t.Fatalf("顺序错误: %v", []string{got[0].Msg, got[1].Msg, got[2].Msg})
	}

	// 溢出覆盖：写入超过容量，最旧被丢弃且保持有序
	for i := 0; i < recentAIErrorCap+5; i++ {
		RecordAIError("overflow-%d", i)
	}
	got = GetRecentAIErrors()
	if len(got) != recentAIErrorCap {
		t.Fatalf("溢出后应为满容量 %d，实际 %d", recentAIErrorCap, len(got))
	}
	if !strings.HasPrefix(got[0].Msg, "overflow-") || !strings.HasPrefix(got[len(got)-1].Msg, "overflow-") {
		t.Fatalf("溢出后内容异常: %s ... %s", got[0].Msg, got[len(got)-1].Msg)
	}
	// 有序：最后一条应是最大序号
	last := got[len(got)-1].Msg
	if last != "overflow-"+itoa(recentAIErrorCap+4) {
		t.Fatalf("最后一条应为最新写入，实际 %s", last)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
