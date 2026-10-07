package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// ================================================================
// 路径安全门 (P0-1)
//
// 仿 atomcode write_approval.rs / sensitive_path.rs 的「解析后判定」，并做一处关键调整：
// atomcode 锚定用户 home + $ATOMCODE_HOME，本实现锚定**当前站点的 RootPath**
// （各站点有自己的 RootPath，见 provider/website.go）。
//
// 三类判定，顺序敏感：
//   1. 敏感 (Sensitive)      —— 密钥/凭据/配置文件，无论站内站外一律需审批，且永不记住授权
//   2. 站点内 (InRoot)       —— 解析后落在 RootPath 内且非敏感 → 自动放行
//   3. 站点外 (OutOfRoot)    —— 解析后落在 RootPath 外 → 需审批
//   4. 无法判定 (Undeterminable) —— RootPath 未配置或路径无法解析 → fail closed，需审批
//
// 关键点：必须先 filepath.Abs/Clean 再判定，否则 "a/../../etc/passwd" 这类
// `..` 逃逸与符号链接逃逸都能绕过子串匹配（原 SensitivePathGateMiddleware 的缺陷）。
// ================================================================

// PathClass 是工具目标路径相对站点 RootPath 的分类结果。
type PathClass int

const (
	// PathInRoot 解析后落在站点根目录内且非敏感 → 自动放行
	PathInRoot PathClass = iota
	// PathOutOfRoot 解析后落在站点根目录外 → 需审批
	PathOutOfRoot
	// PathSensitive 命中敏感标记 → 需审批，且授权永不记住 (fail closed)
	PathSensitive
	// PathUndeterminable 无法判定 (RootPath 未配置 / 路径无法解析) → 需审批
	PathUndeterminable
)

// String 便于日志与前端展示。
func (c PathClass) String() string {
	switch c {
	case PathInRoot:
		return "in_root"
	case PathOutOfRoot:
		return "out_of_root"
	case PathSensitive:
		return "sensitive"
	default:
		return "undeterminable"
	}
}

// NeedsApproval 该分类是否需要人工审批。
func (c PathClass) NeedsApproval() bool { return c != PathInRoot }

// ================================================================
// 敏感判定表
// ================================================================

// gateSensitiveMarkers 路径形标记（小写子串匹配）。刻意写成路径形状（带分隔符），
// 避免把普通内容词误判为敏感。
var gateSensitiveMarkers = []string{
	"/.ssh", "/.aws", "/.gnupg", "/.kube", "/.docker/config", "/.config/gcloud",
	".git-credentials", ".npmrc", ".pypirc", ".netrc",
	"/secrets/", "/.terraform.d",
	".git/", ".git\\",
	"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519",
	"credentials", "private.key",
}

// gateSecretFileNames 文件名精确命中即敏感（解析后的 basename 比较）。
var gateSecretFileNames = []string{
	".env", "config.toml", "config.yaml", "config.json", "config.yml",
	"wp-config.php", "database.yml", "database.yaml",
	"auth.toml", "token.json", "secrets", "credentials", ".git-credentials",
	"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519", "private.key",
	".npmrc", ".pypirc", ".netrc", ".htaccess",
	// shell 启动文件常被写入密钥/令牌，且能被后续命令静默读取
	".bashrc", ".bash_profile", ".zshrc", ".zprofile", ".zshenv",
}

// gateSecretExts 扩展名命中即敏感。
var gateSecretExts = []string{"pem", "key", "p12", "pfx", "der", "keystore"}

// gateEnvTemplateSuffixes 占位模板后缀：这些 .env 变体只含假值且已进版本库，
// 读取它们不是密钥泄露风险，不应弹审批（仿 atomcode ENV_TEMPLATE_SUFFIXES）。
var gateEnvTemplateSuffixes = []string{"example", "sample", "template", "dist", "defaults"}

// gateSystemProtectedPrefixes 系统保护前缀（仿 atomcode SYSTEM_PROTECTED_PREFIXES）。
// 带 /private 变体是因为 macOS 上 /etc、/var、/usr 都是指向 /private/* 的符号链接，
// EvalSymlinks 之后会变成 /private/etc —— 只写 /etc 会整条漏掉。
var gateSystemProtectedPrefixes = []string{
	"/etc", "/private/etc",
	"/root", "/var/root", "/private/var/root",
	"/usr", "/private/usr",
	"/bin", "/sbin", "/lib",
	"/sys", "/proc", "/dev", "/boot",
	"/var", "/private/var",
}

