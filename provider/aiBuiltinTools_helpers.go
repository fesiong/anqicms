package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// ================================================================
//  Parameter Recovery — lenient JSON parsing for LLM output
// ================================================================

// recoverJSON attempts to fix common LLM output issues before JSON decoding:
//   - Strip markdown code fences (```json ... ```)
//   - Strip leading/trailing text outside { }
//   - Handle single trailing comma before closing brace
//   - Unescape unicode sequences
func recoverJSON(raw string) string {
	raw = strings.TrimSpace(raw)

	// Strip markdown code fences
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		if idx := strings.LastIndex(raw, "```"); idx >= 0 {
			raw = raw[:idx]
		}
		raw = strings.TrimSpace(raw)
	}

	// Find the outermost { ... } block
	start := strings.Index(raw, "{")
	if start < 0 {
		return raw
	}
	end := strings.LastIndex(raw, "}")
	if end <= start {
		return raw
	}
	raw = raw[start : end+1]

	// Fix trailing comma before } or ]
	raw = regexp.MustCompile(`,(\s*[}\]])`).ReplaceAllString(raw, "$1")

	return raw
}

// lenientInt parses a JSON value that could be a number, string number, or null.
// Handles common LLM output issues like "50" or 50.0 for an int field.
func lenientInt(v interface{}) (int, bool) {
	if v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return int(val), true
	case int:
		return val, true
	case string:
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
			return n, true
		}
	}
	return 0, false
}

// lenientString extracts a string from a JSON value.
func lenientString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%v", val)
	default:
		b, _ := json.Marshal(val)
		return strings.Trim(string(b), "\"")
	}
}

// lenientBool extracts a bool from a JSON value.
func lenientBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true" || val == "1"
	case float64:
		return val != 0
	}
	return false
}

// rawJSONUnmarshal does a two-pass parse: first as raw map to recover,
// then into the target struct. Returns true if successful.
func rawJSONUnmarshal(data []byte, target interface{}) error {
	// First try standard
	if err := json.Unmarshal(data, target); err == nil {
		return nil
	}

	// Recover and retry
	recovered := recoverJSON(string(data))
	if err := json.Unmarshal([]byte(recovered), target); err == nil {
		return nil
	}

	// Last resort: unmarshal into map and copy fields
	var rawMap map[string]interface{}
	if err := json.Unmarshal([]byte(recovered), &rawMap); err != nil {
		return fmt.Errorf("无法解析JSON参数: %s", err.Error())
	}

	// Re-encode as clean JSON and unmarshal into target
	clean, _ := json.Marshal(rawMap)
	return json.Unmarshal(clean, target)
}

// ================================================================
//  Enhanced Path Security
// ================================================================

// sensitivePathPatterns lists system paths that should never be written to.
var sensitivePathPatterns = []string{
	"/etc/",
	"/sys/",
	"/proc/",
	"/dev/",
	"/boot/",
	"/usr/",
	"/bin/",
	"/sbin/",
	"/lib/",
	"/var/",
	"/tmp/",
	"/root/",
	"~/.ssh",
	"/.ssh",
}

var sensitivePathRe *regexp.Regexp

func initSensitivePaths() {
	// Only for non-Windows
	sensitivePathRe = regexp.MustCompile(strings.Join(sensitivePathPatterns, "|"))
}

func isSensitiveInputPath(path string) bool {
	if sensitivePathRe == nil {
		initSensitivePaths()
	}
	return sensitivePathRe.MatchString(path)
}

// safePathResolve resolves a path relative to projectRoot and validates it.
// More robust than the original safePath — returns friendly error messages.
// All paths are bounded to projectRoot; temp files should use ensureCachePath() instead.
func safePathResolve(path, projectRoot string) (string, error) {
	root := projectRoot
	if root == "" {
		return "", fmt.Errorf("projectRoot 未配置")
	}

	p := path
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	p, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("无法解析路径 '%s': %w", path, err)
	}

	if !strings.HasPrefix(p, root) {
		return "", fmt.Errorf("路径 '%s' 超出项目目录范围", path)
	}

	// Check for symlink escape
	real, err := filepath.EvalSymlinks(p)
	if err == nil && !strings.HasPrefix(real, root) {
		return "", fmt.Errorf("路径 '%s' 通过符号链接指向项目外部", path)
	}

	return p, nil
}

