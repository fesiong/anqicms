package provider

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"gopkg.in/yaml.v3"
	"kandaoni.com/anqicms/config"
)

// ================================================================
//  Skill 结构体 — 与 AtomCode 兼容
// ================================================================

// SkillFrontMatter 技能的 YAML 前置元数据
type SkillFrontMatter struct {
	Name                   string   `yaml:"name" json:"name"`
	Description            string   `yaml:"description" json:"description"`
	Category               string   `yaml:"category" json:"category"`
	Version                string   `yaml:"version" json:"version"`
	Author                 string   `yaml:"author" json:"author"`
	Tags                   []string `yaml:"tags" json:"tags"`
	DisableModelInvocation bool     `yaml:"disable_model_invocation" json:"disable_model_invocation"`
	UserInvocable          bool     `yaml:"user_invocable" json:"user_invocable"`
	ArgumentHint           string   `yaml:"argument_hint" json:"argument_hint"`
	AllowedTools           []string `yaml:"allowed_tools" json:"allowed_tools"`
}

// Skill 是一个已加载的完整技能
type Skill struct {
	SkillFrontMatter
	Content       string    `json:"content"`
	BaseDirectory string    `json:"base_directory"`
	UpdatedAt     time.Time `json:"updated_at"`
	SourcePath    string    `json:"source_path"`
	// 来源标识：global / project / plugin:{name}
	Source string `json:"source"`
	// 命名空间（插件技能使用）
	Namespace string `json:"namespace,omitempty"`
}

// FullName 返回完整的技能名称（含命名空间）
func (s *Skill) FullName() string {
	if s.Namespace != "" {
		return s.Namespace + ":" + s.Name
	}
	return s.Name
}

// Expand 展开模板中的变量替换，与 AtomCode 兼容
func (s *Skill) Expand(arguments string, sessionID string) string {
	result := s.Content

	// 1. $ARGUMENTS[N] 位置参数
	positional := strings.Fields(arguments)
	for i, arg := range positional {
		result = strings.ReplaceAll(result, fmt.Sprintf("$ARGUMENTS[%d]", i), arg)
	}

	// 2. $N 简写
	for i, arg := range positional {
		result = strings.ReplaceAll(result, fmt.Sprintf("$%d", i), arg)
	}

	// 3. $ARGUMENTS — 仅当模板中使用了 $ARGUMENTS 标记时才替换
	if strings.Contains(s.Content, "$ARGUMENTS") {
		result = strings.ReplaceAll(result, "$ARGUMENTS", arguments)
	} else if strings.TrimSpace(arguments) != "" {
		result = result + "\n\nARGUMENTS: " + arguments
	}

	// 4. ${CLAUDE_SESSION_ID}
	result = strings.ReplaceAll(result, "${CLAUDE_SESSION_ID}", sessionID)

	// 5. ${CLAUDE_SKILL_DIR}
	result = strings.ReplaceAll(result, "${CLAUDE_SKILL_DIR}", s.BaseDirectory)

	// 6. !`command` shell 注入
	result = expandShellInjections(result)

	return result
}

// expandShellInjections 执行模板中的 !`command` 并替换为输出
func expandShellInjections(tmpl string) string {
	var result strings.Builder
	remaining := tmpl
	for {
		start := strings.Index(remaining, "!`")
		if start == -1 {
			result.WriteString(remaining)
			break
		}
		result.WriteString(remaining[:start])
		after := remaining[start+2:]
		end := strings.Index(after, "`")
		if end == -1 {
			result.WriteString("!(")
			remaining = after
			continue
		}
		cmd := after[:end]
		output := runShellCommand(cmd)
		result.WriteString(output)
		remaining = after[end+1:]
	}
	return result.String()
}

