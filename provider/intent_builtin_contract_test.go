package provider

import (
	"reflect"
	"strings"
	"testing"

	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// 内置工具收编为意图的回归网。
//
// 背景：read_file / write_file / bash / web_fetch 这批内置工具曾经**同时**以两种身份
// 出现在后台对话的模型面上 —— 既是内置工具，又被 fs_read / shell_exec / web 等意图包装。
// 更关键的是内置那一份不受 ExposedIntents 白名单管辖，导致后台对话比 MCP 多出无人管理的
// 主机级能力。现在内置工具只剩底层能力（cap）身份，模型面唯一出口是意图。
//
// 这个收敛只有在"意图带齐 cap 的参数契约"时才不算能力缩水，而这件事没有编译器能管：
// 意图声明在 pkg/mcp/intent，入参结构体在 provider，两边各改各的就会出现
// "模型按 schema 传参、cap 静默收不到"（fs_replace 此前正是如此：声明 pattern/replacement，
// 结构体只认 search/replace）。因此这里用反射把两边钉在一起。

// capContract 是内置能力名 → 其入参结构体与别名表。
//
// 新增内置能力必须在这里登记，否则 TestBuiltinIntentsCoverEveryCap 会直接失败；
// aliases 描述该 cap 内部互为同义词的字段（如 path 与 file_path），
// 按 cap 逐条列出而非全局同义词表：search_replace 的 search 是"匹配模式"，
// edit_file 的 search 是"待替换文本"，全局别名会让其中一个的错误声明被另一个掩盖。
type capContract struct {
	args    reflect.Type
	aliases map[string]string // cap 字段名 → 意图可声明的等价参数名
}

var capContracts = map[string]capContract{
	"bash": {args: reflect.TypeOf(bashArgs{})},
	"read_file": {
		args:    reflect.TypeOf(fileReadArgs{}),
		aliases: map[string]string{"file_path": "path"},
	},
	"write_file": {
		args:    reflect.TypeOf(fileWriteArgs{}),
		aliases: map[string]string{"file_path": "path"},
	},
	"edit_file": {
		args: reflect.TypeOf(fileEditArgs{}),
		aliases: map[string]string{
			"file_path": "path",
			"search":    "old_string",
			"replace":   "new_string",
		},
	},
	"grep":           {args: reflect.TypeOf(grepArgs{})},
	"search_replace": {args: reflect.TypeOf(searchReplaceArgs{})},
	"glob":           {args: reflect.TypeOf(globArgs{})},
	"list_directory": {args: reflect.TypeOf(listDirArgs{})},
	"web_fetch":      {args: reflect.TypeOf(webFetchArgs{})},
	"web_search":     {args: reflect.TypeOf(webSearchArgs{})},
}

// dispatchKeys 是意图层自己消费、不会传给 cap 的字段。
var dispatchKeys = map[string]bool{"action": true}

func jsonKeysOf(t reflect.Type) map[string]bool {
	out := map[string]bool{}
	if t.Kind() != reflect.Struct {
		return out
	}
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if name := strings.TrimSpace(strings.Split(tag, ",")[0]); name != "" {
			out[name] = true
		}
	}
	return out
}

// capAccepts 判断该 cap 是否认这个参数名（含其别名）。
func capAccepts(c capContract, keys map[string]bool, name string) bool {
	if keys[name] {
		return true
	}
	for field, synonym := range c.aliases {
		if name == field && keys[synonym] {
			return true
		}
	}
	return false
}

// intentCovers 判断意图声明是否覆盖了 cap 的某个入参（含其别名）。
func intentCovers(declared map[string]bool, c capContract, name string) bool {
	if declared[name] {
		return true
	}
	return declared[c.aliases[name]]
}

func intentByName(name string) (*intent.IntentSpec, bool) {
	for _, s := range intent.IntentCatalog {
		if s.Name == name {
			return s, true
		}
	}
	return nil, false
}

// intentForCap 找出包装某个内置能力的意图（Caps 里含该能力名）。
func intentForCap(capName string) (*intent.IntentSpec, bool) {
	for _, s := range intent.IntentCatalog {
		for _, c := range s.Caps {
			if c == capName {
				return s, true
			}
		}
	}
	return nil, false
}

