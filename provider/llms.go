package provider

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

type LLMsBuildStatus struct {
	Status  int    `json:"status"`  // 0 = 未开始，1 = 进行中，2 = 已完成，-1 = 失败
	Percent int    `json:"percent"` // 进度百分比
	Current int64  `json:"current"` // 当前进度数量
	Total   int64  `json:"total"`   // 总进度数量
	Msg     string `json:"msg"`     // 失败原因（status = -1 时有效）

	// 进度字段由后台构建协程写、status 端点按秒读，必须加锁；
	// 对外只给 snapshot() 的副本，别把本结构体直接交给 JSON 编码。
	mu         sync.Mutex
	delayTimer *time.Timer
}

// snapshot 返回进度副本（不带锁与计时器）。手动构造字面量而不是 `*s`，
// 因为复制一个带 sync.Mutex 的结构体会被 go vet 的 copylocks 拦下。
func (s *LLMsBuildStatus) snapshot() *LLMsBuildStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &LLMsBuildStatus{
		Status:  s.Status,
		Percent: s.Percent,
		Current: s.Current,
		Total:   s.Total,
		Msg:     s.Msg,
	}
}

// begin 标记"开始构建"；已有构建在跑时返回 false，调用方应直接返回，不要覆盖别人的进度。
func (s *LLMsBuildStatus) begin() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Status == 1 {
		return false
	}
	s.Status = 1
	s.Percent = 0
	s.Current = 0
	s.Total = 0
	s.Msg = ""
	return true
}

func (s *LLMsBuildStatus) isBuilding() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Status == 1
}

func (s *LLMsBuildStatus) setTotal(total int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Total = total
}

// step 推进一格，返回推进后的 Current。Total 为 0 时不动百分比（避免除零）。
func (s *LLMsBuildStatus) step() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Current++
	if s.Total > 0 {
		percent := int(float64(s.Current) / float64(s.Total) * 100)
		if percent > 100 {
			percent = 100
		}
		s.Percent = percent
	}
	return s.Current
}

func (s *LLMsBuildStatus) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = 2
	s.Percent = 100
	s.delayTimer = nil
}

func (s *LLMsBuildStatus) fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = -1
	if err != nil {
		s.Msg = err.Error()
	}
}

// setDelayTimer 换掉上一个延时任务（先停旧的，避免一次保存触发多次构建）。
func (s *LLMsBuildStatus) setDelayTimer(t *time.Timer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.delayTimer != nil {
		s.delayTimer.Stop()
	}
	s.delayTimer = t
}

// llmsBuildStatuses 按站点 id 保存构建状态（map[uint]*LLMsBuildStatus）。
//
// 为什么不挂在 Website 字段上：CurrentSite/CurrentSubSite 返回的是请求副本
// （cloneWithContext 浅拷贝），副本上给字段**重新赋指针**只改副本自身，主实例仍是 nil。
// 于是 GET /plugin/llms/status 永远读到 nil —— 前端把 {finished,building} 当进度对象
// 渲染出 "undefined / undefined (undefined%)"（2026-11-19 实测）。包级表不经过副本，
// 天然绕开这一整类问题。
var llmsBuildStatuses sync.Map

// llmsBuildState 返回（必要时创建）站点共享的构建状态对象。
func (w *Website) llmsBuildState() *LLMsBuildStatus {
	if w == nil {
		return nil
	}
	if v, ok := llmsBuildStatuses.Load(w.Id); ok {
		return v.(*LLMsBuildStatus)
	}
	actual, _ := llmsBuildStatuses.LoadOrStore(w.Id, &LLMsBuildStatus{})
	return actual.(*LLMsBuildStatus)
}

// GetLLMsBuildStatus 返回状态快照；nil 表示本次进程内还没有任何构建记录。
func (w *Website) GetLLMsBuildStatus() *LLMsBuildStatus {
	if w == nil {
		return nil
	}
	v, ok := llmsBuildStatuses.Load(w.Id)
	if !ok {
		return nil
	}
	return v.(*LLMsBuildStatus).snapshot()
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
	status := w.llmsBuildState()
	if status == nil || status.isBuilding() {
		// 正在生成中，跳过本次
		return
	}

	// 必须在这里就脱离请求上下文：计时器 30 秒后才跑，那时请求 ctx 早已 cancel，
	// 副本上的查询会直接报 context canceled，任务静默跑一半就没了。
	task := w.Async()
	// 延时30秒开始生成（并取消上一个延时任务）
	status.setDelayTimer(time.AfterFunc(time.Second*30, func() {
		if status.isBuilding() {
			// 正在生成中，跳过本次
			return
		}
		_ = task.LLMsBuild()
	}))
}

// StartLLMsBuild 异步触发构建：同步占位（status=1），耗时的写文件放到后台，
// 让前端每秒轮询 /plugin/llms/status 能看到真实进度。
// 返回 false 表示未开启或已有构建在跑（不重复启动）。
func (w *Website) StartLLMsBuild() bool {
	if w.PluginLLMs == nil || !w.PluginLLMs.Open {
		return false
	}
	status := w.llmsBuildState()
	if status == nil || !status.begin() {
		return false
	}
	task := w.Async() // 请求返回后 ctx 会被 cancel，后台任务不能挂在请求上下文上
	go func() {
		// 这是裸 goroutine，没有 iris 的 per-request recover 兜底：
		// 构建过程里任何 panic（脏数据、System/Index 为 nil）都会打挂整个进程，
		// 所以必须自己 recover，并把失败落进状态，前端才不会永远停在"进行中"。
		defer func() {
			if r := recover(); r != nil {
				status.fail(fmt.Errorf("llms build panic: %v", r))
			}
		}()
		if err := task.llmsBuildFile(status); err != nil {
			status.fail(err)
			return
		}
		status.finish()
	}()
	return true
}

