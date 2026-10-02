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
	Confirm  bool   `json:"confirm"`
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
	Offset  int    `json:"offset"`
}

type bashArgs struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type grepArgs struct {
	Pattern string `json:"pattern"`
	Glob    string `json:"glob"`
	Path    string `json:"path"`
	Context int    `json:"context"`
	Offset  int    `json:"offset"`
}

type globArgs struct {
	Pattern string `json:"pattern"`
	Offset  int    `json:"offset"`
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
		Desc: "读取项目内文件的内容。支持 offset（起始行号，从1开始）和 limit（最大行数）参数分段读取大文件。" +
			"大文件（超过300行的 Go 文件）会先返回骨架结构。结果尾部会标明本次实际返回的行区间；" +
			"若还有剩余，按提示传 offset 续读，不要重复同一次调用。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"file_path": {Type: schema.String, Desc: "文件路径，相对项目根目录或绝对路径", Required: true},
			"offset":    {Type: schema.Integer, Desc: "起始行号（从1开始），可选"},
			"limit":     {Type: schema.Integer, Desc: "最大读取行数，可选；实际返回还会受单次结果大小限制，以尾部说明为准"},
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
		budget := svc.resultBudget()
		if offset == 0 && limit == 0 {
			if cached, ok := getReadCache(fullPath, 0, 0, budget, mtime); ok {
				return cached, nil
			}
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("读取文件失败: %w", err)
		}

		relPath, _ := filepath.Rel(svc.projectRoot, fullPath)
		lines := strings.Split(string(data), "\n")
		// 以换行结尾的文件会被 Split 多切出一个空尾项。它不是一行真实内容，
		// 留着会让「共 N 行」和窗口区间凭空多出一行空白。
		if n := len(lines); n > 1 && lines[n-1] == "" {
			lines = lines[:n-1]
		}
		totalLines := len(lines)

		if offset > totalLines {
			return fmt.Sprintf("文件: %s (%d 行)\n\n起始行号 %d 超出文件总行数 %d", relPath, totalLines, offset, totalLines), nil
		}

		// Skeleton mode for large files (>300 lines)。只在整读请求时启用：
		// 模型显式给了 offset/limit，说明它已经知道要哪一段，不该再被换成目录。
		if offset <= 0 && limit <= 0 && totalLines > SkeletonThreshold {
			if skeleton := buildSkeleton(data, fullPath, relPath, totalLines, budget); skeleton != "" {
				setReadCache(fullPath, 0, 0, budget, mtime, skeleton)
				return skeleton, nil
			}
		}

		// 按预算在行边界切窗：请求区间 ∩ 预算区间，剩下的用 offset 续读。
		start := offset
		if start <= 0 {
			start = 1
		}
		end := totalLines
		if limit > 0 && start+limit-1 < end {
			end = start + limit - 1
		}
		header := fmt.Sprintf("文件: %s (%d 行, %d 字节)\n\n", relPath, totalLines, info.Size())
		body, delivered := fitWindow(lines[start-1:end], budget-len(header)-windowTrailerReserve,
			func(i int, l string) string {
				return fmt.Sprintf("%6d| %s\n", start+i, l)
			})
		last := start + delivered - 1
		result := header + body + windowTrailer("行", start, last, totalLines, budget, delivered < end-start+1)
		setReadCache(fullPath, offset, limit, budget, mtime, result)
		return result, nil
	})

	add(&schema.ToolInfo{
		Name: "write_file",
		Desc: "写入或创建文件。如果文件已存在则覆盖。会自动创建父目录。注意：只能操作项目目录内的文件。临时脚本（py/sh等）请写入 cache/ 目录（" + svc.projectRoot + "cache/" + "）。修改模板需要通过 template_reload 工具重载模板才能生效。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"file_path": {Type: schema.String, Desc: "文件路径，相对项目根目录或绝对路径", Required: true},
			"content":   {Type: schema.String, Desc: "文件内容", Required: true},
			"confirm":   {Type: schema.Boolean, Desc: "发生警告仍需写入时，需确认写入", Required: false},
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
		if info, err := os.Stat(fullPath); err == nil && args.Confirm == false {
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
		Desc: "编辑文件内容。支持两种模式：\n1. 文本模式：指定 search/old_string 和 replace/new_string 进行精确文本替换\n2. 行模式：指定 start_line、end_line 和 new_string 替换整段行\n如果 search 或 old_string 参数为空，则自动切换到行模式。修改模板需要通过 template_reload 工具重载模板才能生效。",
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
		Desc: "在多个文件中搜索并替换文本。支持 glob 模式匹配文件和正则表达式搜索。" +
			"单次调用最多改写 20 个匹配文件；未检视完的文件会在结果里给出序号区间，" +
			"传 offset 继续下一批，不要重复同一次调用。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"search":  {Type: schema.String, Desc: "要搜索的文本（或正则表达式）", Required: true},
			"replace": {Type: schema.String, Desc: "替换后的文本", Required: true},
			"glob":    {Type: schema.String, Desc: "文件匹配模式，如 '**/*.go'、'*.html'，默认 '**/*'"},
			"regex":   {Type: schema.Boolean, Desc: "是否将 search 视为正则表达式，默认 false"},
			"offset":  {Type: schema.Integer, Desc: "从第几个匹配文件开始扫描（从1开始），用于续接上一批"},
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

		// 改写是有副作用的批量操作：一次铺开上百个文件既难回滚也难审。
		// 保留上限没问题，问题是过去超出部分默默不做——现在改为显式报告未检视区间。
		const maxFilesPerCall = 20

		start := args.Offset
		if start <= 0 {
			start = 1
		}
		if start > len(allFiles) {
			return fmt.Sprintf("匹配文件共 %d 个，起始序号 %d 超出范围", len(allFiles), start), nil
		}

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

		var changedFiles []string
		var failedFiles []string
		totalReplacements := 0
		scanned := start - 1 // 本次真正检视过的匹配文件序号右界（含未命中的文件）
		relOf := func(p string) string {
			r, err := filepath.Rel(svc.projectRoot, p)
			if err != nil {
				return p
			}
			return r
		}

		for idx := start - 1; idx < len(allFiles); idx++ {
			if len(changedFiles) >= maxFilesPerCall {
				break
			}
			scanned = idx + 1
			fp := allFiles[idx]
			data, rerr := os.ReadFile(fp)
			if rerr != nil {
				failedFiles = append(failedFiles, fmt.Sprintf("  - %s（读取失败：%v）", relOf(fp), rerr))
				continue
			}
			var newData []byte
			var count int
			if args.Regex {
				count = len(re.FindAll(data, -1))
				if count > 0 {
					newData = re.ReplaceAll(data, []byte(args.Replace))
				}
			} else {
				count = bytes.Count(data, searchBytes)
				if count > 0 {
					newData = bytes.ReplaceAll(data, searchBytes, []byte(args.Replace))
				}
			}
			if count == 0 {
				continue
			}
			if werr := os.WriteFile(fp, newData, 0644); werr != nil {
				failedFiles = append(failedFiles, fmt.Sprintf("  - %s（写入失败：%v）", relOf(fp), werr))
				continue
			}
			invalidateReadCache(fp)
			changedFiles = append(changedFiles, fmt.Sprintf("  - %s (%d 处)", relOf(fp), count))
			totalReplacements += count
		}

		var b strings.Builder
		fmt.Fprintf(&b, "搜索替换：扫描第 %d–%d 个匹配文件（共 %d 个），改写 %d 个文件、替换 %d 处",
			start, scanned, len(allFiles), len(changedFiles), totalReplacements)
		if len(changedFiles) > 0 {
			b.WriteString("：\n\n")
			b.WriteString(strings.Join(changedFiles, "\n"))
		}
		if len(failedFiles) > 0 {
			fmt.Fprintf(&b, "\n\n以下 %d 个文件命中但未能改写：\n", len(failedFiles))
			b.WriteString(strings.Join(failedFiles, "\n"))
		}
		if scanned < len(allFiles) {
			fmt.Fprintf(&b, "\n\n[第 %d–%d 个匹配文件本次未检视，其中的匹配内容仍在原地。"+
				"单次调用最多改写 %d 个文件；继续执行请传 offset=%d，不要重复同一次调用]",
				scanned+1, len(allFiles), maxFilesPerCall, scanned+1)
		}
		return b.String(), nil
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
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("命令执行超时（%d秒）", args.Timeout)
		}

		var b strings.Builder
		// 退出码写在开头：输出过长时尾部会被预算裁掉，写在末尾的话模型最先丢掉的
		// 恰恰是「命令到底成功没有」这条最需要的信息。
		if err != nil {
			b.WriteString(fmt.Sprintf("$ %s\n[退出码/错误: %v]\n", args.Command, err))
		} else {
			b.WriteString(fmt.Sprintf("$ %s\n", args.Command))
		}
		if stdout.Len() > 0 {
			b.WriteString(stdout.String())
		}
		if stderr.Len() > 0 {
			b.WriteString("\nSTDERR:\n" + stderr.String())
		}
		return capNonPagingResult(b.String(), svc.resultBudget(), ""), nil
	})

	add(&schema.ToolInfo{
		Name: "grep",
		Desc: "在项目文件中搜索文本或正则表达式。可用 path 限定单个文件或子目录（读回 web_fetch/web_search 的存档就用 path），" +
			"或用 glob 限定文件模式、context 指定上下文行数。" +
			"结果按单次大小上限在匹配条目边界切窗，尾部会说明实际返回的序号区间；" +
			"若提示还有后续，按 offset 续读而不是重复同一查询。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"pattern": {Type: schema.String, Desc: "搜索模式文本", Required: true},
			"path":    {Type: schema.String, Desc: "只搜索该路径（相对项目根的文件或目录），可选；指定单个文件时不受大文件跳过限制"},
			"glob":    {Type: schema.String, Desc: "文件匹配模式，如 '*.go'、'*.html'，默认所有文件"},
			"context": {Type: schema.Integer, Desc: "上下文行数（包含匹配行前后各 N 行），默认 0"},
			"offset":  {Type: schema.Integer, Desc: "从第几处匹配开始返回（从1开始），可选"},
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

		// path 把搜索限定在单个文件或子目录。点名单个文件时不再套用"超大文件跳过"保护：
		// 模型要读的就是它（例如 web_fetch 的全文存档动辄几千行），默默跳过等于宣称
		// "未找到匹配"而其实一眼都没看。
		root := svc.projectRoot
		explicitFile := false
		if args.Path != "" {
			p, perr := svc.safePath(args.Path)
			if perr != nil {
				return "", perr
			}
			fi, serr := os.Stat(p)
			if serr != nil {
				return fmt.Sprintf("路径不存在或无法访问: %s", args.Path), nil
			}
			explicitFile = !fi.IsDir()
			root = p
		}

		var matches []match
		capped := false
		var skippedNames []string
		skippedTotal := 0
		noteSkip := func(rel, why string) {
			skippedTotal++
			if len(skippedNames) < 5 {
				skippedNames = append(skippedNames, fmt.Sprintf("%s（%s）", rel, why))
			}
		}

		filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				if fi != nil && fi.IsDir() {
					base := fi.Name()
					if strings.HasPrefix(base, ".") || base == "vendor" || base == "node_modules" {
						return filepath.SkipDir
					}
				}
				return nil
			}
			relPath, _ := filepath.Rel(svc.projectRoot, path)
			// Check glob
			if args.Glob != "" {
				matched, _ := filepath.Match(args.Glob, fi.Name())
				matchedRel, _ := filepath.Match(args.Glob, relPath)
				if !matched && !matchedRel {
					return nil
				}
			}
			// Skip oversized files（跳过要在结果里说明，否则"没搜到"是假话）
			if fi.Size() > 1024*1024 {
				if !explicitFile {
					noteSkip(relPath, "超过 1MB")
				}
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
			if len(lines) > 5000 && !explicitFile {
				noteSkip(relPath, "超过 5000 行")
				return nil
			}

			for i, line := range lines {
				if !re.MatchString(line) {
					continue
				}
				if len(matches) >= scanCollectCap {
					capped = true
					return errScanDone
				}
				m := match{File: relPath, Line: i + 1, Content: line}
				// Before context
				before := i - args.Context
				if before < 0 {
					before = 0
				}
				for j := before; j < i; j++ {
					m.Before = append(m.Before, fmt.Sprintf("  %d| %s", j+1, lines[j]))
				}
				// After context
				after := i + args.Context + 1
				if after > len(lines) {
					after = len(lines)
				}
				for j := i + 1; j < after; j++ {
					m.After = append(m.After, fmt.Sprintf("  %d| %s", j+1, lines[j]))
				}
				matches = append(matches, m)
			}
			return nil
		})

		// 被跳过的超大文件必须说出来："没搜到"和"没看"是两件事，前者会诱导模型下结论。
		skipNote := ""
		if skippedTotal > 0 {
			skipNote = fmt.Sprintf("\n\n[本次扫描跳过 %d 个超大文件（>1MB 或 >5000 行）：%s"+
				"，匹配可能正在其中。要搜它们请传 path=<单个文件路径>，或用 read_file 分段读取]",
				skippedTotal, strings.Join(skippedNames, "、"))
		}

		if len(matches) == 0 {
			return "未找到匹配的内容" + skipNote, nil
		}

		total := len(matches)
		start := args.Offset
		if start <= 0 {
			start = 1
		}
		if start > total {
			return fmt.Sprintf("共匹配 %d 处，起始序号 %d 超出范围", total, start) + skipNote, nil
		}
		seg := matches[start-1:]
		budget := svc.resultBudget()
		header := fmt.Sprintf("模式 %q 命中 %d 处（从第 %d 处开始）：\n\n", args.Pattern, total, start)
		body, delivered := fitWindow(seg, budget-len(header)-len(skipNote)-windowTrailerReserve,
			func(_ int, m match) string {
				var sb strings.Builder
				fmt.Fprintf(&sb, "%s:%d\n", m.File, m.Line)
				for _, before := range m.Before {
					sb.WriteString(before + "\n")
				}
				fmt.Fprintf(&sb, "  → %s\n", strings.TrimSpace(m.Content))
				for _, after := range m.After {
					sb.WriteString(after + "\n")
				}
				sb.WriteString("\n")
				return sb.String()
			})
		last := start + delivered - 1
		result := header + body + windowTrailer("处", start, last, total, budget, capped || delivered < len(seg)) + skipNote
		if capped {
			result += fmt.Sprintf("\n[命中数已达采集上限 %d，之后仍有未统计的匹配。"+
				"请收窄 pattern 或用 glob 限定文件范围]", scanCollectCap)
		}
		return result, nil
	})

	add(&schema.ToolInfo{
		Name: "glob",
		Desc: "按文件名模式查找文件。支持通配符：* 匹配任意字符，** 匹配任意目录层级。" +
			"结果按路径排序；单次返回会按大小上限切窗，尾部说明实际返回的序号区间，" +
			"若提示还有后续，按 offset 续读而不是换个写法重查。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"pattern": {Type: schema.String, Desc: "文件匹配模式，如 '**/*.go'、'template/**'、'*.html'", Required: true},
			"offset":  {Type: schema.Integer, Desc: "从第几个匹配项开始返回（从1开始），可选"},
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
		capped := false

		_ = filepath.Walk(svc.projectRoot, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if len(results) >= scanCollectCap {
				capped = true
				return errScanDone
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
			return nil
		})

		if len(results) == 0 {
			return "未找到匹配的文件", nil
		}

		// 先排序再切窗：offset 只有在稳定顺序上才有意义，否则续读会漏项或重复。
		sort.Strings(results)
		total := len(results)
		start := args.Offset
		if start <= 0 {
			start = 1
		}
		if start > total {
			return fmt.Sprintf("共匹配 %d 个文件/目录，起始序号 %d 超出范围", total, start), nil
		}
		seg := results[start-1:]
		budget := svc.resultBudget()
		header := fmt.Sprintf("模式 %q 匹配 %d 个文件/目录（从第 %d 个开始）：\n\n", args.Pattern, total, start)
		body, delivered := fitWindow(seg, budget-len(header)-windowTrailerReserve,
			func(_ int, r string) string { return r + "\n" })
		last := start + delivered - 1
		result := header + body + windowTrailer("个", start, last, total, budget, capped || delivered < len(seg))
		if capped {
			result += fmt.Sprintf("\n[匹配数已达采集上限 %d，之后仍有未统计的项。请收窄 pattern]", scanCollectCap)
		}
		return result, nil
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
		Desc: "获取指定URL的网页内容并返回纯文本。用于查看网页信息、API文档等。" +
			"内容超长时只返回前一段，并把全文存档为项目文件——按结果末尾标记里的路径，用 read_file 的 offset 或 grep 续读。",
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
			return svc.capWebResult("web_fetch", fmt.Sprintf("URL: %s\n状态码: %d\n大小: %d 字节\n\n%s",
				args.URL, resp.StatusCode, len(body), string(body))), nil
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

		return svc.capWebResult("web_fetch", fmt.Sprintf("URL: %s\n状态码: %d\n标题: %s\n\n%s",
			args.URL, resp.StatusCode, title, joined)), nil
	})

	add(&schema.ToolInfo{
		Name: "web_search",
		Desc: "搜索互联网获取最新信息。可访问外网时使用 DuckDuckGo，否则回退使用 Bing 搜索。" +
			"结果超长时只返回前一段，并把全文存档为项目文件——按结果末尾标记里的路径用 grep 或 read_file 续读。",
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

		var out string
		var serr error
		if config.GoogleValid {
			out, serr = searchDuckDuckGo(ctx, args.Query)
		} else {
			out, serr = searchBing(ctx, args.Query)
		}
		if serr != nil {
			return "", serr
		}
		return svc.capWebResult("web_search", out), nil
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