// ensureCachePath returns the cache directory path and ensures it exists.
// All temporary/generated files (scripts, build output, etc.) should be written here.
func ensureCachePath(projectRoot string) (string, error) {
	cp := filepath.Join(projectRoot, "cache")
	if err := os.MkdirAll(cp, 0755); err != nil {
		return "", fmt.Errorf("创建缓存目录失败: %w", err)
	}
	return cp, nil
}

// ================================================================
//  File-Not-Found Suggestions
// ================================================================

// findSimilarFiles searches up to maxResults files with the same basename
// within the project, ranked by shared path prefix similarity.
func findSimilarFiles(requestedPath string, maxResults int, projectRoot string) []string {
	filename := filepath.Base(requestedPath)
	if filename == "" || filename == "." || filename == "/" {
		return nil
	}

	var matches []string
	searchDepth := 7

	var walkFn func(dir string, depth int)
	walkFn = func(dir string, depth int) {
		if depth > searchDepth || len(matches) >= maxResults {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if len(matches) >= maxResults {
				return
			}
			name := entry.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				continue
			}
			if entry.IsDir() {
				walkFn(filepath.Join(dir, name), depth+1)
			} else if name == filename {
				matches = append(matches, filepath.Join(dir, name))
			}
		}
	}

	walkFn(projectRoot, 0)

	// Rank by shared path prefix length
	sort.Slice(matches, func(i, j int) bool {
		return sharedPrefixLen(requestedPath, matches[i]) > sharedPrefixLen(requestedPath, matches[j])
	})

	if len(matches) > 5 {
		matches = matches[:5]
	}
	return matches
}

func sharedPrefixLen(a, b string) int {
	partsA := strings.Split(filepath.ToSlash(a), "/")
	partsB := strings.Split(filepath.ToSlash(b), "/")
	minLen := len(partsA)
	if len(partsB) < minLen {
		minLen = len(partsB)
	}
	count := 0
	for i := 0; i < minLen; i++ {
		if partsA[i] == partsB[i] {
			count++
		} else {
			break
		}
	}
	return count
}

// ================================================================
//  Read Cache (mtime-based)
// ================================================================

type readCacheEntry struct {
	mtime  time.Time
	output string
}

var readFileCache sync.Map // key: "path|offset|limit|budget" → *readCacheEntry

// 缓存键必须含 budget：同一 (path, offset, limit) 在不同预算下渲染结果不同，
// 漏掉就会让换了配置的服务实例读到上一份被切得更短（或更长）的窗口。
func getReadCache(path string, offset, limit, budget int, mtime time.Time) (string, bool) {
	key := readCacheKey(path, offset, limit, budget)
	if val, ok := readFileCache.Load(key); ok {
		entry := val.(*readCacheEntry)
		if entry.mtime.Equal(mtime) {
			return entry.output, true
		}
	}
	return "", false
}

func setReadCache(path string, offset, limit, budget int, mtime time.Time, output string) {
	readFileCache.Store(readCacheKey(path, offset, limit, budget), &readCacheEntry{mtime: mtime, output: output})
}

func readCacheKey(path string, offset, limit, budget int) string {
	return fmt.Sprintf("%s|%d|%d|%d", path, offset, limit, budget)
}

func invalidateReadCache(path string) {
	// Invalidate all cache entries for this path
	readFileCache.Range(func(key, value interface{}) bool {
		k := key.(string)
		if strings.HasPrefix(k, path+"|") {
			readFileCache.Delete(key)
		}
		return true
	})
}

// ================================================================
//  Result Window —— 分页工具自持字节预算
//
//  分页工具必须自己决定「这一次返回多少」，并在结尾如实说明返回了第几项到
//  第几项、共几项、以及怎么续读。否则中间件会在工具承诺之后按字节盲切，
//  模型拿到的是一段被腰斩、看起来却完整的输出——公开产品不能这样。
//  切点恒在条目边界（行 / 匹配 / 文件），预算与 ResultTruncatorMiddleware 同源。
// ================================================================

// windowTrailerReserve 是给结尾说明预留的字节数：header + body + trailer
// 必须整体落在预算内，中间件那道盲切才不会被触发。按最长的
// 「窗口说明 + 采集上限补充说明」留足余量。
const windowTrailerReserve = 512

// errScanDone 用于提前终止 filepath.Walk：收集够了就停，不必遍历剩余目录。
var errScanDone = errors.New("scan done")