func runShellCommand(cmd string) string {
	var command exec.Cmd
	command = *exec.Command("sh", "-c", cmd)
	out, err := command.Output()
	if err != nil {
		var stderrBuf bytes.Buffer
		command.Stderr = &stderrBuf
		_ = command.Run()
		if stderrBuf.Len() > 0 {
			return strings.TrimSpace(stderrBuf.String())
		}
		return fmt.Sprintf("[error: %s]", err)
	}
	return strings.TrimSpace(string(out))
}

// ================================================================
//  SkillBackend 接口
// ================================================================

type SkillBackend interface {
	List(ctx context.Context) ([]SkillFrontMatter, error)
	Get(ctx context.Context, name string) (*Skill, error)
	Save(ctx context.Context, skill *Skill) error
	Reload(ctx context.Context) error
}

// ================================================================
//  FilesystemSkillBackend — 多目录扫描 + 命名空间技能
// ================================================================

type FilesystemSkillBackend struct {
	mu     sync.RWMutex
	skills map[string]*Skill // keyed by FullName()
	dirs   []string          // 扫描目录（低→高优先级）
}

// NewFilesystemSkillBackend 创建多目录 SkillBackend
// globalDir: config.ExecPath + "data/skills" (全局技能)
// projectDir: site.RootPath + "/data/skills" (项目技能，可空)
func NewFilesystemSkillBackend(projectDir string) *FilesystemSkillBackend {
	globalDir := filepath.Join(strings.TrimSuffix(config.ExecPath, "/"), "data", "skills")
	// 确保全局技能目录存在
	_ = os.MkdirAll(globalDir, 0755)

	// 内嵌种子：首次运行时将内嵌的默认技能提取到全局目录
	ensureSeedSkills(globalDir)

	dirs := []string{globalDir}
	if projectDir != "" && projectDir != globalDir {
		_ = os.MkdirAll(projectDir, 0755)
		dirs = append(dirs, projectDir)
	}

	// 插件目录
	pluginDir := filepath.Join(strings.TrimSuffix(config.ExecPath, "/"), "data", "plugins")
	if entries, err := os.ReadDir(pluginDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				pSkillDir := filepath.Join(pluginDir, entry.Name(), "skills")
				if info, err := os.Stat(pSkillDir); err == nil && info.IsDir() {
					dirs = append(dirs, pSkillDir)
				}
			}
		}
	}

	b := &FilesystemSkillBackend{
		skills: make(map[string]*Skill),
		dirs:   dirs,
	}
	if err := b.Reload(context.Background()); err != nil {
		log.Printf("[skill] initial reload failed: %v", err)
	}
	log.Printf("[skill] backend initialized, dirs=%v, count=%d", dirs, len(b.skills))
	return b
}

// ================================================================
//  内嵌种子技能
// ================================================================

//go:embed seeds/skills/*/SKILL.md
var seedSkillsFS embed.FS

// ensureSeedSkills 将内嵌的种子技能提取到全局技能目录
// 如果目标目录中已存在同名技能则跳过
func ensureSeedSkills(globalDir string) {
	entries, err := seedSkillsFS.ReadDir("seeds/skills")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillName := entry.Name()
		targetDir := filepath.Join(globalDir, skillName)
		targetFile := filepath.Join(targetDir, "SKILL.md")
		// 已存在则跳过
		if _, err := os.Stat(targetFile); err == nil {
			continue
		}
		// 读取内嵌的 SKILL.md
		embedPath := fmt.Sprintf("seeds/skills/%s/SKILL.md", skillName)
		data, err := seedSkillsFS.ReadFile(embedPath)
		if err != nil {
			log.Printf("[skill] failed to read embedded seed %s: %v", skillName, err)
			continue
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			log.Printf("[skill] failed to create dir for seed %s: %v", skillName, err)
			continue
		}
		if err := os.WriteFile(targetFile, data, 0644); err != nil {
			log.Printf("[skill] failed to write seed %s: %v", skillName, err)
			continue
		}
		log.Printf("[skill] extracted seed skill: %s", skillName)
	}
}

// ================================================================
//  插件技能源管理
// ================================================================

