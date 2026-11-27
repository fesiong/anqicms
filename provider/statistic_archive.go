package provider

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"kandaoni.com/anqicms/model"
)

// 文档统计的时间维度。
const (
	// ArchiveStatDimensionYear 按年聚合，周期形如 "2026"。
	ArchiveStatDimensionYear = "year"
	// ArchiveStatDimensionMonth 按月聚合，周期形如 "2026-11"。
	ArchiveStatDimensionMonth = "month"
)

// 文档统计的统计对象（行粒度）。
const (
	// ArchiveStatGroupModule 每个内容模型一行。
	ArchiveStatGroupModule = "module"
	// ArchiveStatGroupCategory 每个栏目一行。
	ArchiveStatGroupCategory = "category"
)

const (
	// archiveStatMaxMonths 按月统计时单次允许的最大周期数（120 个月 = 10 年）；
	// 按年统计的最大周期数是 archiveStatMaxYears 倍。
	// 为什么要设上限：start/end 直接来自查询串，不设限时 start=1970 会让
	// periods 长出 600+ 个元素，表格和图表都失去可读性，响应体也会被撑大。
	archiveStatMaxMonths = 120
	archiveStatMaxYears  = 60
)

// ArchiveStatItem 统计结果里的一行：一个内容模型，或一个栏目。
//
// Counts 的 key 与 ArchiveStatResult.Periods 一一对应（缺失周期补 0），
// 前端可以直接按 period 取值渲染表格，不必再判空。
type ArchiveStatItem struct {
	// Id 模型 ID（group=module）或栏目 ID（group=category）。
	// group=category 且 Id==0 表示"未分类"文档（category_id=0）。
	Id   int64  `json:"id"`
	Name string `json:"name"`
	// Unassigned 仅 group=category 的 Id==0 行出现：前端据此显示本地化的"未分类"。
	Unassigned bool   `json:"unassigned,omitempty"`
	ModuleId   uint   `json:"module_id"`
	ModuleName string `json:"module_name"`
	// ParentId / ParentName 仅栏目行有意义，用于展示「父栏目 / 子栏目」。
	ParentId   uint             `json:"parent_id"`
	ParentName string           `json:"parent_name"`
	Kind       string           `json:"kind"`
	TableName  string           `json:"table_name"`
	Total      int64            `json:"total"`
	Counts     map[string]int64 `json:"counts"`
}

// ArchiveStatResult 文档数量统计结果。
//
// 语义：只统计 `archives` 表（已发布文档），草稿/待发布/回收站存放在
// `archive_drafts`，按后台「文档列表」默认口径不算文档数量。
//
// Total 恒等于各 Item 的 Total 之和，也恒等于 Totals 各周期之和——表格里出现的
// 数字与「总数」永远自洽。唯一会"看起来对不上"的情形是 group=category 且站点
// 开启了多栏目（MultiCategory=1）：同一篇文档可以挂在多个栏目下，此时各栏目之和
// 必然大于文档去重数，字段 MultiCategory 会把开关带回前端用于提示。
type ArchiveStatResult struct {
	Dimension     string             `json:"dimension"`
	Group         string             `json:"group"`
	MultiCategory bool               `json:"multi_category"`
	Start         string             `json:"start"`
	End           string             `json:"end"`
	Periods       []string           `json:"periods"`
	Items         []*ArchiveStatItem `json:"items"`
	Totals        map[string]int64   `json:"totals"`
	Total         int64              `json:"total"`
}

// archiveStatRow 聚合查询的原始行，只用于中间态，不直接暴露给前端。
//
// 注意不要 Scan 到 model.Archive：archives 的商品属性列已迁到 archive_products，
// model.Archive 上对应字段是 gorm:"-"（见 model/archive.go 顶部说明），
// Scan 会静默归零，且有 TestNoScanIntoArchive 护栏。
type archiveStatRow struct {
	// GroupId 聚合键：group=module 时是 module_id，group=category 时是 category_id。
	GroupId int64  `gorm:"column:group_id"`
	Period  string `gorm:"column:period"`
	Total   int64  `gorm:"column:total"`
}