// ================================================================
// glob 路径匹配
//
// 背景：glob 工具原来的实现有两处缺陷，导致 desc 宣称的语法大半不成立
//（2026-10-03 实测：'**/*.go'、'pkg/mcp/intent/*.go'、'template/**' 全部返回「未找到」）：
//
//  1. 非 `**` 分支只拿 basename 匹配（filepath.Match(pattern, fi.Name())），
//     所以任何带目录前缀的 pattern 永远匹配不到；
//  2. `**` 分支被拆成 prefix/suffix 后用 strings.HasPrefix/HasSuffix 判断 ——
//     那两个函数不做通配，`**/*.go` 的 suffix `*.go` 永远匹配不上；
//     且只有恰好一个 `**` 时才处理，`a/**/b/**/c` 直接失效。
//
// filepath.Match 本身语法是对的（支持 * ? [...]），只是每段匹配、
// 不跨分隔符，所以需要按 / 分段后再递归组合。
// ================================================================

// matchGlobPattern 判断项目内相对路径 rel 是否匹配 pattern。
//
// 语义（刻意区分两类 pattern，兼顾正确性与向后兼容）：
//   - pattern 不含 '/'：视为「文件名模式」，匹配任意层级的文件名。
//     沿用旧行为，否则 '*.go' 会从「全项目 501 个」塌缩成「仅顶层 20 个」，
//     那是对现有调用方的破坏性变更。
//   - pattern 含 '/'：视为「路径模式」，按 / 分段匹配，'**' 可跨任意层级；
//     段内仍支持 * ? [...]（委托 filepath.Match）。
func matchGlobPattern(pattern, rel string) bool {
	pattern = normalizeGlobPath(pattern)
	rel = normalizeGlobPath(rel)
	if !strings.Contains(pattern, "/") {
		if rel == "" {
			return false
		}
		ok, err := filepath.Match(pattern, path.Base(rel))
		return err == nil && ok
	}
	return matchGlobSegments(splitGlobSegments(pattern), splitGlobSegments(rel))
}