// gateSystemProtectedExceptions 命中这些前缀的不算系统保护
// （仿 atomcode SYSTEM_PROTECTED_EXCEPTIONS —— 遗漏它曾导致整类误判）。
//
// 为什么必须有：/usr/local、/var/folders、/Users 这些位置既有系统路径也有大量
// **用户站点与临时目录**。若不加例外，任何部署在 /var/www 或 /usr/local/site 下的
// 站点，其全部站内文件都会被判成敏感 —— 站内自动放行彻底失效。
var gateSystemProtectedExceptions = []string{
	"/usr/local", "/private/usr/local",
	"/Applications", "/Library",
	"/var/folders", "/private/var/folders",
	"/var/tmp", "/private/var/tmp",
	"/tmp", "/private/tmp",
	"/Users", "/home", "/opt", "/srv", "/data",
}

// ================================================================
// 解析与判定
// ================================================================

// IsSecretPath 判断一个**已解析为绝对路径**的路径是否是密钥/凭据类。
// 同时看标记串与解析后的 basename / 扩展名 —— 后者能抓住相对路径 `.ssh/config`
// 与 Windows 反斜杠写法（子串匹配做不到）。
//
// 只管「是不是密钥」，**不管**「是不是系统路径」：系统路径的判定见
// IsSystemProtectedPath，且只在路径位于站点外时才生效（否则部署在 /var/www 的
// 站点会被整站误判）。
func IsSecretPath(abs string) bool {
	if abs == "" {
		return false
	}
	slashed := filepath.ToSlash(abs)
	lower := strings.ToLower(slashed)
	base := strings.ToLower(filepath.Base(abs))

	// ── .env 特殊处理 ──
	// `.env` 精确命中敏感；`.env.local` / `.env.production` 敏感；
	// `.env.example` / `.env.sample` 等模板不敏感。
	if base == ".env" {
		return true
	}
	if strings.HasPrefix(base, ".env.") {
		suffix := base[len(".env."):]
		// 只取字母数字前缀（遇到其他字符停止，如 `.env.local.bak`）
		var sb strings.Builder
		for _, r := range suffix {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				sb.WriteRune(r)
			} else {
				break
			}
		}
		key := sb.String()
		isTemplate := false
		for _, t := range gateEnvTemplateSuffixes {
			if key == t {
				isTemplate = true
				break
			}
		}
		if !isTemplate {
			return true
		}
	}

	// 文件名精确命中
	for _, n := range gateSecretFileNames {
		if base == n {
			return true
		}
	}

	// 扩展名命中
	if ext := strings.TrimPrefix(filepath.Ext(base), "."); ext != "" {
		for _, e := range gateSecretExts {
			if ext == e {
				return true
			}
		}
	}

	// 路径形标记子串命中
	for _, m := range gateSensitiveMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}

	return false
}

// IsSystemProtectedPath 判断已解析的绝对路径是否属于系统保护位置。
// 例外优先：临时目录、/usr/local、/Users 等既存放系统文件也存放用户站点，
// 一律判成保护路径会让这些位置下的站点彻底不可用。
func IsSystemProtectedPath(abs string) bool {
	if abs == "" {
		return false
	}
	slashed := filepath.ToSlash(abs)
	for _, e := range gateSystemProtectedExceptions {
		if slashed == e || strings.HasPrefix(slashed, e+"/") {
			return false
		}
	}
	for _, p := range gateSystemProtectedPrefixes {
		if slashed == p || strings.HasPrefix(slashed, p+"/") {
			return true
		}
	}
	return false
}

// isUnderRoot 判断路径是否落在 root 内（含 root 自身）。
// 用带分隔符的前缀比较，避免 `/data/site` 误匹配 `/data/site-evil`。
func isUnderRoot(p, root string) bool {
	if p == "" || root == "" {
		return false
	}
	cleanRoot := filepath.Clean(root)
	return p == cleanRoot || strings.HasPrefix(p, cleanRoot+string(filepath.Separator))
}

// ResolveToolPath 把工具参数里的原始路径解析为绝对路径，并解除符号链接。
// 返回 (解析后绝对路径, 是否成功)。失败时调用方应 fail closed。
func ResolveToolPath(rawPath, root string) (string, bool) {
	if strings.TrimSpace(rawPath) == "" {
		return "", false
	}
	// 归一化分隔符（Windows 反斜杠、URL 式斜杠）
	p := strings.ReplaceAll(rawPath, "\\", "/")

	var base string
	switch {
	case p == "~" || strings.HasPrefix(p, "~/"):
		// home 目录在站点根之外，`rm -rf ~` 必须是站外而不是站内。
		// 拿不到 home 就 fail closed，绝不退化成「相对站点根」。
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", false
		}
		if p == "~" {
			base = home
		} else {
			base = filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
	case filepath.IsAbs(p):
		base = p
	default:
		if root == "" {
			// 无法锚定站点根目录 → 不猜，fail closed
			return "", false
		}
		base = filepath.Join(root, p)
	}

	// Abs 内部会 Clean，可解开 `..` 与 `.` 的字面逃逸
	abs, err := filepath.Abs(base)
	if err != nil {
		return "", false
	}
	// 符号链接逃逸：路径存在时才 EvalSymlinks（不存在则保持清理后的路径）
	if real, err := filepath.EvalSymlinks(abs); err == nil && real != "" {
		abs = real
	}
	return abs, true
}

