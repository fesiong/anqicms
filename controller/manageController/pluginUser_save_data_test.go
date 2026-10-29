package manageController

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"kandaoni.com/anqicms/model"
)

// user_create / user_update 走同一个端点（POST /plugin/user/detail）。
// 原来成功只回 {code,msg}，调用方（后台表单、AI 的 commerce user_create）拿不到
// 新建用户的 id，想接着改它就得先翻 list 去猜 —— 这正是「建完不能链式操作」的根因。
//
// 契约：保存成功后 data 里必须带 id（以及 user_name，便于确认到底写没写进去）。
func TestUserSaveDataCarriesID(t *testing.T) {
	got := userSaveData(&model.User{Id: 42, UserName: "张三"})

	// 序列化后再断言：MCP 侧看到的是 JSON，不是 Go map。
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("回执序列化失败：%v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("回执反序列化失败：%v", err)
	}
	if id, ok := m["id"]; !ok {
		t.Fatalf("回执缺 id 字段：%s", raw)
	} else if id.(float64) != 42 {
		t.Fatalf("id 应为 42，实际 %v", id)
	}
	if m["user_name"] != "张三" {
		t.Fatalf("user_name 应为 张三，实际 %v", m["user_name"])
	}

	// 零值 id 也必须输出 id 键：键被省略时调用方无法区分「没回 id」与「id=0」。
	zero, _ := json.Marshal(userSaveData(&model.User{}))
	if !strings.Contains(string(zero), `"id"`) {
		t.Fatalf("id 为零值时仍应输出 id 键：%s", zero)
	}
}

// 源码级护栏：handler 成功分支必须把 userSaveData 放进 data。
// 只测 userSaveData 本身锁不住「handler 忘了用它」——那正是原缺陷的形态。
func TestUserDetailFormUsesSaveData(t *testing.T) {
	b, err := os.ReadFile("pluginUser.go")
	if err != nil {
		t.Fatalf("读取 pluginUser.go 失败：%v", err)
	}
	src := string(b)
	start := strings.Index(src, "func PluginUserDetailForm(ctx iris.Context)")
	if start < 0 {
		t.Fatal("未找到 PluginUserDetailForm")
	}
	body := src[start:]
	if end := strings.Index(body[1:], "\nfunc "); end > 0 {
		body = body[:end+1]
	}
	if !strings.Contains(body, `"data": userSaveData(user)`) {
		t.Errorf("PluginUserDetailForm 的成功返回缺 `\"data\": userSaveData(user)`：\n%s", body)
	}
}