// archiveStatEntity 与统计行一一对应的静态信息（名字、归属），与聚合结果分离，
// 这样"哪些行该出现"完全由实体清单决定，SQL 只负责填数字。
type archiveStatEntity struct {
	id         int64
	name       string
	unassigned bool
	moduleId   uint
	moduleName string
	parentId   uint
	parentName string
	kind       string
	tableName  string
}

// archiveStatLayout 返回维度对应的周期布局（Go 时间格式 + MySQL DATE_FORMAT 格式）。
func archiveStatLayout(dimension string) (goLayout string, mysqlLayout string, ok bool) {
	switch dimension {
	case ArchiveStatDimensionYear:
		return "2006", "%Y", true
	case ArchiveStatDimensionMonth:
		return "2006-01", "%Y-%m", true
	default:
		return "", "", false
	}
}

// normalizeArchiveStatDimension 把维度收敛到合法枚举；空值按 month 处理，
// 其他未知值返回 false，由调用方明确报错（不静默降级，否则前端传错维度
// 只会看到"数据像是按月"的错觉）。
func normalizeArchiveStatDimension(dimension string) (string, bool) {
	dim := strings.ToLower(strings.TrimSpace(dimension))
	if dim == "" {
		return ArchiveStatDimensionMonth, true
	}
	if _, _, ok := archiveStatLayout(dim); !ok {
		return "", false
	}
	return dim, true
}

// normalizeArchiveStatGroup 把统计对象收敛到合法枚举；空值按 module 处理。
func normalizeArchiveStatGroup(group string) (string, bool) {
	g := strings.ToLower(strings.TrimSpace(group))
	if g == "" {
		return ArchiveStatGroupModule, true
	}
	if g == ArchiveStatGroupModule || g == ArchiveStatGroupCategory {
		return g, true
	}
	return "", false
}

// archiveStatDefaultRange 返回维度的默认区间：按月取近 12 个月（含当月），
// 按年取近 5 年（含当年）。
func archiveStatDefaultRange(dimension string, now time.Time) (string, string) {
	if dimension == ArchiveStatDimensionYear {
		return now.AddDate(-4, 0, 0).Format("2006"), now.Format("2006")
	}
	return now.AddDate(0, -11, 0).Format("2006-01"), now.Format("2006-01")
}

// buildArchiveStatPeriods 生成 [start, end] 内的完整周期序列（升序）。
//
// start/end 已由调用方解析并保证 start <= end。序列会按维度上限裁剪：
// 超长时**保留最近的周期**（丢掉最早的），这样用户看到的是"最近 N 个月"，
// 而不是"把最近的丢了只剩 1990 年"。
func buildArchiveStatPeriods(dimension, start, end string) []string {
	goLayout, _, ok := archiveStatLayout(dimension)
	if !ok {
		return nil
	}
	startTime, err := time.ParseInLocation(goLayout, start, time.Local)
	if err != nil {
		return nil
	}
	endTime, err := time.ParseInLocation(goLayout, end, time.Local)
	if err != nil {
		return nil
	}
	maxPeriods := archiveStatMaxMonths
	stepBack := func(t time.Time) time.Time { return t.AddDate(0, -1, 0) }
	if dimension == ArchiveStatDimensionYear {
		maxPeriods = archiveStatMaxYears
		stepBack = func(t time.Time) time.Time { return t.AddDate(-1, 0, 0) }
	}

	// 从 end 往回走：区间超长时天然只保留最近 maxPeriods 个周期，
	// 既不需要事后裁剪，也不会因为 start=1970 白跑几千次循环。
	var reversed []string
	for t := endTime; !t.Before(startTime) && len(reversed) < maxPeriods; t = stepBack(t) {
		reversed = append(reversed, t.Format(goLayout))
	}
	periods := make([]string, 0, len(reversed))
	for i := len(reversed) - 1; i >= 0; i-- {
		periods = append(periods, reversed[i])
	}
	return periods
}