// ClassifyPath 把原始路径解析后相对 root 分类。root 为空时只能判定敏感，
// 其余一律 PathUndeterminable（fail closed）。
func ClassifyPath(rawPath, root string) PathClass {
	if strings.TrimSpace(rawPath) == "" {
		return PathUndeterminable
	}
	abs, ok := ResolveToolPath(rawPath, root)
	if !ok {
		return PathUndeterminable
	}

	// 符号链接解析：路径已存在时按**真实位置**判定，
	// 否则站内一个指向外部的链接会被当成站内路径放行。
	resolved := abs
	if real, err := filepath.EvalSymlinks(abs); err == nil && real != "" {
		resolved = real
	}

	// ── 判定顺序是这里的全部要点 ──
	//
	// 1) 密钥/凭据类：站内站外都敏感（站点自己的 config.toml 同样含数据库口令）
	if IsSecretPath(resolved) {
		return PathSensitive
	}
	// 2) 站内：先于系统路径判定，否则部署在 /var/www、/usr/local/site 的站点
	//    会被整站误判为敏感（曾实测踩到：t.TempDir() 在 /var/folders 下，
	//    导致站内文件全被判敏感、站内自动放行失效）。
	if isUnderRoot(resolved, root) {
		return PathInRoot
	}
	// 3) 站外的系统保护位置：升级为敏感（永不被授权记住）
	if IsSystemProtectedPath(resolved) {
		return PathSensitive
	}
	if root == "" {
		// 没有 RootPath 就无法证明「站内」，fail closed
		return PathUndeterminable
	}
	return PathOutOfRoot
}

// ================================================================
// 工具参数 → 目标路径提取
// ================================================================

// gatePathFields 工具参数中表示「路径」的字段名。
var gatePathFields = []string{
	"file_path", "filepath", "path", "file", "dir", "directory",
	"target", "source", "destination", "dest", "old_path", "new_path",
	"glob", "search", "pattern", "script",
}

// gateCommandFields 需要从中抽取路径的词法字段（bash 类）。
var gateCommandFields = []string{"command", "cmd", "command_line"}

// gatePathGatedTools 需要做路径分类的工具。
// 注意：除了写工具，还包含 read_file/grep/glob/list_directory 这类**只读工具** ——
// 只读工具读走 ~/.ssh/id_rsa 或 .env 后，内容会随 tool_result 直送模型供应商，
// 属于密钥外泄（正是 atomcode SensitivePathGate 存在的理由）。
var gatePathGatedTools = map[string]bool{
	"read_file":      true,
	"write_file":     true,
	"edit_file":      true,
	"search_replace": true,
	"bash":           true,
	"grep":           true,
	"glob":           true,
	"list_directory": true,
}

// gateCommandShellTools bash 类工具：能执行任意命令，作用域不等于它提到的某个路径。
// 因此这类工具**永不因「路径在站内」而被自动放行**（`rm -rf template/ && curl ...`
// 同样提到站内路径），路径判定只用于升级为审批 / 标记敏感。
var gateCommandShellTools = map[string]bool{
	"bash":         true,
	"bash_start":   true,
	"shell":        true,
	"shell_exec":   true,
	"execute_bash": true,
	"run_command":  true,
}

// IsCommandShellTool 判断工具是否为命令执行类（可任意副作用）。
func IsCommandShellTool(toolName string) bool {
	return gateCommandShellTools[strings.ToLower(strings.TrimSpace(toolName))]
}

// pathGateCapName 把工具名解析成路径门认识的能力名；返回 "" 表示该工具无需路径分类。
//
// 意图模式下 fs_write/fs_edit/fs_replace/fs_read/grep/glob/list_directory/shell_exec
// 各自单一委托到一个内置能力（spec.Caps 只有一项），分类字段（file_path/command/…）
// 原样透传，所以按能力名分类是准确的。只在"单一委托"时解析：多能力意图的参数
// 语义不由某一个能力决定，硬套反而会把无关字段当路径分类。
func pathGateCapName(toolName string) string {
	n := strings.ToLower(strings.TrimSpace(toolName))
	if gatePathGatedTools[n] {
		return n
	}
	spec, ok := intent.SpecByName(n)
	if !ok || len(spec.Caps) != 1 {
		return ""
	}
	if gatePathGatedTools[spec.Caps[0]] {
		return spec.Caps[0]
	}
	return ""
}