// normalizeGlobPath 把反斜杠统一成 /。
//
// 不能用 filepath.ToSlash：它在非 Windows 平台是 no-op（#os 语义），
// 本项目跑在 macOS/Linux 上，ToSlash(`a\b`) 原样返回 `a\b`，
// 于是 Windows 风格的 pattern 会被当成单段文件名而永不匹配。
func normalizeGlobPath(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// splitGlobSegments 按 / 切分并丢弃空段。
//
// 丢弃空段是必要的，不是洁癖：'**/catalog_*.go' 以 ** 开头，若保留首段空串，
// ** 吃掉零层后会剩下「空段 + catalog_*.go」去匹配单段的 'catalog.go' 而失配
//（这是第一版实现被测试抓到的真 bug）。标准 glob 语义里 'a//b' 等价于 'a/b'，
// 所以丢弃是正确的。
func splitGlobSegments(p string) []string {
	parts := strings.Split(p, "/")
	out := parts[:0]
	for _, s := range parts {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// matchGlobSegments 逐段匹配。pat 对应 seg 的当前位置。
//
// '**' 匹配 0..n 个路径段（标准 glob 语义），实现上枚举它可能吃掉多少段后递归。
// 递归而非迭代：'a/**/b/**/c' 这类多个 '**' 只有递归才表达得了
//（旧实现要求恰好一个 **，多个直接不进分支）。
//
// 末尾的 '**' 直接吞掉剩余全部段，因此 seg 是否还有元素都算匹配 —— 这正是
// 'template/**' 能匹配 'template' 本身（零层）的原因。
func matchGlobSegments(pat, seg []string) bool {
	if len(pat) == 0 {
		return len(seg) == 0
	}
	if pat[0] == "**" {
		if len(pat) == 1 {
			return true
		}
		for i := 0; i <= len(seg); i++ {
			if matchGlobSegments(pat[1:], seg[i:]) {
				return true
			}
		}
		return false
	}
	if len(seg) == 0 {
		return false
	}
	ok, err := filepath.Match(pat[0], seg[0])
	if err != nil || !ok {
		return false
	}
	return matchGlobSegments(pat[1:], seg[1:])
}

// scanCollectCap 是收集类工具（grep/glob）一次遍历最多收集的条目数。
// 达到上限后不再继续遍历，但会在结果里写明「已达采集上限」——
// 静默丢掉后半段才是问题，明确的上限不是：真正的收敛手段是收窄 pattern/glob。
const scanCollectCap = 2000

// resultBudget 返回工具结果的字节预算（按 ChatTuning 补齐默认值）。
func (svc *AiChatService) resultBudget() int {
	return svc.Tuning.Normalized().MaxToolResultBytes
}

// fitWindow 从 items 开头逐条渲染并累加，直到加入下一条会超出 budget。
// 恒至少渲染一条：若一条都放不下就返回空，续读游标会原地打转。
func fitWindow[T any](items []T, budget int, render func(i int, item T) string) (body string, delivered int) {
	var b strings.Builder
	used := 0
	for i, item := range items {
		s := render(i, item)
		if delivered > 0 && used+len(s) > budget {
			break
		}
		b.WriteString(s)
		used += len(s)
		delivered++
	}
	return b.String(), delivered
}

// windowTrailer 如实说明本次真实交付的区间：读到哪了、还剩多少、怎么续读。
// budgetCut 表示这一刀是字节预算切的（而不是请求区间或全集的自然末尾）——
// 两种情况都必须区分开，否则模型会把「按自己要求取的小窗口」误当成读完了，
// 或更糟：把「被预算腰斩的窗口」当成完整结果。
func windowTrailer(unit string, first, last, total, budget int, budgetCut bool) string {
	if last >= total {
		return fmt.Sprintf("\n\n[已返回第 %d–%d %s（共 %d %s，已全部返回）]", first, last, unit, total, unit)
	}
	reason := "已到本次请求区间末尾"
	if budgetCut {
		reason = fmt.Sprintf("本次上限 %d 字节", budget)
	}
	return fmt.Sprintf("\n\n[已返回第 %d–%d %s（共 %d %s，%s）。续读：offset=%d]",
		first, last, unit, total, unit, reason, last+1)
}

// capNonPagingResult 给「没有 offset 参数」的工具输出（命令输出、网页正文、搜索结果）
// 按同一份预算做裁剪。这类结果一旦被裁，尾部默认找不回来，所以标记必须写明原始规模：
// 模型知道丢了多少，才会去收窄命令或改用带 offset 的工具，而不是以为自己看完了。
// archiveRelPath 非空时（网络类工具），全文另存到了项目内的临时文件，标记改为给出
// 续读路径；为空时（bash）如实说明「不可续读」。
func capNonPagingResult(content string, budget int, archiveRelPath string) string {
	if len(content) <= budget {
		return content
	}
	keep := budget - windowTrailerReserve
	if keep < 0 {
		keep = 0
	}
	cut := keep
	for cut > 0 && !utf8.RuneStart(content[cut]) {
		cut--
	}
	// 尽量落在行边界：半行内容比少一行更难被误读成完整记录
	if nl := strings.LastIndexByte(content[:cut], '\n'); nl > keep/2 {
		cut = nl + 1
	}
	delivered := strings.Count(content[:cut], "\n") // 已完整交付的行数
	totalLines := countedLines(content)
	if archiveRelPath != "" {
		return content[:cut] + fmt.Sprintf(
			"\n\n[输出共 %d 字节 / %d 行，本次仅返回前 %d 字节（已完整覆盖前 %d 行）。"+
				"完整内容已存档为项目文件：%s —— 续读用 read_file path=%s offset=%d，"+
				"或先 grep path=%s pattern=<关键词> 定位再取区间]",
			len(content), totalLines, cut, delivered,
			archiveRelPath, archiveRelPath, delivered+1, archiveRelPath)
	}
	return content[:cut] + fmt.Sprintf(
		"\n\n[输出共 %d 字节 / %d 行，超出单次上限，仅保留前 %d 字节（已完整覆盖前 %d 行）；"+
			"尾部已丢弃且无法续读。请在命令里用 head/tail/grep/wc 自行收窄范围后重试]",
		len(content), totalLines, cut, delivered)
}

// countedLines 按 read_file 同一套行模型数行数：以换行结尾的文件不会有额外的空尾行。
// 两处口径不一致的话，标记里给的续读行号就会错位。
func countedLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// 网络类工具（web_fetch / web_search）没有 offset 参数，被裁掉的尾部默认永久丢失。
// 存档把全文落到项目内的临时文件，让 read_file/grep 能把「已丢弃」变成「可再取」。
// cache/ 目录已被 cache/.gitignore 全量忽略，不会污染用户仓库。
const (
	archivedResultDir      = "cache/ai-results"
	archivedResultTTL      = 6 * time.Hour
	archivedResultMaxFiles = 40
	// 存档本身也要有上限：网页响应已限 2MB，这里再兜一层，避免个别站点巨型裸文本把磁盘写满。
	archivedResultMaxBytes = 8 << 20
)

// capWebResult 是网络类工具的收尾：预算内返回前段 + 全文存档 + 给出续读坐标。
// 预算内直接原样返回，不落任何文件——存档只为「确实丢了东西」的情况存在。
func (svc *AiChatService) capWebResult(toolName string, content string) string {
	budget := svc.resultBudget()
	if len(content) <= budget {
		return content
	}
	return capNonPagingResult(content, budget, svc.archiveToolOutput(toolName, content))
}

// archiveToolOutput 把全文写进 cache/ai-results 并返回相对项目根的路径；任何失败都返回
// 空串，调用方据此退回「不可续读」的措辞——宁可承认丢了，也不能谎报能续读。
func (svc *AiChatService) archiveToolOutput(toolName, content string) string {
	if svc.projectRoot == "" || len(content) > archivedResultMaxBytes {
		return ""
	}
	dir := filepath.Join(svc.projectRoot, archivedResultDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ""
	}
	svc.pruneArchivedResults(dir)
	// 存档同样要脱敏：网页与接口响应里可能带着密钥，落盘等于把它写进用户的磁盘。
	body := redactSecrets(content)
	name := fmt.Sprintf("%s-%d.txt", toolName, time.Now().UnixNano())
	full := filepath.Join(dir, name)
	if err := os.WriteFile(full, []byte(body), 0600); err != nil {
		return ""
	}
	rel, err := filepath.Rel(svc.projectRoot, full)
	if err != nil {
		return filepath.ToSlash(full)
	}
	return filepath.ToSlash(rel)
}

// pruneArchivedResults 在每次写入前顺手清理过期与超量的旧存档。
// 不引入后台协程：这些文件的唯一读者是模型自己，写入时清一次就够。
func (svc *AiChatService) pruneArchivedResults(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type archived struct {
		path string
		mod  time.Time
	}
	var alive []archived
	now := time.Now()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if now.Sub(info.ModTime()) > archivedResultTTL {
			_ = os.Remove(path)
			continue
		}
		alive = append(alive, archived{path, info.ModTime()})
	}
	if len(alive) <= archivedResultMaxFiles {
		return
	}
	sort.Slice(alive, func(i, j int) bool { return alive[i].mod.Before(alive[j].mod) })
	for _, a := range alive[:len(alive)-archivedResultMaxFiles] {
		_ = os.Remove(a.path)
	}
}

// ================================================================
//  Skeleton Mode
// ================================================================

// SkeletonThreshold — files above this line count get a skeleton view.
const SkeletonThreshold = 300

// FileSymbol represents a top-level symbol found during skeleton analysis.
type FileSymbol struct {
	Name      string
	Kind      string
	StartLine int
	EndLine   int
}

// buildSkeleton generates a compact symbol-based overview for large files.
// 返回空串表示该文件不适用骨架视图（非 Go 文件或没解析出符号）。
func buildSkeleton(data []byte, path, relPath string, totalLines, budget int) string {
	// Only show skeleton for Go files
	if !strings.HasSuffix(path, ".go") {
		return ""
	}

	// Parse symbols using simple line-by-line analysis
	// (we can't use go/parser here since the file might not compile)
	lines := strings.Split(string(data), "\n")
	var symbols []FileSymbol
	inBlock := false
	var currentSym *FileSymbol
	braceDepth := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineNum := i + 1

		// Detect function/method declarations
		if !inBlock {
			var name, kind string
			if strings.HasPrefix(trimmed, "func ") {
				// func Name(...)
				if idx := strings.Index(trimmed, "("); idx > 5 {
					name = strings.TrimSpace(trimmed[5:idx])
					// Handle methods: func (r *T) Name(...)
					if strings.HasPrefix(name, "(") {
						if closeIdx := strings.Index(name, ")"); closeIdx >= 0 {
							name = strings.TrimSpace(name[closeIdx+1:])
							// Get the method receiver type
							recv := strings.TrimSpace(name[:closeIdx+1])
							if dotIdx := strings.Index(name, "."); dotIdx >= 0 {
								name = name[dotIdx+1:]
							}
							name = fmt.Sprintf("(%s) %s", recv, name)
						}
					}
				}
				kind = "func"
				if idx := strings.Index(name, "("); idx >= 0 {
					name = name[:idx]
				}
				if strings.HasPrefix(name, "(") {
					// Method — clean up the receiver
					if closeIdx := strings.LastIndex(name, ")"); closeIdx >= 0 {
						name = strings.TrimSpace(name[closeIdx+1:])
					}
				}
				if name != "" {
					currentSym = &FileSymbol{Name: name, Kind: kind, StartLine: lineNum}
					inBlock = true
					braceDepth = 0
				}
			} else if strings.HasPrefix(trimmed, "type ") {
				// type Name ...
				if strings.Contains(trimmed, " ") {
					parts := strings.Fields(trimmed)
					if len(parts) >= 2 && parts[1] != "" && parts[1] != "struct" && !strings.HasPrefix(parts[1], "{") {
						name := parts[1]
						kind := "type"
						if strings.Contains(trimmed, "struct {") || trimmed[len(trimmed)-1] == '{' {
							currentSym = &FileSymbol{Name: name, Kind: kind, StartLine: lineNum}
							inBlock = true
							braceDepth = 0
						} else {
							symbols = append(symbols, FileSymbol{Name: name, Kind: kind, StartLine: lineNum, EndLine: lineNum})
						}
					}
				}
			} else if strings.HasPrefix(trimmed, "var ") && strings.Contains(trimmed, "=") {
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					kind := "var"
					if strings.HasPrefix(parts[1], "(") {
						// var ( ... )
						currentSym = &FileSymbol{Name: "var block", Kind: kind, StartLine: lineNum}
						inBlock = true
						braceDepth = 0
					} else {
						symbols = append(symbols, FileSymbol{Name: parts[1], Kind: kind, StartLine: lineNum, EndLine: lineNum})
					}
				}
			} else if strings.HasPrefix(trimmed, "const ") && strings.Contains(trimmed, "=") {
				parts := strings.Fields(trimmed)
				if len(parts) >= 2 {
					if strings.HasPrefix(parts[1], "(") {
						currentSym = &FileSymbol{Name: "const block", Kind: "const", StartLine: lineNum}
						inBlock = true
						braceDepth = 0
					} else {
						symbols = append(symbols, FileSymbol{Name: parts[1], Kind: "const", StartLine: lineNum, EndLine: lineNum})
					}
				}
			}
		} else {
			// Track brace depth to find end of block
			for _, ch := range trimmed {
				if ch == '{' {
					braceDepth++
				} else if ch == '}' {
					braceDepth--
				}
			}
			if braceDepth <= 0 && (strings.HasSuffix(trimmed, "}") || strings.HasSuffix(trimmed, "},")) {
				if currentSym != nil {
					currentSym.EndLine = lineNum
					symbols = append(symbols, *currentSym)
					currentSym = nil
				}
				inBlock = false
			}
		}
	}

	// Close any open block at EOF
	if currentSym != nil {
		currentSym.EndLine = totalLines
		symbols = append(symbols, *currentSym)
	}

	if len(symbols) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📄 %s (%d 行，%d 个符号)\n\n", relPath, totalLines, len(symbols)))
	header := b.String()
	body, delivered := fitWindow(symbols, budget-len(header)-windowTrailerReserve,
		func(_ int, sym FileSymbol) string {
			lineRange := fmt.Sprintf("%d-%d", sym.StartLine, sym.EndLine)
			if sym.StartLine == sym.EndLine {
				lineRange = fmt.Sprintf("%d", sym.StartLine)
			}
			return fmt.Sprintf("  %4s  %s  (%s)\n", lineRange, sym.Name, sym.Kind)
		})
	b.WriteString(body)
	if delivered < len(symbols) {
		next := symbols[delivered].StartLine
		fmt.Fprintf(&b, "\n[文件超过 %d 行，改为显示骨架结构。以上只列出第 %d 行之前的符号；"+
			"续读：read_file offset=%d 取该行起的原文，或用更小的 limit 只取感兴趣的区间]\n",
			SkeletonThreshold, next, next)
	} else {
		fmt.Fprintf(&b, "\n[文件超过 %d 行，改为显示骨架结构。用 read_file 的 offset/limit 参数读取特定部分]\n",
			SkeletonThreshold)
	}
	return b.String()
}