// archiveStatPeriodRange 把周期列表换算成 Unix 秒区间：左闭右开。
// 右端取"最后一个周期结束时刻"，因此不会漏掉 end 周期内任意一天的文档。
func archiveStatPeriodRange(dimension string, periods []string) (int64, int64) {
	goLayout, _, ok := archiveStatLayout(dimension)
	if !ok || len(periods) == 0 {
		return 0, 0
	}
	first, err := time.ParseInLocation(goLayout, periods[0], time.Local)
	if err != nil {
		return 0, 0
	}
	last, err := time.ParseInLocation(goLayout, periods[len(periods)-1], time.Local)
	if err != nil {
		return 0, 0
	}
	var next time.Time
	if dimension == ArchiveStatDimensionYear {
		next = last.AddDate(1, 0, 0)
	} else {
		next = last.AddDate(0, 1, 0)
	}
	return first.Unix(), next.Unix()
}

// resolveArchiveStatRange 把请求参数收敛成实际生效的 [dimension, start, end]。
//
// 容错策略（与其它统计端点一致，尽量不因脏参数报错）：
//   - 维度非法 → 报错，不做静默降级；
//   - start/end 缺失或格式不对 → 各自回落到默认区间的对应端；
//   - start > end → 交换（RangePicker 正常不会出现，直接调用 API 时可能）。
func resolveArchiveStatRange(dimension, start, end string, now time.Time) (string, string, string, error) {
	dim, ok := normalizeArchiveStatDimension(dimension)
	if !ok {
		return "", "", "", fmt.Errorf("dimension 仅支持 %s 或 %s", ArchiveStatDimensionYear, ArchiveStatDimensionMonth)
	}
	goLayout, _, _ := archiveStatLayout(dim)
	defStart, defEnd := archiveStatDefaultRange(dim, now)

	parse := func(v, fallback string) string {
		v = strings.TrimSpace(v)
		if v == "" {
			return fallback
		}
		t, err := time.ParseInLocation(goLayout, v, time.Local)
		if err != nil {
			return fallback
		}
		return t.Format(goLayout)
	}
	startP := parse(start, defStart)
	endP := parse(end, defEnd)
	if startP > endP {
		startP, endP = endP, startP
	}
	return dim, startP, endP, nil
}

// StatisticArchive 按年/按月统计文档数量，可按内容模型（module）或栏目（category）分组。
//
// 返回的 periods / items / totals 三者是同一份数据的三种切法，
// 前端表格（行 × 周期）、总数卡片、分布条都直接消费它们。
func (w *Website) StatisticArchive(dimension, group, start, end string) (*ArchiveStatResult, error) {
	return w.statisticArchiveAt(dimension, group, start, end, time.Now())
}

// statisticArchiveAt 是 StatisticArchive 的可注入时间版本，便于测试固定"当前时间"。
func (w *Website) statisticArchiveAt(dimension, group, start, end string, now time.Time) (*ArchiveStatResult, error) {
	dim, startP, endP, err := resolveArchiveStatRange(dimension, start, end, now)
	if err != nil {
		return nil, err
	}
	grp, ok := normalizeArchiveStatGroup(group)
	if !ok {
		return nil, fmt.Errorf("group 仅支持 %s 或 %s", ArchiveStatGroupModule, ArchiveStatGroupCategory)
	}
	periods := buildArchiveStatPeriods(dim, startP, endP)
	if len(periods) == 0 {
		return nil, fmt.Errorf("统计区间无效：%s ~ %s", startP, endP)
	}
	// 裁剪后的实际起点要回写，前端拿到的 start 与 periods[0] 必须一致。
	startP = periods[0]
	endP = periods[len(periods)-1]
	rangeStart, rangeEnd := archiveStatPeriodRange(dim, periods)
	if rangeStart == 0 || rangeEnd <= rangeStart {
		return nil, fmt.Errorf("统计区间无效：%s ~ %s", startP, endP)
	}

	modules, err := w.GetModules()
	if err != nil {
		return nil, err
	}

	var rows []archiveStatRow
	if grp == ArchiveStatGroupCategory {
		rows, err = w.aggregateArchiveStatByCategory(dim, rangeStart, rangeEnd)
	} else {
		rows, err = w.aggregateArchiveStat(dim, rangeStart, rangeEnd)
	}
	if err != nil {
		return nil, err
	}

	var entities []archiveStatEntity
	if grp == ArchiveStatGroupCategory {
		entities, err = w.archiveStatCategoryEntities(modules)
		if err != nil {
			return nil, err
		}
	} else {
		entities = archiveStatModuleEntities(modules)
	}

	multiCategory := grp == ArchiveStatGroupCategory && w.Content.MultiCategory == 1
	return assembleArchiveStatResult(dim, grp, startP, endP, periods, entities, rows, multiCategory), nil
}