// TestBuiltinIntentsCoverEveryCap 每个内置能力都必须有意图出口，且该意图默认可见。
//
// 默认可见这一条是收敛的前提：内置工具从模型面撤下后，若对应意图仍 DefaultOff，
// 未配白名单的站点就凭空失去读文件/执行命令的能力 —— 那是缩水，不是收口。
func TestBuiltinIntentsCoverEveryCap(t *testing.T) {
	builtinTools, _ := (&AiChatService{}).getBuiltinEinoTools()
	if len(builtinTools) == 0 {
		t.Fatal("未取到内置能力清单，判定条件已失效")
	}
	for _, bt := range builtinTools {
		cc, ok := capContracts[bt.Name]
		if !ok {
			t.Errorf("内置能力 %s 未登记入参契约，无法校验意图是否覆盖它", bt.Name)
			continue
		}
		spec, ok := intentForCap(bt.Name)
		if !ok {
			t.Errorf("内置能力 %s 没有意图包装，撤掉模型面的内置工具后它将不可达", bt.Name)
			continue
		}
		if spec.DefaultOff {
			t.Errorf("意图 %s（覆盖 %s）被 DefaultOff 关闭，内置工具退场后该能力对未配白名单的站点消失",
				spec.Name, bt.Name)
		}
		// 意图名与 cap 名不得相同：同名会让 capHandlers/Handlers 两张表互相覆盖，
		// 委托时递归（见 ResolveCap 注释）。
		if spec.Name == bt.Name {
			t.Errorf("意图 %s 与其 cap 同名，委托会递归", spec.Name)
		}
		if cc.args == nil {
			t.Errorf("内置能力 %s 未登记入参结构体", bt.Name)
		}
	}
}

// TestBuiltinIntentCarriesCapContract 双向核对意图 schema 与 cap 入参：
// 意图不得声明 cap 收不到的字段（模型照 schema 传参后静默失效），
// 也不得漏声明 cap 认得的字段（能力对模型不可见）。
//
// 一个意图可覆盖多个 cap（web = web_fetch + web_search），此时按并集核对：
// 声明的参数只要在任一被覆盖的 cap 里认得即可。
func TestBuiltinIntentCarriesCapContract(t *testing.T) {
	byIntent := map[string]map[string]bool{}  // 意图名 → 声明的参数
	covered := map[string][]reflect.Type{}    // 意图名 → 各 cap 的入参结构体
	aliasOf := map[string]map[string]string{} // 意图名 → 合并后的别名表
	for capName, cc := range capContracts {
		spec, ok := intentForCap(capName)
		if !ok {
			t.Fatalf("内置能力 %s 没有对应意图", capName)
		}
		if _, ok := byIntent[spec.Name]; !ok {
			byIntent[spec.Name] = map[string]bool{}
			for p := range spec.Params {
				byIntent[spec.Name][p] = true
			}
			aliasOf[spec.Name] = map[string]string{}
		}
		covered[spec.Name] = append(covered[spec.Name], cc.args)
		for k, v := range cc.aliases {
			aliasOf[spec.Name][k] = v
		}
	}

	for specName, declared := range byIntent {
		aliases := aliasOf[specName]
		keySets := make([]map[string]bool, 0, len(covered[specName]))
		for _, t_ := range covered[specName] {
			keySets = append(keySets, jsonKeysOf(t_))
		}
		acceptsAny := func(name string) bool {
			for _, keys := range keySets {
				if capAccepts(capContract{aliases: aliases}, keys, name) {
					return true
				}
			}
			return false
		}
		for p := range declared {
			if dispatchKeys[p] {
				continue
			}
			if !acceptsAny(p) {
				t.Errorf("意图 %s 声明了参数 %q，但它覆盖的 cap 入参结构体没有这个字段（会被静默丢弃）",
					specName, p)
			}
		}
		for _, keys := range keySets {
			for k := range keys {
				if declared[k] || declared[aliases[k]] {
					continue
				}
				t.Errorf("cap 接受参数 %q，意图 %s 未声明，模型看不到这项能力", k, specName)
			}
		}
	}
}

// TestBuiltinNamesNotModelFacing 内置能力名不得同时是意图名，
// 否则模型面会出现同一能力的两个工具名（本次修复的原始症状）。
func TestBuiltinNamesNotModelFacing(t *testing.T) {
	builtinTools, _ := (&AiChatService{}).getBuiltinEinoTools()
	for _, bt := range builtinTools {
		if _, ok := intentByName(bt.Name); ok {
			t.Errorf("存在与内置能力同名的意图 %s，模型面会出现重复工具", bt.Name)
		}
	}
}