// InstallSkillPluginFromGit 从 Git 仓库安装技能插件
// 克隆到 data/plugins/{name}/ 然后重新加载
func InstallSkillPluginFromGit(name, repoURL string) error {
	if name == "" || repoURL == "" {
		return fmt.Errorf("name and repo URL are required")
	}
	pluginDir := filepath.Join(strings.TrimSuffix(config.ExecPath, "/"), "data", "plugins", name)
	// 已存在则先更新
	if _, err := os.Stat(pluginDir); err == nil {
		// git pull
		cmd := exec.Command("git", "-C", pluginDir, "pull", "--ff-only")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git pull failed: %s: %w", string(out), err)
		}
	} else {
		// git clone
		parentDir := filepath.Dir(pluginDir)
		_ = os.MkdirAll(parentDir, 0755)
		cmd := exec.Command("git", "clone", repoURL, pluginDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git clone failed: %s: %w", string(out), err)
		}
	}
	return nil
}

// UninstallSkillPlugin 卸载技能插件
func UninstallSkillPlugin(name string) error {
	pluginDir := filepath.Join(strings.TrimSuffix(config.ExecPath, "/"), "data", "plugins", name)
	return os.RemoveAll(pluginDir)
}

// ListSkillPlugins 列出已安装的技能插件
func ListSkillPlugins() ([]string, error) {
	pluginDir := filepath.Join(strings.TrimSuffix(config.ExecPath, "/"), "data", "plugins")
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// ================================================================
//  SkillBackend 实现
// ================================================================

func (b *FilesystemSkillBackend) List(_ context.Context) ([]SkillFrontMatter, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	list := make([]SkillFrontMatter, 0, len(b.skills))
	for _, s := range b.skills {
		list = append(list, s.SkillFrontMatter)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list, nil
}

func (b *FilesystemSkillBackend) Get(_ context.Context, name string) (*Skill, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// 精确查找
	if s, ok := b.skills[name]; ok {
		clone := *s
		return &clone, nil
	}
	// 命名空间回退：如果未指定命名空间，查找唯一匹配的后缀
	if !strings.Contains(name, ":") {
		suffix := ":" + name
		var found *Skill
		for k, s := range b.skills {
			if strings.HasSuffix(k, suffix) {
				if found != nil {
					return nil, fmt.Errorf("skill %q 在多个命名空间中存在，请使用完整名称", name)
				}
				found = s
			}
		}
		if found != nil {
			clone := *found
			return &clone, nil
		}
	}
	return nil, fmt.Errorf("skill %q not found", name)
}

func (b *FilesystemSkillBackend) Save(_ context.Context, skill *Skill) error {
	if skill == nil || skill.Name == "" {
		return fmt.Errorf("skill name is required")
	}

	// 保存到全局目录
	globalDir := b.dirs[0]
	skillDir := filepath.Join(globalDir, skill.Name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("创建技能目录失败: %w", err)
	}

	// 构建 frontmatter + 内容
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(skill.SkillFrontMatter); err != nil {
		return fmt.Errorf("编码 frontmatter 失败: %w", err)
	}
	enc.Close()

	fullContent := fmt.Sprintf("---\n%s---\n\n%s", buf.String(), skill.Content)
	mdPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(mdPath, []byte(fullContent), 0644); err != nil {
		return fmt.Errorf("写入 SKILL.md 失败: %w", err)
	}

	return b.Reload(context.Background())
}

func (b *FilesystemSkillBackend) Reload(_ context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	newSkills := make(map[string]*Skill)
	// 按优先级从低到高扫描，后加载的同名技能覆盖前面的
	for _, dir := range b.dirs {
		b.scanDir(newSkills, dir, "")
	}
	// 扫描插件目录获取命名空间技能
	b.scanPlugins(newSkills)
	b.skills = newSkills
	return nil
}

// scanDir 扫描单个目录（扁平 *.md 和 SKILL.md 子目录）
func (b *FilesystemSkillBackend) scanDir(skills map[string]*Skill, dir string, namespace string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			// 子目录：查找 SKILL.md
			skillMD := filepath.Join(path, "SKILL.md")
			if info, err := os.Stat(skillMD); err == nil && !info.IsDir() {
				if skill := parseSkillFromFile(skillMD, namespace); skill != nil {
					skill.Source = b.skillSource(dir)
					key := skill.FullName()
					skills[key] = skill
				}
			}
		} else if strings.HasSuffix(entry.Name(), ".md") && entry.Name() != "README.md" {
			// 扁平 *.md 文件（仅在顶层）
			if skill := parseSkillFromFile(path, namespace); skill != nil {
				skill.Source = b.skillSource(dir)
				key := skill.FullName()
				skills[key] = skill
			}
		}
	}
}

