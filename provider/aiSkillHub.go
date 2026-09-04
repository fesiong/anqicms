package provider

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kandaoni.com/anqicms/config"
)

// ================================================================
// SkillHub 技能仓库集成 (https://skillhub.cn)
//
// 不使用 skillhub CLI 的 shell 安装方式，而是直接调用其 HTTP API:
//   - 搜索: GET https://api.skillhub.cn/api/v1/search?q={query}&limit={n}
//   - 下载: GET https://api.skillhub.cn/api/v1/download?slug={slug}
//           → 302 重定向到 COS 上的 .zip 包
//   - 备用下载: GET https://skillhub-1388575217.cos.ap-guangzhou.myqcloud.com/skills/{slug}.zip
//
// 下载的 zip 包内含 SKILL.md + 可选资源文件，解压到全局技能目录
// (config.ExecPath/data/skills/{slug}/)。
// ================================================================

const (
	// SkillHubAPIBase SkillHub 公共 API 基址
	SkillHubAPIBase = "https://api.skillhub.cn"
	// SkillHubCosBase SkillHub COS 备用下载基址
	SkillHubCosBase = "https://skillhub-1388575217.cos.ap-guangzhou.myqcloud.com"
	// SkillHubHTTPTimeout HTTP 请求超时
	SkillHubHTTPTimeout = 30 * time.Second
	// SkillHubDownloadMaxBytes 下载 zip 包最大字节数 (50MB)
	SkillHubDownloadMaxBytes = 50 * 1024 * 1024
)

// ── 数据结构 ──

// SkillHubSearchResult SkillHub 搜索 API 返回的单条技能
type SkillHubSearchResult struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Version     string `json:"version"`
	Downloads   int    `json:"downloads"`
	Stars       int    `json:"stars"`
	Installs    int    `json:"installs"`
	Homepage    string `json:"homepage"`
	OwnerName   string `json:"owner_name"`
	Namespace   struct {
		CanonicalName string `json:"canonicalName"`
		Handle        string `json:"handle"`
		PublicSlug    string `json:"publicSlug"`
	} `json:"namespace"`
}

// SkillHubSearchResponse SkillHub 搜索 API 返回结构
type SkillHubSearchResponse struct {
	Results []SkillHubSearchResult `json:"results"`
}

// SkillHubSkillInfo SkillHub 技能详情
type SkillHubSkillInfo struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Version     string `json:"version"`
	Downloads   int    `json:"downloads"`
	Stars       int    `json:"stars"`
	Owner       struct {
		Handle string `json:"handle"`
		Name   string `json:"name"`
	} `json:"owner"`
}

// ── HTTP 客户端 ──

// skillHubHTTPClient 单例 HTTP 客户端 (带超时)
var skillHubHTTPClient = &http.Client{
	Timeout: SkillHubHTTPTimeout,
	// 允许跟随重定向 (download 端点 302 重定向到 COS)
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		// 最多跟随 5 次重定向
		if len(via) >= 5 {
			return fmt.Errorf("stopped after 5 redirects")
		}
		return nil
	},
}

// skillHubDo 执行 HTTP 请求并返回响应体字节
func skillHubDo(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AnQiCMS-SkillHub-Client/1.0")

	resp, err := skillHubHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("请求 SkillHub 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, SkillHubDownloadMaxBytes))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("读取响应失败: %w", err)
	}

	return body, resp.StatusCode, nil
}

// ── 搜索 ──

// SearchSkillHub 在 SkillHub 搜索技能
// query: 搜索关键词 (如 "pdf", "code review")
// limit: 返回结果上限 (1-50)
func SearchSkillHub(ctx context.Context, query string, limit int) (*SkillHubSearchResponse, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	url := fmt.Sprintf("%s/api/v1/search?q=%s&limit=%d",
		SkillHubAPIBase,
		skillHubURLEncode(query),
		limit)

	body, status, err := skillHubDo(ctx, url)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("SkillHub 搜索返回 HTTP %d: %s", status, truncateBody(body))
	}

	var resp SkillHubSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析 SkillHub 搜索结果失败: %w", err)
	}

	return &resp, nil
}

// ── 技能详情 ──