// ================================================================
//  IP Validation (for web tools)
// ================================================================

// isPrivateNetwork checks if a hostname resolves to a private/internal IP.
func isPrivateNetwork(hostname string) bool {
	// Check common private hostnames first
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" || hostname == "0.0.0.0" {
		return true
	}

	// Try to resolve the hostname
	ips, err := net.LookupHost(hostname)
	if err != nil {
		// If we can't resolve, do a basic string check on common patterns
		if strings.HasPrefix(hostname, "10.") ||
			strings.HasPrefix(hostname, "172.16.") ||
			strings.HasPrefix(hostname, "192.168.") ||
			strings.HasPrefix(hostname, "169.254.") {
			return true
		}
		return false
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return true
		}
	}
	return false
}

// ================================================================
//  Shell Command Security
// ================================================================

// dangerousCommands checks if a shell command is dangerous.
// Uses a more sophisticated approach than simple keyword matching.
func dangerousCommand(cmd string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(cmd))

	// Strip shell wrappers to analyze the actual command
	clean := lower
	for _, prefix := range []string{"sh -c ", "bash -c ", "zsh -c ", "cmd /c "} {
		clean = strings.TrimPrefix(clean, prefix)
	}
	clean = strings.TrimSpace(clean)
	clean = strings.Trim(clean, "'\"")
	clean = strings.TrimSpace(clean)

	// Check for destructive operations with proper path analysis
	dangerousPrefixes := []struct {
		prefix  string
		message string
	}{
		{"rm -rf /", "⚠ 危险命令：递归删除根目录 '/'"},
		{"rm -rf /*", "⚠ 危险命令：递归删除根目录 '/*'"},
		{"rm -rf ~", "⚠ 危险命令：递归删除用户目录 '~'"},
		{"rm -rf .", "⚠ 危险命令：递归删除当前目录 '.'"},
		{":(){ :|:& };:", "⚠ 危险命令：fork 炸弹"},
		{"dd if=", "⚠ 危险命令：dd 命令可能破坏磁盘数据"},
		{"mkfs.", "⚠ 危险命令：格式化磁盘"},
		{"fdisk", "⚠ 危险命令：磁盘分区操作"},
		{"mkswap", "⚠ 危险命令：swap 操作"},
		{"reboot", "⚠ 危险命令：重启系统"},
		{"shutdown", "⚠ 危险命令：关闭系统"},
		{"halt", "⚠ 危险命令：停止系统"},
		{"poweroff", "⚠ 危险命令：关闭电源"},
		{"init 0", "⚠ 危险命令：切换到运行级别 0（关机）"},
		{"init 6", "⚠ 危险命令：切换到运行级别 6（重启）"},
	}

	for _, dp := range dangerousPrefixes {
		if strings.Contains(clean, dp.prefix) {
			return dp.message, true
		}
	}

	// Check for sudo/chmod/chown with proper context
	if strings.HasPrefix(clean, "sudo ") {
		return "⚠ 危险命令：禁止使用 sudo 提权", true
	}

	if strings.Contains(clean, "chmod 777") || strings.Contains(clean, "chmod -R 777") {
		return "⚠ 危险命令：chmod 777 过于危险", true
	}

	if strings.Contains(clean, "chown") && !strings.Contains(clean, "chown -R") {
		// chown without -R is usually safe for owned files
		// but warn if targeting system paths
		return "", false
	}
	if strings.Contains(clean, "chown -R") {
		return "⚠ 危险命令：递归 chown 可能影响系统文件", true
	}

	// Block pipe-to-shell: curl/wget ... | sh/bash
	pipePatterns := []string{
		"curl | sh", "curl | bash", "curl | zsh",
		"wget | sh", "wget | bash", "wget | zsh",
		"curl | sudo", "wget | sudo",
	}
	for _, pp := range pipePatterns {
		if strings.Contains(clean, pp) {
			return "⚠ 危险命令：禁止从网络下载后直接执行", true
		}
	}

	// Block reverse shell patterns
	reverseShellPatterns := []string{
		"bash -i >& /dev/tcp/",
		"bash -i >& /dev/udp/",
		"sh -i >& /dev/tcp/",
		"sh -i >& /dev/udp/",
		"python -c 'import pty",
		"python3 -c 'import pty",
		"nc -e ", "ncat -e ",
		"mkfifo /tmp/",
		"exec 5<>/dev/tcp/",
	}
	for _, rsp := range reverseShellPatterns {
		if strings.Contains(clean, rsp) {
			return "⚠ 危险命令：检测到反弹 shell 模式", true
		}
	}

	// Block writing to system directories
	systemDirs := []string{"/etc/", "/usr/", "/bin/", "/sbin/", "/boot/", "/dev/", "/proc/", "/sys/"}
	writeCmds := []string{"> /etc/", "> /usr/", "> /bin/", "> /sbin/", "> /boot/",
		">> /etc/", ">> /usr/", ">> /bin/", ">> /sbin/",
		"cp ", "mv ", "chattr", "mount", "umount"}
	for _, dir := range systemDirs {
		for _, wc := range writeCmds {
			if strings.Contains(clean, wc+dir) {
				return fmt.Sprintf("⚠ 危险命令：禁止写入系统目录 '%s'", dir), true
			}
		}
	}

	// Block obfuscated command execution
	obfuscationPatterns := []string{
		"base64 -d |", "base64 -d|",
		"base64 --decode |", "base64 --decode|",
		"openssl enc -",
		"eval $(", "eval \"$(",
		"`", // backtick command substitution wrapped in... but this is very common, only block specific ones
	}
	for _, op := range obfuscationPatterns {
		if strings.Contains(clean, op) {
			// Only block if it looks like payload delivery
			if strings.Contains(clean, "curl") || strings.Contains(clean, "wget") || strings.Contains(clean, "http") {
				return "⚠ 危险命令：检测到编码执行的恶意命令", true
			}
		}
	}

	// Block direct write to sensitive project files outside allowed paths
	if strings.Contains(clean, ">/") || strings.Contains(clean, ">>/") {
		return "⚠ 危险命令：禁止使用绝对路径写入文件", true
	}

	return "", false
}