// ToolCanAutoApproveInRoot 该工具在「站内 + 非敏感」时是否允许自动放行。
// 命令执行类一律 false —— 自动放行必须留给**作用域等于目标路径**的文件类工具。
func ToolCanAutoApproveInRoot(toolName string) bool {
	if IsCommandShellTool(toolName) {
		return false
	}
	return gatePathGatedTools[strings.ToLower(strings.TrimSpace(toolName))]
}

// ExtractToolTargets 从工具参数 JSON 中提取所有目标路径，去重并排序（保证授权键可复现）。
// 提取失败（非法 JSON）时回落到宽松扫描，绝不静默返回空——空会导致授权键退化。
func ExtractToolTargets(toolName, args string) []string {
	if strings.TrimSpace(args) == "" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		// 去引号
		v = strings.Trim(v, `"'`)
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		out = append(out, v)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(args), &raw); err == nil {
		for k, v := range raw {
			kl := strings.ToLower(k)
			if isInList(kl, gatePathFields) {
				collectPathStrings(v, add)
			} else if isInList(kl, gateCommandFields) {
				if s, ok := v.(string); ok {
					for _, tok := range ExtractBashPaths(s) {
						add(tok)
					}
				}
			}
		}
	} else {
		// 回落：宽松扫描（保持原 extractPathFromArgs 的能力，不因解析失败而放行）
		for _, f := range gatePathFields {
			for _, v := range scanJSONField(args, f) {
				add(v)
			}
		}
		for _, f := range gateCommandFields {
			for _, v := range scanJSONField(args, f) {
				for _, tok := range ExtractBashPaths(v) {
					add(tok)
				}
			}
		}
	}

	sort.Strings(out)
	return out
}

// collectPathStrings 递归收集值中的字符串（支持字符串/数组/嵌套对象）。
func collectPathStrings(v interface{}, add func(string)) {
	switch t := v.(type) {
	case string:
		add(t)
	case []interface{}:
		for _, e := range t {
			collectPathStrings(e, add)
		}
	case map[string]interface{}:
		for _, e := range t {
			collectPathStrings(e, add)
		}
	}
}

// ExtractBashPaths 从 shell 命令中抽取「看起来像路径」的词元。
// 仿 atomcode 的按目标键控：命令里的路径才是授权的作用域，而不是整条命令。
func ExtractBashPaths(cmd string) []string {
	var out []string
	seen := map[string]bool{}
	fields := strings.FieldsFunc(cmd, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '"' || r == '\'' || r == '`'
	})
	for _, f := range fields {
		f = strings.Trim(f, "\"'")
		if f == "" || strings.HasPrefix(f, "-") {
			continue
		}
		// 仅保留像路径/文件名的词元：含分隔符、或以 ~ / . 开头
		if !strings.ContainsAny(f, "/\\") && !strings.HasPrefix(f, "~") && !strings.HasPrefix(f, ".") {
			continue
		}
		if seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// isInList 小写列表包含判定。
func isInList(v string, list []string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// scanJSONField 宽松扫描 JSON 字符串中指定字段的值（解析失败时的回落路径）。
func scanJSONField(args, field string) []string {
	var out []string
	lower := strings.ToLower(args)
	needle := `"` + strings.ToLower(field) + `"`
	search := lower
	for {
		idx := strings.Index(search, needle)
		if idx < 0 {
			break
		}
		rest := args[idx:]
		colon := strings.IndexByte(rest, ':')
		if colon < 0 {
			break
		}
		after := strings.TrimLeft(rest[colon+1:], " \t\n\r")
		if after == "" {
			if len(search) > idx+len(needle) {
				search = search[idx+len(needle):]
				continue
			}
			break
		}
		if after[0] == '"' || after[0] == '\'' {
			q := after[0]
			if end := strings.IndexByte(after[1:], q); end >= 0 {
				out = append(out, after[1:1+end])
			}
		}
		if len(search) > idx+len(needle) {
			search = search[idx+len(needle):]
			continue
		}
		break
	}
	return out
}

// ClassifyToolTargets 对工具的所有目标分类，返回**最严格**的那一类
// （敏感 > 无法判定 > 站外 > 站内），保证不会因为某一个目标安全就整体放行。
func ClassifyToolTargets(toolName, args, root string) (PathClass, []string) {
	targets := ExtractToolTargets(toolName, args)
	if len(targets) == 0 {
		// 没有可识别目标 → 无法判定，fail closed（不静默放行）
		return PathUndeterminable, nil
	}
	worst := PathInRoot
	for _, t := range targets {
		c := ClassifyPath(t, root)
		if classRank(c) > classRank(worst) {
			worst = c
		}
	}
	return worst, targets
}

// classRank 分类的严格程度，数值越大越严格。
func classRank(c PathClass) int {
	switch c {
	case PathInRoot:
		return 0
	case PathOutOfRoot:
		return 1
	case PathUndeterminable:
		return 2
	case PathSensitive:
		return 3
	}
	return 3
}