func (b *FilesystemSkillBackend) scanPlugins(skills map[string]*Skill) {
	pluginDir := filepath.Join(strings.TrimSuffix(config.ExecPath, "/"), "data", "plugins")
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginName := entry.Name()
		pSkillsDir := filepath.Join(pluginDir, pluginName, "skills")
		if info, err := os.Stat(pSkillsDir); err == nil && info.IsDir() {
			b.scanDir(skills, pSkillsDir, pluginName)
		}
	}
}

func (b *FilesystemSkillBackend) skillSource(dir string) string {
	globalDir := b.dirs[0]
	dir = filepath.Clean(dir)
	if dir == filepath.Clean(globalDir) {
		return "global"
	}
	if len(b.dirs) > 1 && dir == filepath.Clean(b.dirs[1]) {
		return "project"
	}
	pluginDir := filepath.Join(strings.TrimSuffix(config.ExecPath, "/"), "data", "plugins")
	if strings.HasPrefix(dir, pluginDir) {
		rel, _ := filepath.Rel(pluginDir, dir)
		parts := strings.SplitN(rel, string(filepath.Separator), 2)
		if len(parts) > 0 {
			return "plugin:" + parts[0]
		}
	}
	return "global"
}

// ================================================================
//  SKILL.md 解析
// ================================================================

func parseSkillFromFile(path string, namespace string) *Skill {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	fm, body := parseSkillFrontmatter(string(data))

	// 名称：文件名（不含扩展名）或 frontmatter 中的 name
	baseName := fm.Name
	if baseName == "" {
		baseName = strings.TrimSuffix(filepath.Base(path), ".md")
		baseName = strings.TrimSuffix(baseName, ".SKILL")
	}
	fm.Name = baseName

	if fm.Description == "" {
		fm.Description = firstParagraph(body)
	}

	skill := &Skill{
		SkillFrontMatter: fm,
		Content:          strings.TrimSpace(body),
		BaseDirectory:    filepath.Dir(path),
		SourcePath:       path,
		Namespace:        namespace,
	}
	if info, err := os.Stat(path); err == nil {
		skill.UpdatedAt = info.ModTime()
	}
	return skill
}

// ================================================================
//  Frontmatter 解析（YAML）
// ================================================================

func parseSkillFrontmatter(content string) (SkillFrontMatter, string) {
	content = strings.TrimSpace(content)
	var fm SkillFrontMatter
	body := content

	if strings.HasPrefix(content, "---") {
		rest := content[3:]
		end := strings.Index(rest, "\n---")
		if end >= 0 {
			yamlBlock := rest[:end]
			body = strings.TrimSpace(rest[end+4:])
			_ = yaml.Unmarshal([]byte(yamlBlock), &fm)
		}
	}

	return fm, body
}

func firstParagraph(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return trimmed
		}
	}
	return ""
}

// ================================================================
//  全局单例
// ================================================================

var (
	globalSkillBackend     *FilesystemSkillBackend
	globalSkillBackendOnce sync.Once
)