// ================================================================
//  Line-mode edit helpers
// ================================================================

// applyLineEdit replaces lines startLine-endLine (1-indexed, inclusive)
// with newString. Returns the modified content.
func applyLineEdit(content string, startLine, endLine int, newString string) string {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	if startLine < 1 {
		startLine = 1
	}
	if endLine > totalLines {
		endLine = totalLines
	}
	if startLine > endLine {
		startLine = endLine
	}

	// Build result: lines before + new string + lines after
	var result []string
	result = append(result, lines[:startLine-1]...)
	result = append(result, newString)
	result = append(result, lines[endLine:]...)

	return strings.Join(result, "\n")
}

// findClosestMatch helps when exact old_string match fails.
// Returns the match position + context for the user to see nearby content.
func findClosestMatch(content, search string) (int, string) {
	lines := strings.Split(content, "\n")
	searchLines := strings.Split(search, "\n")
	firstSearchLine := strings.TrimSpace(searchLines[0])

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, firstSearchLine) || strings.Contains(firstSearchLine, trimmed) {
			start := i - 2
			if start < 0 {
				start = 0
			}
			end := i + 3
			if end > len(lines) {
				end = len(lines)
			}
			context := strings.Join(lines[start:end], "\n")
			return i + 1, fmt.Sprintf(
				"精确匹配失败。在第 %d 行附近找到相似内容，请检查搜索文本是否与文件内容精确一致（包括缩进）：\n\n%s\n",
				i+1, context)
		}
	}
	return 0, "未找到匹配内容。请确保搜索文本与文件中内容完全一致（包括空格和缩进）。"
}

