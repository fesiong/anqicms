package manageController

import (
	"time"

	"github.com/jinzhu/now"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

// StatisticSpider 获取蜘蛛爬行情况统计图表数据。
func StatisticSpider(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	result := currentSite.StatisticSpider()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// StatisticTraffic 获取当前站点的流量统计概览图表数据。
func StatisticTraffic(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	result := currentSite.StatisticTraffic()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// StatisticDates 获取当前站点可用的流量统计日期列表。
func StatisticDates(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	result := currentSite.GetStatisticDates()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// StatisticDetail 获取指定日期和类型的流量统计明细列表，支持分页。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
//   - 查询参数 "date": 统计日期。
//   - 查询参数 "type": 统计类型：spider=蜘蛛爬行情况，traffic=流量统计。
func StatisticDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	date := ctx.URLParam("date")
	searchType := ctx.URLParam("type")

	list, total, _ := currentSite.StatisticDetail(date, searchType, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  list,
	})
}

// GetSpiderIncludeDetail 获取搜索引擎蜘蛛收录明细列表，支持分页。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
func GetSpiderIncludeDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	var list []*model.SpiderInclude
	var total int64

	if currentPage < 1 {
		currentPage = 1
	}
	offset := (currentPage - 1) * pageSize

	builder := currentSite.DB.Model(&model.SpiderInclude{})

	builder.Count(&total).Limit(pageSize).Offset(offset).Order("`id` desc").Find(&list)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  list,
	})
}

