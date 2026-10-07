package intent

import (
	"encoding/json"
	"strings"
)

// risk.go 把「这一次调用到底危不危险」从**工具粒度**降到 **action 粒度**。
//
// 背景：合并意图（如 content_article 的 list/detail/save/delete）整体标 Risk=write，
// 而审批门早期只看 spec.Risk（且只认 ==RiskWrite），于是
//   - 纯查询动作也要人工点确认（弹窗疲劳，用户问题 2）；
//   - RiskSystem 的意图（shell_exec/fs_write/fs_edit/fs_replace）反而**完全不问**（安全绕过）。
//
// 判定按可靠性从高到低取第一处命中：
//  1. 路由表推导（A1）：意图经 invokeRoutes 声明的 "METHOD PATH"，或 api action=invoke
//     本次要执行的 method —— 这是可执行的事实，不是猜测；
//  2. 端点表推导：工具名本身就是底层能力名（cap 模式），按 capEndpoints 的方法判定；
//  3. 动词约定兜底（A3）：按 action 里下划线分隔的 token 从后往前取第一个可识别动词；
//  4. fail closed：推不出来就回落到 spec.Risk（未知工具回落到 RiskSystem）。
//     宁可多问一次，不可静默放行。

// readVerbs 只读取、不改变站点状态的动作词。
// 末尾的 menus/texts/modules/combination 是"名词即列表"的后台习惯
// （admin_menus、user_orders、template_modules），按清单读取处理。
var readVerbs = map[string]bool{
	"list": true, "get": true, "detail": true, "info": true,
	"query": true, "search": true, "find": true, "read": true, "view": true,
	"show": true, "fetch": true, "count": true, "sum": true, "summary": true,
	"statistic": true, "statistics": true, "stats": true, "dashboard": true,
	"report": true, "export": true, "download": true, "preview": true,
	"schema": true, "tree": true, "logs": true, "log": true, "history": true,
	"latest": true, "describe": true, "lookup": true, "version": true,
	"menus": true, "texts": true, "modules": true, "combination": true,
}

// writeVerbs 产生数据变更或触发后台任务的动作词。
var writeVerbs = map[string]bool{
	"create": true, "add": true, "new": true, "save": true, "update": true,
	"modify": true, "edit": true, "set": true, "put": true, "post": true,
	"form": true, "upload": true, "build": true, "rebuild": true, "reload": true,
	"replace": true, "import": true, "publish": true, "draft": true, "install": true,
	"run": true, "execute": true, "start": true, "stop": true, "restart": true,
	"toggle": true, "enable": true, "disable": true, "migrate": true, "backup": true,
	"dump": true, "approve": true, "check": true, "push": true, "sync": true,
	"generate": true, "apply": true, "submit": true, "send": true, "collect": true,
	"upgrade": true, "reset": true, "rename": true, "move": true, "bind": true,
	"register": true, "insert": true, "commit": true, "realname": true,
}

// destructiveVerbs 删除/清空类动作词（不可回滚）。
var destructiveVerbs = map[string]bool{
	"delete": true, "remove": true, "drop": true, "clear": true, "purge": true,
	"destroy": true, "truncate": true, "revoke": true, "uninstall": true, "cancel": true,
}

// builtinCapRisk 声明没有 REST 端点、也没有意图声明可查的内置能力风险。
//
// 这里是唯一一份口径：provider 的审批门不再自己维护"写工具名单"。
// 名单只覆盖**非意图**的工具名（cap 模式的端点级工具、内核元工具、内置文件/shell 工具）。
var builtinCapRisk = map[string]Risk{
	// 内核元工具：只读自身的注册表，不触碰站点数据。
	"mcp_list_intents": RiskRead,
	"mcp_set_scope":    RiskRead,
	"api_list":         RiskRead,
	"api_schema":       RiskRead,
	"api_invoke":       RiskWrite, // 实际风险由本次 method 推导，见 ActionRisk

	// 只读内置工具
	"read_file":      RiskRead,
	"grep":           RiskRead,
	"glob":           RiskRead,
	"list_directory": RiskRead,
	"web_fetch":      RiskRead,

	// 主机级：文件写入与 shell 归 system（路径安全门另有更细的判定）
	"write_file":      RiskSystem,
	"create_file":     RiskSystem,
	"edit_file":       RiskSystem,
	"search_replace":  RiskSystem,
	"bash":            RiskSystem,
	"template_reload": RiskSystem, // 发进程重启信号

	// 站点内写入
	"attachment_upload": RiskWrite,
	"skill_list":        RiskRead,
	"skill_get":         RiskRead,
	"skill_search":      RiskRead,
	"skill_save":        RiskWrite,
	"skill_reload":      RiskWrite,
	"skill_delete":      RiskDestructive, // rm -rf 技能目录，不可恢复
	"skill_install":     RiskSystem,      // 从市场装代码，主机级
	"task":              RiskWrite,

	// 智能体管理
	"agent_list":   RiskRead,
	"agent_detail": RiskRead,
	"agent_create": RiskWrite,
	"agent_edit":   RiskWrite,
	"agent_toggle": RiskWrite,
	"agent_delete": RiskDestructive,
	"agent_run":    RiskWrite,
	"agent_chat":   RiskWrite,
}