// GetSkillHubSkill 获取 SkillHub 上指定 slug 的技能详情
func GetSkillHubSkill(ctx context.Context, slug string) (*SkillHubSkillInfo, error) {
	if strings.TrimSpace(slug) == "" {
		return nil, fmt.Errorf("技能 slug 不能为空")
	}

	url := fmt.Sprintf("%s/api/v1/skills/%s",
		SkillHubAPIBase,
		skillHubURLEncode(slug))

	body, status, err := skillHubDo(ctx, url)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, fmt.Errorf("SkillHub 上不存在技能 %q", slug)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("SkillHub 详情返回 HTTP %d: %s", status, truncateBody(body))
	}

	// API 可能返回 {data: {...}} 或直接 {...}
	var wrapper struct {
		Data *SkillHubSkillInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Data != nil {
		return wrapper.Data, nil
	}

	var info SkillHubSkillInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("解析 SkillHub 技能详情失败: %w", err)
	}
	if info.Slug == "" {
		info.Slug = slug
	}

	return &info, nil
}

// ── 下载并安装 ──

// InstallSkillFromSkillHub 从 SkillHub 下载技能 zip 包并安装到全局技能目录
//
// 流程:
//  1. GET /api/v1/download?slug={slug} → 302 重定向到 COS zip
//  2. 下载 zip 到临时文件
//  3. 解压 zip 到 {globalSkillsDir}/{slug}/
//  4. 校验 SKILL.md 存在
//  5. 重新加载技能后端
//
// force=true 时先删除已存在的同名技能目录
func InstallSkillFromSkillHub(ctx context.Context, slug string, force bool) (*SkillHubSkillInfo, error) {
	if strings.TrimSpace(slug) == "" {
		return nil, fmt.Errorf("技能 slug 不能为空")
	}

	// 1. 获取技能详情 (确认技能存在)
	info, err := GetSkillHubSkill(ctx, slug)
	if err != nil {
		return nil, err
	}

	// 2. 确定安装目录
	globalSkillsDir := filepath.Join(strings.TrimSuffix(config.ExecPath, "/"), "data", "skills")
	installDir := filepath.Join(globalSkillsDir, slug)

	// 3. 检查是否已存在
	if _, statErr := os.Stat(installDir); statErr == nil {
		if !force {
			return info, fmt.Errorf("技能 %q 已安装，使用 force=true 覆盖安装", slug)
		}
		// force: 先删除已有目录
		if rmErr := os.RemoveAll(installDir); rmErr != nil {
			return info, fmt.Errorf("删除已有技能目录失败: %w", rmErr)
		}
	}

	// 4. 下载 zip 包
	zipBytes, err := downloadSkillZip(ctx, slug)
	if err != nil {
		return info, err
	}

	// 5. 创建安装目录
	if mkErr := os.MkdirAll(installDir, 0755); mkErr != nil {
		return info, fmt.Errorf("创建技能目录失败: %w", mkErr)
	}

	// 6. 解压 zip 到安装目录
	if extractErr := extractSkillZip(zipBytes, installDir); extractErr != nil {
		// 解压失败，清理已创建的目录
		_ = os.RemoveAll(installDir)
		return info, fmt.Errorf("解压技能包失败: %w", extractErr)
	}

	// 7. 校验 SKILL.md 存在 (可能在子目录中)
	skillMdPath, findErr := findSkillMd(installDir)
	if findErr != nil {
		_ = os.RemoveAll(installDir)
		return info, fmt.Errorf("技能包缺少 SKILL.md: %w", findErr)
	}

	// 8. 如果 SKILL.md 不在 installDir 根目录，将子目录内容提升到根目录
	if filepath.Dir(skillMdPath) != installDir {
		subDir := filepath.Dir(skillMdPath)
		if promoteErr := promoteSubdir(installDir, subDir); promoteErr != nil {
			return info, fmt.Errorf("提升技能子目录失败: %w", promoteErr)
		}
	}

	// 9. 重新加载全局技能后端
	backend := GetSkillBackend()
	if reloadErr := backend.Reload(ctx); reloadErr != nil {
		// 安装成功但重新加载失败，不回滚，只警告
		return info, fmt.Errorf("技能已安装但重新加载失败: %w", reloadErr)
	}

	return info, nil
}

