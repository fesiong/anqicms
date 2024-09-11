package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/godcong/chronos"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/fate"
	fateConfig "kandaoni.com/anqicms/fate/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"strconv"
	"strings"
	"time"
)

func CreateName(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.NameRequest
	err := ctx.ReadJSON(&req)
	if err != nil {
		//记录
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.LastName == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "请填写你的姓氏",
		})
		return
	}
	// 违禁词检测
	matches := currentSite.MatchSensitiveWords(req.LastName)
	if len(matches) > 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "填写内容不合法",
		})
		return
	}
	//检查选择的农历
	if req.Calendar == "lunar" {
		//获取时间
		//转换成阳历
		req.Calendar = "solar"
		if req.KnowBorn == "time" || req.KnowBorn == "date" {
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
	}

	//将数据写入到表中
	body, _ := json.Marshal(req)
	md5Hash := library.Md5Bytes(body)
	md5Hash = md5Hash[8:24]
	//验证有没有提交过数据
	cacheData := fate.FateCache.Get(md5Hash)
	if cacheData != nil {
		// 已存在，更新时间
		fate.FateCache.Set(md5Hash, cacheData)
	} else {
		//检查日期
		bornDate := ""
		if req.KnowBorn == "time" {
			if req.BornMinute == "" {
				req.BornMinute = "00"
			}
			if req.BornHour == "" {
				req.BornHour = "00"
			}
			bornDate = fmt.Sprintf("%s/%s/%s %s:%s", req.BornYear, req.BornMonth, req.BornDay, req.BornHour, req.BornMinute)
		} else if req.KnowBorn == "date" {
			bornDate = fmt.Sprintf("%s/%s/%s 00:00", req.BornYear, req.BornMonth, req.BornDay)
		} else {
			//不知道时间
			bornDate = "2020/06/01 00:00"
		}

		born := chronos.New(bornDate)

		cfg := fateConfig.DefaultConfig()
		f := fate.NewFate(req.LastName, born.Solar().Time(), fate.ConfigOption(cfg))
		f.SetGender(req.Gender)
		f.Position = req.Position
		f.NameType = req.NameType
		if req.OnlyKangxi == "on" {
			f.OnlyKangxi = true
		}
		f.TabooCharacter = req.TabooCharacter
		f.TabooCharacter = req.TabooSide
		tmpSource := fmt.Sprintf("%v", req.SourceFrom)
		sourceFrom, _ := strconv.Atoi(tmpSource)
		f.SourceFrom = uint(sourceFrom)

		names, e := f.MakeName(context.Background())
		if e != nil {
			NotFound(ctx)
			return
		}
		// 缓存起来
		cacheData = &fate.CacheData{
			Req:   req,
			Fate:  f,
			Names: names,
		}
		fate.FateCache.Set(md5Hash, cacheData)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": md5Hash,
	})
}