// aggregateArchiveStat 按 module_id + 周期聚合。
func (w *Website) aggregateArchiveStat(dimension string, rangeStart, rangeEnd int64) ([]archiveStatRow, error) {
	_, mysqlLayout, _ := archiveStatLayout(dimension)
	// 用 DATE_FORMAT 让 MySQL 直接返回周期字符串：与 ChartDateSQL 同理，
	// 走 DATE()/YEAR() 会被驱动按 parseTime=True 解析成 time.Time，再落到
	// string 字段就变成 RFC3339 长串（见 ChartDateSQL 顶部说明）。
	periodExpr := fmt.Sprintf("DATE_FORMAT(FROM_UNIXTIME(a.`created_time`), '%s')", mysqlLayout)
	var rows []archiveStatRow
	err := w.DB.Table("`archives` as a").
		Select("a.`module_id` as `group_id`, "+periodExpr+" as `period`, COUNT(*) as `total`").
		Where("a.`created_time` >= ? and a.`created_time` < ?", rangeStart, rangeEnd).
		Group("a.`module_id`, `period`").
		Scan(&rows).Error
	return rows, err
}

// aggregateArchiveStatByCategory 按 category_id + 周期聚合。
//
// 开了多栏目（content.multicategory=1）时一篇文档可以挂在多个栏目下，归属关系
// 存在 `archive_categories`，必须走连接表；否则直接用 archives.category_id。
// 判定与后台文档列表（ArchiveList）保持一致，两边口径才不会打架。
func (w *Website) aggregateArchiveStatByCategory(dimension string, rangeStart, rangeEnd int64) ([]archiveStatRow, error) {
	_, mysqlLayout, _ := archiveStatLayout(dimension)
	periodExpr := fmt.Sprintf("DATE_FORMAT(FROM_UNIXTIME(a.`created_time`), '%s')", mysqlLayout)

	query := w.DB.Table("`archives` as a")
	column := "a.`category_id`"
	if w.Content.MultiCategory == 1 {
		query = query.Joins("INNER JOIN `archive_categories` as ac ON ac.`archive_id` = a.`id`")
		column = "ac.`category_id`"
	}
	var rows []archiveStatRow
	err := query.
		Select(column+" as `group_id`, "+periodExpr+" as `period`, COUNT(*) as `total`").
		Where("a.`created_time` >= ? and a.`created_time` < ?", rangeStart, rangeEnd).
		Group(column + ", `period`").
		Scan(&rows).Error
	return rows, err
}

// archiveStatModuleEntities 模型行清单（最终排序在装配层做）。
func archiveStatModuleEntities(modules []model.Module) []archiveStatEntity {
	entities := make([]archiveStatEntity, 0, len(modules))
	for _, m := range modules {
		entities = append(entities, archiveStatEntity{
			id:        int64(m.Id),
			name:      m.Name,
			moduleId:  m.Id,
			kind:      m.Kind,
			tableName: m.TableName,
		})
	}
	return entities
}