// riskFromMethod 按 HTTP 方法给出端点风险。
//
// 本后台的删除类端点走 POST /xxx/delete（不是 HTTP DELETE），只看方法会把
// 不可回滚的删除降级成普通写操作，所以还要看路径动词。
func riskFromMethod(method string) Risk {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "DELETE":
		return RiskDestructive
	case "GET", "HEAD", "OPTIONS":
		return RiskRead
	default:
		return RiskWrite
	}
}

// riskFromEndpoint 综合方法与路径判定端点风险。
func riskFromEndpoint(method, path string) Risk {
	m := strings.ToUpper(strings.TrimSpace(method))
	if m == "GET" || m == "HEAD" || m == "OPTIONS" {
		// 只读方法优先：查询动作不应该因为路径里含 "delete" 字样而被误判。
		return RiskRead
	}
	if r, ok := verbRisk(path); ok && r == RiskDestructive {
		return RiskDestructive
	}
	return riskFromMethod(m)
}

// riskFromTarget 解析 "METHOD PATH" 声明。
func riskFromTarget(target string) (Risk, bool) {
	parts := strings.Fields(strings.TrimSpace(target))
	if len(parts) != 2 {
		return "", false
	}
	return riskFromEndpoint(parts[0], parts[1]), true
}

// verbRisk 按动词约定推断一段名字（action 名或路径）的风险。
//
// 从后往前扫下划线分隔的 token：动作语义通常落在最后一段（guestbook_setting_save→save、
// push_logs→logs），前面的段多是资源名（archive/comment/site）。资源名恰好撞上动词时
// （如 "check" 类名词）也会被更靠后的动词优先覆盖。
func verbRisk(name string) (Risk, bool) {
	fields := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(name)), func(r rune) bool {
		return r == '_' || r == '/' || r == '-' || r == ' '
	})
	for i := len(fields) - 1; i >= 0; i-- {
		tok := fields[i]
		switch {
		case destructiveVerbs[tok]:
			return RiskDestructive, true
		case readVerbs[tok]:
			return RiskRead, true
		case writeVerbs[tok]:
			return RiskWrite, true
		}
	}
	return "", false
}

func stringArg(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func hasCap(spec *IntentSpec, capName string) bool {
	if spec == nil {
		return false
	}
	for _, c := range spec.Caps {
		if c == capName {
			return true
		}
	}
	return false
}

// ActionRisk 推导一次调用的真实风险等级。
//
// toolName 既可能是意图名（意图模式），也可能是能力名（cap 模式/内置工具）。
// args 是模型给的参数（已解析）；nil 时按最保守口径处理。
func ActionRisk(toolName string, args map[string]any) Risk {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return RiskSystem
	}
	action := stringArg(args, "action")

	if spec, ok := SpecByName(toolName); ok {
		// 1) 意图自己声明的端点路由：与执行用的是同一份字面量，最可靠。
		if action != "" {
			if r, ok2 := riskFromTarget(intentRouteRegistry[toolName][action]); ok2 {
				return r
			}
		}
		// 2) 通用调用：api action=invoke 的风险就是这次要执行的 method。
		//    不区分的话 list/schema 这两个纯发现动作也会弹窗。
		if action == "invoke" && hasCap(spec, "api_invoke") {
			return riskFromEndpoint(stringArg(args, "method"), stringArg(args, "path"))
		}
		// 3) 动词约定兜底。
		if action != "" {
			if r, ok2 := verbRisk(action); ok2 {
				return r
			}
		}
		// 4) fail closed：回落到意图声明的整体风险。
		return spec.Risk
	}

	// 非意图工具：先查端点表，再查内置清单，再按名字兜底。
	if ep, ok2 := capEndpoints[toolName]; ok2 {
		return riskFromEndpoint(ep.Method, ep.Path)
	}
	if toolName == "api_invoke" {
		return riskFromEndpoint(stringArg(args, "method"), stringArg(args, "path"))
	}
	if r, ok2 := builtinCapRisk[toolName]; ok2 {
		return r
	}
	if r, ok2 := verbRisk(toolName); ok2 {
		return r
	}
	return RiskSystem
}

// ActionRiskFromArgsJSON 是 ActionRisk 的 JSON 参数入口，供只持有原始 arguments
// 字符串的调用方（middleware / 审批门）使用。解析失败时回落到意图整体风险。
func ActionRiskFromArgsJSON(toolName, argsJSON string) Risk {
	var args map[string]any
	if s := strings.TrimSpace(argsJSON); s != "" && s != "{}" {
		_ = json.Unmarshal([]byte(s), &args)
	}
	return ActionRisk(toolName, args)
}

// NeedsApproval 判定该风险是否需要人工确认。
//
// 三档都要问：write（建/改）、destructive（删除）、system（主机级/全局配置）。
// 早先只认 write，RiskSystem 的 shell/文件写意图因此完全绕过了审批门。
func NeedsApproval(r Risk) bool {
	return r == RiskWrite || r == RiskDestructive || r == RiskSystem
}

// ActionNeedsApproval 是「这次调用要不要审批」的唯一入口。
func ActionNeedsApproval(toolName, argsJSON string) bool {
	return NeedsApproval(ActionRiskFromArgsJSON(toolName, argsJSON))
}

// ActionGrantSuffix 返回会话级授权键的动作后缀。
//
// 合并意图的授权粒度应该停在 action：用户对 content_article action=list 放行
// 一次，不该连带放行 save/delete。无 action 的工具返回 ""（授权键退回整工具/参数级）。
func ActionGrantSuffix(args map[string]any) string {
	return stringArg(args, "action")
}
