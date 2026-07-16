package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/format"
	"go/token"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/cloudwego/eino/schema"
	"kandaoni.com/anqicms/config"
)

// ---- Arg types for built-in tools ----

type fileReadArgs struct {
	Path     string `json:"path"`
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

type fileWriteArgs struct {
	Path     string `json:"path"`
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type fileEditArgs struct {
	Path      string `json:"path"`
	FilePath  string `json:"file_path"`
	Search    string `json:"search"`
	Replace   string `json:"replace"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type searchReplaceArgs struct {
	Search  string `json:"search"`
	Replace string `json:"replace"`
	Glob    string `json:"glob"`
	Regex   bool   `json:"regex"`
}

type bashArgs struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type grepArgs struct {
	Pattern string `json:"pattern"`
	Glob    string `json:"glob"`
	Context int    `json:"context"`
}

type globArgs struct {
	Pattern string `json:"pattern"`
}

type listDirArgs struct {
	Path  string `json:"path"`
	Depth int    `json:"depth"`
}

type webFetchArgs struct {
	URL string `json:"url"`
}

type webSearchArgs struct {
	Query string `json:"query"`
}

type symbolArgs struct {
	File    string `json:"file"`
	Package string `json:"package"`
	Symbol  string `json:"symbol"`
}

type importArgs struct {
	File string `json:"file"`
}

// ---- Project root (safe boundary) ----
// All file operations are restricted to projectRoot.
// projectRoot 由 AiChatService 传入（各站点有自己的 RootPath）

func (svc *AiChatService) safePath(path string) (string, error) {
	if svc.projectRoot == "" {
		return "", fmt.Errorf("projectRoot 未配置")
	}
	p := path
	if !filepath.IsAbs(p) {
		p = filepath.Join(svc.projectRoot, p)
	}
	p, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("无法解析路径: %w", err)
	}
	if !strings.HasPrefix(p, svc.projectRoot) {
		return "", fmt.Errorf("路径超出项目目录范围: %s", p)
	}
	return p, nil
}

// getBuiltinEinoTools returns the built-in file/system/code tools for AnQiCMS.
func (svc *AiChatService) getBuiltinEinoTools() ([]*schema.ToolInfo, map[string]toolHandler) {
	tools := make([]*schema.ToolInfo, 0)
	handlers := make(map[string]toolHandler)

	add := func(ti *schema.ToolInfo, fn toolHandler) {
		tools = append(tools, ti)
		handlers[ti.Name] = fn
	}

	// ================================================================
	//  File & Shell tools
	// ================================================================

	add(&schema.ToolInfo{
		Name: "read_file",
		Desc: "读取项目内文件的内容。支持 offset（起始行号，从1开始）和 limit（最大行数）参数分段读取大文件。大文件（超过300行）会显示骨架结构。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"file_path": {Type: schema.String, Desc: "文件路径，相对项目根目录或绝对路径", Required: true},
			"offset":    {Type: schema.Integer, Desc: "起始行号（从1开始），可选"},
			"limit":     {Type: schema.Integer, Desc: "最大读取行数，可选"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args fileReadArgs
		if err := rawJSONUnmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		path := args.Path
		if path == "" {
			path = args.FilePath
		}
		if path == "" {
			return "错误：文件路径不能为空", nil
		}

		// Check for sensitive paths before resolving
		if isSensitiveInputPath(path) {
			return "错误：禁止访问系统敏感路径", nil
		}

		fullPath, err := safePathResolve(path, svc.projectRoot)
		if err != nil {
			return friendlyPathError(err), nil
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				// Show file-not-found suggestions
				similar := findSimilarFiles(path, 20, svc.projectRoot)
				msg := fmt.Sprintf("错误：文件不存在: %s", path)
				if len(similar) > 0 {
					msg += "\n\n您是不是要查找：\n"
					for _, s := range similar {
						rel, _ := filepath.Rel(svc.projectRoot, s)
						msg += fmt.Sprintf("  %s\n", rel)
					}
				}
				return msg, nil
			}
			return "", fmt.Errorf("访问文件失败: %w", err)
		}
		if info.IsDir() {
			return fmt.Sprintf("错误：%s 是一个目录，请使用 list_directory 查看目录内容", path), nil
		}
		if info.Size() > 5*1024*1024 {
			return "错误：文件超过 5MB 限制，无法读取", nil
		}

		// Check cache
		mtime := info.ModTime()
		offset := args.Offset
		limit := args.Limit
		if offset == 0 && limit == 0 {
			if cached, ok := getReadCache(fullPath, 0, 0, mtime); ok {
				return cached, nil
			}
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("读取文件失败: %w", err)
		}

		relPath, _ := filepath.Rel(svc.projectRoot, fullPath)
		lines := strings.Split(string(data), "\n")
		totalLines := len(lines)

		// Header
		var b strings.Builder
		b.WriteString(fmt.Sprintf("文件: %s (%d 行, %d 字节)\n\n", relPath, totalLines, info.Size()))

		if offset > totalLines {
			return fmt.Sprintf("文件: %s (%d 行)\n\n起始行号 %d 超出文件总行数 %d", relPath, totalLines, offset, totalLines), nil
		}

		// Handle offset/limit
		if offset > 0 || limit > 0 {
			startLine := offset
			if startLine <= 0 {
				startLine = 1
			}
			endLine := len(lines)
			if limit > 0 {
				endLine = startLine + limit - 1
				if endLine > len(lines) {
					endLine = len(lines)
				}
			}
			for i := startLine - 1; i < endLine; i++ {
				b.WriteString(fmt.Sprintf("%6d| %s\n", i+1, lines[i]))
			}
			result := b.String()
			setReadCache(fullPath, offset, limit, mtime, result)
			return result, nil
		}

		// Skeleton mode for large files (>300 lines)
		if totalLines > SkeletonThreshold {
			skeleton := buildSkeleton(data, fullPath, relPath, totalLines)
			result := skeleton
			if result != "" {
				setReadCache(fullPath, 0, 0, mtime, result)
				return result, nil
			}
		}

		// Full content for smaller files
		maxLines := 10000
		for i := 0; i < totalLines && i < maxLines; i++ {
			b.WriteString(fmt.Sprintf("%6d| %s\n", i+1, lines[i]))
		}
		if totalLines > maxLines {
			b.WriteString(fmt.Sprintf("\n... (文件较大，仅显示前 %d 行)", maxLines))
		}
		result := b.String()
		setReadCache(fullPath, 0, 0, mtime, result)
		return result, nil
	})

	add(&schema.ToolInfo{
		Name: "write_file",
		Desc: "写入或创建文件。如果文件已存在则覆盖。会自动创建父目录。注意：只能操作项目目录内的文件。临时脚本（py/sh等）请写入 cache/ 目录（" + svc.projectRoot + "cache/" + "）。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"file_path": {Type: schema.String, Desc: "文件路径，相对项目根目录或绝对路径", Required: true},
			"content":   {Type: schema.String, Desc: "文件内容", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args fileWriteArgs
		if err := rawJSONUnmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Path == "" {
			args.Path = args.FilePath
		}
		if args.Path == "" {
			return "错误：文件路径不能为空", nil
		}
		if isSensitiveInputPath(args.Path) {
			return "错误：禁止写入系统敏感路径", nil
		}
		fullPath, err := safePathResolve(args.Path, svc.projectRoot)
		if err != nil {
			return friendlyPathError(err), nil
		}
		// Check if overwriting an existing file
		if info, err := os.Stat(fullPath); err == nil {
			oldSize := info.Size()
			newSize := len(args.Content)
			relPath, _ := filepath.Rel(svc.projectRoot, fullPath)
			if newSize < int(oldSize/2) && oldSize > 100 {
				return fmt.Sprintf("⚠ 警告：文件 %s 将缩小超过 50%%（从 %d 字节到 %d 字节），是否确认？请检查内容是否完整。", relPath, oldSize, newSize), nil
			}
		}
		// Create parent directories
		parent := filepath.Dir(fullPath)
		if err := os.MkdirAll(parent, 0755); err != nil {
			return "", fmt.Errorf("创建目录失败: %w", err)
		}
		if err := os.WriteFile(fullPath, []byte(args.Content), 0644); err != nil {
			return "", fmt.Errorf("写入文件失败: %w", err)
		}
		// Invalidate cache
		invalidateReadCache(fullPath)
		relPath, _ := filepath.Rel(svc.projectRoot, fullPath)
		return fmt.Sprintf("文件写入成功: %s (%d 字节)", relPath, len(args.Content)), nil
	})

	add(&schema.ToolInfo{
		Name: "edit_file",
		Desc: "编辑文件内容。支持两种模式：\n1. 文本模式：指定 search/old_string 和 replace/new_string 进行精确文本替换\n2. 行模式：指定 start_line、end_line 和 new_string 替换整段行\n如果 search 或 old_string 参数为空，则自动切换到行模式。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"file_path":  {Type: schema.String, Desc: "文件路径", Required: true},
			"search":     {Type: schema.String, Desc: "（文本模式）要搜索的旧文本"},
			"replace":    {Type: schema.String, Desc: "（文本模式）替换后的新文本"},
			"old_string": {Type: schema.String, Desc: "（文本模式）要搜索的旧文本（同 search）"},
			"new_string": {Type: schema.String, Desc: "（行模式）替换后的新文本"},
			"start_line": {Type: schema.Integer, Desc: "（行模式）起始行号（从1开始）"},
			"end_line":   {Type: schema.Integer, Desc: "（行模式）结束行号（从1开始），默认等于 start_line"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args fileEditArgs
		if err := rawJSONUnmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		path := args.Path
		if path == "" {
			path = args.FilePath
		}

		// Detect which mode to use
		searchText := args.Search
		if searchText == "" {
			searchText = args.OldString
		}
		replaceText := args.Replace
		if replaceText == "" {
			replaceText = args.NewString
		}

		if path == "" {
			return "错误：文件路径和搜索文本不能为空", nil
		}
		if searchText == "" && args.StartLine == 0 && args.EndLine == 0 {
			return "错误：文件路径和搜索文本不能为空", nil
		}

		fullPath, err := svc.safePath(path)
		if err != nil {
			return "错误：" + err.Error(), nil
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Sprintf("错误：文件不存在: %s", path), nil
			}
			return "", fmt.Errorf("读取文件失败: %w", err)
		}

		// Line mode: replace lines start_line-end_line with new_string
		if args.StartLine > 0 && replaceText != "" && searchText == "" {
			content := string(data)
			endLine := args.EndLine
			if endLine <= 0 {
				endLine = args.StartLine
			}
			result := applyLineEdit(content, args.StartLine, endLine, replaceText)
			if err := os.WriteFile(fullPath, []byte(result), 0644); err != nil {
				return "", fmt.Errorf("写入文件失败: %w", err)
			}
			invalidateReadCache(fullPath)
			relPath, _ := filepath.Rel(svc.projectRoot, fullPath)
			return fmt.Sprintf("文件 %s 已更新（行 %d-%d 已替换）", relPath, args.StartLine, endLine), nil
		}

		// Text mode: search and replace
		oldStr := searchText
		if !strings.Contains(string(data), oldStr) {
			// Try closest match for better error message
			line, hint := findClosestMatch(string(data), oldStr)
			if line > 0 {
				return hint, nil
			}
			return "错误：未找到匹配的文本，请检查搜索内容", nil
		}
		result := strings.Replace(string(data), oldStr, replaceText, 1)
		if err := os.WriteFile(fullPath, []byte(result), 0644); err != nil {
			return "", fmt.Errorf("写入文件失败: %w", err)
		}
		invalidateReadCache(fullPath)
		count := strings.Count(string(data), oldStr)
		relPath, _ := filepath.Rel(svc.projectRoot, fullPath)
		msg := fmt.Sprintf("文件 %s 已更新，共替换 1 处", relPath)
		if count > 1 {
			msg += fmt.Sprintf("\n(注：文件中包含 %d 处匹配，仅替换第 1 处。如需全部替换请使用 search_replace 工具)", count)
		}
		return msg, nil
	})

	add(&schema.ToolInfo{
		Name: "search_replace",
		Desc: "在多个文件中搜索并替换文本。支持 glob 模式匹配文件和正则表达式搜索。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"search":  {Type: schema.String, Desc: "要搜索的文本（或正则表达式）", Required: true},
			"replace": {Type: schema.String, Desc: "替换后的文本", Required: true},
			"glob":    {Type: schema.String, Desc: "文件匹配模式，如 '**/*.go'、'*.html'，默认 '**/*'"},
			"regex":   {Type: schema.Boolean, Desc: "是否将 search 视为正则表达式，默认 false"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args searchReplaceArgs
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Search == "" {
			return "错误：搜索文本不能为空", nil
		}
		if args.Glob == "" {
			args.Glob = "**/*"
		}

		// Find matching files
		_, err := filepath.Glob(filepath.Join(svc.projectRoot, args.Glob))
		if err != nil {
			return "", fmt.Errorf("文件匹配失败: %w", err)
		}
		// filepath.Glob doesn't support ** — do manual walk
		var allFiles []string
		err = filepath.Walk(svc.projectRoot, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil // skip inaccessible
			}
			if fi.IsDir() {
				// Skip hidden dirs and vendor/node_modules
				base := fi.Name()
				if strings.HasPrefix(base, ".") || base == "vendor" || base == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, _ := filepath.Rel(svc.projectRoot, path)
			matched, err := filepath.Match(args.Glob, rel)
			if err != nil {
				return nil
			}
			if matched || args.Glob == "**/*" {
				// Also check ** matching
				if strings.Contains(args.Glob, "**") {
					parts := strings.Split(args.Glob, "**")
					if len(parts) == 2 {
						if strings.HasPrefix(rel, strings.TrimRight(parts[0], "/")) &&
							strings.HasSuffix(rel, strings.TrimLeft(parts[1], "/")) {
							allFiles = append(allFiles, path)
						}
					}
				} else if matched {
					allFiles = append(allFiles, path)
				}
			}
			if args.Glob == "**/*" {
				allFiles = append(allFiles, path)
			}
			return nil
		})
		if args.Glob == "**/*" {
			// Already collected everything — need to redo properly
			allFiles = nil
			filepath.Walk(svc.projectRoot, func(path string, fi os.FileInfo, err error) error {
				if err != nil || fi.IsDir() {
					if fi != nil && fi.IsDir() {
						base := fi.Name()
						if strings.HasPrefix(base, ".") || base == "vendor" || base == "node_modules" {
							return filepath.SkipDir
						}
					}
					return nil
				}
				allFiles = append(allFiles, path)
				return nil
			})
		}
		if err != nil {
			return "", fmt.Errorf("遍历文件失败: %w", err)
		}

		if len(allFiles) > 200 {
			return fmt.Sprintf("匹配文件过多 (%d)，请缩小 glob 范围", len(allFiles)), nil
		}
		if len(allFiles) == 0 {
			return "未找到匹配的文件", nil
		}

		var matchedFiles []string
		totalReplacements := 0
		limit := 20 // max files to modify

		var searchBytes []byte
		var re *regexp.Regexp
		if args.Regex {
			re, err = regexp.Compile(args.Search)
			if err != nil {
				return "", fmt.Errorf("正则表达式编译失败: %w", err)
			}
		} else {
			searchBytes = []byte(args.Search)
		}

		for _, fp := range allFiles {
			if len(matchedFiles) >= limit {
				break
			}
			data, err := os.ReadFile(fp)
			if err != nil {
				continue
			}
			var newData []byte
			var count int
			if args.Regex {
				matches := re.FindAll(data, -1)
				count = len(matches)
				if count > 0 {
					newData = re.ReplaceAll(data, []byte(args.Replace))
				}
			} else {
				count = bytes.Count(data, searchBytes)
				if count > 0 {
					newData = bytes.ReplaceAll(data, searchBytes, []byte(args.Replace))
				}
			}
			if count > 0 {
				if err := os.WriteFile(fp, newData, 0644); err != nil {
					continue
				}
				relPath, _ := filepath.Rel(svc.projectRoot, fp)
				matchedFiles = append(matchedFiles, fmt.Sprintf("  - %s (%d 处)", relPath, count))
				totalReplacements += count
			}
		}

		if len(matchedFiles) == 0 {
			return "未找到匹配的内容", nil
		}
		result := fmt.Sprintf("搜索替换完成，共修改 %d 个文件，替换 %d 处：\n\n", len(matchedFiles), totalReplacements)
		result += strings.Join(matchedFiles, "\n")
		if len(allFiles) > limit {
			result += fmt.Sprintf("\n\n(还有 %d 个文件未处理，请缩小搜索范围)", len(allFiles)-limit)
		}
		return result, nil
	})

	add(&schema.ToolInfo{
		Name: "bash",
		Desc: "在项目根目录执行 shell 命令。用于运行构建、测试、代码生成等开发命令。注意：不能使用交互式命令。临时生成的文件（脚本、输出等）请写入 cache/ 目录。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {Type: schema.String, Desc: "要执行的 shell 命令", Required: true},
			"timeout": {Type: schema.Integer, Desc: "超时时间（秒），默认 30，最大 120"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args bashArgs
		if err := rawJSONUnmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Command == "" {
			return "错误：命令不能为空", nil
		}
		if args.Timeout <= 0 || args.Timeout > 120 {
			args.Timeout = 30
		}

		// Improved security checks
		if msg, dangerous := dangerousCommand(args.Command); dangerous {
			return msg, nil
		}

		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/C", args.Command)
		} else {
			cmd = exec.Command("sh", "-c", args.Command)
		}
		cmd.Dir = svc.projectRoot

		timeout := time.Duration(args.Timeout) * time.Second
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		var b strings.Builder
		b.WriteString(fmt.Sprintf("$ %s\n", args.Command))
		if stdout.Len() > 0 {
			out := stdout.String()
			if len(out) > 50000 {
				out = out[:50000] + "\n... (输出截断，超过 50000 字符)"
			}
			b.WriteString(out)
		}
		if stderr.Len() > 0 {
			errStr := stderr.String()
			if len(errStr) > 10000 {
				errStr = errStr[:10000] + "\n... (错误输出截断)"
			}
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString("STDERR:\n" + errStr)
		}
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return "", fmt.Errorf("命令执行超时（%d秒）", args.Timeout)
			}
			b.WriteString(fmt.Sprintf("\n退出码: %v", err))
		}
		return b.String(), nil
	})

	add(&schema.ToolInfo{
		Name: "grep",
		Desc: "在项目文件中搜索文本或正则表达式。支持指定文件匹配模式和上下文行数。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"pattern": {Type: schema.String, Desc: "搜索模式文本", Required: true},
			"glob":    {Type: schema.String, Desc: "文件匹配模式，如 '*.go'、'*.html'，默认所有文件"},
			"context": {Type: schema.Integer, Desc: "上下文行数（包含匹配行前后各 N 行），默认 0"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args grepArgs
		if err := rawJSONUnmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Pattern == "" {
			return "错误：搜索模式不能为空", nil
		}
		if args.Context < 0 {
			args.Context = 0
		}

		// Try as regex first, fall back to literal string if it fails
		re, err := regexp.Compile(args.Pattern)
		if err != nil {
			// Escape the pattern so it matches literally
			re, err = regexp.Compile(regexp.QuoteMeta(args.Pattern))
			if err != nil {
				return "", fmt.Errorf("正则表达式编译失败: %w", err)
			}
		}

		type match struct {
			File    string
			Line    int
			Content string
			Before  []string
			After   []string
		}

		var matches []match
		maxResults := 100

		filepath.Walk(svc.projectRoot, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				if fi != nil && fi.IsDir() {
					base := fi.Name()
					if strings.HasPrefix(base, ".") || base == "vendor" || base == "node_modules" {
						return filepath.SkipDir
					}
				}
				return nil
			}
			// Check glob
			if args.Glob != "" {
				rel, _ := filepath.Rel(svc.projectRoot, path)
				matched, _ := filepath.Match(args.Glob, fi.Name())
				matchedRel, _ := filepath.Match(args.Glob, rel)
				if !matched && !matchedRel {
					return nil
				}
			}
			// Skip binary files
			if fi.Size() > 1024*1024 {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()

			var lines []string
			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
			if len(lines) > 5000 {
				return nil // skip large files
			}

			relPath, _ := filepath.Rel(svc.projectRoot, path)
			for i, line := range lines {
				if re.MatchString(line) {
					if len(matches) >= maxResults {
						return fmt.Errorf("reached max results")
					}
					m := match{File: relPath, Line: i + 1, Content: line}
					// Before context
					start := i - args.Context
					if start < 0 {
						start = 0
					}
					for j := start; j < i; j++ {
						m.Before = append(m.Before, fmt.Sprintf("  %d| %s", j+1, lines[j]))
					}
					// After context
					end := i + args.Context + 1
					if end > len(lines) {
						end = len(lines)
					}
					for j := i + 1; j < end; j++ {
						m.After = append(m.After, fmt.Sprintf("  %d| %s", j+1, lines[j]))
					}
					matches = append(matches, m)
				}
			}
			return nil
		})

		if len(matches) == 0 {
			return "未找到匹配的内容", nil
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("共找到 %d 处匹配：\n\n", len(matches)))
		for _, m := range matches {
			b.WriteString(fmt.Sprintf("%s:%d\n", m.File, m.Line))
			for _, before := range m.Before {
				b.WriteString(before + "\n")
			}
			b.WriteString(fmt.Sprintf("  → %s\n", strings.TrimSpace(m.Content)))
			for _, after := range m.After {
				b.WriteString(after + "\n")
			}
			b.WriteString("\n")
		}
		if len(matches) >= maxResults {
			b.WriteString("... (结果过多，仅显示前 100 条)")
		}
		return b.String(), nil
	})

	add(&schema.ToolInfo{
		Name: "glob",
		Desc: "按文件名模式查找文件。支持通配符：* 匹配任意字符，** 匹配任意目录层级。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"pattern": {Type: schema.String, Desc: "文件匹配模式，如 '**/*.go'、'template/**'、'*.html'", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args globArgs
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Pattern == "" {
			return "错误：文件匹配模式不能为空", nil
		}

		var results []string
		maxResults := 200

		_ = filepath.Walk(svc.projectRoot, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(svc.projectRoot, path)
			// Skip hidden dirs
			if fi.IsDir() {
				base := fi.Name()
				if strings.HasPrefix(base, ".") || base == "vendor" || base == "node_modules" {
					return filepath.SkipDir
				}
				matched, err := filepath.Match(args.Pattern, rel)
				if err == nil && matched {
					results = append(results, rel+"/")
				}
				return nil
			}

			if strings.Contains(args.Pattern, "**") {
				parts := strings.Split(args.Pattern, "**")
				if len(parts) == 2 {
					prefix := strings.TrimRight(parts[0], "/")
					suffix := strings.TrimLeft(parts[1], "/")
					if (prefix == "" || strings.HasPrefix(rel, prefix)) &&
						(strings.HasSuffix(rel, suffix) || suffix == "") {
						results = append(results, rel)
					}
				}
			} else {
				matched, err := filepath.Match(args.Pattern, fi.Name())
				if err == nil && matched {
					results = append(results, rel)
				}
			}
			if len(results) > maxResults {
				return fmt.Errorf("too many results")
			}
			return nil
		})

		if len(results) == 0 {
			return "未找到匹配的文件", nil
		}

		sort.Strings(results)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共找到 %d 个文件/目录：\n\n", len(results)))
		for _, r := range results {
			b.WriteString(r + "\n")
		}
		if len(results) > maxResults {
			b.WriteString("... (结果过多，仅显示前 200 条)")
		}
		return b.String(), nil
	})

	add(&schema.ToolInfo{
		Name: "list_directory",
		Desc: "列出目录结构和文件。可指定目录路径和递归深度。隐藏目录会自动跳过。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path":  {Type: schema.String, Desc: "目录路径，相对项目根目录或绝对路径，默认根目录"},
			"depth": {Type: schema.Integer, Desc: "递归深度，默认 2，最大 5"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args listDirArgs
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		basePath := svc.projectRoot
		if args.Path != "" {
			var err error
			basePath, err = svc.safePath(args.Path)
			if err != nil {
				return "错误：" + err.Error(), nil
			}
		}
		depth := args.Depth
		if depth <= 0 {
			depth = 2
		}
		if depth > 5 {
			depth = 5
		}

		info, err := os.Stat(basePath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Sprintf("错误：目录不存在: %s", args.Path), nil
			}
			return "", fmt.Errorf("访问目录失败: %w", err)
		}
		if !info.IsDir() {
			return fmt.Sprintf("错误：%s 是一个文件，不是目录", args.Path), nil
		}

		var b strings.Builder
		rel, _ := filepath.Rel(svc.projectRoot, basePath)
		if rel == "." {
			rel = svc.projectRoot
		}
		b.WriteString(fmt.Sprintf("📁 %s/\n", rel))

		var walk func(path string, prefix string, remainingDepth int)
		walk = func(path string, prefix string, remainingDepth int) {
			entries, err := os.ReadDir(path)
			if err != nil {
				return
			}
			// Sort: dirs first, then files
			sort.Slice(entries, func(i, j int) bool {
				if entries[i].IsDir() != entries[j].IsDir() {
					return entries[i].IsDir()
				}
				return entries[i].Name() < entries[j].Name()
			})
			for i, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".") {
					continue
				}
				isLast := i == len(entries)-1
				connector := "├── "
				if isLast {
					connector = "└── "
				}
				if entry.IsDir() {
					b.WriteString(fmt.Sprintf("%s%s📁 %s/\n", prefix, connector, entry.Name()))
					if remainingDepth > 1 {
						childPrefix := prefix
						if isLast {
							childPrefix += "    "
						} else {
							childPrefix += "│   "
						}
						walk(filepath.Join(path, entry.Name()), childPrefix, remainingDepth-1)
					}
				} else {
					fi, _ := entry.Info()
					size := ""
					if fi != nil {
						size = fmt.Sprintf(" (%d B)", fi.Size())
					}
					b.WriteString(fmt.Sprintf("%s%s📄 %s%s\n", prefix, connector, entry.Name(), size))
				}
			}
		}
		walk(basePath, "", depth)
		return b.String(), nil
	})

	// ================================================================
	//  Web tools
	// ================================================================

	add(&schema.ToolInfo{
		Name: "web_fetch",
		Desc: "获取指定URL的网页内容并返回纯文本。用于查看网页信息、API文档等。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"url": {Type: schema.String, Desc: "要获取的网页URL", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args webFetchArgs
		if err := rawJSONUnmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.URL == "" {
			return "错误：URL 不能为空", nil
		}

		// Validate URL
		parsedURL, err := url.Parse(args.URL)
		if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
			return "错误：URL 格式不正确，仅支持 http/https", nil
		}

		// Block private IPs and localhost with proper DNS resolution
		host := parsedURL.Hostname()
		if isPrivateNetwork(host) {
			return "错误：不允许访问内网地址", nil
		}

		client := &http.Client{Timeout: 15 * time.Second}
		req, err := http.NewRequestWithContext(ctx, "GET", args.URL, nil)
		if err != nil {
			return "", fmt.Errorf("创建请求失败: %w", err)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AnQiCMS AI Bot)")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("请求失败: %w", err)
		}
		defer resp.Body.Close()

		// Limit response size
		body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		if err != nil {
			return "", fmt.Errorf("读取响应失败: %w", err)
		}

		// Parse HTML and extract text
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
		if err != nil {
			// Not HTML, return raw text
			return fmt.Sprintf("URL: %s\n状态码: %d\n大小: %d 字节\n\n%s",
				args.URL, resp.StatusCode, len(body), string(body)), nil
		}

		// Remove script, style, nav, footer, header
		doc.Find("script, style, nav, footer, header, aside, noscript, iframe, svg, form").Remove()

		var textParts []string
		doc.Find("p, h1, h2, h3, h4, h5, h6, li, td, th, blockquote, pre, code, div.text, div.content, article, section").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if len(text) > 20 {
				textParts = append(textParts, text)
			}
		})
		if len(textParts) == 0 {
			// Fallback: get body text
			textParts = append(textParts, strings.TrimSpace(doc.Find("body").Text()))
		}

		title := doc.Find("title").Text()
		joined := strings.Join(textParts, "\n\n")
		if len(joined) > 30000 {
			joined = joined[:30000] + "\n\n... (内容截断，超过 30000 字符)"
		}

		return fmt.Sprintf("URL: %s\n状态码: %d\n标题: %s\n\n%s",
			args.URL, resp.StatusCode, title, joined), nil
	})

	add(&schema.ToolInfo{
		Name: "web_search",
		Desc: "搜索互联网获取最新信息。可访问外网时使用 DuckDuckGo，否则回退使用 Bing 搜索。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {Type: schema.String, Desc: "搜索关键词", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args webSearchArgs
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Query == "" {
			return "错误：搜索关键词不能为空", nil
		}

		if config.GoogleValid {
			return searchDuckDuckGo(ctx, args.Query)
		}
		return searchBing(ctx, args.Query)
	})

	return tools, handlers
}

// searchDuckDuckGo 使用 DuckDuckGo 搜索
func searchDuckDuckGo(ctx context.Context, query string) (string, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AnQiCMS AI Bot)")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("搜索请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("解析搜索结果失败: %w", err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("搜索结果: %s\n\n", query))

	count := 0
	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if count >= 10 {
			return
		}
		title := strings.TrimSpace(s.Find(".result__title a").Text())
		link, _ := s.Find(".result__url").Attr("href")
		snippet := strings.TrimSpace(s.Find(".result__snippet").Text())

		if title == "" {
			title = strings.TrimSpace(s.Find("h2 a").Text())
			link, _ = s.Find("h2 a").Attr("href")
			snippet = strings.TrimSpace(s.Find(".result__snippet, .snippet").Text())
		}
		if strings.Contains(link, "//duckduckgo.com/l/?uddg=") {
			u, err := url.Parse(link)
			if err == nil {
				if decoded := u.Query().Get("uddg"); decoded != "" {
					link = decoded
				}
			}
		}
		if title != "" {
			b.WriteString(fmt.Sprintf("%d. %s\n", count+1, title))
			if link != "" {
				b.WriteString(fmt.Sprintf("   %s\n", link))
			}
			if snippet != "" {
				b.WriteString(fmt.Sprintf("   %s\n", snippet))
			}
			b.WriteString("\n")
			count++
		}
	})

	if count == 0 {
		doc.Find(".results_links").Each(func(i int, s *goquery.Selection) {
			if count >= 10 {
				return
			}
			title := strings.TrimSpace(s.Find(".results_links_title a").Text())
			link, _ := s.Find(".results_links_title a").Attr("href")
			snippet := strings.TrimSpace(s.Find(".results_links_snippet").Text())
			if title != "" {
				b.WriteString(fmt.Sprintf("%d. %s\n", count+1, title))
				if link != "" {
					b.WriteString(fmt.Sprintf("   %s\n", link))
				}
				if snippet != "" {
					b.WriteString(fmt.Sprintf("   %s\n", snippet))
				}
				b.WriteString("\n")
				count++
			}
		})
	}

	if count == 0 {
		return fmt.Sprintf("未找到关于 \"%s\" 的搜索结果，请尝试其他关键词", query), nil
	}
	return b.String(), nil
}

// searchBing 使用 Bing 搜索（国内可访问）
func searchBing(ctx context.Context, query string) (string, error) {
	searchURL := fmt.Sprintf("https://www.bing.com/search?q=%s&count=10", url.QueryEscape(query))
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("搜索请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("解析搜索结果失败: %w", err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("搜索结果: %s\n\n", query))

	count := 0
	doc.Find("#b_results > .b_algo").Each(func(i int, s *goquery.Selection) {
		if count >= 10 {
			return
		}
		title := strings.TrimSpace(s.Find("h2 a").Text())
		link, _ := s.Find("h2 a").Attr("href")
		snippet := strings.TrimSpace(s.Find(".b_caption p").Text())
		if snippet == "" {
			snippet = strings.TrimSpace(s.Find(".b_lineclamp2").Text())
		}

		if title != "" {
			b.WriteString(fmt.Sprintf("%d. %s\n", count+1, title))
			if link != "" {
				b.WriteString(fmt.Sprintf("   %s\n", link))
			}
			if snippet != "" {
				b.WriteString(fmt.Sprintf("   %s\n", snippet))
			}
			b.WriteString("\n")
			count++
		}
	})

	if count == 0 {
		return fmt.Sprintf("未找到关于 \"%s\" 的搜索结果，请尝试其他关键词", query), nil
	}
	return b.String(), nil
}

// renderNode renders an AST node back to formatted Go source string
func renderNode(fset *token.FileSet, node any) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return fmt.Sprintf("%v", node)
	}
	return buf.String()
}
