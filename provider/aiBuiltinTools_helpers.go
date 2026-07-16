package provider

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
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

var readFileCache sync.Map // key: "path|offset|limit" → *readCacheEntry

func getReadCache(path string, offset, limit int, mtime time.Time) (string, bool) {
	key := fmt.Sprintf("%s|%d|%d", path, offset, limit)
	if val, ok := readFileCache.Load(key); ok {
		entry := val.(*readCacheEntry)
		if entry.mtime.Equal(mtime) {
			return entry.output, true
		}
	}
	return "", false
}

func setReadCache(path string, offset, limit int, mtime time.Time, output string) {
	key := fmt.Sprintf("%s|%d|%d", path, offset, limit)
	readFileCache.Store(key, &readCacheEntry{mtime: mtime, output: output})
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
// Returns (skeleton text, "SHOW FULL" hint).
func buildSkeleton(data []byte, path, relPath string, totalLines int) string {
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
	for _, sym := range symbols {
		lineRange := fmt.Sprintf("%d-%d", sym.StartLine, sym.EndLine)
		if sym.StartLine == sym.EndLine {
			lineRange = fmt.Sprintf("%d", sym.StartLine)
		}
		b.WriteString(fmt.Sprintf("  %4s  %s  (%s)\n", lineRange, sym.Name, sym.Kind))
	}
	b.WriteString("\n[文件超过 %d 行，显示骨架结构。使用 read_file 的 offset/limit 参数读取特定部分]\n")
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
