package intent

import "testing"

// TestIntentAllowed 是 intent 包内的对照测试：IntentAllowed 与 Kernel.allowed
// 必须共用同一份曝光规则（R29），且行为与改造前一致。
func TestIntentAllowed(t *testing.T) {
	// 未知意图默认拒绝：曝光规则不兜底。
	if IntentAllowed(Config{}, "no_such_intent") {
		t.Error("未知意图必须拒绝")
	}

	// DefaultOff：即使白名单为空也必须拒绝。
	if IntentAllowed(Config{}, "system_plugin") {
		t.Error("DefaultOff 意图在未点名时必须拒绝")
	}
	// 点名后开放。
	if !IntentAllowed(Config{ExposedIntents: []string{"system_plugin"}}, "system_plugin") {
		t.Error("DefaultOff 意图被 ExposedIntents 点名后必须开放")
	}

	// "*" 全开：所有 DefaultOff 意图都应被点亮（含未来新增的）。
	starCfg := Config{ExposedIntents: []string{"*"}}
	var defaultOffCount int
	for _, spec := range IntentCatalog {
		if spec == nil || !spec.DefaultOff {
			continue
		}
		defaultOffCount++
		if !IntentAllowed(starCfg, spec.Name) {
			t.Errorf("ExposedIntents=[*] 时 DefaultOff 意图 %s 必须开放", spec.Name)
		}
	}
	if defaultOffCount == 0 {
		t.Fatal("IntentCatalog 里没有 DefaultOff 意图，本测试失去对照意义")
	}

	// 域匹配：整域名与 domain: 前缀两种写法都算命中。
	if !IntentAllowed(Config{ExposedIntents: []string{"builtin"}}, "fs_read") {
		t.Error("整域名 builtin 必须命中 fs_read")
	}
	if !IntentAllowed(Config{ExposedIntents: []string{"domain:builtin"}}, "shell_exec") {
		t.Error("domain:builtin 必须命中 shell_exec")
	}

	// 白名单非空但不命中 → 拒绝。
	if IntentAllowed(Config{ExposedIntents: []string{"content"}}, "fs_read") {
		t.Error("白名单未命中的意图必须拒绝")
	}

	// ExposedTools 兜底：按底层能力命中。
	if !IntentAllowed(Config{ExposedTools: []string{"read_file"}}, "fs_read") {
		t.Error("ExposedTools 命中 read_file 时 fs_read 必须开放")
	}
	if IntentAllowed(Config{ExposedTools: []string{"bash"}}, "fs_read") {
		t.Error("ExposedTools 未命中时必须拒绝")
	}

	// 同源对照：Kernel.allowed 走的是同一个实现。
	k := NewKernel(Config{ExposedIntents: []string{"content"}}, nil, nil)
	if !k.allowed(&IntentSpec{Name: "demo_intent", Domain: DomainContent}) {
		t.Error("Kernel.allowed 必须与 IntentAllowed 同源：content 域应放行")
	}
	if k.allowed(&IntentSpec{Name: "demo_builtin", Domain: DomainBuiltin}) {
		t.Error("Kernel.allowed 必须与 IntentAllowed 同源：builtin 域应拒绝")
	}
}