// GetSkillBackend 返回全局技能后端（无项目路径）
func GetSkillBackend() *FilesystemSkillBackend {
	globalSkillBackendOnce.Do(func() {
		globalSkillBackend = NewFilesystemSkillBackend("")
	})
	return globalSkillBackend
}

// GetSkillBackendForSite 返回站点特定的技能后端（含项目级技能）
func GetSkillBackendForSite(projectRoot string) *FilesystemSkillBackend {
	projectDir := filepath.Join(projectRoot, "data", "skills")
	return NewFilesystemSkillBackend(projectDir)
}

// ================================================================
//  AI 工具：skill_list / skill_get / skill_reload / skill_save
// ================================================================

func skillListTool() (*schema.ToolInfo, toolHandler) {
	return &schema.ToolInfo{
			Name:        "skill_list",
			Desc:        "列出所有可用的技能(SKILL)。返回每个技能的名称、描述、分类和来源。先查看可用技能，再调用 skill_get 加载具体内容。",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
		}, func(ctx context.Context, argsJSON string) (string, error) {
			backend := GetSkillBackend()
			list, err := backend.List(ctx)
			if err != nil {
				return "", fmt.Errorf("获取技能列表失败: %w", err)
			}
			if len(list) == 0 {
				return "当前没有可用的技能。", nil
			}
			var sb strings.Builder
			sb.WriteString("## 📋 可用技能列表\n\n")
			for _, s := range list {
				fullName := s.Name
				sb.WriteString(fmt.Sprintf("### %s\n", fullName))
				sb.WriteString(fmt.Sprintf("- **描述**: %s\n", s.Description))
				if s.Category != "" {
					sb.WriteString(fmt.Sprintf("- **分类**: %s\n", s.Category))
				}
				if s.Version != "" {
					sb.WriteString(fmt.Sprintf("- **版本**: %s\n", s.Version))
				}
				if len(s.Tags) > 0 {
					sb.WriteString(fmt.Sprintf("- **标签**: %s\n", strings.Join(s.Tags, ", ")))
				}
				sb.WriteString("\n")
			}
			return sb.String(), nil
		}
}

func skillGetTool() (*schema.ToolInfo, toolHandler) {
	return &schema.ToolInfo{
			Name: "skill_get",
			Desc: "获取指定技能(SKILL)的完整内容，支持变量替换。参数: name=技能名称(必填), arguments=传递给技能的参数(可选)。返回技能完整说明文档。",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"name": {
					Type: schema.String,
					Desc: "技能名称，必填。插件技能用 {插件名}:{技能名} 格式。",
				},
				"arguments": {
					Type: schema.String,
					Desc: "传递给技能的参数（可选），可在模板中用 $ARGUMENTS、$0、$1 等引用",
				},
			}),
		}, func(ctx context.Context, argsJSON string) (string, error) {
			var args struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				return "", fmt.Errorf("无法解析参数: %w", err)
			}
			if args.Name == "" {
				return "错误：技能名称不能为空，请先使用 skill_list 查看可用技能。", nil
			}
			backend := GetSkillBackend()
			skill, err := backend.Get(ctx, args.Name)
			if err != nil {
				// 尝试查找相似技能
				list, _ := backend.List(ctx)
				var similar []string
				for _, s := range list {
					if strings.Contains(strings.ToLower(s.Name), strings.ToLower(args.Name)) {
						similar = append(similar, s.Name)
					}
				}
				if len(similar) > 0 {
					return fmt.Sprintf("未找到技能 %q。相似技能: %s", args.Name, strings.Join(similar, ", ")), nil
				}
				return fmt.Sprintf("未找到技能 %q。请先用 skill_list 查看可用技能列表。", args.Name), nil
			}

			// 展开变量
			expanded := skill.Expand(args.Arguments, "")

			result := fmt.Sprintf("# %s\n\n**描述**: %s\n**来源**: %s\n\n---\n\n%s",
				skill.FullName(), skill.Description, skill.Source, expanded)
			return result, nil
		}
}