// downloadSkillZip 下载技能 zip 包
// 优先使用 API 端点 (会处理重定向)，失败时回退到 COS 直链
func downloadSkillZip(ctx context.Context, slug string) ([]byte, error) {
	// 主下载端点: GET /api/v1/download?slug={slug}
	primaryURL := fmt.Sprintf("%s/api/v1/download?slug=%s",
		SkillHubAPIBase, skillHubURLEncode(slug))

	zipBytes, _, err := skillHubDo(ctx, primaryURL)
	if err == nil && isZipArchive(zipBytes) {
		return zipBytes, nil
	}

	// 备用下载端点: COS 直链
	fallbackURL := fmt.Sprintf("%s/skills/%s.zip",
		SkillHubCosBase, skillHubURLEncode(slug))

	zipBytes, status, err := skillHubDo(ctx, fallbackURL)
	if err != nil {
		return nil, fmt.Errorf("下载技能包失败 (主+备): %w", err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("下载技能包返回 HTTP %d", status)
	}
	if !isZipArchive(zipBytes) {
		return nil, fmt.Errorf("下载的内容不是有效的 zip 包")
	}

	return zipBytes, nil
}

// extractSkillZip 将 zip 字节流解压到目标目录
// 安全约束: 防止 zip slip (路径穿越攻击)
func extractSkillZip(zipBytes []byte, destDir string) error {
	// 使用 zip.NewReader 避免写入临时文件
	reader := strings.NewReader(string(zipBytes))
	zipReader, err := zip.NewReader(reader, int64(len(zipBytes)))
	if err != nil {
		return fmt.Errorf("打开 zip 包失败: %w", err)
	}

	// 确保目标目录存在
	if mkErr := os.MkdirAll(destDir, 0755); mkErr != nil {
		return fmt.Errorf("创建目标目录失败: %w", mkErr)
	}

	for _, f := range zipReader.File {
		// 安全检查: 防止 zip slip
		fpath := filepath.Join(destDir, f.Name)
		if !strings.HasPrefix(filepath.Clean(fpath), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("zip slip 检测: 非法路径 %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			// 创建目录
			if mkErr := os.MkdirAll(fpath, 0755); mkErr != nil {
				return fmt.Errorf("创建目录 %s 失败: %w", fpath, mkErr)
			}
			continue
		}

		// 确保父目录存在
		if mkErr := os.MkdirAll(filepath.Dir(fpath), 0755); mkErr != nil {
			return fmt.Errorf("创建父目录失败: %w", mkErr)
		}

		// 打开 zip 内文件
		rc, openErr := f.Open()
		if openErr != nil {
			return fmt.Errorf("打开 zip 内文件 %s 失败: %w", f.Name, openErr)
		}

		// 写入目标文件
		outFile, createErr := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if createErr != nil {
			rc.Close()
			return fmt.Errorf("创建文件 %s 失败: %w", fpath, createErr)
		}

		if _, copyErr := io.Copy(outFile, rc); copyErr != nil {
			rc.Close()
			outFile.Close()
			return fmt.Errorf("写入文件 %s 失败: %w", fpath, copyErr)
		}

		rc.Close()
		outFile.Close()
	}

	return nil
}

// ── 辅助函数 ──

// isZipArchive 检查字节流是否为 zip 包 (PK\x03\x04 魔数)
func isZipArchive(data []byte) bool {
	return len(data) >= 4 && data[0] == 0x50 && data[1] == 0x4B && data[2] == 0x03 && data[3] == 0x04
}

// findSkillMd 在目录树中查找 SKILL.md
func findSkillMd(rootDir string) (string, error) {
	// 先检查根目录
	rootSkillMd := filepath.Join(rootDir, "SKILL.md")
	if _, err := os.Stat(rootSkillMd); err == nil {
		return rootSkillMd, nil
	}

	// 递归查找 (最多 3 层深度)
	var found string
	walkErr := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "SKILL.md" {
			found = path
			return filepath.SkipDir
		}
		return nil
	})

	if walkErr != nil {
		return "", walkErr
	}
	if found == "" {
		return "", fmt.Errorf("未找到 SKILL.md")
	}
	return found, nil
}

