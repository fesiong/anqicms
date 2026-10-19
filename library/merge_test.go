package library

import (
	"encoding/json"
	"testing"
)

type mergeTestItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// mergeTestConfig 模拟设置类配置结构体：含常规字段、`-` 字段、无 tag 字段。
type mergeTestConfig struct {
	SiteName string          `json:"site_name"`
	Count    int             `json:"count"`
	Enabled  bool            `json:"enabled"`
	Items    []mergeTestItem `json:"items"`
	Hidden   string          `json:"-"`
	NoTag    string
}

func mustRaw(t *testing.T, s string) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("payload 解析失败: %v", err)
	}
	return m
}

// 部分更新：只传 site_name，其余字段保持基底值不被清空。
func TestMergeJSONPayloadPartialUpdate(t *testing.T) {
	dst := mergeTestConfig{
		SiteName: "旧名称",
		Count:    5,
		Enabled:  true,
		Items:    []mergeTestItem{{Name: "a"}},
		Hidden:   "keep",
		NoTag:    "keep",
	}
	err := MergeJSONPayload(mustRaw(t, `{"site_name":"新名称"}`), &dst)
	if err != nil {
		t.Fatalf("MergeJSONPayload: %v", err)
	}
	if dst.SiteName != "新名称" {
		t.Errorf("site_name 应被更新，得到 %q", dst.SiteName)
	}
	if dst.Count != 5 || !dst.Enabled {
		t.Errorf("未传字段应保持原值，得到 count=%d enabled=%v", dst.Count, dst.Enabled)
	}
	if len(dst.Items) != 1 || dst.Items[0].Name != "a" {
		t.Errorf("未传的 items 应保持原值，得到 %+v", dst.Items)
	}
	if dst.Hidden != "keep" || dst.NoTag != "keep" {
		t.Errorf("`-` 与无 tag 字段不应被更新，得到 hidden=%q noTag=%q", dst.Hidden, dst.NoTag)
	}
}

// 显式清空：传空字符串 / 0 必须生效，这是「主动清空」与「未传」的关键区别。
func TestMergeJSONPayloadExplicitClear(t *testing.T) {
	dst := mergeTestConfig{
		SiteName: "旧名称",
		Count:    5,
		Enabled:  true,
		Items:    []mergeTestItem{{Name: "a"}},
	}
	err := MergeJSONPayload(mustRaw(t, `{"site_name":"","count":0,"enabled":false,"items":[]}`), &dst)
	if err != nil {
		t.Fatalf("MergeJSONPayload: %v", err)
	}
	if dst.SiteName != "" || dst.Count != 0 || dst.Enabled || len(dst.Items) != 0 {
		t.Errorf("显式传入的空值应覆盖原值，得到 %+v", dst)
	}
}

// 未知字段应被忽略而不是报错（向前兼容）。
func TestMergeJSONPayloadIgnoresUnknownFields(t *testing.T) {
	dst := mergeTestConfig{SiteName: "旧名称"}
	err := MergeJSONPayload(mustRaw(t, `{"site_name":"新名称","ghost":123}`), &dst)
	if err != nil {
		t.Fatalf("未知字段不应报错: %v", err)
	}
	if dst.SiteName != "新名称" {
		t.Errorf("site_name 应被更新，得到 %q", dst.SiteName)
	}
}

// 类型不匹配必须报错，不能静默吞掉。
func TestMergeJSONPayloadTypeMismatch(t *testing.T) {
	dst := mergeTestConfig{Count: 1}
	err := MergeJSONPayload(mustRaw(t, `{"count":"abc"}`), &dst)
	if err == nil {
		t.Fatal("类型不匹配应返回错误")
	}
	if dst.Count != 1 {
		t.Errorf("出错时字段应保持原值，得到 %d", dst.Count)
	}
}

// dst 必须是结构体指针。
func TestMergeJSONPayloadRejectsNonStruct(t *testing.T) {
	var m map[string]string
	if err := MergeJSONPayload(mustRaw(t, `{"a":"b"}`), &m); err == nil {
		t.Fatal("非结构体指针应返回错误")
	}
}