// LLMsBuild 同步生成 llms.txt（定时任务、文章变更后的延时构建走这里）。
func (w *Website) LLMsBuild() error {
	// 检查LLMs配置是否开启
	if w.PluginLLMs == nil || !w.PluginLLMs.Open {
		return nil
	}
	status := w.llmsBuildState()
	if status == nil || !status.begin() {
		// 已有构建在跑，跳过本次，别把进行中的进度重置掉
		return nil
	}
	if err := w.llmsBuildFile(status); err != nil {
		status.fail(err)
		return err
	}
	status.finish()
	return nil
}

// llmsBuildJob 一条待写入的条目：页面（isPage）或某个模型下的分类。
type llmsBuildJob struct {
	category *model.Category
	moduleId uint
	isPage   bool
}

// llmsBuildJobs 按写入顺序列出待处理条目：先页面，再按模块顺序排各分类。
//
// 排除项（页面 / 模块 / 分类）只在这里过滤一次，"算总进度"和"写文件"都用这份清单，
// 避免两处各写一遍过滤条件后慢慢长歪。
func (w *Website) llmsBuildJobs() []llmsBuildJob {
	categories := w.GetCacheCategories()
	jobs := make([]llmsBuildJob, 0, len(categories))
	for _, pageCategory := range categories {
		if pageCategory.Type != 3 || llmsIdInList(w.PluginLLMs.ExcludePageIds, pageCategory.Id) {
			continue
		}
		jobs = append(jobs, llmsBuildJob{category: pageCategory, isPage: true})
	}
	for _, module := range w.GetCacheModules() {
		if llmsIdInList(w.PluginLLMs.ExcludeModuleIds, module.Id) {
			continue
		}
		for _, category := range categories {
			if category.ModuleId != module.Id || llmsIdInList(w.PluginLLMs.ExcludeCategoryIds, category.Id) {
				continue
			}
			jobs = append(jobs, llmsBuildJob{category: category, moduleId: module.Id})
		}
	}
	return jobs
}

func llmsIdInList(list []uint, id uint) bool {
	for _, item := range list {
		if item == id {
			return true
		}
	}
	return false
}

// countArchivesByCategory 一次分组查询拿到各分类的文章数，用于把进度分母算准
// （每个分类最多写 limit 篇，所以分母不能直接用全站文章数）。
func (w *Website) countArchivesByCategory() map[uint]int64 {
	counts := make(map[uint]int64)
	if w.DB == nil {
		return counts
	}
	var rows []struct {
		CategoryId uint
		Total      int64
	}
	err := w.DB.WithContext(w.Ctx()).Model(&model.Archive{}).
		Select("category_id, count(*) as total").
		Group("category_id").
		Scan(&rows).Error
	if err != nil {
		// 统计失败不影响生成，只是进度分母退化成"条目数"
		return counts
	}
	for _, row := range rows {
		counts[row.CategoryId] = row.Total
	}
	return counts
}

// llmsBuildFile 是真正的生成逻辑，进度通过 status 对外可见；
// 状态的开始/结束由调用方负责（这样同步与异步两个入口可以共用一份实现）。
func (w *Website) llmsBuildFile(status *LLMsBuildStatus) error {
	// 准备生成LLMs.txt文件
	filePath := w.PublicPath + "llms.txt"
	fullFilePath := w.PublicPath + "llms-full.txt"
	fullFileUrl := w.System.BaseUrl + "/llms-full.txt"

	// 收集需要包含的文章数据。
	//
	// 总数按"实际会写入的条数"算：原实现拿全站文章数当分母，而每个分类最多只写
	// limit 篇（还有排除项），于是进度条走到十几%就跳到 100%
	//（2026-11-19 实测 521 / 3494，写完了还显示 15%）。
	jobs := w.llmsBuildJobs()

	limit := w.PluginLLMs.MaxPostPerType
	if limit == 0 {
		limit = 100 // 不设置，则默认100
	}
	maxWords := w.PluginLLMs.MaxWords
	if maxWords == 0 {
		maxWords = 250 // 不设置，则默认250
	}

	archiveCounts := w.countArchivesByCategory()
	totalCount := int64(len(jobs))
	for _, job := range jobs {
		if job.isPage {
			continue
		}
		count := archiveCounts[job.category.Id]
		if count > int64(limit) {
			count = int64(limit)
		}
		totalCount += count
	}
	// 设置总进度
	status.setTotal(totalCount)

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
	for _, job := range jobs {
		if !job.isPage {
			continue
		}
		pageCategory := job.category
		status.step()
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

	// 获取所有模块（排除项已在 llmsBuildJobs 里过滤，这里只按模块分组输出）
	modules := w.GetCacheModules()
	for _, module := range modules {
		for _, job := range jobs {
			if job.isPage || job.moduleId != module.Id {
				continue
			}
			category := job.category
			status.step()
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
				// 只看前1000条文章的内容写入 fullFilePath（按累计条目数，与原实现一致）
				position := status.step()
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
				if position < 1000 {
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

	return nil
}

func (w *Website) CheckLLMsUpdateFrequency() {
	// 未开启LLMs
	if w.PluginLLMs == nil || !w.PluginLLMs.Open {
		return
	}
	// 每天更新一次
	if w.PluginLLMs.UpdateFrequency == 1 {
		_ = w.LLMsBuild()
		return
	}
	// 每周更新一次
	if w.PluginLLMs.UpdateFrequency == 2 && time.Now().Weekday() == time.Sunday {
		_ = w.LLMsBuild()
		return
	}
}