// ================================================================
//  Git-aware file walk (skip .git, node_modules, vendor)
// ================================================================

// shouldSkipDir checks if a directory should be skipped during walks.
func shouldSkipDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules"
}

// walkGoFiles walks the project directory and finds all .go files.
func walkGoFiles(root string, maxFiles int) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}
		if fi.IsDir() {
			if shouldSkipDir(fi.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
			if maxFiles > 0 && len(files) >= maxFiles {
				return fmt.Errorf("max files reached")
			}
		}
		return nil
	})
	return files, err
}

// walkAllFiles walks the project directory finding all non-binary files.
func walkAllFiles(root string, maxFiles int) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			if shouldSkipDir(fi.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, path)
		if maxFiles > 0 && len(files) >= maxFiles {
			return fmt.Errorf("max files reached")
		}
		return nil
	})
	return files, err
}

// ================================================================
//  Markdown formatting helpers
// ================================================================

// mdCodeBlock wraps content in a markdown code block with optional language.
func mdCodeBlock(content, lang string) string {
	if lang != "" {
		return fmt.Sprintf("```%s\n%s\n```", lang, content)
	}
	return fmt.Sprintf("```\n%s\n```", content)
}

// mdBold wraps text in bold markdown.
func mdBold(text string) string {
	return fmt.Sprintf("**%s**", text)
}

// ================================================================
//  Streamlined error response helpers (make tests happy)
// ================================================================

// friendlyPathError returns a user-friendly error message for path issues.
func friendlyPathError(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "超出项目目录") {
		return "错误：文件路径超出项目目录范围"
	}
	if strings.Contains(msg, "无法解析路径") {
		return "错误：" + msg
	}
	return "错误：" + msg
}

// containsString checks if a string slice contains a value.
func containsString(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