// GetSpiderInclude 获取搜索引擎蜘蛛收录统计图表数据。
func GetSpiderInclude(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var result = make([]response.ChartData, 0, 30*5)

	timeStamp := now.BeginningOfDay().AddDate(0, 0, -30).Unix()

	var includeLogs []model.SpiderInclude
	currentSite.DB.Model(&model.SpiderInclude{}).Where("`created_time` >= ?", timeStamp).
		Order("created_time asc").
		Scan(&includeLogs)

	lastDate := ""
	for _, v := range includeLogs {
		date := time.Unix(v.CreatedTime, 0).Format("01-02")
		if date == lastDate {
			continue
		}
		lastDate = date
		result = append(result, response.ChartData{
			Date:  date,
			Label: ctx.Tr("Baidu"),
			Value: v.BaiduCount,
		}, response.ChartData{
			Date:  date,
			Label: ctx.Tr("Sogou"),
			Value: v.SogouCount,
		}, response.ChartData{
			Date:  date,
			Label: ctx.Tr("Soso"),
			Value: v.SoCount,
		}, response.ChartData{
			Date:  date,
			Label: ctx.Tr("Bing"),
			Value: v.BingCount,
		}, response.ChartData{
			Date:  date,
			Label: ctx.Tr("Google"),
			Value: v.GoogleCount,
		})
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// GetStatisticCoupon 统计优惠券的按天领取量与使用量。
//
// 参数说明：
//   - 查询参数 "start_time": 统计起始时间，日期字符串（如 2026-10-01；也可带时分秒）；为空或解析失败时取 30 天前，并按当天 00:00:00 截断。
//   - 查询参数 "end_time": 统计结束时间，格式同上；为空或解析失败时取当前时间，并按当天 23:59:59 截断。
func GetStatisticCoupon(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	start := ctx.URLParam("start_time")
	end := ctx.URLParam("end_time")
	startParse, err := now.Parse(start)
	if err != nil {
		// 30天
		startParse = time.Now().AddDate(0, 0, -30)
	}
	endParse, err := now.Parse(end)
	if err != nil {
		endParse = time.Now()
	}
	startTime := now.With(startParse).BeginningOfDay().Unix()
	endTime := now.With(endParse).EndOfDay().Unix()

	result := currentSite.StatisticCoupon(startTime, endTime)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// GetStatisticSearch 统计站内搜索的按天搜索次数。
//
// 参数说明：
//   - 查询参数 "start_time": 统计起始时间，日期字符串（如 2026-10-01；也可带时分秒）；为空或解析失败时取 30 天前，并按当天 00:00:00 截断。
//   - 查询参数 "end_time": 统计结束时间，格式同上；为空或解析失败时取当前时间，并按当天 23:59:59 截断。
func GetStatisticSearch(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	start := ctx.URLParam("start_time")
	end := ctx.URLParam("end_time")
	startParse, err := now.Parse(start)
	if err != nil {
		// 30天
		startParse = time.Now().AddDate(0, 0, -30)
	}
	endParse, err := now.Parse(end)
	if err != nil {
		endParse = time.Now()
	}
	startTime := now.With(startParse).BeginningOfDay().Unix()
	endTime := now.With(endParse).EndOfDay().Unix()

	result := currentSite.StatisticSearch(startTime, endTime)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// GetStatisticSearchDetail 分页查询站内搜索日志明细（不按时间过滤）。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
func GetStatisticSearchDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	result, total := currentSite.StatisticSearchDetail(currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"data":  result,
		"total": total,
	})
}

// GetStatisticCustomer 统计按天的新注册用户数与登录次数。
//
// 参数说明：
//   - 查询参数 "start_time": 统计起始时间，日期字符串（如 2026-10-01；也可带时分秒）；为空或解析失败时取 30 天前，并按当天 00:00:00 截断。
//   - 查询参数 "end_time": 统计结束时间，格式同上；为空或解析失败时取当前时间，并按当天 23:59:59 截断。
func GetStatisticCustomer(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	start := ctx.URLParam("start_time")
	end := ctx.URLParam("end_time")
	startParse, err := now.Parse(start)
	if err != nil {
		// 30天
		startParse = time.Now().AddDate(0, 0, -30)
	}
	endParse, err := now.Parse(end)
	if err != nil {
		endParse = time.Now()
	}
	startTime := now.With(startParse).BeginningOfDay().Unix()
	endTime := now.With(endParse).EndOfDay().Unix()

	result := currentSite.StatisticCustomer(startTime, endTime)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// GetStatisticProduct 统计按天的商品上新数与商品浏览量。
//
// 参数说明：
//   - 查询参数 "start_time": 统计起始时间，日期字符串（如 2026-10-01；也可带时分秒）；为空或解析失败时取 30 天前，并按当天 00:00:00 截断。
//   - 查询参数 "end_time": 统计结束时间，格式同上；为空或解析失败时取当前时间，并按当天 23:59:59 截断。
func GetStatisticProduct(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	start := ctx.URLParam("start_time")
	end := ctx.URLParam("end_time")
	startParse, err := now.Parse(start)
	if err != nil {
		// 30天
		startParse = time.Now().AddDate(0, 0, -30)
	}
	endParse, err := now.Parse(end)
	if err != nil {
		endParse = time.Now()
	}
	startTime := now.With(startParse).BeginningOfDay().Unix()
	endTime := now.With(endParse).EndOfDay().Unix()

	result := currentSite.StatisticProduct(startTime, endTime)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// GetStatisticOrder 统计按天的下单数、支付笔数与支付金额。
//
// 参数说明：
//   - 查询参数 "start_time": 统计起始时间，日期字符串（如 2026-10-01；也可带时分秒）；为空或解析失败时取 30 天前，并按当天 00:00:00 截断。
//   - 查询参数 "end_time": 统计结束时间，格式同上；为空或解析失败时取当前时间，并按当天 23:59:59 截断。
func GetStatisticOrder(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	start := ctx.URLParam("start_time")
	end := ctx.URLParam("end_time")
	startParse, err := now.Parse(start)
	if err != nil {
		// 30天
		startParse = time.Now().AddDate(0, 0, -30)
	}
	endParse, err := now.Parse(end)
	if err != nil {
		endParse = time.Now()
	}
	startTime := now.With(startParse).BeginningOfDay().Unix()
	endTime := now.With(endParse).EndOfDay().Unix()

	result := currentSite.StatisticOrder(startTime, endTime)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// GetStatisticConversion 统计按天的转化漏斗（点击/加购/结算/确认/支付）。
//
// 参数说明：
//   - 查询参数 "start_time": 统计起始时间，日期字符串（如 2026-10-01；也可带时分秒）；为空或解析失败时取 30 天前，并按当天 00:00:00 截断。
//   - 查询参数 "end_time": 统计结束时间，格式同上；为空或解析失败时取当前时间，并按当天 23:59:59 截断。
func GetStatisticConversion(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	start := ctx.URLParam("start_time")
	end := ctx.URLParam("end_time")
	startParse, err := now.Parse(start)
	if err != nil {
		// 30天
		startParse = time.Now().AddDate(0, 0, -30)
	}
	endParse, err := now.Parse(end)
	if err != nil {
		endParse = time.Now()
	}
	startTime := now.With(startParse).BeginningOfDay().Unix()
	endTime := now.With(endParse).EndOfDay().Unix()

	result := currentSite.StatisticConversion(startTime, endTime)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// GetStatisticLive 实时看板数据（销售额、平均订单金额等，按天分组）。
//
// 参数说明：
//   - 查询参数 "start_time": 统计起始时间，日期字符串（如 2026-10-01；也可带时分秒）；为空或解析失败时取 30 天前，并按当天 00:00:00 截断。
//   - 查询参数 "end_time": 统计结束时间，格式同上；为空或解析失败时取当前时间，并按当天 23:59:59 截断。
func GetStatisticLive(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	start := ctx.URLParam("start_time")
	end := ctx.URLParam("end_time")
	startParse, err := now.Parse(start)
	if err != nil {
		// 30天
		startParse = time.Now().AddDate(0, 0, -30)
	}
	endParse, err := now.Parse(end)
	if err != nil {
		endParse = time.Now()
	}
	startTime := now.With(startParse).BeginningOfDay().Unix()
	endTime := now.With(endParse).EndOfDay().Unix()

	result := currentSite.StatisticLive(startTime, endTime)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}
