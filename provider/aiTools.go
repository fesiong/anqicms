package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/pkg/ai/eino"
	"kandaoni.com/anqicms/provider/fulltext"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

type ArgId struct {
	Id int64 `json:"id"`
}

// toolHandler executes a tool given its JSON arguments and returns a text result.
type toolHandler func(ctx context.Context, argsJSON string) (string, error)

// getEinoTools returns tool definitions (schema.ToolInfo) and a name→handler map.
// The handlers use the site stored in the service.
func (svc *AiChatService) getEinoTools() ([]*schema.ToolInfo, map[string]toolHandler) {
	tools := make([]*schema.ToolInfo, 0)
	handlers := make(map[string]toolHandler)

	add := func(ti *schema.ToolInfo, fn toolHandler) {
		tools = append(tools, ti)
		handlers[ti.Name] = fn
	}

	// ---- Archive tools ----
	add(&schema.ToolInfo{
		Name: "archive_list",
		Desc: "分页获取文档列表，支持按分类、状态、关键词筛选和排序。返回文档列表和总数。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"page":        {Type: schema.Integer, Desc: "页码，从1开始，默认1"},
			"page_size":   {Type: schema.Integer, Desc: "每页数量，最大100，默认10"},
			"category_id": {Type: schema.Integer, Desc: "分类ID，筛选指定分类的文档"},
			"module_id":   {Type: schema.Integer, Desc: "模型ID，筛选指定模型的文档"},
			"parent_id":   {Type: schema.Integer, Desc: "父文档ID，筛选指定父文档的下级文档"},
			"status":      {Type: schema.String, Desc: "状态筛选：draft=草稿，ok=已发布，plan=定时发布，默认为ok"},
			"keyword":     {Type: schema.String, Desc: "关键词搜索，匹配标题和内容"},
			"flag":        {Type: schema.String, Desc: "文档属性筛选，h=头条，c=推荐，f=幻灯，a=特荐，s=滚动，b=加粗，p=图片，j=跳转"},
			"order_by":    {Type: schema.String, Desc: "排序字段：created_time, updated_time, views"},
			"order_dir":   {Type: schema.String, Desc: "排序方向：asc 升序, desc 降序"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Page       int    `json:"page"`
			PageSize   int    `json:"page_size"`
			CategoryID uint   `json:"category_id"`
			ModuleID   uint   `json:"module_id"`
			ParentID   uint   `json:"parent_id"`
			Flag       string `json:"flag"`
			Status     string `json:"status"`
			Keyword    string `json:"keyword"`
			OrderBy    string `json:"order_by"`
			OrderDir   string `json:"order_dir"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Page <= 0 {
			args.Page = 1
		}
		if args.PageSize <= 0 || args.PageSize > 100 {
			args.PageSize = 10
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		order := "created_time DESC"
		if args.OrderBy != "" {
			dir := "DESC"
			if args.OrderDir == "asc" {
				dir = "ASC"
			}
			order = args.OrderBy + " " + dir
		}
		isDraft := 0
		if args.Status == "draft" || args.Status == "plan" {
			isDraft = 1
		}
		offset := (args.Page - 1) * args.PageSize
		var fulltextSearch bool
		var fulltextTotal int64
		archives, total, err := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
			if args.CategoryID > 0 {
				tx = tx.Where("category_id = ?", args.CategoryID)
			}
			if args.ModuleID > 0 {
				tx = tx.Where("module_id = ?", args.ModuleID)
			}
			if args.ParentID > 0 {
				tx = tx.Where("parent_id = ?", args.ParentID)
			}
			if args.Status != "" {
				if args.Status == "draft" {
					tx = tx.Where("status = ?", config.ContentStatusDraft)
				} else if args.Status == "plan" {
					tx = tx.Where("status = ?", config.ContentStatusPlan)
				}
			}
			if args.Keyword != "" {
				var ids []int64
				// 如果开启了全文索引，则尝试使用全文索引搜索，status = "ok" 时有效
				if isDraft == 0 {
					var tmpDocs []fulltext.TinyArchive
					var err2 error
					tmpDocs, fulltextTotal, err2 = svc.site.Search(args.Keyword, args.ModuleID, args.Page, args.PageSize)
					if err2 == nil {
						fulltextSearch = true
						// 只保留文档
						for _, doc := range tmpDocs {
							if doc.Type == fulltext.ArchiveType {
								ids = append(ids, doc.Id)
							}
						}
						if len(tmpDocs) == 0 || len(ids) == 0 {
							ids = append(ids, 0)
						}
						offset = 0
					}
				}
				if fulltextSearch == true {
					// 使用了全文索引，拿到了ID
					tx = tx.Where("archives.`id` IN(?)", ids)
				} else {
					// 如果文章数量达到10万，则只能匹配开头，否则就模糊搜索
					var allArchives int64
					if args.Status == "ok" {
						allArchives = svc.site.GetExplainCount("SELECT id FROM archives")
					} else {
						allArchives = svc.site.GetExplainCount("SELECT id FROM archive_drafts")
					}
					if allArchives > 100000 {
						tx = tx.Where("`title` like ?", args.Keyword+"%")
					} else {
						tx = tx.Where("`title` like ?", "%"+args.Keyword+"%")
					}
				}
			}
			if args.Flag != "" {
				tx = tx.Joins("INNER JOIN archive_flags ON archives.id = archive_flags.archive_id and archive_flags.flag = ?", args.Flag)
			}
			return tx
		}, order, args.Page, args.PageSize, offset, isDraft)
		if err != nil {
			return "", fmt.Errorf("查询文档列表失败: %w", err)
		}
		if fulltextSearch {
			total = fulltextTotal
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共 %d 篇文档，当前第 %d 页：\n\n", total, args.Page))
		for _, a := range archives {
			b.WriteString(fmt.Sprintf("- [%d] %s (分类ID: %d)\n", a.Id, a.Title, a.CategoryId))
		}
		return b.String(), nil
	})

	add(&schema.ToolInfo{
		Name: "archive_get",
		Desc: "获取单篇文档的完整详情，包括标题、内容、关键词、描述等。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "文档ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		archive, err := w.GetArchiveById(args.Id)
		if err != nil {
			archiveDraft, err := w.GetArchiveDraftById(args.Id)
			if err != nil {
				return "", fmt.Errorf("获取文档失败: %w", err)
			}
			archive = &archiveDraft.Archive
		}
		data, _ := w.GetArchiveDataById(args.Id)
		content := ""
		if data != nil {
			content = data.Content
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("标题: %s\n", archive.Title))
		b.WriteString(fmt.Sprintf("ID: %d\n", archive.Id))
		b.WriteString(fmt.Sprintf("模型ID: %d\n", archive.ModuleId))
		b.WriteString(fmt.Sprintf("分类ID: %d\n", archive.CategoryId))
		b.WriteString(fmt.Sprintf("关键词: %s\n", archive.Keywords))
		b.WriteString(fmt.Sprintf("描述: %s\n", archive.Description))
		b.WriteString(fmt.Sprintf("创建时间: %s\n", time.Unix(archive.CreatedTime, 0).Format("2006-01-02 15:04:05")))
		b.WriteString(fmt.Sprintf("内容: %s\n", content))
		return b.String(), nil
	})

	add(&schema.ToolInfo{
		Name: "archive_create",
		Desc: "创建新文档。必填字段：title（标题）、content（内容）、category_id（分类ID）。创建成功返回文档ID。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"title":        {Type: schema.String, Desc: "文档标题", Required: true},
			"content":      {Type: schema.String, Desc: "文档内容，支持Markdown格式", Required: true},
			"category_id":  {Type: schema.Integer, Desc: "分类ID", Required: true},
			"keywords":     {Type: schema.String, Desc: "关键词，多个用逗号分隔"},
			"description":  {Type: schema.String, Desc: "文档摘要/描述"},
			"logo":         {Type: schema.String, Desc: "封面图片URL"},
			"draft":        {Type: schema.Boolean, Desc: "状态：true=草稿，false=发布"},
			"created_time": {Type: schema.Integer, Desc: "创建时间，Unix时间戳，默认当前时间，大于当前时间时将使用指定时间创建文档"},
			"tags":         {Type: schema.Array, Desc: "标签列表，JSON数组格式如 [\"tag1\",\"tag2\"]"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Title       string   `json:"title"`
			Content     string   `json:"content"`
			CategoryID  uint     `json:"category_id"`
			Keywords    string   `json:"keywords"`
			Description string   `json:"description"`
			Logo        string   `json:"logo"`
			Draft       bool     `json:"draft"`
			CreatedTime int64    `json:"created_time"`
			Tags        []string `json:"tags"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Title == "" {
			return "错误：文档标题不能为空", nil
		}
		if args.Content == "" {
			return "错误：文档内容不能为空", nil
		}
		if args.CategoryID == 0 {
			return "错误：请指定分类ID", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		req := &request.Archive{
			Title:       args.Title,
			Content:     args.Content,
			CategoryId:  args.CategoryID,
			Keywords:    args.Keywords,
			Description: args.Description,
			CreatedTime: args.CreatedTime,
			Tags:        args.Tags,
		}
		if args.Logo != "" {
			req.Images = []string{args.Logo}
		}
		archive, err := w.SaveArchive(req)
		if err != nil {
			return "", fmt.Errorf("创建文档失败: %w", err)
		}
		return fmt.Sprintf("文档创建成功！ID: %d，标题: %s", archive.Id, archive.Title), nil
	})

	add(&schema.ToolInfo{
		Name: "archive_delete",
		Desc: "删除文档（软删除）。需要传入文档ID。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "要删除的文档ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		archive, err := w.GetArchiveById(args.Id)
		if err != nil {
			return "", fmt.Errorf("获取文档失败: %w", err)
		}
		if err := w.DeleteArchive(archive); err != nil {
			return "", fmt.Errorf("删除文档失败: %w", err)
		}
		return fmt.Sprintf("文档 [%d] %s 已成功删除", archive.Id, archive.Title), nil
	})

	add(&schema.ToolInfo{
		Name: "archive_publish",
		Desc: "发布或下架文档。传入archive_id和status（1=发布，0=下架）。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":     {Type: schema.Integer, Desc: "文档ID", Required: true},
			"status": {Type: schema.Integer, Desc: "状态：1=发布，0=下架", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			ID     int64 `json:"id"`
			Status uint  `json:"status"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		archive, err := w.GetArchiveById(args.ID)
		if err != nil {
			return "", fmt.Errorf("获取文档失败: %w", err)
		}
		updateReq := &request.ArchivesUpdateRequest{
			Ids:    []int64{args.ID},
			Status: args.Status,
		}
		if err := w.UpdateArchiveStatus(updateReq); err != nil {
			return "", fmt.Errorf("更新文档状态失败: %w", err)
		}
		statusStr := "已发布"
		if args.Status == 0 {
			statusStr = "已下架"
		}
		return fmt.Sprintf("文档 [%d] %s %s", archive.Id, archive.Title, statusStr), nil
	})

	add(&schema.ToolInfo{
		Name: "archive_update",
		Desc: "编辑已有文档。需要传入文档ID。仅传入的字段会被更新，未传入的字段保持不变。支持修改标题、内容、分类、关键词、描述、封面图、标签。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":           {Type: schema.Integer, Desc: "文档ID", Required: true},
			"title":        {Type: schema.String, Desc: "文档标题"},
			"content":      {Type: schema.String, Desc: "文档内容，支持Markdown格式"},
			"category_id":  {Type: schema.Integer, Desc: "分类ID"},
			"keywords":     {Type: schema.String, Desc: "关键词，多个用逗号分隔"},
			"description":  {Type: schema.String, Desc: "文档摘要/描述"},
			"logo":         {Type: schema.String, Desc: "封面图片URL"},
			"draft":        {Type: schema.Boolean, Desc: "状态：true=草稿，false=发布"},
			"created_time": {Type: schema.Integer, Desc: "创建时间，Unix时间戳，默认当前时间，大于当前时间时将使用指定时间创建文档"},
			"tags":         {Type: schema.Array, Desc: "标签列表，JSON数组格式如 [\"tag1\",\"tag2\"]"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			ID          int64    `json:"id"`
			Title       string   `json:"title"`
			Content     string   `json:"content"`
			CategoryID  uint     `json:"category_id"`
			Keywords    string   `json:"keywords"`
			Description string   `json:"description"`
			Logo        string   `json:"logo"`
			Draft       bool     `json:"draft"`
			CreatedTime int64    `json:"created_time"`
			Tags        []string `json:"tags"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.ID == 0 {
			return "错误：请传入文档ID", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		// 获取已有文档
		archive, err := w.GetArchiveById(args.ID)
		if err != nil {
			return "", fmt.Errorf("获取文档失败: %w", err)
		}
		// 只覆盖传入的字段
		req := &request.Archive{
			Id: args.ID,
		}
		if args.Title != "" {
			req.Title = args.Title
		} else {
			req.Title = archive.Title
		}
		if args.Content != "" {
			req.Content = args.Content
		}
		if args.CategoryID > 0 {
			req.CategoryId = args.CategoryID
		} else {
			req.CategoryId = archive.CategoryId
		}
		if args.Keywords != "" {
			req.Keywords = args.Keywords
		} else {
			req.Keywords = archive.Keywords
		}
		if args.Description != "" {
			req.Description = args.Description
		} else {
			req.Description = archive.Description
		}
		if args.Logo != "" {
			req.Images = []string{args.Logo}
		}
		if len(args.Tags) > 0 {
			req.Tags = args.Tags
		}
		// 使用QuickSave，避免其它字段覆盖
		req.UpdateAll = false
		archive, err = w.SaveArchive(req)
		if err != nil {
			return "", fmt.Errorf("更新文档失败: %w", err)
		}
		return fmt.Sprintf("文档 [%d] %s 更新成功！", archive.Id, archive.Title), nil
	})

	// ---- Module tools ----
	add(&schema.ToolInfo{
		Name:        "module_list",
		Desc:        "获取自定义模型列表，返回所有模型的ID、名称、表名和URL别名等信息。",
		// ParamsOneOf left nil intentionally: no parameters needed
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		modules, err := w.GetModules()
		if err != nil {
			return "", fmt.Errorf("获取模型列表失败: %w", err)
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共 %d 个自定义模型：\n\n", len(modules)))
		for _, m := range modules {
			b.WriteString(fmt.Sprintf("- [%d] %s (表: %s, URL别名: %s)", m.Id, m.Name, m.TableName, m.UrlToken))
			if m.IsSystem == 1 {
				b.WriteString(" [系统]")
			}
			b.WriteString("\n")
		}
		return b.String(), nil
	})

	add(&schema.ToolInfo{
		Name: "module_get",
		Desc: "获取单个自定义模型的详细信息，包括模型字段配置。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "模型ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		mod, err := w.GetModuleById(uint(args.Id))
		if err != nil {
			return "", fmt.Errorf("获取模型失败: %w", err)
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("模型信息：\nID: %d\n名称: %s\n表名: %s\nURL别名: %s\n", mod.Id, mod.Title, mod.TableName, mod.UrlToken))
		b.WriteString(fmt.Sprintf("关键词: %s\n描述: %s\n", mod.Keywords, mod.Description))
		b.WriteString(fmt.Sprintf("标题字段名: %s\n", mod.Title))
		sys := "否"
		if mod.IsSystem == 1 {
			sys = "是"
		}
		b.WriteString(fmt.Sprintf("系统模型: %s\n", sys))
		statusStr := "启用"
		if mod.Status != 1 {
			statusStr = "禁用"
		}
		b.WriteString(fmt.Sprintf("状态: %s\n", statusStr))
		if len(mod.Fields) > 0 {
			b.WriteString(fmt.Sprintf("自定义字段: %d 个\n", len(mod.Fields)))
			for _, f := range mod.Fields {
				b.WriteString(fmt.Sprintf("  - %s (%s, %s),字段内容、默认值: %s\n", f.FieldName, f.Type, f.Name, strings.ReplaceAll(f.Content, "\n", ",")))
			}
		} else {
			b.WriteString("无自定义字段。\n")
		}
		return b.String(), nil
	})

	add(&schema.ToolInfo{
		Name: "module_create",
		Desc: "创建新自定义模型。必填字段：title（模型名称）、table_name（表名）。创建成功返回模型ID。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"title":       {Type: schema.String, Desc: "模型名称", Required: true},
			"table_name":  {Type: schema.String, Desc: "数据库表名（英文小写）", Required: true},
			"url_token":   {Type: schema.String, Desc: "URL别名"},
			"name":        {Type: schema.String, Desc: "模型标识"},
			"keywords":    {Type: schema.String, Desc: "关键词"},
			"description": {Type: schema.String, Desc: "描述"},
			"status":      {Type: schema.Integer, Desc: "状态：1=启用，-1=禁用，默认1"},
			"fields": {
				Type: schema.Array,
				Desc: "自定义字段",
				ElemInfo: &schema.ParameterInfo{
					Type: schema.Object,
					Desc: "字段信息",
					SubParams: map[string]*schema.ParameterInfo{
						"field_name": {
							Type:     schema.String,
							Desc:     "字段名",
							Required: true,
						},
						"name": {
							Type:     schema.String,
							Desc:     "字段描述",
							Required: true,
						},
						"type": {
							Type:     schema.String,
							Desc:     "字段类型，text|textarea|select|checkbox|radio|number|texts|editor|image|images|file",
							Required: true,
						},
						"content": {
							Type: schema.String,
							Desc: "字段内容、默认值，type为checkbox、radio、select时为选项列表，多个选项用英文逗号分隔",
						},
					},
				},
			},
			"category_fields": {
				Type: schema.Array,
				Desc: "自定义字段",
				ElemInfo: &schema.ParameterInfo{
					Type: schema.Object,
					Desc: "字段信息",
					SubParams: map[string]*schema.ParameterInfo{
						"field_name": {
							Type:     schema.String,
							Desc:     "字段名",
							Required: true,
						},
						"name": {
							Type:     schema.String,
							Desc:     "字段描述",
							Required: true,
						},
						"type": {
							Type:     schema.String,
							Desc:     "字段类型，text|textarea|select|checkbox|radio|number|texts|editor|image|images|file",
							Required: true,
						},
						"content": {
							Type: schema.String,
							Desc: "字段内容、默认值，type为checkbox、radio、select时为选项列表，多个选项用英文逗号分隔",
						},
					},
				},
			},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Title          string               `json:"title"`
			TableName      string               `json:"table_name"`
			UrlToken       string               `json:"url_token"`
			Name           string               `json:"name"`
			Keywords       string               `json:"keywords"`
			Description    string               `json:"description"`
			Status         int                  `json:"status"`
			Fields         []config.CustomField `json:"fields"`
			CategoryFields []config.CustomField `json:"category_fields"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Title == "" {
			return "错误：模型名称不能为空", nil
		}
		if args.TableName == "" {
			return "错误：表名不能为空", nil
		}
		args.TableName = strings.ToLower(args.TableName)
		if args.UrlToken == "" {
			args.UrlToken = args.TableName
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		if args.Status == -1 {
			args.Status = 0
		} else {
			args.Status = 1
		}
		if args.Fields != nil {
			for i := range args.Fields {
				args.Fields[i].FieldName = strings.ToLower(args.Fields[i].FieldName)
				args.Fields[i].Content = strings.ReplaceAll(args.Fields[i].Content, ",", "\n")
			}
		}
		req := &request.ModuleRequest{
			Title:          args.Title,
			TableName:      args.TableName,
			UrlToken:       args.UrlToken,
			Name:           args.Name,
			Keywords:       args.Keywords,
			Description:    args.Description,
			Status:         uint(args.Status),
			Fields:         args.Fields,
			CategoryFields: args.CategoryFields,
		}
		mod, err := w.SaveModule(req)
		if err != nil {
			return "", fmt.Errorf("创建模型失败: %w", err)
		}
		return fmt.Sprintf("模型创建成功！ID: %d, 名称: %s, 表名: %s", mod.Id, mod.Title, mod.TableName), nil
	})

	add(&schema.ToolInfo{
		Name: "module_update",
		Desc: "修改模型。必填字段：模型ID。数据库表名不可修改。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":          {Type: schema.Integer, Desc: "模型ID", Required: true},
			"title":       {Type: schema.String, Desc: "模型名称"},
			"table_name":  {Type: schema.String, Desc: "数据库表名（英文小写）"},
			"url_token":   {Type: schema.String, Desc: "URL别名"},
			"name":        {Type: schema.String, Desc: "模型标识"},
			"keywords":    {Type: schema.String, Desc: "关键词"},
			"description": {Type: schema.String, Desc: "描述"},
			"status":      {Type: schema.Integer, Desc: "状态：1=启用，-1=禁用，默认1"},
			"fields": {
				Type: schema.Array,
				Desc: "自定义字段",
				ElemInfo: &schema.ParameterInfo{
					Type: schema.Object,
					Desc: "字段信息",
					SubParams: map[string]*schema.ParameterInfo{
						"field_name": {
							Type:     schema.String,
							Desc:     "字段名",
							Required: true,
						},
						"name": {
							Type:     schema.String,
							Desc:     "字段描述",
							Required: true,
						},
						"type": {
							Type:     schema.String,
							Desc:     "字段类型，text|textarea|select|checkbox|radio|number|texts|editor|image|images|file",
							Required: true,
						},
						"content": {
							Type: schema.String,
							Desc: "字段内容、默认值，type为checkbox、radio、select时为选项列表，多个选项用英文逗号分隔",
						},
					},
				},
			},
			"category_fields": {
				Type: schema.Array,
				Desc: "自定义字段",
				ElemInfo: &schema.ParameterInfo{
					Type: schema.Object,
					Desc: "字段信息",
					SubParams: map[string]*schema.ParameterInfo{
						"field_name": {
							Type:     schema.String,
							Desc:     "字段名",
							Required: true,
						},
						"name": {
							Type:     schema.String,
							Desc:     "字段描述",
							Required: true,
						},
						"type": {
							Type:     schema.String,
							Desc:     "字段类型，text|textarea|select|checkbox|radio|number|texts|editor|image|images|file",
							Required: true,
						},
						"content": {
							Type: schema.String,
							Desc: "字段内容、默认值，type为checkbox、radio、select时为选项列表，多个选项用英文逗号分隔",
						},
					},
				},
			},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			ID             uint                 `json:"id"`
			Title          string               `json:"title"`
			TableName      string               `json:"table_name"`
			UrlToken       string               `json:"url_token"`
			Name           string               `json:"name"`
			Keywords       string               `json:"keywords"`
			Description    string               `json:"description"`
			Status         int                  `json:"status"`
			Fields         []config.CustomField `json:"fields"`
			CategoryFields []config.CustomField `json:"category_fields"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.ID == 0 {
			return "错误：模型ID不存在", nil
		}
		if args.TableName == "" {
			return "错误：表名不能为空", nil
		}
		args.TableName = strings.ToLower(args.TableName)
		if args.UrlToken == "" {
			args.UrlToken = args.TableName
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		exist, err := w.GetModuleById(args.ID)
		if err != nil {
			return "错误：模型不存在", nil
		}
		if args.Status == -1 {
			args.Status = 0
		} else {
			args.Status = 1
		}
		if args.Fields != nil {
			for i := range args.Fields {
				args.Fields[i].FieldName = strings.ToLower(args.Fields[i].FieldName)
				args.Fields[i].Content = strings.ReplaceAll(args.Fields[i].Content, ",", "\n")
			}
		}

		req := &request.ModuleRequest{
			Id:             args.ID,
			Title:          args.Title,
			TableName:      args.TableName,
			UrlToken:       args.UrlToken,
			Name:           args.Name,
			Keywords:       args.Keywords,
			Description:    args.Description,
			Status:         uint(args.Status),
			Fields:         args.Fields,
			CategoryFields: args.CategoryFields,
			UpdateAll:      false,
		}
		if req.Title == "" {
			req.Title = exist.Title
		}
		if req.TableName == "" {
			req.TableName = exist.TableName
		}
		if req.UrlToken == "" {
			req.UrlToken = exist.UrlToken
		}
		if req.Name == "" {
			req.Name = exist.Name
		}
		if req.Keywords == "" {
			req.Keywords = exist.Keywords
		}
		if req.Description == "" {
			req.Description = exist.Description
		}
		if req.Fields == nil {
			req.Fields = exist.Fields
		}
		if req.CategoryFields == nil {
			req.CategoryFields = exist.CategoryFields
		}
		mod, err := w.SaveModule(req)
		if err != nil {
			return "", fmt.Errorf("修改模型失败: %w", err)
		}
		return fmt.Sprintf("模型修改成功！ID: %d, 名称: %s, 表名: %s", mod.Id, mod.Title, mod.TableName), nil
	})

	add(&schema.ToolInfo{
		Name: "module_delete",
		Desc: "删除自定义模型。需要传入模型ID。注意：删除模型会同时删除该模型的所有数据。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "要删除的模型ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		mod, err := w.GetModuleById(uint(args.Id))
		if err != nil {
			return "", fmt.Errorf("获取模型失败: %w", err)
		}
		if mod.IsSystem == 1 {
			return "错误：系统模型不允许删除", nil
		}
		if err := w.DeleteModule(mod); err != nil {
			return "", fmt.Errorf("删除模型失败: %w", err)
		}
		return fmt.Sprintf("模型 [%d] %s 已成功删除", mod.Id, mod.Title), nil
	})

	// ---- Category tools ----
	add(&schema.ToolInfo{
		Name: "category_list",
		Desc: "获取分类列表，返回所有分类的ID、标题和层级结构。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "要获取分类列表的模型ID", Required: false},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		categories, err := w.GetCategories(func(tx *gorm.DB) *gorm.DB {
			if args.Id > 0 {
				tx = tx.Where("module_id = ?", args.Id)
			}
			return tx.Where("type = ?", config.CategoryTypeArchive).Order("module_id asc,sort asc")
		}, 0, 1)
		if err != nil {
			return "", fmt.Errorf("获取分类列表失败: %w", err)
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共 %d 个分类：\n\n", len(categories)))
		for _, c := range categories {
			b.WriteString(fmt.Sprintf("- [%d] %s", c.Id, c.Title))
			if c.ParentId > 0 {
				b.WriteString(fmt.Sprintf(" (父分类ID: %d)", c.ParentId))
			}
			b.WriteString("\n")
		}
		return b.String(), nil
	})

	add(&schema.ToolInfo{
		Name: "category_get",
		Desc: "获取单个分类的详细信息。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "分类ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		cat, err := w.GetCategoryById(uint(args.Id))
		if err != nil {
			return "", fmt.Errorf("获取分类失败: %w", err)
		}
		return fmt.Sprintf("分类信息：\nID: %d\n标题: %s\n父分类ID: %d\n描述: %s\nURL别名: %s",
			cat.Id, cat.Title, cat.ParentId, cat.Description, cat.UrlToken), nil
	})

	add(&schema.ToolInfo{
		Name: "category_create",
		Desc: "创建新分类。必填字段：title（分类名称）、module_id（模型ID）。创建成功返回分类ID。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"title":       {Type: schema.String, Desc: "分类名称", Required: true},
			"parent_id":   {Type: schema.Integer, Desc: "父分类ID，默认为0（顶级分类）"},
			"module_id":   {Type: schema.Integer, Desc: "所属模型ID，默认为1（文档分类）"},
			"description": {Type: schema.String, Desc: "分类描述"},
			"keywords":    {Type: schema.String, Desc: "分类关键词"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Title       string `json:"title"`
			ParentID    uint   `json:"parent_id"`
			Description string `json:"description"`
			Keywords    string `json:"keywords"`
			ModuleID    uint   `json:"module_id"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Title == "" {
			return "错误：分类名称不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		req := &request.Category{
			Title:       args.Title,
			ParentId:    args.ParentID,
			Description: args.Description,
			Keywords:    args.Keywords,
			ModuleId:    args.ModuleID,
			Type:        config.CategoryTypeArchive,
			Status:      1,
		}
		// 检查是否重复了
		exist, err := w.GetCategoryByTitle(req.Title)
		if err == nil && exist.Id != 0 {
			return fmt.Sprintf("错误：分类名称重复, ID: %d", exist.Id), nil
		}
		cat, err := w.SaveCategory(req)
		if err != nil {
			return "", fmt.Errorf("创建分类失败: %w", err)
		}
		return fmt.Sprintf("分类创建成功！ID: %d, 标题: %s", cat.Id, cat.Title), nil
	})

	add(&schema.ToolInfo{
		Name: "category_delete",
		Desc: "删除分类。需要传入分类ID。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "要删除的分类ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		cat, err := w.GetCategoryById(uint(args.Id))
		if err != nil {
			return "", fmt.Errorf("获取分类失败: %w", err)
		}
		err = w.DB.Unscoped().Delete(cat).Error
		if err != nil {
			return "", fmt.Errorf("删除分类失败: %w", err)
		}
		w.DeleteCacheCategories()
		return fmt.Sprintf("分类 [%d] %s 已成功删除", cat.Id, cat.Title), nil
	})

	// ---- Page Tools ----
	add(&schema.ToolInfo{
		Name:        "page_list",
		Desc:        "获取页面列表，返回所有页面的ID和标题。",
		// ParamsOneOf left nil intentionally: no parameters needed
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		categories, err := w.GetCategories(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("type = ?", config.CategoryTypePage).Order("sort asc")
		}, 0, 1)
		if err != nil {
			return "", fmt.Errorf("获取页面列表失败: %w", err)
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共 %d 个页面：\n\n", len(categories)))
		for _, c := range categories {
			b.WriteString(fmt.Sprintf("- [%d] %s", c.Id, c.Title))
			b.WriteString("\n")
		}
		return b.String(), nil
	})

	add(&schema.ToolInfo{
		Name: "page_get",
		Desc: "获取单个页面的详细信息。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "页面ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		cat, err := w.GetCategoryById(uint(args.Id))
		if err != nil {
			return "", fmt.Errorf("获取页面失败: %w", err)
		}
		return fmt.Sprintf("页面信息：\nID: %d\n标题: %s\n描述: %s\nURL别名: %s",
			cat.Id, cat.Title, cat.Description, cat.UrlToken), nil
	})

	add(&schema.ToolInfo{
		Name: "page_create",
		Desc: "创建新页面。必填字段：title（页面名称）。创建成功返回页面ID。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"title":       {Type: schema.String, Desc: "页面名称", Required: true},
			"description": {Type: schema.String, Desc: "页面描述"},
			"keywords":    {Type: schema.String, Desc: "页面关键词"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Keywords    string `json:"keywords"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Title == "" {
			return "错误：页面名称不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		req := &request.Category{
			Title:       args.Title,
			Description: args.Description,
			Keywords:    args.Keywords,
			Type:        config.CategoryTypePage,
			Status:      1,
		}
		// 检查是否重复了
		exist, err := w.GetCategoryByTitle(req.Title)
		if err == nil && exist.Id != 0 {
			return fmt.Sprintf("错误：页面名称重复, ID: %d", exist.Id), nil
		}
		cat, err := w.SaveCategory(req)
		if err != nil {
			return "", fmt.Errorf("创建页面失败: %w", err)
		}
		return fmt.Sprintf("页面创建成功！ID: %d, 标题: %s", cat.Id, cat.Title), nil
	})

	add(&schema.ToolInfo{
		Name: "page_delete",
		Desc: "删除页面。需要传入页面ID。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "要删除的页面ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		cat, err := w.GetCategoryById(uint(args.Id))
		if err != nil {
			return "", fmt.Errorf("获取页面失败: %w", err)
		}
		err = w.DB.Unscoped().Delete(cat).Error
		if err != nil {
			return "", fmt.Errorf("删除页面失败: %w", err)
		}
		w.DeleteCacheCategories()
		return fmt.Sprintf("页面 [%d] %s 已成功删除", cat.Id, cat.Title), nil
	})
	add(&schema.ToolInfo{
		Name: "page_update",
		Desc: "更新单页面内容。可以修改标题、描述、关键词、页面内容等。需要传入页面ID。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":          {Type: schema.Integer, Desc: "页面ID", Required: true},
			"title":       {Type: schema.String, Desc: "页面标题"},
			"url_token":   {Type: schema.String, Desc: "URL别名"},
			"description": {Type: schema.String, Desc: "页面描述"},
			"keywords":    {Type: schema.String, Desc: "页面关键词"},
			"content":     {Type: schema.String, Desc: "页面内容，支持HTML格式"},
			"sort":        {Type: schema.Integer, Desc: "排序值，越小越靠前"},
			"status":      {Type: schema.Integer, Desc: "状态：1=启用,0=禁用"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		var args request.Category
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Id == 0 {
			return "错误：页面ID不能为空", nil
		}
		cat, err := w.GetCategoryById(args.Id)
		if err != nil {
			return "", fmt.Errorf("获取页面失败: %w", err)
		}
		if args.Title != "" {
			cat.Title = args.Title
		}
		if args.UrlToken != "" {
			cat.UrlToken = args.UrlToken
		}
		if args.Description != "" {
			cat.Description = args.Description
		}
		if args.Keywords != "" {
			cat.Keywords = args.Keywords
		}
		if args.Content != "" {
			cat.Content = args.Content
		}
		if args.Sort > 0 {
			cat.Sort = args.Sort
		}
		if args.Status > 0 {
			cat.Status = args.Status
		}
		args.UpdateAll = false
		args.Type = config.CategoryTypePage
		category, err := w.SaveCategory(&args)
		if err != nil {
			return "", fmt.Errorf("更新页面失败: %w", err)
		}
		w.DeleteCacheCategories()
		return fmt.Sprintf("页面 [%d] %s 已更新", category.Id, category.Title), nil
	})

	// ---- Tag tools ----
	add(&schema.ToolInfo{
		Name: "tag_list",
		Desc: "获取标签列表，返回所有标签的ID和名称。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"page":        {Type: schema.Integer, Desc: "页码，从1开始，默认1"},
			"page_size":   {Type: schema.Integer, Desc: "每页数量，最大100，默认10"},
			"category_id": {Type: schema.Integer, Desc: "分类ID，筛选指定分类的文档"},
			"archive_id":  {Type: schema.Integer, Desc: "文档ID，筛选指定文档的标签"},
			"keyword":     {Type: schema.String, Desc: "关键词搜索，匹配标题和内容"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Page       int    `json:"page"`
			PageSize   int    `json:"page_size"`
			CategoryId uint   `json:"category_id"`
			ArchiveId  int64  `json:"archive_id"`
			Keyword    string `json:"keyword"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		if args.Page <= 0 {
			args.Page = 1
		}
		if args.PageSize <= 0 || args.PageSize > 100 {
			args.PageSize = 10
		}
		var catIds []uint
		if args.CategoryId > 0 {
			catIds = append(catIds, args.CategoryId)
		}
		tags, total, err := w.GetTagList(args.ArchiveId, args.Keyword, catIds, "", args.Page, args.PageSize, 0, "id ASC")
		if err != nil {
			return "", fmt.Errorf("获取标签列表失败: %w", err)
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共 %d 个标签：\n\n", total))
		for _, t := range tags {
			b.WriteString(fmt.Sprintf("- [%d] %s", t.Id, t.Title))
			if t.FirstLetter != "" {
				b.WriteString(fmt.Sprintf(" (%s)", t.FirstLetter))
			}
			b.WriteString("\n")
		}
		return b.String(), nil
	})

	add(&schema.ToolInfo{
		Name: "tag_get",
		Desc: "获取单个标签的详细信息。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "标签ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		tag, err := w.GetTagById(uint(args.Id))
		if err != nil {
			return "", fmt.Errorf("获取标签失败: %w", err)
		}
		return fmt.Sprintf("标签信息：\nID: %d\n标题: %s\n首字母: %s\n描述: %s",
			tag.Id, tag.Title, tag.FirstLetter, tag.Description), nil
	})

	add(&schema.ToolInfo{
		Name: "tag_create",
		Desc: "创建新标签。必填字段：title（标签名称）。创建成功返回标签ID。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"title":       {Type: schema.String, Desc: "标签名称", Required: true},
			"description": {Type: schema.String, Desc: "标签描述"},
			"category_id": {Type: schema.Integer, Desc: "标签分类ID"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			CategoryID  uint   `json:"category_id"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Title == "" {
			return "错误：标签名称不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		tag, err := w.SaveTag(&request.PluginTag{
			Title:       args.Title,
			Description: args.Description,
			CategoryId:  args.CategoryID,
		})
		if err != nil {
			return "", fmt.Errorf("创建标签失败: %w", err)
		}
		return fmt.Sprintf("标签创建成功！ID: %d, 名称: %s", tag.Id, tag.Title), nil
	})

	add(&schema.ToolInfo{
		Name: "tag_delete",
		Desc: "删除标签。需要传入标签ID。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "要删除的标签ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		tag, err := w.GetTagById(uint(args.Id))
		if err != nil {
			return "", fmt.Errorf("获取标签失败: %w", err)
		}
		if err := w.DeleteTag(uint(args.Id)); err != nil {
			return "", fmt.Errorf("删除标签失败: %w", err)
		}
		return fmt.Sprintf("标签 [%d] %s 已成功删除", tag.Id, tag.Title), nil
	})

	add(&schema.ToolInfo{
		Name: "archive_tag_update",
		Desc: "将标签绑定到文档。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"tags": {Type: schema.Array, Desc: "标签列表，JSON数组格式如 [\"tag1\",\"tag2\"]", Required: true},
			"ids":  {Type: schema.Array, Desc: "文档ID列表，JSON数组格式如 [1,2,3]", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Tags []string `json:"tags"`
			Ids  []int64  `json:"ids"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if len(args.Tags) == 0 || len(args.Ids) == 0 {
			return "错误：标签列表不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		err := w.UpdateArchiveTags(&request.ArchivesUpdateRequest{
			Ids:  args.Ids,
			Tags: args.Tags,
		})
		if err != nil {
			return "", fmt.Errorf("更新文档标签失败: %w", err)
		}
		return "文档标签更新成功", nil
	})

	// ---- Template tools ----
	add(&schema.ToolInfo{
		Name:        "template_get_info",
		Desc:        "获取当前启用的模板信息。返回模板名称、类型、模板路径、以及所有模板文件列表。用于了解当前站点使用哪个模板。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		designList := w.GetDesignList()
		typeName := "自适应"
		if w.System.TemplateType == config.TemplateTypeAdapt {
			typeName = "代码适配"
		} else if w.System.TemplateType == config.TemplateTypeSeparate {
			typeName = "电脑+手机"
		}
		var info = ""
		info += fmt.Sprintf("当前模板：%s\n", w.System.TemplateName)
		info += fmt.Sprintf("模板类型：%s\n", typeName)
		info += fmt.Sprintf("模板路径：%s\n", w.RootPath+"template/"+w.System.TemplateName+"/")
		info += fmt.Sprintf("可用模板数量：%d\n\n", len(designList))
		info += "所有可用模板列表：\n"
		for _, d := range designList {
			status := "未启用"
			if d.Status == 1 {
				status = "当前启用"
			}
			info += fmt.Sprintf("  - %s (%s) 版本: %s 状态: %s\n", d.Name, d.Package, d.Version, status)
		}
		// Show current template's files
		if w.System.TemplateName != "" {
			// Show template files
			tplFiles, err := w.GetDesignTemplateFiles(w.System.TemplateName)
			if err == nil && len(tplFiles) > 0 {
				info += fmt.Sprintf("\n当前模板文件列表（%d个）：\n", len(tplFiles))
				for _, f := range tplFiles {
					info += fmt.Sprintf("  - template/%s/%s\n", w.System.TemplateName, f.Path)
				}
			}
			// Show static asset files (CSS/JS/images)
			designInfo, err := w.GetDesignInfo(w.System.TemplateName, false)
			if err == nil && len(designInfo.StaticFiles) > 0 {
				info += fmt.Sprintf("\n静态资源文件列表（%d个）：\n", len(designInfo.StaticFiles))
				for _, f := range designInfo.StaticFiles {
					info += fmt.Sprintf("  - static/%s/%s\n", w.System.TemplateName, f.Path)
				}
			}
		}
		return info, nil
	})

	add(&schema.ToolInfo{
		Name: "template_get_file",
		Desc: "获取模板文件的具体内容和路径。需要传入模板包名和文件相对路径。例如：package='default' path='index.html'",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"package": {Type: schema.String, Desc: "模板包名，如 default", Required: true},
			"path":    {Type: schema.String, Desc: "模板文件相对路径，如 index.html 或 category/list.html", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Package string `json:"package"`
			Path    string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Package == "" || args.Path == "" {
			return "错误：package 和 path 不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		// Use the design file detail method to get file content
		designFile, err := w.GetDesignTplFileDetail(args.Package, response.DesignFile{Path: args.Path})
		if err != nil {
			return "", fmt.Errorf("获取模板文件失败: %w", err)
		}
		if designFile == nil || designFile.Content == "" {
			return fmt.Sprintf("模板文件 template/%s/%s 不存在或内容为空", args.Package, args.Path), nil
		}
		result := fmt.Sprintf("文件路径：template/%s/%s\n文件大小：%d bytes\n最后修改：%d\n\n内容：\n%s",
			args.Package, args.Path, designFile.Size, designFile.LastMod, designFile.Content)
		return result, nil
	})

	add(&schema.ToolInfo{
		Name: "template_modify_file",
		Desc: "修改/添加模板文件的内容。修改后需要通过 template_reload 工具重载模板才能生效。需要传入模板包名、文件相对路径和新的文件内容。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"package": {Type: schema.String, Desc: "模板包名，如 default", Required: true},
			"path":    {Type: schema.String, Desc: "模板文件相对路径，如 index.html 或 category/list.html", Required: true},
			"content": {Type: schema.String, Desc: "文件的新内容", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Package string `json:"package"`
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Package == "" || args.Path == "" {
			return "错误：package 和 path 不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		// Check template exists
		_, err := w.GetDesignInfo(args.Package, false)
		if err != nil {
			return "", fmt.Errorf("模板包 '%s' 不存在: %w", args.Package, err)
		}
		// Save the template file
		err = w.SaveDesignTplFile(request.SaveDesignFileRequest{
			Package: args.Package,
			Path:    args.Path,
			Content: args.Content,
		})
		if err != nil {
			return "", fmt.Errorf("保存模板文件失败: %w", err)
		}
		return fmt.Sprintf("模板文件 template/%s/%s 已成功修改。请使用 template_reload 工具重载模板以生效。", args.Package, args.Path), nil
	})

	add(&schema.ToolInfo{
		Name: "template_get_static",
		Desc: "获取静态资源文件（CSS/JS/字体等）的内容和路径。需要传入模板包名和文件相对路径。例如：package='default' path='css/style.css'",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"package": {Type: schema.String, Desc: "模板包名，如 default", Required: true},
			"path":    {Type: schema.String, Desc: "静态文件相对路径，如 css/style.css 或 js/app.js", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Package string `json:"package"`
			Path    string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Package == "" || args.Path == "" {
			return "错误：package 和 path 不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		designFile, err := w.GetDesignStaticFileDetail(args.Package, response.DesignFile{Path: args.Path})
		if err != nil {
			return "", fmt.Errorf("获取静态文件失败: %w", err)
		}
		if designFile == nil || (designFile.Content == "" && designFile.Size == 0) {
			return fmt.Sprintf("静态文件 static/%s/%s 不存在", args.Package, args.Path), nil
		}
		// 对于二进制文件（图片等），仅返回元信息，不返回内容
		isBinary := strings.HasSuffix(args.Path, ".png") || strings.HasSuffix(args.Path, ".jpg") ||
			strings.HasSuffix(args.Path, ".jpeg") || strings.HasSuffix(args.Path, ".gif") ||
			strings.HasSuffix(args.Path, ".svg") || strings.HasSuffix(args.Path, ".ico") ||
			strings.HasSuffix(args.Path, ".webp") || strings.HasSuffix(args.Path, ".bmp") ||
			strings.HasSuffix(args.Path, ".ttf") || strings.HasSuffix(args.Path, ".woff") ||
			strings.HasSuffix(args.Path, ".woff2") || strings.HasSuffix(args.Path, ".eot") ||
			strings.HasSuffix(args.Path, ".zip") || strings.HasSuffix(args.Path, ".pdf")
		if isBinary {
			result := fmt.Sprintf("文件路径：static/%s/%s\n文件大小：%d bytes\n最后修改：%d\n\n此文件为二进制文件，内容未显示。如需修改请上传新文件。",
				args.Package, args.Path, designFile.Size, designFile.LastMod)
			return result, nil
		}
		result := fmt.Sprintf("文件路径：static/%s/%s\n文件大小：%d bytes\n最后修改：%d\n\n内容：\n%s",
			args.Package, args.Path, designFile.Size, designFile.LastMod, designFile.Content)
		return result, nil
	})

	add(&schema.ToolInfo{
		Name: "template_modify_static",
		Desc: "修改/添加静态资源文件（CSS/JS/字体等）的内容。修改后立即生效，无需重载。支持 CSS、JS 等文本文件。需要传入模板包名、文件相对路径和新的文件内容。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"package": {Type: schema.String, Desc: "模板包名，如 default", Required: true},
			"path":    {Type: schema.String, Desc: "静态文件相对路径，如 css/style.css 或 js/app.js", Required: true},
			"content": {Type: schema.String, Desc: "文件的新内容", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Package string `json:"package"`
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Package == "" || args.Path == "" {
			return "错误：package 和 path 不能为空", nil
		}
		if args.Content == "" {
			return "错误：content 不能为空", nil
		}
		// 检查是否为文本文件（禁止修改图片等二进制文件）
		binaryExts := []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp", ".bmp",
			".ttf", ".woff", ".woff2", ".eot", ".zip", ".pdf"}
		for _, ext := range binaryExts {
			if strings.HasSuffix(strings.ToLower(args.Path), ext) {
				return fmt.Sprintf("错误：不支持修改二进制文件 %s，请通过后台界面上传", args.Path), nil
			}
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		// Check template exists
		_, err := w.GetDesignInfo(args.Package, false)
		if err != nil {
			return "", fmt.Errorf("模板包 '%s' 不存在: %w", args.Package, err)
		}
		// Save the static file
		err = w.SaveDesignStaticFile(request.SaveDesignFileRequest{
			Package: args.Package,
			Path:    args.Path,
			Content: args.Content,
		})
		if err != nil {
			return "", fmt.Errorf("保存静态文件失败: %w", err)
		}
		return fmt.Sprintf("静态文件 static/%s/%s 已成功修改，修改已立即生效。", args.Package, args.Path), nil
	})

	add(&schema.ToolInfo{
		Name:        "template_reload",
		Desc:        "重新加载模板。在修改了模板文件内容或切换模板后，需要调用此工具使更改生效。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		config.RestartChan <- 0
		return fmt.Sprintf("模板重载信号已发送，模板将在1秒内重新加载。当前模板：%s", w.System.TemplateName), nil
	})

	// ---- Category Update ----
	add(&schema.ToolInfo{
		Name: "category_update",
		Desc: "更新已有分类的标题、描述、关键词、父分类等信息。需要传入分类ID，至少传入一个要更新的字段。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":          {Type: schema.Integer, Desc: "要更新的分类ID", Required: true},
			"title":       {Type: schema.String, Desc: "新的分类名称"},
			"description": {Type: schema.String, Desc: "新的分类描述"},
			"keywords":    {Type: schema.String, Desc: "新的分类关键词"},
			"parent_id":   {Type: schema.Integer, Desc: "新的父分类ID，0表示顶级分类"},
			"sort":        {Type: schema.Integer, Desc: "排序值，越小越靠前"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Id          uint   `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Keywords    string `json:"keywords"`
			ParentID    uint   `json:"parent_id"`
			Sort        uint   `json:"sort"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Id == 0 {
			return "错误：分类ID不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		cat, err := w.GetCategoryById(args.Id)
		if err != nil {
			return "", fmt.Errorf("获取分类失败: %w", err)
		}
		req := &request.Category{
			Id:        cat.Id,
			Title:     cat.Title,
			Keywords:  cat.Keywords,
			ParentId:  cat.ParentId,
			Sort:      cat.Sort,
			Status:    cat.Status,
			Type:      cat.Type,
			ModuleId:  cat.ModuleId,
			Images:    cat.Images,
			IsInherit: cat.IsInherit,
			UpdateAll: false,
		}
		changed := false
		if args.Title != "" {
			req.Title = args.Title
			changed = true
		}
		if args.Description != "" {
			req.Description = args.Description
			changed = true
		}
		if args.Keywords != "" {
			req.Keywords = args.Keywords
			changed = true
		}
		req.ParentId = args.ParentID
		changed = true
		if args.Sort > 0 {
			req.Sort = args.Sort
			changed = true
		}
		if !changed {
			return "未提供要更新的字段", nil
		}
		cat, err = w.SaveCategory(req)
		if err != nil {
			return "", fmt.Errorf("更新分类失败: %w", err)
		}
		return fmt.Sprintf("分类 [%d] %s 已成功更新", cat.Id, cat.Title), nil
	})

	// ---- Tag Update ----
	add(&schema.ToolInfo{
		Name: "tag_update",
		Desc: "更新已有标签的标题、描述等信息。需要传入标签ID，至少传入一个要更新的字段。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":          {Type: schema.Integer, Desc: "要更新的标签ID", Required: true},
			"title":       {Type: schema.String, Desc: "新的标签名称"},
			"description": {Type: schema.String, Desc: "新的标签描述"},
			"category_id": {Type: schema.Integer, Desc: "标签分类ID"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Id          uint   `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			CategoryID  uint   `json:"category_id"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Id == 0 {
			return "错误：标签ID不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		tag, err := w.GetTagById(args.Id)
		if err != nil {
			return "", fmt.Errorf("获取标签失败: %w", err)
		}
		req := &request.PluginTag{
			Id:          tag.Id,
			Title:       tag.Title,
			Description: tag.Description,
			CategoryId:  tag.CategoryId,
			UpdateAll:   false,
		}
		changed := false
		if args.Title != "" {
			req.Title = args.Title
			changed = true
		}
		if args.Description != "" {
			req.Description = args.Description
			changed = true
		}
		if args.CategoryID > 0 {
			req.CategoryId = args.CategoryID
			changed = true
		}
		if !changed {
			return "未提供要更新的字段", nil
		}
		tag, err = w.SaveTag(req)
		if err != nil {
			return "", fmt.Errorf("更新标签失败: %w", err)
		}
		return fmt.Sprintf("标签 [%d] %s 已成功更新", tag.Id, tag.Title), nil
	})

	// ---- Attachment Tools ----
	add(&schema.ToolInfo{
		Name: "attachment_list",
		Desc: "分页获取附件列表，支持按分类和关键词过滤。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"page":        {Type: schema.Integer, Desc: "页码，从1开始，默认1"},
			"page_size":   {Type: schema.Integer, Desc: "每页数量，最大100，默认10"},
			"category_id": {Type: schema.Integer, Desc: "附件分类ID，筛选指定分类"},
			"keyword":     {Type: schema.String, Desc: "搜索关键词，匹配文件名"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Page       int    `json:"page"`
			PageSize   int    `json:"page_size"`
			CategoryID uint   `json:"category_id"`
			Keyword    string `json:"keyword"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Page <= 0 {
			args.Page = 1
		}
		if args.PageSize <= 0 || args.PageSize > 100 {
			args.PageSize = 10
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		attachments, total, err := w.GetAttachmentList(args.CategoryID, args.Keyword, args.Page, args.PageSize)
		if err != nil {
			return "", fmt.Errorf("获取附件列表失败: %w", err)
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共 %d 个附件（当前页 %d 个）：\n\n", total, len(attachments)))
		for _, a := range attachments {
			a.GetThumb(w.PluginStorage.StorageUrl)
			imgType := "文件"
			if a.IsImage == 1 {
				imgType = "图片"
			}
			b.WriteString(fmt.Sprintf("- [%d] %s (%s, %d×%d) %s\n", a.Id, a.FileName, imgType, a.Width, a.Height, a.Logo))
		}
		return b.String(), nil
	})

	add(&schema.ToolInfo{
		Name: "attachment_get",
		Desc: "获取单个附件的详细信息，包括URL。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "附件ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		a, err := w.GetAttachmentById(uint(args.Id))
		if err != nil {
			return "", fmt.Errorf("获取附件失败: %w", err)
		}
		a.GetThumb(w.PluginStorage.StorageUrl)
		imgType := "文件"
		if a.IsImage == 1 {
			imgType = "图片"
		}
		return fmt.Sprintf("附件信息：\nID: %d\n文件名: %s\n类型: %s\n尺寸: %d×%d\n大小: %d 字节\nURL: %s\n缩略图: %s",
			a.Id, a.FileName, imgType, a.Width, a.Height, a.FileSize, a.Logo, a.Thumb), nil
	})

	add(&schema.ToolInfo{
		Name: "attachment_upload",
		Desc: "通过远程URL或本地文件路径上传附件（图片）到站点。本地文件路径通常是AI聊天上传按钮上传的临时文件路径（file_path）。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"url":       {Type: schema.String, Desc: "图片的远程URL地址，或本地文件路径（绝对路径或基于站点项目目录的相对路径）", Required: true},
			"file_name": {Type: schema.String, Desc: "保存的文件名（不含扩展名），可选"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			URL      string `json:"url"`
			FileName string `json:"file_name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.URL == "" {
			return "错误：URL不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		var attachment *model.Attachment
		// 判断是否为本地文件路径
		if strings.HasPrefix(args.URL, "http://") || strings.HasPrefix(args.URL, "https://") {
			// 远程URL下载
			var err error
			attachment, err = w.DownloadRemoteImage(args.URL, args.FileName, 0)
			if err != nil {
				return "", fmt.Errorf("上传附件失败: %w", err)
			}
		} else {
			// 本地文件路径
			localPath := args.URL
			if !filepath.IsAbs(localPath) {
				localPath = filepath.Join(svc.projectRoot, localPath)
			}
			// 安全检查：防止路径遍历
			localPath, err := filepath.Abs(localPath)
			if err != nil {
				return "", fmt.Errorf("无法解析路径: %w", err)
			}
			if svc.projectRoot != "" && !strings.HasPrefix(localPath, svc.projectRoot) {
				return "", fmt.Errorf("路径超出项目目录范围: %s", localPath)
			}
			// 打开文件
			file, err := os.Open(localPath)
			if err != nil {
				return "", fmt.Errorf("无法打开文件: %w", err)
			}
			defer file.Close()
			stat, err := file.Stat()
			if err != nil {
				return "", fmt.Errorf("无法获取文件信息: %w", err)
			}
			fileName := args.FileName
			if fileName == "" {
				fileName = strings.TrimSuffix(stat.Name(), filepath.Ext(stat.Name()))
			}
			fileHeader := &multipart.FileHeader{
				Filename: stat.Name(),
				Size:     stat.Size(),
			}
			attachment, err = w.AttachmentUpload(file, fileHeader, 0, 0, 0)
			if err != nil {
				return "", fmt.Errorf("上传附件失败: %w", err)
			}
		}
		attachment.GetThumb(w.PluginStorage.StorageUrl)
		return fmt.Sprintf("附件上传成功！ID: %d\n文件名: %s\nURL: %s", attachment.Id, attachment.FileName, attachment.Logo), nil
	})

	add(&schema.ToolInfo{
		Name: "attachment_delete",
		Desc: "删除附件。注意：此操作会同时删除物理文件，不可恢复。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "要删除的附件ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		attach, err := w.GetAttachmentById(uint(args.Id))
		if err != nil {
			return "", fmt.Errorf("获取附件失败: %w", err)
		}
		err = w.DeleteAttachment(attach)
		if err != nil {
			return "", fmt.Errorf("删除附件失败: %w", err)
		}
		return fmt.Sprintf("附件 [%d] %s 已成功删除", attach.Id, attach.FileName), nil
	})

	// ---- Navigation tools ----
	add(&schema.ToolInfo{
		Name: "nav_list",
		Desc: "获取导航菜单列表，按树形结构返回。type_id=1 为主导航，其他值可能对应不同导航位置。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"type_id":   {Type: schema.Integer, Desc: "导航类型ID，默认1（主导航）"},
			"show_type": {Type: schema.String, Desc: "展示类型：list=平铺列表，children=树形结构（默认）"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			TypeId   uint   `json:"type_id"`
			ShowType string `json:"show_type"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.TypeId == 0 {
			args.TypeId = 1
		}
		if args.ShowType == "" {
			args.ShowType = "children"
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		navs, err := w.GetNavList(args.TypeId, args.ShowType)
		if err != nil {
			return "", fmt.Errorf("获取导航列表失败: %w", err)
		}
		var b strings.Builder
		if len(navs) == 0 {
			b.WriteString("暂无导航菜单")
		} else {
			b.WriteString(fmt.Sprintf("共 %d 个导航项（type_id=%d）：\n\n", len(navs), args.TypeId))
			var printNav func(navs []*model.Nav, depth int)
			printNav = func(navs []*model.Nav, depth int) {
				for _, n := range navs {
					prefix := ""
					for i := 0; i < depth; i++ {
						prefix += "  "
					}
					navType := "系统"
					switch n.NavType {
					case model.NavTypeCategory:
						navType = "分类"
					case model.NavTypeOutlink:
						navType = "外链"
					case model.NavTypeArchive:
						navType = "文档"
					}
					status := "启用"
					if n.Status == 0 {
						status = "禁用"
					}
					b.WriteString(fmt.Sprintf("%s- [%d] %s (类型:%s, 排序:%d, 状态:%s)\n", prefix, n.Id, n.Title, navType, n.Sort, status))
					if len(n.NavList) > 0 {
						printNav(n.NavList, depth+1)
					}
				}
			}
			printNav(navs, 0)
		}
		return b.String(), nil
	})

	add(&schema.ToolInfo{
		Name: "nav_create",
		Desc: "创建或编辑导航菜单。如果指定ID则更新已有导航，否则创建新导航。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":          {Type: schema.Integer, Desc: "导航ID，留空表示创建新导航"},
			"title":       {Type: schema.String, Desc: "导航显示名称", Required: true},
			"sub_title":   {Type: schema.String, Desc: "副标题"},
			"description": {Type: schema.String, Desc: "导航描述"},
			"parent_id":   {Type: schema.Integer, Desc: "父导航ID，0表示顶级"},
			"nav_type":    {Type: schema.Integer, Desc: "导航类型：0=系统页,1=分类,2=外链,3=文档"},
			"page_id":     {Type: schema.Integer, Desc: "关联的页面ID（分类ID/文档ID等）"},
			"type_id":     {Type: schema.Integer, Desc: "导航位置类型ID，默认1"},
			"link":        {Type: schema.String, Desc: "外链URL（nav_type=2时必填）"},
			"sort":        {Type: schema.Integer, Desc: "排序值，越小越靠前"},
			"status":      {Type: schema.Integer, Desc: "状态：1=启用,0=禁用，默认1"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Id          uint   `json:"id"`
			Title       string `json:"title"`
			SubTitle    string `json:"sub_title"`
			Description string `json:"description"`
			ParentId    uint   `json:"parent_id"`
			NavType     uint   `json:"nav_type"`
			PageId      int64  `json:"page_id"`
			TypeId      uint   `json:"type_id"`
			Link        string `json:"link"`
			Sort        uint   `json:"sort"`
			Status      uint   `json:"status"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Title == "" {
			return "错误：导航显示名称不能为空", nil
		}
		if args.TypeId == 0 {
			args.TypeId = 1
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		req := &request.NavConfig{
			Id:          args.Id,
			Title:       args.Title,
			SubTitle:    args.SubTitle,
			Description: args.Description,
			ParentId:    args.ParentId,
			NavType:     args.NavType,
			PageId:      args.PageId,
			TypeId:      args.TypeId,
			Link:        args.Link,
			Sort:        args.Sort,
			Status:      args.Status,
			UpdateAll:   false,
		}
		if req.Status == 0 {
			req.Status = 1
		}
		nav, err := w.SaveNav(req)
		if err != nil {
			return "", fmt.Errorf("保存导航失败: %w", err)
		}
		w.DeleteCacheNavs()
		return fmt.Sprintf("导航 [%d] %s 已成功保存", nav.Id, nav.Title), nil
	})

	add(&schema.ToolInfo{
		Name: "nav_delete",
		Desc: "删除导航菜单。注意：此操作不可恢复。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "要删除的导航ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		nav, err := w.GetNavById(uint(args.Id))
		if err != nil {
			return "", fmt.Errorf("获取导航失败: %w", err)
		}
		err = nav.Delete(w.DB)
		if err != nil {
			return "", fmt.Errorf("删除导航失败: %w", err)
		}
		w.DeleteCacheNavs()
		return fmt.Sprintf("导航 [%d] %s 已成功删除", nav.Id, nav.Title), nil
	})
	add(&schema.ToolInfo{
		Name: "nav_update",
		Desc: "更新导航菜单。使用 SettingNavForm 逻辑更新导航的全部字段。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":          {Type: schema.Integer, Desc: "导航ID", Required: true},
			"title":       {Type: schema.String, Desc: "导航显示名称"},
			"sub_title":   {Type: schema.String, Desc: "副标题"},
			"description": {Type: schema.String, Desc: "导航描述"},
			"parent_id":   {Type: schema.Integer, Desc: "父导航ID"},
			"nav_type":    {Type: schema.Integer, Desc: "导航类型：0=系统页,1=分类,2=外链,3=文档"},
			"page_id":     {Type: schema.Integer, Desc: "关联的页面ID"},
			"type_id":     {Type: schema.Integer, Desc: "导航位置类型ID，默认1"},
			"link":        {Type: schema.String, Desc: "外链URL"},
			"sort":        {Type: schema.Integer, Desc: "排序值"},
			"status":      {Type: schema.Integer, Desc: "状态：1=启用,0=禁用"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		var args request.NavConfig
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Id == 0 {
			return "错误：导航ID不能为空", nil
		}
		args.UpdateAll = false
		nav, err := w.SaveNav(&args)
		if err != nil {
			return "", fmt.Errorf("更新导航失败: %w", err)
		}
		w.DeleteCacheNavs()
		w.DeleteCacheIndex()
		return fmt.Sprintf("导航 [%d] %s 已更新", nav.Id, nav.Title), nil
	})

	// ---- Friend link tools ----
	add(&schema.ToolInfo{
		Name:        "friendlink_list",
		Desc:        "获取友情链接列表，按排序值排列。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		links, err := w.GetLinkList()
		if err != nil {
			return "", fmt.Errorf("获取友链列表失败: %w", err)
		}
		var b strings.Builder
		if len(links) == 0 {
			b.WriteString("暂无友情链接")
		} else {
			b.WriteString(fmt.Sprintf("共 %d 条友情链接：\n\n", len(links)))
			for _, l := range links {
				status := "待审"
				if l.Status == model.LinkStatusOk {
					status = "正常"
				} else if l.Status == model.LinkStatusNofollow {
					status = "nofollow"
				} else if l.Status == model.LinkStatusNotTitle {
					status = "缺少标题"
				} else if l.Status == model.LinkStatusNotMatch {
					status = "不匹配"
				}
				b.WriteString(fmt.Sprintf("- [%d] %s -> %s (状态:%s, 排序:%d)\n", l.Id, l.Title, l.Link, status, l.Sort))
			}
		}
		return b.String(), nil
	})

	add(&schema.ToolInfo{
		Name: "friendlink_create",
		Desc: "添加或更新友情链接。如果相同链接已存在则更新，否则创建新链接。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"title":     {Type: schema.String, Desc: "站点名称", Required: true},
			"link":      {Type: schema.String, Desc: "站点URL", Required: true},
			"back_link": {Type: schema.String, Desc: "回链URL（可选）"},
			"my_title":  {Type: schema.String, Desc: "我方显示标题"},
			"my_link":   {Type: schema.String, Desc: "我方链接"},
			"contact":   {Type: schema.String, Desc: "联系方式（QQ/邮箱）"},
			"remark":    {Type: schema.String, Desc: "备注"},
			"nofollow":  {Type: schema.Integer, Desc: "是否nofollow：1=是,0=否"},
			"sort":      {Type: schema.Integer, Desc: "排序值，越小越靠前"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Title    string `json:"title"`
			Link     string `json:"link"`
			BackLink string `json:"back_link"`
			MyTitle  string `json:"my_title"`
			MyLink   string `json:"my_link"`
			Contact  string `json:"contact"`
			Remark   string `json:"remark"`
			Nofollow uint   `json:"nofollow"`
			Sort     uint   `json:"sort"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Title == "" || args.Link == "" {
			return "错误：站点名称和URL不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		friendLink, err := w.GetLinkByLinkAndTitle(args.Link, args.Title)
		if err != nil {
			friendLink = &model.Link{Status: 0}
		}
		friendLink.Title = args.Title
		friendLink.Link = args.Link
		friendLink.BackLink = args.BackLink
		if args.MyTitle != "" {
			friendLink.MyTitle = args.MyTitle
		}
		if args.MyLink != "" {
			friendLink.MyLink = args.MyLink
		}
		if args.Contact != "" {
			friendLink.Contact = args.Contact
		}
		if args.Remark != "" {
			friendLink.Remark = args.Remark
		}
		friendLink.Nofollow = args.Nofollow
		if args.Sort > 0 {
			friendLink.Sort = args.Sort
		}
		if friendLink.Status == model.LinkStatusOk {
			// 重新申请审核
			friendLink.Status = 0
		}
		err = friendLink.Save(w.DB)
		if err != nil {
			return "", fmt.Errorf("保存友链失败: %w", err)
		}
		w.DeleteCacheIndex()
		return fmt.Sprintf("友链 [%d] %s -> %s 已成功保存", friendLink.Id, friendLink.Title, friendLink.Link), nil
	})

	add(&schema.ToolInfo{
		Name: "friendlink_delete",
		Desc: "删除友情链接。可以通过ID或URL+标题删除。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":    {Type: schema.Integer, Desc: "友链ID（与link二选一）"},
			"link":  {Type: schema.String, Desc: "友链URL（与id二选一）"},
			"title": {Type: schema.String, Desc: "友链标题（配合link使用）"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Id    uint   `json:"id"`
			Link  string `json:"link"`
			Title string `json:"title"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		var friendLink *model.Link
		var err error
		if args.Id > 0 {
			friendLink, err = w.GetLinkById(args.Id)
		} else if args.Link != "" {
			friendLink, err = w.GetLinkByLinkAndTitle(args.Link, args.Title)
		} else {
			return "错误：请提供友链ID或URL", nil
		}
		if err != nil {
			return "", fmt.Errorf("获取友链失败: %w", err)
		}
		err = friendLink.Delete(w.DB)
		if err != nil {
			return "", fmt.Errorf("删除友链失败: %w", err)
		}
		w.DeleteCacheIndex()
		return fmt.Sprintf("友链 [%d] %s 已成功删除", friendLink.Id, friendLink.Title), nil
	})

	// ---- Website info tool ----
	add(&schema.ToolInfo{
		Name:        "website_info",
		Desc:        "获取当前站点基本信息，包括站点名称、Logo、联系方式、备案号等。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		s := w.System
		c := w.Content
		contact := w.Contact
		index := w.Index
		return fmt.Sprintf(`站点信息：
━━━━━━━━━━━━━━━━━━━━━
名称: %s
Logo: %s
网址: %s
IPC备案: %s
版权信息: %s
邮箱: %s
电话: %s
地址: %s
━━━━━━━━━━━━━━━━━━━━━
内容设置：
远程下载: %d
过滤外链: %d
URL模式: %d
默认模板: %s
━━━━━━━━━━━━━━━━━━━━━
SEO信息：
标题: %s
关键词: %s
描述: %s`,
			s.SiteName, s.SiteLogo, s.BaseUrl, s.SiteIcp,
			s.SiteCopyright, contact.Email, contact.Cellphone, contact.Address,
			c.RemoteDownload, c.FilterOutlink, c.UrlTokenType, s.TemplateName,
			index.SeoTitle, index.SeoKeywords, index.SeoDescription), nil
	})

	// ---- System info & Setting tools ----
	add(&schema.ToolInfo{
		Name:        "version",
		Desc:        "查看当前系统版本号和试用状态。",
		// ParamsOneOf left nil intentionally: no parameters needed
	}, func(ctx context.Context, argsJSON string) (string, error) {
		trial := "否"
		if config.Trial {
			trial = "是"
		}
		return fmt.Sprintf("版本信息：\n版本号: %s\n试用版: %s", config.Version, trial), nil
	})
	add(&schema.ToolInfo{
		Name:        "anqi_info",
		Desc:        "查看安企CMS授权登录信息，包括是否已登录、会员到期时间等。",
		// ParamsOneOf left nil intentionally: no parameters needed
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		go w.AnqiCheckLogin(false)
		info := GetAuthInfo()
		if info == nil {
			return "暂未获取到授权信息，请稍后重试", nil
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("授权信息：\n"))
		b.WriteString(fmt.Sprintf("用户名: %s\n", info.UserName))
		b.WriteString(fmt.Sprintf("授权ID: %d\n", info.AuthId))
		b.WriteString(fmt.Sprintf("积分余额: %d\n", info.Integral))
		if info.Valid && info.ExpireTime > 0 {
			b.WriteString(fmt.Sprintf("会员类型: VIP会员\n"))
			b.WriteString(fmt.Sprintf("会员到期: %s\n", time.Unix(info.ExpireTime, 0).Format("2006-01-02 15:04:05")))
		} else {
			b.WriteString(fmt.Sprintf("会员类型: 普通会员\n"))
		}
		b.WriteString(fmt.Sprintf("累计使用Token: %d\n", info.TotalToken))
		b.WriteString(fmt.Sprintf("未支付Token: %d\n", info.UnPayToken))
		if info.IsOweFee == 1 {
			b.WriteString(fmt.Sprintf("是否已欠费: %d\n", "是"))
		} else {
			b.WriteString(fmt.Sprintf("是否已欠费: %d\n", "否"))
		}
		return b.String(), nil
	})
	add(&schema.ToolInfo{
		Name:        "setting_system",
		Desc:        "查看系统设置，包括站点名称、Logo、备案号、网址、语言等基础配置。",
		// ParamsOneOf left nil intentionally: no parameters needed
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		s := w.System
		return fmt.Sprintf(`系统设置：
━━━━━━━━━━━━━━━━━━━━━
站点名称: %s
Logo: %s
网址: %s
前台网址: %s
手机网址: %s
后台域名: %s
ICP备案: %s
版权信息: %s
语言: %s
站点关闭: %d
关闭提示: %s
禁用爬虫: %d
模板类型：%d
模板: %s
━━━━━━━━━━━━━━━━━━━━━`,
			s.SiteName, s.SiteLogo, s.BaseUrl, s.FrontUrl, s.MobileUrl,
			s.AdminUrl, s.SiteIcp, s.SiteCopyright, s.Language,
			s.SiteClose, s.SiteCloseTips, s.BanSpider, s.TemplateType, s.TemplateName), nil
	})
	add(&schema.ToolInfo{
		Name: "setting_system_form",
		Desc: "更新系统设置。可修改站点名称、Logo、网址、备案号、版权、语言等。只传入要更新的字段。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"site_name":       {Type: schema.String, Desc: "站点名称"},
			"site_logo":       {Type: schema.String, Desc: "站点Logo URL"},
			"base_url":        {Type: schema.String, Desc: "站点网址"},
			"front_url":       {Type: schema.String, Desc: "前台网址"},
			"mobile_url":      {Type: schema.String, Desc: "手机网址（仅模板类型为电脑+手机时有效）"},
			"admin_url":       {Type: schema.String, Desc: "后台域名（需http开头）"},
			"site_icp":        {Type: schema.String, Desc: "ICP备案号"},
			"site_copyright":  {Type: schema.String, Desc: "版权信息"},
			"language":        {Type: schema.String, Desc: "语言包（如 zh）"},
			"site_close":      {Type: schema.Integer, Desc: "站点关闭：1=关闭,0=开启"},
			"site_close_tips": {Type: schema.String, Desc: "关闭时提示信息"},
			"ban_spider":      {Type: schema.Integer, Desc: "禁用爬虫：1=禁用,0=允许"},
			"template_type":   {Type: schema.Integer, Desc: "模板类型：0=自适应,1=代码适配,2=电脑+手机"},
			"template_name":   {Type: schema.String, Desc: "模板"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args config.SystemConfig
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.AdminUrl != "" && !strings.HasPrefix(args.AdminUrl, "http") {
			return "错误：后台域名必须以 http 或 https 开头", nil
		}
		if args.SiteLogo != "" {
			args.SiteLogo = strings.TrimPrefix(args.SiteLogo, w.PluginStorage.StorageUrl)
		}
		if args.BaseUrl != "" {
			args.BaseUrl = strings.TrimRight(args.BaseUrl, "/")
		}
		if args.FrontUrl != "" {
			args.FrontUrl = strings.TrimRight(args.FrontUrl, "/")
		}
		if args.MobileUrl != "" {
			w.System.MobileUrl = args.MobileUrl
		}
		if args.SiteName != "" {
			w.System.SiteName = args.SiteName
		}
		if args.SiteLogo != "" {
			w.System.SiteLogo = args.SiteLogo
		}
		if args.SiteIcp != "" {
			w.System.SiteIcp = args.SiteIcp
		}
		if args.SiteCopyright != "" {
			w.System.SiteCopyright = args.SiteCopyright
		}
		if args.AdminUrl != "" {
			w.System.AdminUrl = args.AdminUrl
		}
		if args.BaseUrl != "" {
			w.System.BaseUrl = args.BaseUrl
		}
		if args.FrontUrl != "" {
			w.System.FrontUrl = args.FrontUrl
		}
		if args.Language != "" {
			w.System.Language = args.Language
		}
		if args.SiteCloseTips != "" {
			w.System.SiteCloseTips = args.SiteCloseTips
		}
		if args.TemplateName != "" {
			w.System.TemplateName = args.TemplateName
		}
		w.System.TemplateType = args.TemplateType
		w.System.SiteClose = args.SiteClose
		w.System.BanSpider = args.BanSpider
		if err := w.SaveSettingValue(SystemSettingKey, w.System); err != nil {
			return "", fmt.Errorf("保存系统设置失败: %w", err)
		}
		w.DeleteCacheIndex()
		return "系统设置已更新", nil
	})
	add(&schema.ToolInfo{
		Name:        "setting_contact",
		Desc:        "查看联系方式设置，包括联系人、电话、地址、邮箱、微信、QQ等。",
		// ParamsOneOf left nil intentionally: no parameters needed
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		c := w.Contact
		return fmt.Sprintf(`联系方式：
━━━━━━━━━━━━━━━━━━━━━
联系人: %s
电话: %s
地址: %s
邮箱: %s
微信: %s
QQ: %s
WhatsApp: %s
Facebook: %s
Twitter: %s
Tiktok: %s
Pinterest: %s
Linkedin: %s
Instagram: %s
Youtube: %s
二维码: %s`,
			c.UserName, c.Cellphone, c.Address, c.Email, c.Wechat, c.QQ,
			c.WhatsApp, c.Facebook, c.Twitter, c.Tiktok, c.Pinterest,
			c.Linkedin, c.Instagram, c.Youtube, c.Qrcode), nil
	})
	add(&schema.ToolInfo{
		Name: "setting_contact_form",
		Desc: "更新联系方式设置。可修改联系人、电话、地址、邮箱、微信、QQ等。只传入要更新的字段。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"user_name": {Type: schema.String, Desc: "联系人"},
			"cellphone": {Type: schema.String, Desc: "电话"},
			"address":   {Type: schema.String, Desc: "地址"},
			"email":     {Type: schema.String, Desc: "邮箱"},
			"wechat":    {Type: schema.String, Desc: "微信号"},
			"qq":        {Type: schema.String, Desc: "QQ号"},
			"whats_app": {Type: schema.String, Desc: "WhatsApp"},
			"facebook":  {Type: schema.String, Desc: "Facebook"},
			"twitter":   {Type: schema.String, Desc: "Twitter"},
			"tiktok":    {Type: schema.String, Desc: "TikTok"},
			"pinterest": {Type: schema.String, Desc: "Pinterest"},
			"linkedin":  {Type: schema.String, Desc: "LinkedIn"},
			"instagram": {Type: schema.String, Desc: "Instagram"},
			"youtube":   {Type: schema.String, Desc: "Youtube"},
			"qrcode":    {Type: schema.String, Desc: "二维码图片URL"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args config.ContactConfig
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.UserName != "" {
			w.Contact.UserName = args.UserName
		}
		if args.Cellphone != "" {
			w.Contact.Cellphone = args.Cellphone
		}
		if args.Address != "" {
			w.Contact.Address = args.Address
		}
		if args.Email != "" {
			w.Contact.Email = args.Email
		}
		if args.Wechat != "" {
			w.Contact.Wechat = args.Wechat
		}
		if args.QQ != "" {
			w.Contact.QQ = args.QQ
		}
		if args.WhatsApp != "" {
			w.Contact.WhatsApp = args.WhatsApp
		}
		if args.Facebook != "" {
			w.Contact.Facebook = args.Facebook
		}
		if args.Twitter != "" {
			w.Contact.Twitter = args.Twitter
		}
		if args.Tiktok != "" {
			w.Contact.Tiktok = args.Tiktok
		}
		if args.Pinterest != "" {
			w.Contact.Pinterest = args.Pinterest
		}
		if args.Linkedin != "" {
			w.Contact.Linkedin = args.Linkedin
		}
		if args.Instagram != "" {
			w.Contact.Instagram = args.Instagram
		}
		if args.Youtube != "" {
			w.Contact.Youtube = args.Youtube
		}
		if args.Qrcode != "" {
			args.Qrcode = strings.TrimPrefix(args.Qrcode, w.PluginStorage.StorageUrl)
			w.Contact.Qrcode = args.Qrcode
		}
		if err := w.SaveSettingValue(ContactSettingKey, w.Contact); err != nil {
			return "", fmt.Errorf("保存联系方式失败: %w", err)
		}
		w.DeleteCacheIndex()
		return "联系方式已更新", nil
	})
	add(&schema.ToolInfo{
		Name:        "setting_diy_field",
		Desc:        "查看自定义字段设置，返回所有自定义字段的配置信息。",
		// ParamsOneOf left nil intentionally: no parameters needed
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		fields := w.GetDiyFieldSetting()
		if len(fields) == 0 {
			return "暂无自定义字段配置", nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共 %d 个自定义字段：\n\n", len(fields)))
		for _, f := range fields {
			b.WriteString(fmt.Sprintf("  - %s (%s, %s),默认值: %s。字段值: %s\n", f.FieldName, f.Type, f.Name, strings.ReplaceAll(f.Content, "\n", ","), f.Value))

			b.WriteString(fmt.Sprintf("- %s (%s) 描述: %s\n", f.FieldName, f.Type, f.Name))
		}
		return b.String(), nil
	})
	add(&schema.ToolInfo{
		Name: "setting_diy_field_form",
		Desc: "更新自定义字段配置。需要传入完整的字段列表，会覆盖现有配置。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"fields": {Type: schema.Array, Desc: "自定义字段列表，JSON数组", Required: true,
				ElemInfo: &schema.ParameterInfo{
					Type: schema.Object,
					Desc: "字段信息",
					SubParams: map[string]*schema.ParameterInfo{
						"field_name": {Type: schema.String, Desc: "字段名", Required: true},
						"name":       {Type: schema.String, Desc: "字段描述", Required: true},
						"type":       {Type: schema.String, Desc: "字段类型: text|textarea|select|checkbox|radio|number|texts|editor|image|images|file", Required: true},
						"content":    {Type: schema.String, Desc: "默认值"},
						"value":      {Type: schema.String, Desc: "字段值"},
					},
				},
			},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			Fields []config.CustomField `json:"fields"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		for i, field := range args.Fields {
			if field.Name == "" {
				return fmt.Sprintf("错误：字段[%d]名称不能为空", i), nil
			}
			if field.Type == "" {
				return fmt.Sprintf("错误：字段[%d]类型不能为空", i), nil
			}
		}
		if err := w.SaveSettingValue(DiyFieldsKey, args.Fields); err != nil {
			return "", fmt.Errorf("保存自定义字段失败: %w", err)
		}
		w.Cache.Delete(DiyFieldsKey)
		w.DeleteCacheIndex()
		return fmt.Sprintf("自定义字段已更新，共 %d 个字段", len(args.Fields)), nil
	})
	add(&schema.ToolInfo{
		Name:        "setting_index",
		Desc:        "查看首页TDK设置（SEO标题、关键词、描述）。",
		// ParamsOneOf left nil intentionally: no parameters needed
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		idx := w.Index
		return fmt.Sprintf(`首页TDK设置：
━━━━━━━━━━━━━━━━━━━━━
SEO标题: %s
SEO关键词: %s
SEO描述: %s
标题分隔符: %s`,
			idx.SeoTitle, idx.SeoKeywords, idx.SeoDescription, idx.Sep), nil
	})
	add(&schema.ToolInfo{
		Name: "setting_index_form",
		Desc: "更新首页TDK设置（SEO标题、关键词、描述）。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"seo_title":       {Type: schema.String, Desc: "SEO标题"},
			"seo_keywords":    {Type: schema.String, Desc: "SEO关键词，多个用逗号分隔"},
			"seo_description": {Type: schema.String, Desc: "SEO描述"},
			"sep":             {Type: schema.String, Desc: "标题分隔符"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args config.IndexConfig
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.SeoTitle != "" {
			w.Index.SeoTitle = args.SeoTitle
		}
		if args.SeoKeywords != "" {
			w.Index.SeoKeywords = args.SeoKeywords
		}
		if args.SeoDescription != "" {
			w.Index.SeoDescription = args.SeoDescription
		}
		if args.Sep != "" {
			w.Index.Sep = args.Sep
		}
		if err := w.SaveSettingValue(IndexSettingKey, w.Index); err != nil {
			return "", fmt.Errorf("保存首页TDK失败: %w", err)
		}
		w.DeleteCacheIndex()
		return "首页TDK已更新", nil
	})
	add(&schema.ToolInfo{
		Name:        "setting_content",
		Desc:        "查看内容设置，包括远程下载、过滤外链、URL模式、缩略图、编辑器等。",
		// ParamsOneOf left nil intentionally: no parameters needed
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		c := w.Content
		return fmt.Sprintf(`内容设置：
━━━━━━━━━━━━━━━━━━━━━
远程下载: %d
过滤外链: %d
自定义URL格式: %d
使用Webp: %d
转换Gif: %d
图片质量: %d
缩放图片: %d
缩放宽度: %d
裁剪缩略图模式: %d
缩略图宽: %d
缩略图高: %d
默认缩略图来源：%d
默认缩略图列表：%v
默认缩略图分类：%d
启用文档多分类: %d
启用排序: %d
编辑器类型: %s
最大页码: %d
最大条数: %d`,
			c.RemoteDownload, c.FilterOutlink, c.UrlTokenType,
			c.UseWebp, c.ConvertGif, c.Quality, c.ResizeImage,
			c.ResizeWidth, c.ThumbCrop, c.ThumbWidth, c.ThumbHeight,
			c.DefaultThumbType, c.DefaultThumbs, c.ThumbCategoryId,
			c.MultiCategory, c.UseSort, c.Editor, c.MaxPage, c.MaxLimit), nil
	})
	add(&schema.ToolInfo{
		Name: "setting_content_form",
		Desc: "更新内容设置。需传入所有参数，不更改的参数请设置为原值。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"remote_download":    {Type: schema.Integer, Desc: "远程下载：1=开启,0=关闭"},
			"filter_outlink":     {Type: schema.Integer, Desc: "过滤外链：1=开启,0=关闭"},
			"url_token_type":     {Type: schema.Integer, Desc: "自定义URL格式：0=全拼音,1=首字母"},
			"use_webp":           {Type: schema.Integer, Desc: "使用Webp：1=启用,0=禁用"},
			"convert_gif":        {Type: schema.Integer, Desc: "转换GIF：1=启用,0=禁用"},
			"quality":            {Type: schema.Integer, Desc: "图片质量（1-100）"},
			"resize_image":       {Type: schema.Integer, Desc: "图片缩放：1=启用,0=禁用"},
			"resize_width":       {Type: schema.Integer, Desc: "缩放宽度，默认800"},
			"thumb_crop":         {Type: schema.Integer, Desc: "缩略图裁剪模式：0=按最长边等比缩放,1=按最长边补白,2 =按最短边裁剪"},
			"thumb_width":        {Type: schema.Integer, Desc: "缩略图宽度"},
			"thumb_height":       {Type: schema.Integer, Desc: "缩略图高度"},
			"default_thumb_type": {Type: schema.String, Desc: "默认缩略图来源：0=图片列表,3=图片库分类ID"},
			"default_thumbs":     {Type: schema.Array, Desc: "图片列表"},
			"thumb_category_id":  {Type: schema.Integer, Desc: "图片库分类ID"},
			"multi_category":     {Type: schema.Integer, Desc: "启用文档多分类：1=启用,0=禁用"},
			"use_sort":           {Type: schema.Integer, Desc: "启用文档排序：1=启用,0=禁用"},
			"editor":             {Type: schema.String, Desc: "编辑器类型：default 或 markdown"},
			"max_page":           {Type: schema.Integer, Desc: "最大显示页码"},
			"max_limit":          {Type: schema.Integer, Desc: "最大显示条数"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args config.ContentConfig
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w.Content.RemoteDownload = args.RemoteDownload
		w.Content.FilterOutlink = args.FilterOutlink
		w.Content.UrlTokenType = args.UrlTokenType
		w.Content.UseWebp = args.UseWebp
		w.Content.ConvertGif = args.ConvertGif
		w.Content.Quality = args.Quality
		w.Content.ResizeImage = args.ResizeImage
		w.Content.ResizeWidth = args.ResizeWidth
		w.Content.ThumbCrop = args.ThumbCrop
		w.Content.ThumbWidth = args.ThumbWidth
		w.Content.ThumbHeight = args.ThumbHeight
		w.Content.DefaultThumbType = args.DefaultThumbType
		w.Content.DefaultThumbs = args.DefaultThumbs
		w.Content.ThumbCategoryId = args.ThumbCategoryId
		w.Content.MultiCategory = args.MultiCategory
		w.Content.UseSort = args.UseSort
		if args.Editor != "" {
			w.Content.Editor = args.Editor
		}
		if args.MaxPage > 0 {
			w.Content.MaxPage = args.MaxPage
		}
		if args.MaxLimit > 0 {
			w.Content.MaxLimit = args.MaxLimit
		}
		if err := w.SaveSettingValue(ContentSettingKey, w.Content); err != nil {
			return "", fmt.Errorf("保存内容设置失败: %w", err)
		}
		w.DeleteCacheIndex()
		return "内容设置已更新", nil
	})
	add(&schema.ToolInfo{
		Name:        "setting_safe",
		Desc:        "查看内容安全设置，包括验证码、频率限制、内容过滤、IP黑名单等。",
		// ParamsOneOf left nil intentionally: no parameters needed
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		s := w.Safe
		return fmt.Sprintf(`安全设置：
━━━━━━━━━━━━━━━━━━━━━
验证码: %d
同IP每日提交限制: %d
提交留言内容至少字数: %d
间隔限制: %d
禁止的内容关键词: %s
IP黑名单: %s
UA黑名单: %s
API开放: %d
API发布: %d
后台验证码: %d`,
			s.Captcha, s.DailyLimit, s.ContentLimit, s.IntervalLimit,
			s.ContentForbidden, s.IPForbidden, s.UAForbidden,
			s.APIOpen, s.APIPublish, s.AdminCaptchaOff), nil
	})
	add(&schema.ToolInfo{
		Name: "setting_safe_form",
		Desc: "更新内容安全设置。值为数字的字段必传，字符串字段为空则不更新。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"captcha":           {Type: schema.Integer, Desc: "验证码：1=开启,0=关闭"},
			"daily_limit":       {Type: schema.Integer, Desc: "同IP每日提交限制，0=不限制"},
			"content_limit":     {Type: schema.Integer, Desc: "提交留言内容至少字数，0=不限制"},
			"interval_limit":    {Type: schema.Integer, Desc: "提交留言间隔(秒)，0=不限制"},
			"content_forbidden": {Type: schema.String, Desc: "禁止的内容关键词，一行一个"},
			"ip_forbidden":      {Type: schema.String, Desc: "禁止的IP，一行一个"},
			"ua_forbidden":      {Type: schema.String, Desc: "禁止的User-Agent，一行一个"},
			"api_open":          {Type: schema.Integer, Desc: "API开放：1=开启,0=关闭"},
			"api_publish":       {Type: schema.Integer, Desc: "API发布：1=允许,0=禁止"},
			"admin_captcha_off": {Type: schema.Integer, Desc: "后台验证码：1=关闭,0=开启"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args config.SafeConfig
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		w.Safe.Captcha = args.Captcha
		w.Safe.DailyLimit = args.DailyLimit
		w.Safe.ContentLimit = args.ContentLimit
		w.Safe.IntervalLimit = args.IntervalLimit
		if args.ContentForbidden != "" {
			w.Safe.ContentForbidden = args.ContentForbidden
		}
		if args.IPForbidden != "" {
			w.Safe.IPForbidden = args.IPForbidden
		}
		if args.UAForbidden != "" {
			w.Safe.UAForbidden = args.UAForbidden
		}
		w.Safe.APIOpen = args.APIOpen
		w.Safe.APIPublish = args.APIPublish
		w.Safe.AdminCaptchaOff = args.AdminCaptchaOff
		if err := w.SaveSettingValue(SafeSettingKey, w.Safe); err != nil {
			return "", fmt.Errorf("保存安全设置失败: %w", err)
		}
		w.DeleteCacheIndex()
		return "安全设置已更新", nil
	})
	add(&schema.ToolInfo{
		Name:        "setting_migrate_db",
		Desc:        "更新数据库表结构，自动同步模型字段变更到数据库。",
		// ParamsOneOf left nil intentionally: no parameters needed
	}, func(ctx context.Context, argsJSON string) (string, error) {
		if svc.site == nil {
			return "错误：站点未初始化", nil
		}
		if err := AutoMigrateDB(svc.site.DB, true); err != nil {
			return "", fmt.Errorf("更新数据库表结构失败: %w", err)
		}
		return "数据库表结构已更新", nil
	})

	// ---- Sitemap & Push tools ----
	add(&schema.ToolInfo{
		Name:        "sitemap_rebuild",
		Desc:        "重新生成站点地图(sitemap)。生成后搜索引擎能更快发现新内容。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		if err := w.BuildSitemap(); err != nil {
			return fmt.Sprintf("生成 sitemap 失败: %s", err.Error()), nil
		}
		return "站点地图 sitemap 已重新生成", nil
	})
	add(&schema.ToolInfo{
		Name: "url_push",
		Desc: "推送指定的 URL 到百度、必应等搜索引擎，加快收录。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"url": {Type: schema.String, Desc: "要推送的完整 URL", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			Url string `json:"url"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Url == "" {
			return "参数错误：必须提供 url", nil
		}
		w.PushArchive(args.Url)
		return fmt.Sprintf("URL 已推送到搜索引擎: %s", args.Url), nil
	})

	// ---- Comment tools ----
	add(&schema.ToolInfo{
		Name: "comment_list",
		Desc: "查看评论列表，支持按文档ID筛选和排序。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"archive_id": {Type: schema.Integer, Desc: "文档ID（可选），按文章筛选评论"},
			"page":       {Type: schema.Integer, Desc: "页码，从1开始，默认1"},
			"page_size":  {Type: schema.Integer, Desc: "每页条数，默认20"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			ArchiveId int64 `json:"archive_id"`
			Page      int   `json:"page"`
			PageSize  int   `json:"page_size"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		if args.Page < 1 {
			args.Page = 1
		}
		if args.PageSize < 1 || args.PageSize > 100 {
			args.PageSize = 20
		}
		comments, total, err := w.GetCommentList(args.ArchiveId, 0, "", args.Page, args.PageSize, 0)
		if err != nil {
			return fmt.Sprintf("获取评论失败: %s", err.Error()), nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共 %d 条评论（当前页 %d 条）：\n\n", total, len(comments)))
		for _, c := range comments {
			status := "待审核"
			if c.Status == 1 {
				status = "正常"
			} else if c.Status == 2 {
				status = "疑似垃圾"
			} else if c.Status == 3 {
				status = "垃圾"
			}
			parentInfo := ""
			if c.Parent != nil {
				parentInfo = fmt.Sprintf(" [回复: %s]", c.Parent.UserName)
			}
			b.WriteString(fmt.Sprintf("#%d%s | %s | %s\n", c.Id, parentInfo, c.UserName, status))
			b.WriteString(fmt.Sprintf("  内容: %s\n", c.Content))
			if c.ItemTitle != "" {
				b.WriteString(fmt.Sprintf("  文章: %s\n", c.ItemTitle))
			}
			b.WriteString("\n")
		}
		return b.String(), nil
	})
	add(&schema.ToolInfo{
		Name: "comment_approve",
		Desc: "审核通过评论或回复评论。设置 status=1 为审核通过。也可以回复指定评论。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":      {Type: schema.Integer, Desc: "评论ID", Required: true},
			"status":  {Type: schema.Integer, Desc: "状态：1=通过, 2=疑似垃圾, 3=垃圾"},
			"content": {Type: schema.String, Desc: "回复内容（可选），填写则作为管理员回复"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			Id      uint   `json:"id"`
			Status  uint   `json:"status"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Id == 0 {
			return "参数错误：必须提供评论 id", nil
		}
		req := &request.PluginComment{
			Id:     args.Id,
			Status: args.Status,
		}
		if args.Status == 0 {
			req.Status = 1 // 默认通过
		}
		if args.Content != "" {
			// 管理员回复 - 先获取原评论获取 archive_id
			orig, err := w.GetCommentById(args.Id)
			if err != nil {
				return fmt.Sprintf("获取原评论失败: %s", err.Error()), nil
			}
			req.Content = args.Content
			req.ArchiveId = orig.ArchiveId
			req.UserName = "管理员"
		}
		if _, err := w.SaveComment(req); err != nil {
			return fmt.Sprintf("操作失败: %s", err.Error()), nil
		}
		if args.Content != "" {
			return fmt.Sprintf("评论 #%d 已回复", args.Id), nil
		}
		return fmt.Sprintf("评论 #%d 状态已更新为 %d", args.Id, req.Status), nil
	})
	add(&schema.ToolInfo{
		Name: "comment_delete",
		Desc: "删除评论。注意：此操作不可恢复。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "评论ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Id == 0 {
			return "参数错误：必须提供评论 id", nil
		}
		comment, err := w.GetCommentById(uint(args.Id))
		if err != nil {
			return fmt.Sprintf("评论不存在: %s", err.Error()), nil
		}
		if err := comment.Delete(w.DB); err != nil {
			return fmt.Sprintf("删除失败: %s", err.Error()), nil
		}
		return fmt.Sprintf("评论 #%d 已删除", args.Id), nil
	})

	// ---- Keyword tools ----
	add(&schema.ToolInfo{
		Name: "keyword_list",
		Desc: "获取关键词库列表，支持关键词搜索。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"keyword":   {Type: schema.String, Desc: "搜索关键词（可选），按标题模糊搜索"},
			"page":      {Type: schema.Integer, Desc: "页码，从1开始，默认1"},
			"page_size": {Type: schema.Integer, Desc: "每页条数，默认20"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			Keyword  string `json:"keyword"`
			Page     int    `json:"page"`
			PageSize int    `json:"page_size"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		if args.Page < 1 {
			args.Page = 1
		}
		if args.PageSize < 1 || args.PageSize > 100 {
			args.PageSize = 20
		}
		keywords, total, err := w.GetKeywordList(args.Keyword, args.Page, args.PageSize)
		if err != nil {
			return fmt.Sprintf("获取关键词列表失败: %s", err.Error()), nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共 %d 个关键词（当前页 %d 个）：\n\n", total, len(keywords)))
		for _, k := range keywords {
			b.WriteString(fmt.Sprintf("#%d | %s (分类ID:%d, 文章数:%d)\n", k.Id, k.Title, k.CategoryId, k.ArticleCount))
		}
		return b.String(), nil
	})
	add(&schema.ToolInfo{
		Name: "keyword_create",
		Desc: "添加关键词到词库。如果关键词已存在则更新其分类。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"title":       {Type: schema.String, Desc: "关键词", Required: true},
			"category_id": {Type: schema.Integer, Desc: "分类ID（可选）"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			Title      string `json:"title"`
			CategoryId uint   `json:"category_id"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Title == "" {
			return "参数错误：必须提供 title", nil
		}
		keyword, err := w.GetKeywordByTitle(args.Title)
		if err != nil {
			keyword = &model.Keyword{
				Title:      args.Title,
				Status:     1,
				CategoryId: args.CategoryId,
			}
		} else if args.CategoryId > 0 {
			keyword.CategoryId = args.CategoryId
		}
		if err := keyword.Save(w.DB); err != nil {
			return fmt.Sprintf("保存关键词失败: %s", err.Error()), nil
		}
		return fmt.Sprintf("关键词「%s」已保存（ID: %d）", keyword.Title, keyword.Id), nil
	})
	add(&schema.ToolInfo{
		Name: "keyword_delete",
		Desc: "删除关键词。可以通过ID或标题删除。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":    {Type: schema.Integer, Desc: "关键词ID（与title二选一）"},
			"title": {Type: schema.String, Desc: "关键词标题（与id二选一）"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			Id    uint   `json:"id"`
			Title string `json:"title"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || (args.Id == 0 && args.Title == "") {
			return "参数错误：必须提供 id 或 title", nil
		}
		var keyword *model.Keyword
		var err error
		if args.Id > 0 {
			keyword, err = w.GetKeywordById(args.Id)
		} else {
			keyword, err = w.GetKeywordByTitle(args.Title)
		}
		if err != nil {
			return fmt.Sprintf("未找到关键词: %s", err.Error()), nil
		}
		if err := keyword.Delete(w.DB); err != nil {
			return fmt.Sprintf("删除失败: %s", err.Error()), nil
		}
		return fmt.Sprintf("关键词「%s」已删除", keyword.Title), nil
	})

	// ---- Anchor tool ----
	add(&schema.ToolInfo{
		Name: "anchor_list",
		Desc: "获取锚文本链接列表，支持按关键词搜索。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"keyword":   {Type: schema.String, Desc: "搜索关键词（可选），按锚文本或链接模糊搜索"},
			"page":      {Type: schema.Integer, Desc: "页码，从1开始，默认1"},
			"page_size": {Type: schema.Integer, Desc: "每页条数，默认20"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			Keyword  string `json:"keyword"`
			Page     int    `json:"page"`
			PageSize int    `json:"page_size"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		if args.Page < 1 {
			args.Page = 1
		}
		if args.PageSize < 1 || args.PageSize > 100 {
			args.PageSize = 20
		}
		anchors, total, err := w.GetAnchorList(args.Keyword, args.Page, args.PageSize)
		if err != nil {
			return fmt.Sprintf("获取锚文本列表失败: %s", err.Error()), nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共 %d 个锚文本（当前页 %d 个）：\n\n", total, len(anchors)))
		for _, a := range anchors {
			b.WriteString(fmt.Sprintf("#%d | %s → %s (权重:%d)\n", a.Id, a.Title, a.Link, a.Weight))
		}
		return b.String(), nil
	})
	add(&schema.ToolInfo{
		Name: "anchor_create",
		Desc: "添加或更新锚文本链接。如果指定ID则更新，否则创建。标题和链接必填。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":     {Type: schema.Integer, Desc: "锚文本ID，留空表示创建"},
			"title":  {Type: schema.String, Desc: "锚文本关键词", Required: true},
			"link":   {Type: schema.String, Desc: "锚文本链接地址", Required: true},
			"weight": {Type: schema.Integer, Desc: "权重值，越大越优先替换"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args request.PluginAnchor
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Title == "" || args.Link == "" {
			return "参数错误：必须提供 title 和 link", nil
		}
		frontUrl := w.System.BaseUrl
		if w.System.FrontUrl != "" {
			frontUrl = w.System.FrontUrl
		}
		args.Link = strings.TrimPrefix(args.Link, frontUrl)

		var anchor *model.Anchor
		var err error
		if args.Id > 0 {
			anchor, err = w.GetAnchorById(args.Id)
			if err != nil {
				return "", fmt.Errorf("获取锚文本失败: %w", err)
			}
		} else {
			anchor, _ = w.GetAnchorByTitle(args.Title)
			if anchor == nil {
				anchor = &model.Anchor{Status: 1}
			}
		}
		anchor.Title = args.Title
		anchor.Link = args.Link
		anchor.Weight = args.Weight
		if err := anchor.Save(w.DB); err != nil {
			return "", fmt.Errorf("保存锚文本失败: %w", err)
		}
		return fmt.Sprintf("锚文本 [%d] %s -> %s 已保存", anchor.Id, anchor.Title, anchor.Link), nil
	})
	add(&schema.ToolInfo{
		Name: "anchor_delete",
		Desc: "删除锚文本链接。可以通过ID或ID列表删除。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":  {Type: schema.Integer, Desc: "锚文本ID"},
			"ids": {Type: schema.Array, Desc: "锚文本ID列表，JSON数组如 [1,2,3]"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args request.PluginAnchorDelete
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || (args.Id == 0 && len(args.Ids) == 0) {
			return "参数错误：必须提供 id 或 ids", nil
		}
		if args.Id > 0 {
			anchor, err := w.GetAnchorById(args.Id)
			if err != nil {
				return "", fmt.Errorf("获取锚文本失败: %w", err)
			}
			if err := w.DeleteAnchor(anchor); err != nil {
				return "", fmt.Errorf("删除锚文本失败: %w", err)
			}
		} else if len(args.Ids) > 0 {
			for _, id := range args.Ids {
				anchor, err := w.GetAnchorById(id)
				if err != nil {
					continue
				}
				_ = w.DeleteAnchor(anchor)
			}
		}
		return "锚文本已删除", nil
	})

	// ---- Redirect tools ----
	add(&schema.ToolInfo{
		Name: "redirect_list",
		Desc: "获取URL重定向列表，支持按来源URL搜索。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"keyword":   {Type: schema.String, Desc: "搜索关键词（可选），按from_url模糊搜索"},
			"page":      {Type: schema.Integer, Desc: "页码，从1开始，默认1"},
			"page_size": {Type: schema.Integer, Desc: "每页条数，默认20"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			Keyword  string `json:"keyword"`
			Page     int    `json:"page"`
			PageSize int    `json:"page_size"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		if args.Page < 1 {
			args.Page = 1
		}
		if args.PageSize < 1 || args.PageSize > 100 {
			args.PageSize = 20
		}
		redirects, total, err := w.GetRedirectList(args.Keyword, args.Page, args.PageSize)
		if err != nil {
			return fmt.Sprintf("获取重定向列表失败: %s", err.Error()), nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共 %d 条重定向规则（当前页 %d 条）：\n\n", total, len(redirects)))
		for _, r := range redirects {
			b.WriteString(fmt.Sprintf("#%d | %s → %s\n", r.Id, r.FromUrl, r.ToUrl))
		}
		return b.String(), nil
	})
	add(&schema.ToolInfo{
		Name: "redirect_create",
		Desc: "添加或更新URL重定向规则。如果来源URL已存在则更新目标URL，否则创建新规则。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"from_url": {Type: schema.String, Desc: "来源URL（被重定向的地址），支持相对路径如 /old-page.html 或完整URL", Required: true},
			"to_url":   {Type: schema.String, Desc: "目标URL（重定向到的地址）", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args request.PluginRedirectRequest
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.FromUrl == "" || args.ToUrl == "" {
			return "参数错误：必须提供 from_url 和 to_url", nil
		}
		// 标准化路径
		if !strings.HasPrefix(args.FromUrl, "http") && !strings.HasPrefix(args.FromUrl, "/") {
			args.FromUrl = "/" + args.FromUrl
		}
		if !strings.HasPrefix(args.ToUrl, "http") && !strings.HasPrefix(args.ToUrl, "/") {
			args.ToUrl = "/" + args.ToUrl
		}
		if args.FromUrl == args.ToUrl {
			return "错误：来源URL和目标URL不能相同", nil
		}
		redirect, err := w.GetRedirectByFromUrl(args.FromUrl)
		if err != nil {
			redirect = &model.Redirect{
				FromUrl: args.FromUrl,
			}
		}
		redirect.ToUrl = args.ToUrl
		if err := w.DB.Save(redirect).Error; err != nil {
			return fmt.Sprintf("保存重定向失败: %s", err.Error()), nil
		}
		w.DeleteCacheRedirects()
		return fmt.Sprintf("重定向规则已保存：%s → %s（ID: %d）", redirect.FromUrl, redirect.ToUrl, redirect.Id), nil
	})

	// ---- Statistics tools (P3) ----
	add(&schema.ToolInfo{
		Name: "statistic_dashboard",
		Desc: "查看站点统计概览，包括文章数、分类数、链接数、留言数、流量和爬虫统计等。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"exact": {Type: schema.Boolean, Desc: "是否强制刷新缓存并取精确值（可选，默认false使用缓存）"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			Exact bool `json:"exact"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		stat := w.GetStatisticsSummary(args.Exact)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("📊 站点统计概览（精确模式: %v）\n\n", stat.Exact))
		b.WriteString(fmt.Sprintf("📄 文章总数: %d\n", stat.ArchiveCount.Total))
		b.WriteString(fmt.Sprintf("   ├ 上周新增: %d\n", stat.ArchiveCount.LastWeek))
		b.WriteString(fmt.Sprintf("   ├ 今日发布: %d\n", stat.ArchiveCount.Today))
		b.WriteString(fmt.Sprintf("   ├ 草稿: %d\n", stat.ArchiveCount.Draft))
		b.WriteString(fmt.Sprintf("   └ 待发布: %d\n", stat.ArchiveCount.UnRelease))
		if len(stat.ModuleCounts) > 0 {
			b.WriteString(fmt.Sprintf("\n📂 各模型文章数:\n"))
			for _, m := range stat.ModuleCounts {
				b.WriteString(fmt.Sprintf("   ├ %s: %d\n", m.Name, m.Total))
			}
		}
		b.WriteString(fmt.Sprintf("\n🏷️ 分类数: %d\n", stat.CategoryCount))
		b.WriteString(fmt.Sprintf("🔗 友链数: %d\n", stat.LinkCount))
		b.WriteString(fmt.Sprintf("💬 留言数: %d\n", stat.GuestbookCount))
		b.WriteString(fmt.Sprintf("📋 页面数: %d\n", stat.PageCount))
		b.WriteString(fmt.Sprintf("🖼️ 附件数: %d\n", stat.AttachmentCount))
		b.WriteString(fmt.Sprintf("🧩 模板数: %d\n", stat.TemplateCount))
		b.WriteString(fmt.Sprintf("\n🌐 流量 (今日/总计): %d / %d\n", stat.TrafficCount.Today, stat.TrafficCount.Total))
		b.WriteString(fmt.Sprintf("🕷️ 爬虫 (今日/总计): %d / %d\n", stat.SpiderCount.Today, stat.SpiderCount.Total))
		if stat.IncludeCount.BaiduCount > 0 || stat.IncludeCount.BingCount > 0 || stat.IncludeCount.SogouCount > 0 {
			b.WriteString(fmt.Sprintf("\n🔍 搜索引擎收录:\n"))
			if stat.IncludeCount.BaiduCount > 0 {
				b.WriteString(fmt.Sprintf("   ├ 百度: %d\n", stat.IncludeCount.BaiduCount))
			}
			if stat.IncludeCount.BingCount > 0 {
				b.WriteString(fmt.Sprintf("   ├ 必应: %d\n", stat.IncludeCount.BingCount))
			}
			if stat.IncludeCount.SogouCount > 0 {
				b.WriteString(fmt.Sprintf("   └ 搜狗: %d\n", stat.IncludeCount.SogouCount))
			}
		}
		return b.String(), nil
	})
	add(&schema.ToolInfo{
		Name:        "statistic_spider",
		Desc:        "查看近30天的爬虫访问趋势数据（按天汇总各爬虫访问次数）。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		data := w.StatisticSpider()
		if len(data) == 0 {
			return "暂无可用的爬虫统计数据", nil
		}
		// 按日期汇总
		agg := make(map[string]map[string]int)
		for _, d := range data {
			if agg[d.Date] == nil {
				agg[d.Date] = make(map[string]int)
			}
			agg[d.Date][d.Label] += d.Value
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("🕷️ 近30天爬虫统计（共 %d 条记录）\n\n", len(data)))
		// 表头
		var dates []string
		for date := range agg {
			dates = append(dates, date)
		}
		sort.Strings(dates)
		for _, date := range dates {
			b.WriteString(fmt.Sprintf("📅 %s\n", date))
			for spider, count := range agg[date] {
				b.WriteString(fmt.Sprintf("   ├ %s: %d\n", spider, count))
			}
		}
		return b.String(), nil
	})
	add(&schema.ToolInfo{
		Name:        "statistic_traffic",
		Desc:        "查看近30天的网站流量趋势数据（PV/IP按天汇总）。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		data := w.StatisticTraffic()
		if len(data) == 0 {
			return "暂无可用的流量统计数据", nil
		}
		// 按日期 + 标签汇总
		type dayStat struct {
			PV int
			IP int
		}
		agg := make(map[string]*dayStat)
		for _, d := range data {
			if agg[d.Date] == nil {
				agg[d.Date] = &dayStat{}
			}
			if d.Label == "PV" {
				agg[d.Date].PV += d.Value
			} else if d.Label == "IP" {
				agg[d.Date].IP += d.Value
			}
		}
		var dates []string
		for date := range agg {
			dates = append(dates, date)
		}
		sort.Strings(dates)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("🌐 近30天流量统计\n\n"))
		b.WriteString(fmt.Sprintf("%-12s %8s %8s\n", "日期", "PV", "IP"))
		b.WriteString(fmt.Sprintf("%-12s %8s %8s\n", "────", "──", "──"))
		for _, date := range dates {
			b.WriteString(fmt.Sprintf("%-12s %8d %8d\n", date, agg[date].PV, agg[date].IP))
		}
		return b.String(), nil
	})

	// ---- Plugin tools (P4) ----
	add(&schema.ToolInfo{
		Name:        "plugin_robots_get",
		Desc:        "查看当前的 robots.txt 内容。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		robots := w.GetRobots()
		if robots == "" {
			return "robots.txt 文件不存在或为空", nil
		}
		return fmt.Sprintf("当前 robots.txt 内容：\n```\n%s\n```", robots), nil
	})
	add(&schema.ToolInfo{
		Name: "plugin_robots_set",
		Desc: "修改 robots.txt 内容。注意：此操作会覆盖现有文件。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"content": {Type: schema.String, Desc: "完整的 robots.txt 文本内容", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Content == "" {
			return "参数错误：必须提供 content", nil
		}
		if err := w.SaveRobots(args.Content); err != nil {
			return fmt.Sprintf("保存 robots.txt 失败: %s", err.Error()), nil
		}
		return "robots.txt 已更新", nil
	})
	add(&schema.ToolInfo{
		Name:        "rewrite_get",
		Desc:        "查看当前的 URL 重写（伪静态）配置。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		if w.PluginRewrite == nil {
			return "URL 重写配置为空", nil
		}
		modeStr := "未知"
		switch w.PluginRewrite.Mode {
		case config.RewriteNumberMode:
			modeStr = "数字模式"
		case config.RewriteStringMode1:
			modeStr = "命名模式1"
		case config.RewriteStringMode2:
			modeStr = "命名模式2"
		case config.RewriteStringMode3:
			modeStr = "命名模式3"
		case config.RewritePatternMode:
			modeStr = "正则模式"
		}
		return fmt.Sprintf("URL 重写配置：\n模式: %s (%d)\n规则: %s", modeStr, w.PluginRewrite.Mode, w.PluginRewrite.Patten), nil
	})
	add(&schema.ToolInfo{
		Name: "rewrite_form",
		Desc: "更新 URL 重写（伪静态）配置。可以修改模式和匹配规则。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"mode":   {Type: schema.Integer, Desc: "模式：0=数字模式,1=命名模式1,2=命名模式2,3=命名模式3,4=正则模式", Required: true},
			"patten": {Type: schema.String, Desc: "URL匹配规则", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args config.PluginRewriteConfig
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if w.PluginRewrite.Mode != args.Mode || w.PluginRewrite.Patten != args.Patten {
			w.PluginRewrite.Mode = args.Mode
			w.PluginRewrite.Patten = args.Patten
			if err := w.SaveSettingValue(RewriteSettingKey, w.PluginRewrite); err != nil {
				return "", fmt.Errorf("保存重写配置失败: %w", err)
			}
			w.ParsePattern(true)
			w.RemoveHtmlCache()
		}
		return fmt.Sprintf("URL 重写配置已更新：模式=%d, 规则=%s", args.Mode, args.Patten), nil
	})
	add(&schema.ToolInfo{
		Name:        "plugin_htmlcache_build",
		Desc:        "生成静态 HTML 缓存。此操作会在后台异步执行，完成后能大幅提升页面访问速度。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		if w.PluginHtmlCache.Open == false {
			return "HTML 缓存功能未开启，请先在后台配置开启", nil
		}
		go func() {
			w.BuildIndexCache()
			w.BuildArchiveCache()
		}()
		return "HTML 缓存生成任务已提交（首页+文章详情页），请在后台查看进度", nil
	})
	add(&schema.ToolInfo{
		Name:        "plugin_fulltext_rebuild",
		Desc:        "重建全文搜索索引。此操作会关闭现有索引并重新构建。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		if w.PluginFulltext == nil || !w.PluginFulltext.Open {
			return "全文搜索功能未开启，请先在后台配置开启", nil
		}
		w.CloseFulltext()
		go w.InitFulltext(true)
		return "全文搜索索引重建任务已提交，后台异步执行中", nil
	})
	add(&schema.ToolInfo{
		Name:        "plugin_backup_dump",
		Desc:        "备份数据库。此操作会导出完整的数据库结构和数据到备份文件，后台异步执行。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		bs, err := w.NewBackup()
		if err != nil {
			return fmt.Sprintf("创建备份失败: %s", err.Error()), nil
		}
		go bs.BackupData()
		return "数据库备份任务已提交，后台异步执行中", nil
	})
	add(&schema.ToolInfo{
		Name: "user_list",
		Desc: "查看用户列表。支持分页查询。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"page":      {Type: schema.Integer, Desc: "页码，从1开始，默认1"},
			"page_size": {Type: schema.Integer, Desc: "每页条数，默认20"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			Page     int `json:"page"`
			PageSize int `json:"page_size"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		if args.Page < 1 {
			args.Page = 1
		}
		if args.PageSize < 1 || args.PageSize > 100 {
			args.PageSize = 20
		}
		users, total := w.GetUserList(nil, args.Page, args.PageSize)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共 %d 个用户（当前页 %d 个）：\n\n", total, len(users)))
		for _, u := range users {
			status := "正常"
			if u.Status == 0 {
				status = "待审核"
			} else if u.Status == -1 {
				status = "禁用"
			}
			group := ""
			if u.Group != nil {
				group = u.Group.Title
			}
			b.WriteString(fmt.Sprintf("#%d | %s (%s) | %s | 余额:%d\n", u.Id, u.UserName, group, status, u.Balance))
		}
		return b.String(), nil
	})
	add(&schema.ToolInfo{
		Name: "user_get",
		Desc: "获取单个用户的详细信息。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "用户ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		users, _ := w.GetUserList(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("id = ?", args.Id)
		}, 1, 1)
		if len(users) == 0 {
			return "未找到该用户", nil
		}
		u := users[0]
		status := "正常"
		if u.Status == 0 {
			status = "待审核"
		} else if u.Status == -1 {
			status = "禁用"
		}
		group := ""
		if u.Group != nil {
			group = u.Group.Title
		}
		return fmt.Sprintf("用户信息：\nID: %d\n用户名: %s\n分组: %s\n状态: %s\n余额: %d\n手机: %s\n邮箱: %s",
			u.Id, u.UserName, group, status, u.Balance, u.Phone, u.Email), nil
	})
	add(&schema.ToolInfo{
		Name: "user_create",
		Desc: "添加新用户。必填字段：user_name。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"user_name": {Type: schema.String, Desc: "用户名", Required: true},
			"password":  {Type: schema.String, Desc: "密码"},
			"phone":     {Type: schema.String, Desc: "手机号"},
			"email":     {Type: schema.String, Desc: "邮箱"},
			"group_id":  {Type: schema.Integer, Desc: "用户组ID"},
			"status":    {Type: schema.Integer, Desc: "状态：0=待审核,1=正常,-1=禁用"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			UserName string `json:"user_name"`
			Password string `json:"password"`
			Phone    string `json:"phone"`
			Email    string `json:"email"`
			GroupId  uint   `json:"group_id"`
			Status   int    `json:"status"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.UserName == "" {
			return "参数错误：必须提供 user_name", nil
		}
		req := request.UserRequest{
			UserName: args.UserName,
			Password: args.Password,
			Phone:    args.Phone,
			Email:    args.Email,
			GroupId:  args.GroupId,
			Status:   args.Status,
		}
		user, err := w.SaveUserInfo(&req)
		if err != nil {
			return "", fmt.Errorf("创建用户失败: %w", err)
		}
		return fmt.Sprintf("用户创建成功！ID: %d, 用户名: %s", user.Id, user.UserName), nil
	})
	add(&schema.ToolInfo{
		Name: "user_update",
		Desc: "更新用户信息。需要传入用户ID，至少传入一个要更新的字段。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":        {Type: schema.Integer, Desc: "用户ID", Required: true},
			"user_name": {Type: schema.String, Desc: "用户名"},
			"password":  {Type: schema.String, Desc: "密码"},
			"phone":     {Type: schema.String, Desc: "手机号"},
			"email":     {Type: schema.String, Desc: "邮箱"},
			"group_id":  {Type: schema.Integer, Desc: "用户组ID"},
			"status":    {Type: schema.Integer, Desc: "状态：0=待审核,1=正常,-1=禁用"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			Id       uint   `json:"id"`
			UserName string `json:"user_name"`
			Password string `json:"password"`
			Phone    string `json:"phone"`
			Email    string `json:"email"`
			GroupId  uint   `json:"group_id"`
			Status   int    `json:"status"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Id == 0 {
			return "参数错误：必须提供 id", nil
		}
		// 获取已有用户
		users, _ := w.GetUserList(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("id = ?", args.Id)
		}, 1, 1)
		if len(users) == 0 {
			return "未找到该用户", nil
		}
		req := request.UserRequest{
			Id:       args.Id,
			UserName: args.UserName,
			Password: args.Password,
			Phone:    args.Phone,
			Email:    args.Email,
			GroupId:  args.GroupId,
			Status:   args.Status,
		}
		user, err := w.SaveUserInfo(&req)
		if err != nil {
			return "", fmt.Errorf("更新用户失败: %w", err)
		}
		return fmt.Sprintf("用户 [%d] %s 已更新", user.Id, user.UserName), nil
	})
	add(&schema.ToolInfo{
		Name: "user_delete",
		Desc: "删除用户。需要传入用户ID。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "用户ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			Id uint `json:"id"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Id == 0 {
			return "参数错误：必须提供 id", nil
		}
		if err := w.DeleteUserInfo(args.Id); err != nil {
			return "", fmt.Errorf("删除用户失败: %w", err)
		}
		return fmt.Sprintf("用户 #%d 已删除", args.Id), nil
	})
	add(&schema.ToolInfo{
		Name: "order_list",
		Desc: "查看订单列表。支持按用户ID、订单号、用户名、状态筛选。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"user_id":   {Type: schema.Integer, Desc: "用户ID（可选）"},
			"order_id":  {Type: schema.String, Desc: "订单号（可选）"},
			"user_name": {Type: schema.String, Desc: "用户名（可选），模糊搜索"},
			"status":    {Type: schema.String, Desc: "订单状态：waiting(待支付), paid(已支付), delivery(发货中), finished(已完成), closed(已关闭)"},
			"page":      {Type: schema.Integer, Desc: "页码，从1开始，默认1"},
			"page_size": {Type: schema.Integer, Desc: "每页条数，默认20"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		var args struct {
			UserId   uint   `json:"user_id"`
			OrderId  string `json:"order_id"`
			UserName string `json:"user_name"`
			Status   string `json:"status"`
			Page     int    `json:"page"`
			PageSize int    `json:"page_size"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		if args.Page < 1 {
			args.Page = 1
		}
		if args.PageSize < 1 || args.PageSize > 100 {
			args.PageSize = 20
		}
		orders, total := w.GetOrderList(args.UserId, args.OrderId, args.UserName, args.Status, args.Page, args.PageSize)
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共 %d 个订单（当前页 %d 个）：\n\n", total, len(orders)))
		for _, o := range orders {
			statusStr := "待支付"
			switch o.Status {
			case 0:
				statusStr = "待支付"
			case 1:
				statusStr = "已支付"
			case 2:
				statusStr = "发货中"
			case 3:
				statusStr = "已完成"
			case -1:
				statusStr = "已关闭"
			}
			b.WriteString(fmt.Sprintf("#%d | %s | %s\n", o.Id, o.OrderId, statusStr))
			b.WriteString(fmt.Sprintf("  金额: %.2f | 用户ID: %d | 时间: %s\n",
				float64(o.Amount)/100, o.UserId,
				time.Unix(o.CreatedTime, 0).Format("2006-01-02 15:04")))
		}
		return b.String(), nil
	})

	// ---- Agent 管理工具 ----
	add(&schema.ToolInfo{
		Name: "agent_create",
		Desc: "创建一个 AI 智能体（Agent），拥有独立的会话和记忆，可定时执行任务。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"name":     {Type: schema.String, Desc: "智能体名称，如 '每日热词写作'", Required: true},
			"strategy": {Type: schema.String, Desc: "执行策略。描述每次执行时需要做什么，包括步骤、标准、输出要求。如 '每天搜索互联网热词，据此写3篇文章并发布'", Required: true},
			"cron":     {Type: schema.String, Desc: "Cron 表达式，如 '0 8 * * *' 表示每天8点执行。留空表示仅手动触发"},
			"max_runs": {Type: schema.Integer, Desc: "最大执行次数，0=不限（默认0）"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Name     string `json:"name"`
			Strategy string `json:"strategy"`
			Cron     string `json:"cron"`
			MaxRuns  int    `json:"max_runs"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Name == "" || args.Strategy == "" {
			return "错误：名称和策略不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}

		agent := &model.AiAgent{
			Name:      args.Name,
			Strategy:  args.Strategy,
			CronExpr:  args.Cron,
			MaxRuns:   args.MaxRuns,
			Enabled:   1,
			SessionId: "", // 创建后分配
		}
		if err := w.DB.Create(agent).Error; err != nil {
			return "", fmt.Errorf("创建智能体失败: %w", err)
		}
		// 分配专属会话 ID
		agent.SessionId = fmt.Sprintf("agent_%d", agent.Id)
		w.DB.Model(agent).Update("session_id", agent.SessionId)

		// 如果有 cron 表达式，计算首次执行时间
		if agent.CronExpr != "" {
			scheduler, err := cron.ParseStandard(agent.CronExpr)
			if err == nil {
				agent.NextRunAt = scheduler.Next(time.Now()).Unix()
				w.DB.Model(agent).Update("next_run_at", agent.NextRunAt)
			}
		}

		// 加入内存缓存
		svc.agentsMu.Lock()
		svc.agents[agent.Id] = agent
		svc.agentsMu.Unlock()

		return fmt.Sprintf("智能体创建成功！ID: %d\n名称: %s\n策略: %s\nCron: %s\n会话ID: %s",
			agent.Id, agent.Name, agent.Strategy, agent.CronExpr, agent.SessionId), nil
	})
	add(&schema.ToolInfo{
		Name:        "agent_list",
		Desc:        "查看所有 AI 智能体列表，包含状态、上次运行时间、运行次数。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		var agents []model.AiAgent
		w.DB.Order("id ASC").Find(&agents)
		if len(agents) == 0 {
			return "暂无智能体。使用 `agent_create` 创建一个。", nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("共 %d 个智能体：\n\n", len(agents)))
		for _, a := range agents {
			enabledStr := "✅ 运行中"
			if a.Enabled == 0 {
				enabledStr = "⏸ 已暂停"
			}
			lastRun := "从未"
			if a.LastRunAt > 0 {
				lastRun = time.Unix(a.LastRunAt, 0).Format("2006-01-02 15:04")
			}
			cronStr := a.CronExpr
			if cronStr == "" {
				cronStr = "仅手动"
			}
			b.WriteString(fmt.Sprintf("#%d | %s | %s\n", a.Id, a.Name, enabledStr))
			b.WriteString(fmt.Sprintf("  策略: %s\n", truncate(a.Strategy, 100)))
			b.WriteString(fmt.Sprintf("  Cron: %s | 上次: %s | 运行: %d次\n", cronStr, lastRun, a.RunCount))
		}
		return b.String(), nil
	})
	add(&schema.ToolInfo{
		Name: "agent_delete",
		Desc: "删除指定 AI 智能体及其所有执行日志。不可恢复。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "智能体ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Id <= 0 {
			return "错误：ID必须大于0", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		var agent model.AiAgent
		if err := w.DB.Where("id = ?", args.Id).First(&agent).Error; err != nil {
			return "错误：智能体不存在", nil
		}
		// 删除执行日志
		w.DB.Where("agent_id = ?", args.Id).Delete(&model.AiAgentLog{})
		// 删除 Agent
		w.DB.Delete(&agent)
		// 从内存缓存移除
		svc.agentsMu.Lock()
		delete(svc.agents, uint(args.Id))
		svc.agentsMu.Unlock()
		return fmt.Sprintf("智能体 #%d 已删除", args.Id), nil
	})
	add(&schema.ToolInfo{
		Name: "agent_toggle",
		Desc: "暂停或恢复一个 AI 智能体。暂停后不会定时执行。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":      {Type: schema.Integer, Desc: "智能体ID", Required: true},
			"enabled": {Type: schema.Integer, Desc: "1=启用(运行中)，0=暂停", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Id      int `json:"id"`
			Enabled int `json:"enabled"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Id <= 0 {
			return "错误：ID必须大于0", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		var agent model.AiAgent
		if err := w.DB.Where("id = ?", args.Id).First(&agent).Error; err != nil {
			return "错误：智能体不存在", nil
		}
		enabled := 0
		if args.Enabled == 1 {
			enabled = 1
		}
		w.DB.Model(&agent).Update("enabled", enabled)
		svc.agentsMu.Lock()
		if existing, ok := svc.agents[uint(args.Id)]; ok {
			existing.Enabled = enabled
		}
		svc.agentsMu.Unlock()
		status := "已暂停"
		if enabled == 1 {
			status = "运行中"
		}
		return fmt.Sprintf("智能体 #%d 状态已更改为: %s", args.Id, status), nil
	})
	add(&schema.ToolInfo{
		Name: "agent_run",
		Desc: "手动触发一个 AI 智能体立即执行一次任务。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "智能体ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Id <= 0 {
			return "错误：ID必须大于0", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		svc.agentsMu.RLock()
		agent, exists := svc.agents[uint(args.Id)]
		svc.agentsMu.RUnlock()
		if !exists {
			return "错误：智能体不存在或未加载", nil
		}
		result, err := svc.ExecuteAgent(agent)
		if err != nil {
			return fmt.Sprintf("智能体 #%d 执行失败: %s", args.Id, err.Error()), nil
		}
		return fmt.Sprintf("智能体 #%d 执行完成！\n\n执行结果:\n%s", args.Id, result), nil
	})
	add(&schema.ToolInfo{
		Name: "agent_chat",
		Desc: "与指定 AI 智能体的专属会话对话。可以查看它的执行历史、调整策略、询问进度等。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":      {Type: schema.Integer, Desc: "智能体ID", Required: true},
			"message": {Type: schema.String, Desc: "发送给智能体的消息", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Id      int    `json:"id"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Id <= 0 || args.Message == "" {
			return "错误：ID和消息不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		svc.agentsMu.RLock()
		agent, exists := svc.agents[uint(args.Id)]
		svc.agentsMu.RUnlock()
		if !exists {
			return "错误：智能体不存在或未加载", nil
		}

		// 获取 Eino client
		client, err := eino.GetClient()
		if err != nil {
			return "", fmt.Errorf("AI client not available: %w", err)
		}
		if len(svc.Tools) > 0 {
			if err := client.BindTools(svc.Tools); err != nil {
				return "", fmt.Errorf("failed to bind tools: %w", err)
			}
		}

		// 构建消息：Agent 的历史上下文 + 用户消息
		systemPrompt := `你是 AnQiCMS 的 AI 智能体。以下是你的策略和对话历史。
用户正在与你对话，请回答问题并可以执行工具。`

		if agent.Strategy != "" {
			systemPrompt += "\n\n## 你的策略\n" + agent.Strategy
		}
		if agent.LastSummary != "" {
			systemPrompt += "\n\n## 上次执行摘要\n" + agent.LastSummary
		}

		messages := svc.BuildToolMessages(agent.SessionId, systemPrompt)
		messages = append(messages, schema.UserMessage(args.Message))

		// 非流式调用，单轮回复
		msg, err := client.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("AI generate failed: %w", err)
		}

		// 保存到会话历史
		svc.AddMessage(agent.SessionId, ChatMessage{
			Role:    "user",
			Content: args.Message,
		})
		svc.AddMessage(agent.SessionId, ChatMessage{
			Role:    "assistant",
			Content: msg.Content,
		})

		return msg.Content, nil
	})
	add(skillListTool())
	add(skillGetTool())
	add(skillReloadTool())
	add(skillSaveTool())

	return tools, handlers
}

// ── 能力包系统 ──
// 能力包常量
const (
	CapabilityContent   = "content"   // 文档、分类、标签、附件、评论
	CapabilityStructure = "structure" // 页面、导航、友链、模块、重定向
	CapabilityDesign    = "design"    // 模板文件、静态文件、锚文本
	CapabilitySEO       = "seo"       // 关键词、站点地图、统计
	CapabilityAdmin     = "admin"     // 系统设置、插件、用户、订单
	CapabilityAgent     = "agent"     // Agent 管理
	CapabilityUniversal = "universal" // 始终需要的通用工具
)

// toolCapabilityMapping 工具名称 → 能力包列表
// 一个工具可属于多个包（如 category_list 同时用于内容管理和结构管理）
var toolCapabilityMapping = map[string][]string{
	// === content ===
	"archive_list":      {CapabilityContent},
	"archive_get":       {CapabilityContent},
	"archive_create":    {CapabilityContent},
	"archive_delete":    {CapabilityContent},
	"archive_publish":   {CapabilityContent},
	"archive_update":    {CapabilityContent},
	"archive_tag_update": {CapabilityContent},
	"category_list":     {CapabilityContent, CapabilityStructure},
	"category_get":      {CapabilityContent, CapabilityStructure},
	"category_create":   {CapabilityContent},
	"category_delete":   {CapabilityContent},
	"category_update":   {CapabilityContent},
	"tag_list":          {CapabilityContent},
	"tag_get":           {CapabilityContent},
	"tag_create":        {CapabilityContent},
	"tag_delete":        {CapabilityContent},
	"tag_update":        {CapabilityContent},
	"attachment_list":   {CapabilityContent},
	"attachment_get":    {CapabilityContent},
	"attachment_upload": {CapabilityContent},
	"attachment_delete": {CapabilityContent},
	"comment_list":      {CapabilityContent},
	"comment_approve":   {CapabilityContent},
	"comment_delete":    {CapabilityContent},

	// === structure ===
	"page_list":    {CapabilityStructure},
	"page_get":     {CapabilityStructure},
	"page_create":  {CapabilityStructure},
	"page_delete":  {CapabilityStructure},
	"page_update":  {CapabilityStructure},
	"module_list":  {CapabilityStructure},
	"module_get":   {CapabilityStructure},
	"module_create": {CapabilityStructure},
	"module_delete": {CapabilityStructure},
	"module_update": {CapabilityStructure},
	"nav_list":     {CapabilityStructure},
	"nav_create":   {CapabilityStructure},
	"nav_delete":   {CapabilityStructure},
	"nav_update":   {CapabilityStructure},
	"friendlink_list":  {CapabilityStructure},
	"friendlink_create": {CapabilityStructure},
	"friendlink_delete": {CapabilityStructure},
	"redirect_list":  {CapabilityStructure},
	"redirect_create": {CapabilityStructure},

	// === design ===
	"template_get_info":     {CapabilityDesign},
	"template_get_file":     {CapabilityDesign},
	"template_modify_file":  {CapabilityDesign},
	"template_get_static":   {CapabilityDesign},
	"template_modify_static": {CapabilityDesign},
	"template_reload":       {CapabilityDesign},
	"anchor_list":   {CapabilityDesign},
	"anchor_create": {CapabilityDesign},
	"anchor_delete": {CapabilityDesign},

	// === seo ===
	"keyword_list":   {CapabilitySEO},
	"keyword_create": {CapabilitySEO},
	"keyword_delete": {CapabilitySEO},
	"sitemap_rebuild": {CapabilitySEO},
	"url_push":       {CapabilitySEO},
	"statistic_dashboard": {CapabilitySEO},
	"statistic_spider":    {CapabilitySEO},
	"statistic_traffic":   {CapabilitySEO},

	// === admin ===
	"setting_system":      {CapabilityAdmin},
	"setting_system_form": {CapabilityAdmin},
	"setting_contact":      {CapabilityAdmin},
	"setting_contact_form": {CapabilityAdmin},
	"setting_diy_field":    {CapabilityAdmin},
	"setting_diy_field_form": {CapabilityAdmin},
	"setting_index":        {CapabilityAdmin},
	"setting_index_form":   {CapabilityAdmin},
	"setting_content":      {CapabilityAdmin},
	"setting_content_form": {CapabilityAdmin},
	"setting_safe":         {CapabilityAdmin},
	"setting_safe_form":    {CapabilityAdmin},
	"setting_migrate_db":   {CapabilityAdmin},
	"plugin_robots_get":    {CapabilityAdmin},
	"plugin_robots_set":    {CapabilityAdmin},
	"rewrite_get":          {CapabilityAdmin},
	"rewrite_form":         {CapabilityAdmin},
	"plugin_htmlcache_build":  {CapabilityAdmin},
	"plugin_fulltext_rebuild": {CapabilityAdmin},
	"plugin_backup_dump":      {CapabilityAdmin},
	"user_list":   {CapabilityAdmin},
	"user_get":    {CapabilityAdmin},
	"user_create": {CapabilityAdmin},
	"user_update": {CapabilityAdmin},
	"user_delete": {CapabilityAdmin},
	"order_list":  {CapabilityAdmin},

	// === agent ===
	"agent_create": {CapabilityAgent},
	"agent_list":   {CapabilityAgent},
	"agent_delete": {CapabilityAgent},
	"agent_toggle": {CapabilityAgent},
	"agent_run":    {CapabilityAgent},
	"agent_chat":   {CapabilityAgent},

	// === universal（信息查询、系统状态） ===
	"website_info": {CapabilityUniversal},
	"version":      {CapabilityUniversal},
	"anqi_info":    {CapabilityUniversal, CapabilityAdmin},

	// === skill 工具 ===
	"skill_list":   {CapabilityAdmin},
	"skill_get":    {CapabilityAdmin},
	"skill_reload": {CapabilityAdmin},
	"skill_save":   {CapabilityAdmin},

	// === 内置工具（文件、shell、web 等）=== universal
	"read_file":       {CapabilityUniversal},
	"write_file":      {CapabilityUniversal},
	"edit_file":       {CapabilityUniversal},
	"search_replace":  {CapabilityUniversal},
	"bash":            {CapabilityUniversal},
	"grep":            {CapabilityUniversal},
	"glob":            {CapabilityUniversal},
	"list_directory":  {CapabilityUniversal},
	"web_fetch":       {CapabilityUniversal},
	"web_search":      {CapabilityUniversal},
}

// getToolPackages returns the capability packages a tool belongs to.
// Falls back to [CapabilityAdmin] for unknown tools.
func getToolPackages(name string) []string {
	if pkgs, ok := toolCapabilityMapping[name]; ok {
		return pkgs
	}
	return []string{CapabilityAdmin}
}

// GetEinoToolsSummary returns a summary of all tools with minimal information
// (empty parameters) for the first-round capability declaration.
// Each tool's description includes its capability package tags like [content].
func (svc *AiChatService) GetEinoToolsSummary() []*schema.ToolInfo {
	summary := make([]*schema.ToolInfo, 0, len(svc.Tools))
	for _, ti := range svc.Tools {
		if ti.Name == "declare_capability_packages" {
			continue // 不包含自身
		}
		// Keep name & desc for the model to know the tool exists; clear params
		s := &schema.ToolInfo{
			Name: ti.Name,
			Desc: ti.Desc,
		}
		// Add package tags to description
		pkgs := getToolPackages(ti.Name)
		if len(pkgs) > 0 {
			s.Desc = ti.Desc + " [" + strings.Join(pkgs, ",") + "]"
		}
		summary = append(summary, s)
	}
	return summary
}

// getToolsByCapabilityPackages returns full tool definitions filtered by capability packages.
// Always includes tools that belong to CapabilityUniversal.
// This is used after the model declares its needed packages.
func (svc *AiChatService) GetToolsByCapabilityPackages(packages []string) ([]*schema.ToolInfo, map[string]toolHandler) {
	// Build a set of requested packages (always include universal)
	pkgSet := map[string]bool{CapabilityUniversal: true}
	for _, p := range packages {
		pkgSet[p] = true
	}

	allTools, allHandlers := svc.getEinoTools()

	filteredTools := make([]*schema.ToolInfo, 0)
	filteredHandlers := make(map[string]toolHandler)

	for _, ti := range allTools {
		pkgs := getToolPackages(ti.Name)
		// Check if any of the tool's packages are in the requested set
		for _, p := range pkgs {
			if pkgSet[p] {
				filteredTools = append(filteredTools, ti)
				if fn, ok := allHandlers[ti.Name]; ok {
					filteredHandlers[ti.Name] = fn
				}
				break
			}
		}
	}

	return filteredTools, filteredHandlers
}

// BuildDeclareTool creates the special declaration tool that the model must
// call on the first turn to declare its required capability packages.
func BuildDeclareTool() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "declare_capability_packages",
		Desc: "声明本次对话需要的功能包。必须在第一轮对话中首先调用此工具，列出你需要的所有能力包。可用包：content（内容管理）、structure（结构管理）、design（模板设计）、seo（SEO优化）、admin（系统管理）、agent（智能体管理）。根据用户的请求选择合适的包组合。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"packages": {
				Type: schema.Array,
				Desc: "需要的能力包名称列表，例如 [\"content\", \"universal\"]。可用值：content, structure, design, seo, admin, agent, universal",
				ElemInfo: &schema.ParameterInfo{
					Type: schema.String,
				},
			},
		}),
	}
}
