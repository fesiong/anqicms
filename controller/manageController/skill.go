package manageController

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

// SkillListResponse is the response for skill list
type SkillListResponse struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Version     string   `json:"version"`
	Tags        []string `json:"tags"`
	UpdatedAt   string   `json:"updated_at"`
	FileCount   int      `json:"file_count"`
}

// SkillDetailResponse is the response for skill detail
type SkillDetailResponse struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Tags        []string `json:"tags"`
	Content     string   `json:"content"`
	BaseDir     string   `json:"base_dir"`
	UpdatedAt   string   `json:"updated_at"`
}

// SkillList returns all available skills
func SkillList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	backend := provider.GetSkillBackend()
	if currentSite != nil {
		backend = provider.GetSkillBackendForSite(currentSite.RootPath)
	}
	list, err := backend.List(context.Background())
	if err != nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  err.Error(),
		})
		return
	}

	result := make([]SkillListResponse, 0, len(list))
	for _, s := range list {
		r := SkillListResponse{
			Name:        s.Name,
			Description: s.Description,
			Category:    s.Category,
			Version:     s.Version,
			Tags:        s.Tags,
		}
		// Get file count from directory
		if s.Name != "" {
			skillsDir := filepath.Join(strings.TrimSuffix(config.ExecPath, "/"), "data", "skills", s.Name)
			if entries, err := os.ReadDir(skillsDir); err == nil {
				r.FileCount = len(entries)
			}
			// Get update time from SKILL.md
			mdPath := filepath.Join(skillsDir, "SKILL.md")
			if info, err := os.Stat(mdPath); err == nil {
				r.UpdatedAt = info.ModTime().Format("2006-01-02 15:04:05")
			}
		}
		result = append(result, r)
	}

	ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "success",
		"data": result,
	})
}

// SkillDetail returns the full content of a skill
func SkillDetail(ctx iris.Context) {
	name := ctx.URLParam("name")
	if name == "" {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  "name is required",
		})
		return
	}

	currentSite := provider.CurrentSubSite(ctx)
	backend := provider.GetSkillBackend()
	if currentSite != nil {
		backend = provider.GetSkillBackendForSite(currentSite.RootPath)
	}
	skill, err := backend.Get(context.Background(), name)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  err.Error(),
		})
		return
	}

	result := SkillDetailResponse{
		Name:        skill.Name,
		Description: skill.Description,
		Category:    skill.Category,
		Version:     skill.Version,
		Author:      skill.Author,
		Tags:        skill.Tags,
		Content:     skill.Content,
		BaseDir:     skill.BaseDirectory,
	}
	if !skill.UpdatedAt.IsZero() {
		result.UpdatedAt = skill.UpdatedAt.Format("2006-01-02 15:04:05")
	}

	ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "success",
		"data": result,
	})
}

// SkillEditRequest is the request for editing a skill
type SkillEditRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Tags        []string `json:"tags"`
	Content     string   `json:"content"`
}

// SkillEdit creates or updates a skill
func SkillEdit(ctx iris.Context) {
	var req SkillEditRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  "invalid request",
		})
		return
	}

	if req.Name == "" {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  "name is required",
		})
		return
	}

	// Validate name (only allow safe characters)
	for _, c := range req.Name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			ctx.JSON(iris.Map{
				"code": -1,
				"msg":  "name只能包含字母、数字、中划线和下划线",
			})
			return
		}
	}

	// Build frontmatter
	var frontmatter strings.Builder
	frontmatter.WriteString("---\n")
	frontmatter.WriteString(fmt.Sprintf("name: %s\n", req.Name))
	if req.Description != "" {
		frontmatter.WriteString(fmt.Sprintf("description: %s\n", req.Description))
	}
	if req.Category != "" {
		frontmatter.WriteString(fmt.Sprintf("category: %s\n", req.Category))
	}
	if req.Version != "" {
		frontmatter.WriteString(fmt.Sprintf("version: %s\n", req.Version))
	}
	if req.Author != "" {
		frontmatter.WriteString(fmt.Sprintf("author: %s\n", req.Author))
	}
	if len(req.Tags) > 0 {
		frontmatter.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(req.Tags, ", ")))
	}
	frontmatter.WriteString("---\n\n")

	content := frontmatter.String() + req.Content

	// Write SKILL.md
	skillDir := filepath.Join(strings.TrimSuffix(config.ExecPath, "/"), "data", "skills", req.Name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  fmt.Sprintf("创建目录失败: %v", err),
		})
		return
	}

	mdPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(mdPath, []byte(content), 0644); err != nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  fmt.Sprintf("写入文件失败: %v", err),
		})
		return
	}

	// Reload backend
	backend := provider.GetSkillBackend()
	if err := backend.Reload(context.Background()); err != nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  fmt.Sprintf("重载技能失败: %v", err),
		})
		return
	}

	provider.GetSkillBackend() // ensure init
	ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "保存成功",
	})
}

// SkillDelete deletes a skill directory
func SkillDelete(ctx iris.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  "invalid request",
		})
		return
	}

	if req.Name == "" {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  "name is required",
		})
		return
	}

	// Validate name (only allow safe characters)
	for _, c := range req.Name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			ctx.JSON(iris.Map{
				"code": -1,
				"msg":  "name只能包含字母、数字、中划线和下划线",
			})
			return
		}
	}

	skillDir := filepath.Join(strings.TrimSuffix(config.ExecPath, "/"), "data", "skills", req.Name)
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  "技能不存在",
		})
		return
	}

	// Remove all files in the skill directory
	if err := os.RemoveAll(skillDir); err != nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  fmt.Sprintf("删除失败: %v", err),
		})
		return
	}

	// Reload backend
	backend := provider.GetSkillBackend()
	if err := backend.Reload(context.Background()); err != nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  fmt.Sprintf("重载技能失败: %v", err),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "删除成功",
	})
}

// SkillReload reloads all skills from disk
func SkillReload(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	backend := provider.GetSkillBackend()
	if currentSite != nil {
		backend = provider.GetSkillBackendForSite(currentSite.RootPath)
	}
	if err := backend.Reload(context.Background()); err != nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  fmt.Sprintf("重载失败: %v", err),
		})
		return
	}

	list, _ := backend.List(context.Background())
	ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "成功",
		"data": iris.Map{
			"count": len(list),
			"time":  time.Now().Format("2006-01-02 15:04:05"),
		},
	})
}