func NameChoose(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := ctx.URLParam("id")

	cacheData := fate.FateCache.Get(id)
	if cacheData == nil {
		NotFound(ctx)
		return
	}

	ajax := ctx.URLParam("ajax")
	page := ctx.URLParamIntDefault("page", 1)
	ctx.ViewData("page", page)

	total := len(cacheData.Names)
	limit := 10
	offset := (page - 1) * limit
	nextOffset := offset + limit
	var list []*fate.Name
	if total >= nextOffset {
		list = cacheData.Names[offset:nextOffset]
	} else if total > offset {
		list = cacheData.Names[offset:]
	} else {
		//没有数据
		list = cacheData.Names[0:0]
	}

	var firstList []*fate.Name
	var moreList []*fate.Name
	if total >= 40 {
		moreList = cacheData.Names[20:40]
	} else if total > 20 {
		moreList = cacheData.Names[20:]
	}
	if total >= 20 {
		firstList = cacheData.Names[:20]
	} else {
		firstList = cacheData.Names
	}

	var firstName *fate.Name
	if total > 0 {
		firstName = cacheData.Names[0]
	}

	ctx.ViewData("firstName", firstName)
	ctx.ViewData("nameList", list)
	ctx.ViewData("firstList", firstList)
	ctx.ViewData("moreList", moreList)

	if ajax != "" {
		ctx.View("partial/namelist.html")
		return
	}

	if page > 1 && len(list) == 0 {
		NotFound(ctx)
		return
	}

	//获取姓氏信息
	surname, _ := currentSite.GetSurnameByTitle(cacheData.Req.LastName)

	baGua := cacheData.Fate.BaGua()
	ctx.ViewData("total", total)
	ctx.ViewData("wuxing", cacheData.Fate.XiYong())
	ctx.ViewData("bazi", cacheData.Fate.BaZi)
	ctx.ViewData("zodiac", cacheData.Fate.Zodiac)
	cacheData.Req.PositionName = cacheData.Req.GetPosition()
	cacheData.Req.SourceName = cacheData.Req.GetSource()
	ctx.ViewData("parseContent", cacheData.Req)
	ctx.ViewData("born", cacheData.Fate.Born)
	ctx.ViewData("surname", surname)
	ctx.ViewData("lastChar", cacheData.Fate.LastChar)
	ctx.ViewData("star", cacheData.Fate.Star)
	ctx.ViewData("sanCai", cacheData.Fate.SanCai())
	ctx.ViewData("baGua", baGua)
	guaMing := []string{
		baGua.Get(0).GuaMing,
		baGua.Get(1).GuaMing,
		baGua.Get(2).GuaMing,
		baGua.Get(3).GuaMing,
		baGua.Get(4).GuaMing,
	}

	ctx.ViewData("guaMing", strings.Join(guaMing, "、"))
	lunarDate := fate.NewLunarDate(cacheData.Fate.Born)
	ctx.ViewData("lunarDate", lunarDate)

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = fmt.Sprintf("%s出生姓%s男宝宝女宝宝取名周易起名_姓%s宝宝生辰八字起名", cacheData.Fate.Born.LunarDate(), cacheData.Req.LastName, cacheData.Req.LastName)
		webInfo.Keywords = fmt.Sprintf("姓%s宝宝起名,姓%s孩子起名,姓%s男宝宝起名,姓%s女宝宝取名,姓%s男孩取名,姓%s女孩取名", cacheData.Req.LastName, cacheData.Req.LastName, cacheData.Req.LastName, cacheData.Req.LastName, cacheData.Req.LastName, cacheData.Req.LastName)
		webInfo.Description = "明泽网智能起名系统综合了网上流行的五格起名，生辰八字起名，周易卦象起名，生肖起名，以及现代的音形义起名为一体的多维综合智能起名系统，并加入了真太阳时较对，让孩子起名更准确，更好听更吉祥。"
		//设置页面名称，方便tags识别
		webInfo.PageName = "name"
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
	ctx.View("name/choose.html")
}

func NameCheckout(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	lastDetails, _, _ := currentSite.GetNameDetailList(1, 22, "")
	ctx.ViewData("lastDetails", lastDetails)

	//八字查询
	lastHoroscopes, _, _ := currentSite.GetHoroscopeList(1, 12, "")
	ctx.ViewData("lastHoroscopes", lastHoroscopes)

	ctx.ViewData("today", library.GetToday())

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = "姓名智能测试_姓名打分_姓名解释_姓名五行解读_姓名八字解读"
		webInfo.Keywords = "姓名智能测试,姓名打分,姓名解释,姓名五行解读,姓名八字解读"
		webInfo.Description = "明泽网智能起名系统提供免费的姓名测试、姓名五行解读、姓名八字解读服务。"
		//设置页面名称，方便tags识别
		webInfo.PageName = "name"
		webInfo.CanonicalUrl = currentSite.GetUrl("/checkout", nil, 0)
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
	ctx.View("name/checkout.html")
}

func CreateNameDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.NameCheckoutRequest
	err := ctx.ReadJSON(&req)
	if err != nil {
		//记录
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	nameRune := []rune(req.FullName)
	nameLen := len(nameRune)
	if nameLen < 2 || nameLen > 4 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "姓名只支持2-4个字",
		})
		return
	}
	//提取姓
	lastName := ""
	firstName := ""
	if nameLen == 2 {
		lastName = string(nameRune[:1])
		firstName = string(nameRune[1:2])
	} else if nameLen == 4 {
		lastName = string(nameRune[:2])
		firstName = string(nameRune[2:4])
	} else {
		lastName = string(nameRune[:2])
		firstName = string(nameRune[2:3])
		_, err := currentSite.GetSurnameByTitle(lastName)
		if err != nil {
			//表示不存在复姓
			lastName = string(nameRune[:1])
			firstName = string(nameRune[1:3])
		}
	}
	//检查日期
	bornDate := ""

	//检查选择的农历
	if req.Calendar == "lunar" {
		//获取时间
		//转换成阳历
		req.Calendar = "solar"
		if req.KnowBorn == "time" || req.KnowBorn == "date" {
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
	}

	if req.KnowBorn == "time" {
		if req.BornMinute == "" {
			req.BornMinute = "00"
		}
		if req.BornHour == "" {
			req.BornHour = "00"
		}
		bornDate = fmt.Sprintf("%s/%s/%s %s:%s", req.BornYear, req.BornMonth, req.BornDay, req.BornHour, req.BornMinute)
	} else if req.KnowBorn == "date" {
		bornDate = fmt.Sprintf("%s/%s/%s 00:00", req.BornYear, req.BornMonth, req.BornDay)
	} else {
		//不知道时间
		bornDate = "2020/06/01 00:00"
	}

	born := chronos.New(bornDate)
	bornTime := int(born.Solar().Time().Unix())

	md5Hash := library.Md5(fmt.Sprintf("%s-%s-%s-%d", lastName, firstName, req.Gender, bornTime))
	md5Hash = md5Hash[8:24]

	exists, err := currentSite.GetNameDetailByHash(md5Hash)
	if err != nil {
		//不存在，插入一条
		exists = &model.NameDetail{
			Hash:      md5Hash,
			LastName:  lastName,
			FirstName: firstName,
			Gender:    req.Gender,
			Born:      bornTime,
		}
		currentSite.DB.Save(exists)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": exists.Hash,
	})
}

func NameDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := ctx.Params().GetString("id")
	detail, err := currentSite.GetNameDetailByHash(id)
	if err != nil {
		NotFound(ctx)
		return
	}
	detail.AddViews(currentSite.DB)

	calendar := chronos.New(time.Unix(int64(detail.Born), 0))
	f := fate.NewFate(detail.LastName, calendar.Solar().Time())
	f.SetGender(detail.Gender)
	f.CheckName(detail.FirstName)
	fullName := f.Name.FullName()
	//获取姓氏信息
	surname, err := currentSite.GetSurnameByTitle(detail.LastName)
	ctx.ViewData("detail", detail)
	ctx.ViewData("surname", surname)
	ctx.ViewData("born", calendar)
	ctx.ViewData("name", f.Name)
	ctx.ViewData("bazi", f.Name.BaZi())
	ctx.ViewData("zodiac", f.Name.Zodiac())

	baGua := f.BaGua()
	ctx.ViewData("wuxing", f.XiYong())
	ctx.ViewData("lastChar", f.LastChar)
	ctx.ViewData("star", f.Star)
	ctx.ViewData("sanCai", f.SanCai())
	ctx.ViewData("baGua", baGua)
	ctx.ViewData("daYan", f.Name.DaYan())
	guaMing := []string{
		baGua.Get(0).GuaMing,
		baGua.Get(1).GuaMing,
		baGua.Get(2).GuaMing,
		baGua.Get(3).GuaMing,
		baGua.Get(4).GuaMing,
	}
	//for _, v := range result {
	//	fmt.Println(v.WuGe().WuGeScore())
	//}
	//fmt.Println(fate.GetYiScore(baGua))
	ctx.ViewData("guaMing", strings.Join(guaMing, "、"))
	lunarDate := fate.NewLunarDate(f.Born)
	ctx.ViewData("lunarDate", lunarDate)

	detailRelated, _ := currentSite.GetNameDetailRelated(detail.Id, 12)
	ctx.ViewData("detailRelated", detailRelated)

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = fmt.Sprintf("%s名字解释_%s姓名测试打分_%s生辰八字解释", fullName, fullName, fullName)
		webInfo.Keywords = fmt.Sprintf("%s名字解释,%s姓名测试打分,%s生辰八字解释", fullName, fullName, fullName)
		webInfo.Description = f.Name.String()
		//设置页面名称，方便tags识别
		webInfo.PageName = "name"
		webInfo.CanonicalUrl = currentSite.GetUrl("/detail/"+detail.Hash, nil, 0)
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
	ctx.View("name/detail.html")
}
