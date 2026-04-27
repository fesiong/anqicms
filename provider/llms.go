package provider

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/library"
)

type LLMsBuildStatus struct {
	Status     int         `json:"status"` // 0 = 未开始，1 = 进行中，2 = 已完成
	Percent    int         `json:"percent"`
	Current    int64       `json:"current"`
	Total      int64       `json:"total"`
	DelayTimer *time.Timer `json:"-"` // 延时计时器
}

func (w *Website) GetLLMsBuildStatus() *LLMsBuildStatus {
	return w.llmsBuildStatus
}

// ImmediateLLMsBuild 立即生成LLMs.txt，配置为0时调用
func (w *Website) ImmediateLLMsBuild() {
	// 未开启LLMs
	if w.PluginLLMs == nil || !w.PluginLLMs.Open {
		return
	}
	if w.PluginLLMs.UpdateFrequency != 0 {
		return
	}
	// 检查是否有生成任务正在进行中
	if w.llmsBuildStatus != nil && w.llmsBuildStatus.Status == 1 {
		// 正在生成中，跳过本次
		return
	}

	// 初始化llmsBuildStatus
	if w.llmsBuildStatus == nil {
		w.llmsBuildStatus = &LLMsBuildStatus{}
	}

	// 取消上一个延时任务
	if w.llmsBuildStatus.DelayTimer != nil {
		w.llmsBuildStatus.DelayTimer.Stop()
	}

	// 延时30秒开始生成
	w.llmsBuildStatus.DelayTimer = time.AfterFunc(time.Second*30, func() {
		// 检查是否有生成任务正在进行中
		if w.llmsBuildStatus != nil && w.llmsBuildStatus.Status == 1 {
			// 正在生成中，跳过本次
			return
		}
		w.LLMsBuild()
	})
}

