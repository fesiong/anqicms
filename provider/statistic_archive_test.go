package provider

import (
	"testing"
	"time"

	"kandaoni.com/anqicms/model"
)

// fixedNow 固定"当前时间"，避免默认区间断言随运行日期漂移。
func fixedNow() time.Time {
	return time.Date(2026, 11, 10, 16, 0, 0, 0, time.Local)
}

// TestBuildArchiveStatPeriodsMonth 验证按月周期序列是闭区间且不跳月（含跨年）。
func TestBuildArchiveStatPeriodsMonth(t *testing.T) {
	got := buildArchiveStatPeriods(ArchiveStatDimensionMonth, "2025-11", "2026-02")
	want := []string{"2025-11", "2025-12", "2026-01", "2026-02"}
	if len(got) != len(want) {
		t.Fatalf("周期数量不对: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个周期不对: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestBuildArchiveStatPeriodsYear 验证按年维度的周期序列。
func TestBuildArchiveStatPeriodsYear(t *testing.T) {
	got := buildArchiveStatPeriods(ArchiveStatDimensionYear, "2023", "2026")
	want := []string{"2023", "2024", "2025", "2026"}
	if len(got) != len(want) {
		t.Fatalf("周期数量不对: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个周期不对: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestArchiveStatPeriodsCapKeepsRecent 长区间必须裁剪，且保留最近的周期。
//
// 反向对照：如果把裁剪写成保留最早的 120 个，periods 会从 1970-01 开始、
// 最后停在 1979-12，本用例立刻红（实测过一次：从 end 往回走才是正解）。
func TestArchiveStatPeriodsCapKeepsRecent(t *testing.T) {
	got := buildArchiveStatPeriods(ArchiveStatDimensionMonth, "1970-01", "2026-11")
	if len(got) != archiveStatMaxMonths {
		t.Fatalf("应裁剪到 %d 个周期，实际 %d", archiveStatMaxMonths, len(got))
	}
	if got[len(got)-1] != "2026-11" {
		t.Fatalf("裁剪后必须保留最近的周期，末尾是 %q", got[len(got)-1])
	}
	if got[0] != "2016-12" {
		t.Fatalf("裁剪后起点应为 2016-12（2026-11 往前 119 个月），实际 %q", got[0])
	}
}

// TestResolveArchiveStatRangeDefaults 缺参时按维度回落到「近 12 个月 / 近 5 年」。
func TestResolveArchiveStatRangeDefaults(t *testing.T) {
	now := fixedNow()

	dim, start, end, err := resolveArchiveStatRange("", "", "", now)
	if err != nil {
		t.Fatalf("空参数不应报错: %v", err)
	}
	if dim != ArchiveStatDimensionMonth || start != "2025-12" || end != "2026-11" {
		t.Fatalf("按月默认区间不对: dim=%q start=%q end=%q", dim, start, end)
	}

	dim, start, end, err = resolveArchiveStatRange("year", "", "", now)
	if err != nil {
		t.Fatalf("空参数不应报错: %v", err)
	}
	if dim != ArchiveStatDimensionYear || start != "2022" || end != "2026" {
		t.Fatalf("按年默认区间不对: dim=%q start=%q end=%q", dim, start, end)
	}
}

// TestResolveArchiveStatRangeToleratesDirtyParams 非法/颠倒的参数不应让统计页 500。
func TestResolveArchiveStatRangeToleratesDirtyParams(t *testing.T) {
	now := fixedNow()

	// 起点晚于终点 → 交换成合法区间。
	_, start, end, err := resolveArchiveStatRange("month", "2026-11", "2026-01", now)
	if err != nil {
		t.Fatalf("颠倒区间不应报错: %v", err)
	}
	if start != "2026-01" || end != "2026-11" {
		t.Fatalf("颠倒区间应被交换: start=%q end=%q", start, end)
	}

	// 格式不对 → 该端回落默认值，另一端保留用户输入。
	_, start, end, err = resolveArchiveStatRange("month", "not-a-date", "2026-03", now)
	if err != nil {
		t.Fatalf("脏参数不应报错: %v", err)
	}
	if start != "2025-12" || end != "2026-03" {
		t.Fatalf("脏参数回落不对: start=%q end=%q", start, end)
	}
}

// TestResolveArchiveStatRangeRejectsUnknownDimension 未知维度必须报错。
//
// 这是刻意不做静默降级：传 dimension=week 却拿到按月数据，
// 前端只会以为"按月统计是坏的"，既不报错也查不出原因。
func TestResolveArchiveStatRangeRejectsUnknownDimension(t *testing.T) {
	if _, _, _, err := resolveArchiveStatRange("week", "2026-01", "2026-02", fixedNow()); err == nil {
		t.Fatal("未知维度应报错，实际通过")
	}
}

// TestNormalizeArchiveStatGroup 分组粒度只接受 module / category，空值回落 module。
func TestNormalizeArchiveStatGroup(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", ArchiveStatGroupModule, true},
		{"module", ArchiveStatGroupModule, true},
		{" Category ", ArchiveStatGroupCategory, true},
		{"page", "", false},
	}
	for _, c := range cases {
		got, ok := normalizeArchiveStatGroup(c.in)
		if ok != c.ok || got != c.want {
			t.Fatalf("normalizeArchiveStatGroup(%q) = (%q,%v)，期望 (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestArchiveStatPeriodRangeCoversWholeLastPeriod 区间右端必须覆盖最后一个周期的最后一秒。
func TestArchiveStatPeriodRangeCoversWholeLastPeriod(t *testing.T) {
	start, end := archiveStatPeriodRange(ArchiveStatDimensionMonth, []string{"2026-01", "2026-02"})
	if start != time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local).Unix() {
		t.Fatalf("左端不对: %d", start)
	}
	if end != time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local).Unix() {
		t.Fatalf("右端应为 2026-03-01 00:00:00（左闭右开），实际 %d", end)
	}
	// 2 月最后一天的 23:59:59 必须落在区间内。
	lastMoment := time.Date(2026, 2, 28, 23, 59, 59, 0, time.Local).Unix()
	if lastMoment < start || lastMoment >= end {
		t.Fatalf("2 月最后一秒 %d 未被 [%d, %d) 覆盖", lastMoment, start, end)
	}
}

// TestAssembleArchiveStatResultModuleInvariants 模型分组的两条不变量。
func TestAssembleArchiveStatResultModuleInvariants(t *testing.T) {
	modules := []model.Module{
		{Id: 1, Name: "文章", Kind: model.ModuleKindArticle, TableName: "article"},
		{Id: 2, Name: "产品", Kind: model.ModuleKindProduct, TableName: "product"},
		{Id: 3, Name: "空模型", Kind: model.ModuleKindArticle, TableName: "empty"},
	}
	periods := []string{"2026-09", "2026-10", "2026-11"}
	rows := []archiveStatRow{
		{GroupId: 1, Period: "2026-10", Total: 5},
		{GroupId: 1, Period: "2026-11", Total: 3},
		{GroupId: 2, Period: "2026-10", Total: 2},
		{GroupId: 99, Period: "2026-10", Total: 7}, // 模型已删除
		{GroupId: 1, Period: "2026-08", Total: 4},  // 落在 periods 之外
	}

	got := assembleArchiveStatResult(ArchiveStatDimensionMonth, ArchiveStatGroupModule,
		periods[0], periods[2], periods, archiveStatModuleEntities(modules), rows, false)

	if got.Total != 10 {
		t.Fatalf("总数应为 10（5+3+2），实际 %d", got.Total)
	}
	var sum int64
	for _, p := range periods {
		sum += got.Totals[p]
	}
	if sum != got.Total {
		t.Fatalf("各周期之和 %d != 总数 %d", sum, got.Total)
	}
	if len(got.Items) != 3 {
		t.Fatalf("行数应为 3，实际 %d", len(got.Items))
	}
	// 按文档数倒序：文章(8) → 产品(2) → 空模型(0)
	if got.Items[0].Id != 1 || got.Items[1].Id != 2 || got.Items[2].Id != 3 {
		t.Fatalf("排序不对: %+v", got.Items)
	}
	if got.Items[2].Total != 0 || got.Items[2].Counts["2026-09"] != 0 {
		t.Fatalf("空模型必须显式补 0，实际 %+v", got.Items[2])
	}
	if _, exists := got.Totals["2026-08"]; exists {
		t.Fatal("periods 之外的桶不应出现在 Totals 里")
	}
	if _, exists := got.Items[0].Counts["2026-08"]; exists {
		t.Fatal("periods 之外的桶不应出现在 Item Counts 里")
	}
}

// TestAssembleArchiveStatResultCategory 栏目分组：父栏目名、未分类行、多栏目标记。
func TestAssembleArchiveStatResultCategory(t *testing.T) {
	entities := []archiveStatEntity{
		{id: 10, name: "使用帮助", moduleId: 1, moduleName: "文章模块"},
		{id: 11, name: "基础", moduleId: 1, moduleName: "文章模块", parentId: 10, parentName: "使用帮助"},
		{id: 20, name: "空栏目", moduleId: 2, moduleName: "产品模块"},
	}
	periods := []string{"2026-10", "2026-11"}
	rows := []archiveStatRow{
		{GroupId: 11, Period: "2026-10", Total: 6},
		{GroupId: 11, Period: "2026-11", Total: 4},
		{GroupId: 0, Period: "2026-11", Total: 2},    // 未分类
		{GroupId: 999, Period: "2026-10", Total: 99}, // 栏目已删除
	}

	got := assembleArchiveStatResult(ArchiveStatDimensionMonth, ArchiveStatGroupCategory,
		periods[0], periods[1], periods, entities, rows, true)

	if !got.MultiCategory {
		t.Fatal("多栏目标记应被带出")
	}
	if got.Total != 12 {
		t.Fatalf("总数应为 12（6+4+2），实际 %d", got.Total)
	}
	if len(got.Items) != 4 {
		t.Fatalf("行数应为 4（3 个栏目 + 未分类），实际 %d", len(got.Items))
	}
	// 第一行是文档最多的栏目，父栏目名要能拿到
	first := got.Items[0]
	if first.Id != 11 || first.ParentName != "使用帮助" || first.ModuleName != "文章模块" {
		t.Fatalf("栏目行信息不完整: %+v", first)
	}
	// 未分类行必须存在并被标记（Total=2 排在零行之前，所以不能假定它一定在最末）
	var unassigned *ArchiveStatItem
	for _, item := range got.Items {
		if item.Id == 0 {
			unassigned = item
		}
	}
	if unassigned == nil || !unassigned.Unassigned || unassigned.Total != 2 {
		t.Fatalf("未分类行不对: %+v", unassigned)
	}
	if _, exists := got.Items[0].Counts["2026-09"]; exists {
		t.Fatal("periods 之外的桶不应出现")
	}
}

// TestAssembleArchiveStatResultEmpty 无文档时也要返回完整骨架，前端才能渲染空表。
func TestAssembleArchiveStatResultEmpty(t *testing.T) {
	periods := []string{"2026-10", "2026-11"}
	entities := []archiveStatEntity{{id: 1, name: "文章"}}

	got := assembleArchiveStatResult(ArchiveStatDimensionMonth, ArchiveStatGroupModule,
		periods[0], periods[1], periods, entities, nil, false)

	if got.Total != 0 {
		t.Fatalf("无文档时总数应为 0，实际 %d", got.Total)
	}
	if len(got.Periods) != 2 || len(got.Items) != 1 {
		t.Fatalf("骨架不完整: periods=%v items=%d", got.Periods, len(got.Items))
	}
	for _, p := range periods {
		if v, ok := got.Totals[p]; !ok || v != 0 {
			t.Fatalf("周期 %q 应显式补 0，实际 %v (exists=%v)", p, v, ok)
		}
	}
}
