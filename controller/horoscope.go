package controller

import (
	"fmt"
	"github.com/godcong/chronos"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/fate"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func HoroscopeIndex(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	lastHoroscopes, _, _ := currentSite.GetHoroscopeList(1, 12, "")
	ctx.ViewData("lastHoroscopes", lastHoroscopes)

	lastDetails, _, _ := currentSite.GetNameDetailList(1, 22, "")
	ctx.ViewData("lastDetails", lastDetails)

	ctx.ViewData("today", library.GetToday())

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = "查生辰八字_八字喜用神查询_五行八字分析_八字起名注意事项"
		webInfo.Keywords = "查生辰八字,八字喜用神查询,五行八字分析,八字起名注意事项"
		webInfo.Description = "明泽起名网提供生辰八字、五行四柱、八字喜用神、五行纳音、天干地支、八字起名注意事项等查询。"
		//设置页面名称，方便tags识别
		webInfo.PageName = "horoscope"
		webInfo.CanonicalUrl = currentSite.GetUrl("/horoscope", nil, 0)
		ctx.ViewData("webInfo", webInfo)
	}

	inApi, _ := ctx.Values().GetBool("inApi")
	if inApi {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": ctx.GetViewData(),
		})
		return
	}
	ctx.View("horoscope/index.html")
}

func CreateHoroscope(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.NameHoroscope
	err := ctx.ReadJSON(&req)
	if err != nil {
		//记录
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	//检查日期

	//检查选择的农历
	if req.Calendar == "lunar" {
		//获取时间
		//转换成阳历
		req.Calendar = "solar"
		bornDate := fmt.Sprintf("%s-%s-%s", req.BornYear, req.BornMonth, req.BornDay)
		//查找对应的阳历日期记录
		leap, _ := strconv.Atoi(req.Leap)
		calendar, err := currentSite.GetCalendarByLunar(bornDate, leap)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "农历日期超出可查询范围",
			})
			return
		}
		//修正闰月
		req.Leap = strconv.Itoa(calendar.LunarLeap)
		solarDate := strings.Split(calendar.SolarDate, "-")
		req.BornYear = solarDate[0]
		req.BornMonth = solarDate[1]
		req.BornDay = solarDate[2]
	}

	if req.BornMinute == "" {
		req.BornMinute = "00"
	}
	if req.BornHour == "" {
		req.BornHour = "00"
	}
	bornDate := fmt.Sprintf("%s/%s/%s %s:%s", req.BornYear, req.BornMonth, req.BornDay, req.BornHour, req.BornMinute)

	born := chronos.New(bornDate)
	bornTime := int(born.Solar().Time().Unix())

	md5Hash := library.Md5(fmt.Sprintf("%d", bornTime))
	md5Hash = md5Hash[8:24]

	exists, err := currentSite.GetHoroscopeByHash(md5Hash)
	if err != nil {
		//不存在，插入一条
		exists = &model.Horoscope{
			Hash: md5Hash,
			Born: bornTime,
		}
		currentSite.DB.Save(exists)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": exists.Hash,
	})
}

func HoroscopeDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := ctx.Params().GetString("id")
	if id == "" {
		NotFound(ctx)
		return
	}
	horoscope, err := currentSite.GetHoroscopeByHash(id)
	if err != nil {
		NotFound(ctx)
		return
	}

	horoscopeRelated, _ := currentSite.GetHoroscopeRelated(horoscope.Id, 12)
	ctx.ViewData("horoscopeRelated", horoscopeRelated)

	born := chronos.New(time.Unix(int64(horoscope.Born), 0))
	bazi := fate.NewBazi(born)
	lunarDate := fate.NewLunarDate(born)

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = fmt.Sprintf("农历%s%s出生宝宝生辰八字_公历%s出生宝宝生辰八字_喜用神是%s", born.LunarDate(), bazi.ShiChen, born.Solar().Time().Format("2006年01月02日15点"), bazi.XiYong().Shen())
		webInfo.Keywords = fmt.Sprintf("公历%s%s出生宝宝生辰八字,农历%s出生宝宝生辰八字,喜用神是%s", born.LunarDate(), bazi.ShiChen, born.Solar().Time().Format("2006年01月02日15点"), bazi.XiYong().Shen())
		re, _ := regexp.Compile("\\<[\\S\\s]+?\\>")
		webInfo.Description = re.ReplaceAllString(bazi.String(), "")
		//设置页面名称，方便tags识别
		webInfo.PageName = "horoscope"
		webInfo.CanonicalUrl = currentSite.GetUrl("/horoscope/"+horoscope.Hash, nil, 0)
		ctx.ViewData("webInfo", webInfo)
	}

	ctx.ViewData("horoscope", horoscope)
	ctx.ViewData("born", born)
	ctx.ViewData("bazi", bazi)
	ctx.ViewData("lunarDate", lunarDate)

	inApi, _ := ctx.Values().GetBool("inApi")
	if inApi {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": ctx.GetViewData(),
		})
		return
	}
	ctx.View("horoscope/detail.html")
}