func (w *Website) LLMsBuild() error {
	// 检查LLMs配置是否开启
	if w.PluginLLMs == nil || !w.PluginLLMs.Open {
		return nil
	}
	if w.llmsBuildStatus == nil {
		w.llmsBuildStatus = &LLMsBuildStatus{}
	}
	// 先重置
	w.llmsBuildStatus.Status = 1 // 设置为进行中
	w.llmsBuildStatus.Current = 0
	w.llmsBuildStatus.Total = 0
	w.llmsBuildStatus.Percent = 0

	// 准备生成LLMs.txt文件
	filePath := w.PublicPath + "llms.txt"
	fullFilePath := w.PublicPath + "llms-full.txt"
	fullFileUrl := w.System.BaseUrl + "/llms-full.txt"

	// 收集需要包含的文章数据
	var totalCount int64
	limit := w.PluginLLMs.MaxPostPerType
	if limit == 0 {
		limit = 100 // 不设置，则默认100
	}
	maxWords := w.PluginLLMs.MaxWords
	if maxWords == 0 {
		maxWords = 250 // 不设置，则默认250
	}

	categories := w.GetCacheCategories()
	totalCount = int64(len(categories))
	archiveCount := w.GetExplainCount("SELECT id FROM archives")
	totalCount += archiveCount
	// 设置总进度
	w.llmsBuildStatus.Total = totalCount

	// 开始生成文件
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	// 准备生成 fullFilePath
	fullFile, err := os.Create(fullFilePath)
	if err != nil {
		return err
	}
	defer fullFile.Close()

	// 写入标题
	if w.PluginLLMs.LLMSTitle != "" {
		_, err = file.WriteString("# " + w.PluginLLMs.LLMSTitle + "\n\n")
		if err != nil {
			return err
		}
	} else {
		_, err = file.WriteString("# " + w.System.SiteName + "\n\n")
		if err != nil {
			return err
		}
	}

	// 写入描述
	if w.PluginLLMs.LLMSDescrption != "" {
		_, err = file.WriteString("> " + w.PluginLLMs.LLMSDescrption + "\n\n")
		if err != nil {
			return err
		}
	} else {
		_, err = file.WriteString("> " + w.Index.SeoDescription + "\n\n")
		if err != nil {
			return err
		}
	}

	// 写入额外描述
	if w.PluginLLMs.LLMSAfterDescription != "" {
		_, err = file.WriteString("> " + w.PluginLLMs.LLMSAfterDescription + "\n\n")
		if err != nil {
			return err
		}
	}

	// 写入 fullFilePath
	file.WriteString("## Full Content Export\n\n")
	file.WriteString("- **URL**: " + fullFileUrl + "\n\n")

	// 先生成 page
	file.WriteString("## Page\n\n")
	for _, pageCategory := range categories {
		if pageCategory.Type != 3 {
			continue
		}
		// 排除指定的页面
		excludePage := false
		for _, excludeId := range w.PluginLLMs.ExcludePageIds {
			if excludeId == pageCategory.Id {
				excludePage = true
				break
			}
		}
		if excludePage {
			continue
		}
		w.llmsBuildStatus.Current++
		w.llmsBuildStatus.Percent = int(float64(w.llmsBuildStatus.Current) / float64(w.llmsBuildStatus.Total) * 100)
		// 写入页面链接
		link := w.GetUrl("page", pageCategory, 0)
		tmpData := "- [" + pageCategory.Title + "](" + link + ")"
		if pageCategory.Description != "" {
			description := library.ParseDescription(pageCategory.Description, maxWords)
			tmpData += ": " + description
		}
		_, err = file.WriteString(tmpData + "\n")
		if err != nil {
			return err
		}
		// 写入 fullFilePath
		fullData := "---\ntitle: " + pageCategory.Title + "\n"
		if w.PluginLLMs.IncludeMetadata {
			published := time.Unix(pageCategory.CreatedTime, 0).Format("2006-01-02")
			modified := time.Unix(pageCategory.UpdatedTime, 0).Format("2006-01-02")
			fullData += "- published: " + published + "\n- modified: " + modified + "\n"
		}
		if w.PluginLLMs.IncludeDescription {
			description := library.ParseDescription(pageCategory.Description, maxWords)
			fullData += "- description: " + description + "\n"
		}
		fullData += "---\n\n"
		fullData += library.HtmlToMarkdown(pageCategory.Content)

		_, err = fullFile.WriteString(fullData + "\n\n")
		if err != nil {
			return err
		}
	}
	file.WriteString("\n")

	// 获取所有模块
	modules := w.GetCacheModules()
	for _, module := range modules {
		// 排除指定的模块
		excludeModule := false
		for _, excludeId := range w.PluginLLMs.ExcludeModuleIds {
			if excludeId == module.Id {
				excludeModule = true
				break
			}
		}
		if excludeModule {
			continue
		}
		for _, category := range categories {
			if category.ModuleId != module.Id {
				continue
			}
			w.llmsBuildStatus.Current++
			w.llmsBuildStatus.Percent = int(float64(w.llmsBuildStatus.Current) / float64(w.llmsBuildStatus.Total) * 100)
			// 写入分类链接
			file.WriteString("## " + category.Title + "\n\n")
			link := w.GetUrl("category", category, 0)
			tmpData := "[" + category.Title + "](" + link + ")"
			if category.Description != "" {
				description := library.ParseDescription(category.Description, maxWords)
				tmpData += " " + description
			}
			_, err = file.WriteString(tmpData + "\n\n")
			if err != nil {
				return err
			}
			// 写入该分类下的所有文章
			archives, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				tx = tx.Where("`module_id` = ? AND `category_id` = ?", module.Id, category.Id)
				return tx
			}, "archives.id DESC", 0, limit, 0)
			for _, archive := range archives {
				w.llmsBuildStatus.Current++
				w.llmsBuildStatus.Percent = int(float64(w.llmsBuildStatus.Current) / float64(w.llmsBuildStatus.Total) * 100)
				// 写入文章链接
				link := w.GetUrl("archive", archive, 0)
				tmpData := "- [" + archive.Title + "](" + link + ")"
				if archive.Description != "" {
					description := library.ParseDescription(archive.Description, maxWords)
					tmpData += ": " + description
				}
				_, err = file.WriteString(tmpData + "\n")
				if err != nil {
					return err
				}
				// 前1000条文章才写入 fullFilePath
				if w.llmsBuildStatus.Current < 1000 {
					// 写入 fullFilePath
					fullData := "---\ntitle: " + archive.Title + "\n"
					if w.PluginLLMs.IncludeMetadata {
						published := time.Unix(archive.CreatedTime, 0).Format("2006-01-02")
						modified := time.Unix(archive.UpdatedTime, 0).Format("2006-01-02")
						fullData += "- published: " + published + "\n- modified: " + modified + "\n"
					}
					if w.PluginLLMs.IncludeDescription {
						description := library.ParseDescription(archive.Description, maxWords)
						fullData += "- description: " + description + "\n"
					}
					if w.PluginLLMs.IncludeCategory {
						fullData += "- category: " + category.Title + "\n"
					}
					if w.PluginLLMs.IncludeTag {
						tags := w.GetTagsByItemId(archive.Id)
						if len(tags) > 0 {
							tagNames := make([]string, 0, len(tags))
							for _, tag := range tags {
								tagNames = append(tagNames, tag.Title)
							}
							fullData += "- tags: " + strings.Join(tagNames, ", ") + "\n"
						}
					}
					if w.PluginLLMs.IncludeExtra {
						archiveParams := w.GetArchiveExtra(archive.ModuleId, archive.Id, true)
						if len(archiveParams) > 0 {
							for _, param := range archiveParams {
								if param.Value != nil && param.Value != "" {
									fullData += "- " + param.Name + ": " + fmt.Sprint(param.Value) + "\n"
								}
							}
						}
					}
					fullData += "---\n\n"
					fullData += library.HtmlToMarkdown(archive.Content)

					_, err = fullFile.WriteString(fullData + "\n\n")
					if err != nil {
						return err
					}
				}
			}
			file.WriteString("\n")
		}
	}

	// 写入结尾描述
	if w.PluginLLMs.LLMSEndDescription != "" {
		_, err = file.WriteString("> " + w.PluginLLMs.LLMSEndDescription + "\n")
		if err != nil {
			return err
		}
	}

	// 更新配置信息
	w.PluginLLMs.LastUpdate = time.Now().Unix()
	w.PluginLLMs.FileStatus = true

	// 完成生成
	w.llmsBuildStatus.Status = 2
	w.llmsBuildStatus.Percent = 100

	return nil
}

func (w *Website) CheckLLMsUpdateFrequency() {
	// 未开启LLMs
	if w.PluginLLMs == nil || !w.PluginLLMs.Open {
		return
	}
	// 每天更新一次
	if w.PluginLLMs.UpdateFrequency == 1 {
		w.LLMsBuild()
		return
	}
	// 每周更新一次
	if w.PluginLLMs.UpdateFrequency == 2 && time.Now().Weekday() == time.Sunday {
		w.LLMsBuild()
		return
	}
}