func skillReloadTool() (*schema.ToolInfo, toolHandler) {
	return &schema.ToolInfo{
			Name:        "skill_reload",
			Desc:        "重新加载所有技能(SKILL)，从所有技能目录重新扫描。管理员编辑或新增技能后调用此工具使其生效。",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
		}, func(ctx context.Context, argsJSON string) (string, error) {
			backend := GetSkillBackend()
			if err := backend.Reload(ctx); err != nil {
				return "", fmt.Errorf("重新加载技能失败: %w", err)
			}
			list, _ := backend.List(ctx)
			return fmt.Sprintf("技能已重新加载，当前有 %d 个可用技能。", len(list)), nil
		}
}

func skillSaveTool() (*schema.ToolInfo, toolHandler) {
	return &schema.ToolInfo{
			Name: "skill_save",
			Desc: "创建或更新技能(SKILL)。技能是存储在 data/skills/ 下的结构化知识文档，包含 frontmatter 元数据和 Markdown 正文。如果同名技能已存在则更新，不存在则创建。创建后自动生效。",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"name": {
					Type: schema.String,
					Desc: "技能名称，应使用英文小写连字符格式，如 seo-analyzer",
				},
				"description": {
					Type: schema.String,
					Desc: "技能描述，一句话说明技能的用途",
				},
				"category": {
					Type: schema.String,
					Desc: "技能分类，如 SEO、写作、运维等",
				},
				"content": {
					Type: schema.String,
					Desc: "技能内容（Markdown 格式）。可在内容中使用 $ARGUMENTS 引用传入的参数",
				},
				"tags": {
					Type: schema.String,
					Desc: "标签（英文逗号分隔），如 seo, analysis, keyword",
				},
				"version": {
					Type: schema.String,
					Desc: "版本号，如 1.0",
				},
				"author": {
					Type: schema.String,
					Desc: "作者",
				},
			}),
		}, func(ctx context.Context, argsJSON string) (string, error) {
			var args struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Category    string `json:"category"`
				Content     string `json:"content"`
				Tags        string `json:"tags"`
				Version     string `json:"version"`
				Author      string `json:"author"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				return "", fmt.Errorf("无法解析参数: %w", err)
			}
			if args.Name == "" || args.Content == "" {
				return "错误：名称和内容不能为空", nil
			}
			var tags []string
			if args.Tags != "" {
				for _, t := range strings.Split(args.Tags, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						tags = append(tags, t)
					}
				}
			}
			skill := &Skill{
				SkillFrontMatter: SkillFrontMatter{
					Name:        args.Name,
					Description: args.Description,
					Category:    args.Category,
					Version:     args.Version,
					Author:      args.Author,
					Tags:        tags,
				},
				Content: args.Content,
			}
			backend := GetSkillBackend()
			if err := backend.Save(ctx, skill); err != nil {
				return "", fmt.Errorf("保存技能失败: %w", err)
			}
			return fmt.Sprintf("技能 %q 已保存，路径: data/skills/%s/SKILL.md", args.Name, args.Name), nil
		}
}

// ================================================================
//  系统提示构建
// ================================================================

// BuildSkillSystemPrompt 返回技能系统的使用指导
func BuildSkillSystemPrompt() string {
	return `## 技能系统(Skills)
技能是预设的专业工作流程模板，可在执行专业任务时加载指导。
- skill_list: 列出所有可用技能
- skill_get: 加载指定技能的完整内容，支持 $ARGUMENTS 变量替换
- skill_reload: 管理员编辑技能后重新加载

使用 skill_get 加载技能后，技能内容可能引用本系统的其他工具，按需调用即可。`
}

// BuildSkillToolDescription 返回技能工具的 MCP 描述
func BuildSkillToolDescription() string {
	listTool, _ := skillListTool()
	getTool, _ := skillGetTool()
	return fmt.Sprintf("- %s: %s\n- %s: %s", listTool.Name, listTool.Desc, getTool.Name, getTool.Desc)
}