// archiveStatCategoryEntities 栏目行清单（含父栏目名与所属模型名）。
//
// 栏目行不按 type 过滤：单页面（type=3）同样占用 archives 里的记录，
// 排除掉会让"栏目合计"莫名小于文档总数。
func (w *Website) archiveStatCategoryEntities(modules []model.Module) ([]archiveStatEntity, error) {
	var categories []*model.Category
	if err := w.DB.Model(&model.Category{}).
		Omit("content", "extra").
		Order("id asc").
		Find(&categories).Error; err != nil {
		return nil, err
	}
	moduleNames := make(map[uint]string, len(modules))
	for _, m := range modules {
		moduleNames[m.Id] = m.Name
	}
	titles := make(map[uint]string, len(categories))
	for _, c := range categories {
		titles[c.Id] = c.Title
	}

	entities := make([]archiveStatEntity, 0, len(categories))
	for _, c := range categories {
		entities = append(entities, archiveStatEntity{
			id:         int64(c.Id),
			name:       c.Title,
			moduleId:   c.ModuleId,
			moduleName: moduleNames[c.ModuleId],
			parentId:   c.ParentId,
			parentName: titles[c.ParentId],
		})
	}
	return entities, nil
}

// assembleArchiveStatResult 把聚合行装配成前端可直接渲染的结果。
//
// 单独抽出来是为了能脱离数据库测两条不变量（见 statistic_archive_test.go）：
//  1. Total == 各 Item 的 Total 之和 == Totals 各周期之和；
//  2. 未被 periods 覆盖的行（时区错位）一律丢弃，不产生幽灵列。
func assembleArchiveStatResult(dim, group, startP, endP string, periods []string, entities []archiveStatEntity, rows []archiveStatRow, multiCategory bool) *ArchiveStatResult {
	result := &ArchiveStatResult{
		Dimension:     dim,
		Group:         group,
		MultiCategory: multiCategory,
		Start:         startP,
		End:           endP,
		Periods:       periods,
		Items:         make([]*ArchiveStatItem, 0, len(entities)),
		Totals:        make(map[string]int64, len(periods)),
	}
	for _, p := range periods {
		result.Totals[p] = 0
	}

	byId := make(map[int64]*ArchiveStatItem, len(entities))
	newItem := func(e archiveStatEntity) *ArchiveStatItem {
		counts := make(map[string]int64, len(periods))
		for _, p := range periods {
			counts[p] = 0
		}
		item := &ArchiveStatItem{
			Id:         e.id,
			Name:       e.name,
			Unassigned: e.unassigned,
			ModuleId:   e.moduleId,
			ModuleName: e.moduleName,
			ParentId:   e.parentId,
			ParentName: e.parentName,
			Kind:       e.kind,
			TableName:  e.tableName,
			Counts:     counts,
		}
		result.Items = append(result.Items, item)
		byId[e.id] = item
		return item
	}
	for _, e := range entities {
		newItem(e)
	}

	for _, row := range rows {
		item, exists := byId[row.GroupId]
		if !exists {
			// 栏目模式下的 category_id=0 是合法状态（文档未归类），补一行"未分类"；
			// 其余是模型/栏目已删除的遗留文档：不单独成行（没有名字可展示），
			// 也不计入总数，保证「总数 = 各行之和」这条不变量成立。
			if group != ArchiveStatGroupCategory || row.GroupId != 0 {
				continue
			}
			item = newItem(archiveStatEntity{id: 0, unassigned: true})
		}
		if _, inRange := result.Totals[row.Period]; !inRange {
			// 时区错位（MySQL 会话时区与进程本地时区不一致时可能差一天）
			// 落在区间外的桶：丢弃，避免表格出现 periods 之外的空列。
			continue
		}
		item.Counts[row.Period] += row.Total
		item.Total += row.Total
		result.Totals[row.Period] += row.Total
		result.Total += row.Total
	}

	// 文档多的行排前面，空行自然沉底——统计页的第一屏应该是最有信息量的部分。
	sort.SliceStable(result.Items, func(i, j int) bool {
		if result.Items[i].Total != result.Items[j].Total {
			return result.Items[i].Total > result.Items[j].Total
		}
		return result.Items[i].Id < result.Items[j].Id
	})

	return result
}