// promoteSubdir 将子目录内容移动到父目录 (处理 zip 包内多了一层目录的情况)
func promoteSubdir(parentDir, subDir string) error {
	// 读取子目录所有条目
	entries, err := os.ReadDir(subDir)
	if err != nil {
		return err
	}

	// 将子目录条目移动到父目录
	for _, entry := range entries {
		srcPath := filepath.Join(subDir, entry.Name())
		dstPath := filepath.Join(parentDir, entry.Name())
		if renameErr := os.Rename(srcPath, dstPath); renameErr != nil {
			return renameErr
		}
	}

	// 删除空的子目录
	return os.Remove(subDir)
}

// skillHubURLEncode 对字符串做 URL 编码
func skillHubURLEncode(s string) string {
	// 简单实现: 替换空格等特殊字符
	// 完整实现可用 net/url.QueryEscape
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "%20"), "/", "%2F")
}

// truncateBody 截断响应体用于错误消息
func truncateBody(body []byte) string {
	if len(body) > 200 {
		return string(body[:200]) + "..."
	}
	return string(body)
}

// ── 批量安装 ──

// BatchInstallSkillsFromSkillHub 批量安装技能
// 成功安装的 slug 列表和失败的 slug→error 映射
func BatchInstallSkillsFromSkillHub(ctx context.Context, slugs []string, force bool) (installed []string, failed map[string]string) {
	failed = make(map[string]string)
	for _, slug := range slugs {
		_, err := InstallSkillFromSkillHub(ctx, slug, force)
		if err != nil {
			failed[slug] = err.Error()
		} else {
			installed = append(installed, slug)
		}
	}
	return
}

// ── 格式化输出 (供 AI 工具调用返回) ──

// FormatSkillHubSearchResults 格式化搜索结果为 AI 可读文本
func FormatSkillHubSearchResults(resp *SkillHubSearchResponse, query string) string {
	if resp == nil || len(resp.Results) == 0 {
		return fmt.Sprintf("未找到与 %q 相关的技能。", query)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## SkillHub 搜索结果: %q\n\n", query))
	sb.WriteString(fmt.Sprintf("找到 %d 个技能:\n\n", len(resp.Results)))

	for i, skill := range resp.Results {
		sb.WriteString(fmt.Sprintf("### %d. %s\n", i+1, skill.DisplayName))
		sb.WriteString(fmt.Sprintf("- **slug**: %s\n", skill.Slug))
		desc := skill.Summary
		if desc == "" {
			desc = skill.Description
		}
		sb.WriteString(fmt.Sprintf("- **描述**: %s\n", desc))
		sb.WriteString(fmt.Sprintf("- **分类**: %s\n", skill.Category))
		sb.WriteString(fmt.Sprintf("- **版本**: %s\n", skill.Version))
		sb.WriteString(fmt.Sprintf("- **下载量**: %d | **收藏**: %d | **安装量**: %d\n",
			skill.Downloads, skill.Stars, skill.Installs))
		if skill.Namespace.Handle != "" {
			sb.WriteString(fmt.Sprintf("- **命名空间**: %s (%s)\n",
				skill.Namespace.CanonicalName, skill.Namespace.Handle))
		}
		if skill.OwnerName != "" {
			sb.WriteString(fmt.Sprintf("- **作者**: %s\n", skill.OwnerName))
		}
		sb.WriteString(fmt.Sprintf("\n安装命令: `skill_install slug=%s`\n\n", skill.Slug))
	}

	return sb.String()
}

// FormatSkillHubInstallResult 格式化安装结果为 AI 可读文本
func FormatSkillHubInstallResult(info *SkillHubSkillInfo, err error) string {
	if err != nil {
		return fmt.Sprintf("❌ 技能安装失败: %s", err.Error())
	}

	var sb strings.Builder
	sb.WriteString("✅ 技能安装成功！\n\n")
	sb.WriteString(fmt.Sprintf("- **名称**: %s\n", info.DisplayName))
	sb.WriteString(fmt.Sprintf("- **slug**: %s\n", info.Slug))
	sb.WriteString(fmt.Sprintf("- **版本**: %s\n", info.Version))
	sb.WriteString(fmt.Sprintf("- **描述**: %s\n", info.Summary))
	if info.Owner.Handle != "" {
		sb.WriteString(fmt.Sprintf("- **作者**: %s (%s)\n", info.Owner.Name, info.Owner.Handle))
	}
	sb.WriteString("\n使用 `skill_get name=" + info.Slug + "` 加载技能内容。")
	return sb.String()
}
