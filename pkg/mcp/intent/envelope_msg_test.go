package intent

import (
	"encoding/json"
	"testing"
)

// envelopeOf 只拿到业务载荷、看不到端点响应，所以 msg 只能写死成空串。
// 结果：只要工具带了 data，端点说的「保存成功」「配置已更新」就全被吞掉——
// 调用方看到 code=0/ok=true 却不知道端点到底回了什么。
//
// 实测（2026-10-28）：给 user_create 补上 data.id 之后，回执里的 msg 从
// 「保存成功」变成了空串，就是踩在这条路径上。makeHandler 出口必须先补齐
// 端点 msg，再序列化给 Text / StructuredContent 两个通道。
func TestMergeEndpointMsgFillsEndpointMessage(t *testing.T) {
	cases := []struct {
		name string
		obj  any
		raw  string
		want string // 期望的 msg；"" 表示不该有 msg 键或为空
	}{
		{
			name: "载荷没有 msg 键时补上端点 msg",
			obj:  map[string]any{"id": 42},
			raw:  `{"code":0,"msg":"保存成功","data":{"id":42}}`,
			want: "保存成功",
		},
		{
			name: "载荷 msg 为空串时补上",
			obj:  map[string]any{"id": 42, "msg": ""},
			raw:  `{"code":0,"msg":"配置已更新"}`,
			want: "配置已更新",
		},
		{
			name: "Compose 自己写的 msg 不被覆盖",
			obj:  map[string]any{"msg": "⚠ 警告：已截断"},
			raw:  `{"code":0,"msg":"保存成功"}`,
			want: "⚠ 警告：已截断",
		},
		{
			name: "端点 msg 也为空时保持原样",
			obj:  map[string]any{"id": 1},
			raw:  `{"code":0,"msg":""}`,
			want: "",
		},
		{
			name: "端点文本不是 JSON 时不动",
			obj:  map[string]any{"id": 1},
			raw:  `Saved successfully`,
			want: "",
		},
		{
			name: "端点文本为空时不动",
			obj:  map[string]any{"id": 1},
			raw:  "",
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mergeEndpointMsg(c.obj, c.raw)
			m, ok := got.(map[string]any)
			if !ok {
				t.Fatalf("返回值应仍是 map，实际 %T", got)
			}
			s, _ := m["msg"].(string)
			if s != c.want {
				t.Fatalf("msg 应为 %q，实际 %q（原文 %s）", c.want, s, c.raw)
			}
		})
	}

	// 非 map 载荷（裸数组/标量）不应被改动：它们由 structuredObject 另行包裹。
	t.Run("非 map 载荷原样返回", func(t *testing.T) {
		in := []any{map[string]any{"id": 1}}
		got := mergeEndpointMsg(in, `{"code":0,"msg":"保存成功"}`)
		raw, _ := json.Marshal(got)
		if string(raw) != `[{"id":1}]` {
			t.Fatalf("非 map 载荷被改动了：%s", raw)
		}
	})
}

// 端到端形状检查：合并后仍必须是合法信封（含 ok），且 msg 与 data 同时存在。
func TestMergeEndpointMsgKeepsEnvelopeShape(t *testing.T) {
	obj := structuredObject(map[string]any{"id": 7, "user_name": "张三"})
	obj = mergeEndpointMsg(obj, `{"code":0,"msg":"保存成功","data":{"id":7}}`)

	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("反序列化失败：%v", err)
	}
	if m["ok"] != true {
		t.Errorf("ok 应为 true：%s", raw)
	}
	if m["msg"] != "保存成功" {
		t.Errorf("msg 应为 保存成功：%s", raw)
	}
	d, ok := m["data"].(map[string]any)
	if !ok || d["id"].(float64) != 7 {
		t.Errorf("data.id 应保留为 7：%s", raw)
	}
}
