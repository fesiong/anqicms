package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fesiong/yi"
	"github.com/godcong/chronos"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/fate"
	fateConfig "kandaoni.com/anqicms/fate/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strconv"
	"strings"
)

func ApiNameCharacter(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := strings.TrimSpace(ctx.URLParam("id"))
	if id == "" {
		// 返回列表
		totalCharacters, _ := currentSite.GetCharacterList("total", 100)
		maleCharacters, _ := currentSite.GetCharacterList("male", 100)
		femaleCharacters, _ := currentSite.GetCharacterList("female", 100)

		var totalTitles []string
		var maleTitles []string
		var femaleTitles []string
		for _, v := range totalCharacters {
			totalTitles = append(totalTitles, v.Ch)
		}
		for _, v := range maleCharacters {
			maleTitles = append(maleTitles, v.Ch)
		}
		for _, v := range femaleCharacters {
			femaleTitles = append(femaleTitles, v.Ch)
		}

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": iris.Map{
				"totalCharacters":  totalTitles,
				"maleCharacters":   maleTitles,
				"femaleCharacters": femaleTitles,
			},
		})
		return
	}

	character, err := currentSite.GetCharacterByCh(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "查无此字",
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": character,
	})
}

func ApiNameSurname(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := strings.TrimSpace(ctx.URLParam("id"))
	if id == "" {
		// 返回列表
		surnames, _ := currentSite.GetSurnameList()

		var names []string
		for _, v := range surnames {
			names = append(names, v.Title)
		}

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": names,
		})
		return
	}
	surname, err := currentSite.GetSurnameByTitle(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "查无此姓氏",
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": surname,
	})
}

// ApiNameFortune 八字
func ApiNameFortune(ctx iris.Context) {
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
	bazi := fate.NewBazi(born)
	lunarDate := fate.NewLunarDate(born)
	var solarDate = map[string]string{
		"year":   born.Solar().Time().Format("2006"),
		"month":  born.Solar().Time().Format("01"),
		"day":    born.Solar().Time().Format("02"),
		"hour":   born.Solar().Time().Format("15"),
		"minute": born.Solar().Time().Format("04"),
	}

	baziData := parseBazi(bazi)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"bazi":      baziData,
			"lunarDate": lunarDate,
			"solarDate": solarDate,
		},
	})
}

func ApiNameDetail(ctx iris.Context) {
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
	// 违禁词检测
	matches := currentSite.MatchSensitiveWords(req.FullName)
	if len(matches) > 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "填写内容不合法",
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
		_, err = currentSite.GetSurnameByTitle(lastName)
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
			bornDate = fmt.Sprintf("%s-%s-%s", req.BornYear, req.BornMonth, req.BornDay)
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

	f := fate.NewFate(lastName, born.Solar().Time())
	f.SetGender(req.Gender)
	f.CheckName(firstName)
	//获取姓氏信息
	var surnameText string
	surname, err := currentSite.GetSurnameByTitle(lastName)
	if err == nil {
		surnameText = surname.Description
	}

	var solarDate = map[string]string{
		"year":   born.Solar().Time().Format("2006"),
		"month":  born.Solar().Time().Format("01"),
		"day":    born.Solar().Time().Format("02"),
		"hour":   born.Solar().Time().Format("15"),
		"minute": born.Solar().Time().Format("04"),
	}

	baziData := parseBazi(f.Name.BaZi())

	baGua := f.BaGua()
	guaXiang := []*yi.GuaXiang{
		baGua.Get(0),
		baGua.Get(1),
		baGua.Get(2),
		baGua.Get(3),
		baGua.Get(4),
	}
	lunarDate := fate.NewLunarDate(f.Born)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"name":    parseName(f.Name, true),
			"surname": surnameText,
			"sanCai":  f.SanCai(),
			"zodiac": map[string]string{
				"name":      f.Name.Zodiac().Name,
				"xiRadical": f.Name.Zodiac().XiRadical,
			},
			"bazi":      baziData,
			"lunarDate": lunarDate,
			"solarDate": solarDate,
			"star":      f.Star,
			"guaXiang":  guaXiang,
		},
	})
}

func ApiNameChoose(ctx iris.Context) {
	page := ctx.URLParamIntDefault("page", 1)
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
	if provider.QQMsgSecCheck(req.LastName) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "提交内容含有违法违规信息",
		})
		return
	}
	surname, err := currentSite.GetSurnameByTitle(req.LastName)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "暂不支持您填写的姓氏",
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

	result, e := f.MakeName(context.Background())
	if e != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  e.Error(),
		})
		return
	}

	//将数据写入到表中
	body, _ := json.Marshal(req)
	md5Hash := library.Md5Bytes(body)
	md5Hash = md5Hash[8:24]

	total := len(result)
	limit := 10
	offset := (page - 1) * limit
	nextOffset := offset + limit
	var list []*fate.Name
	if total >= nextOffset {
		list = result[offset:nextOffset]
	} else if total > offset {
		list = result[offset:]
	} else {
		//没有数据
		list = result[0:0]
	}
	var nameList []map[string]interface{}
	for _, v := range list {
		nameList = append(nameList, parseName(v, false))
	}
	// 更多名字，200个，只显示姓名
	var moreList = make([]string, 0, 200)
	for i, v := range result {
		if i < 10 {
			continue
		}
		if i >= 200 {
			break
		}
		moreList = append(moreList, v.FullName())
	}
	var firstName = &fate.Name{}
	if total > 0 {
		firstName = result[0]
	}

	req.PositionName = req.GetPosition()
	req.SourceName = req.GetSource()

	var solarDate = map[string]string{
		"year":   born.Solar().Time().Format("2006"),
		"month":  born.Solar().Time().Format("01"),
		"day":    born.Solar().Time().Format("02"),
		"hour":   born.Solar().Time().Format("15"),
		"minute": born.Solar().Time().Format("04"),
	}
	lunarDate := fate.NewLunarDate(f.Born)

	//获取姓氏信息
	surnameText := surname.Description
	//baGua := f.BaGua()
	//guaXiang := []*yi.GuaXiang{
	//	baGua.Get(0),
	//	baGua.Get(1),
	//	baGua.Get(2),
	//	baGua.Get(3),
	//	baGua.Get(4),
	//}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"firstName": firstName.FullName(),
			"nameList":  nameList,
			"moreList":  moreList, // 仅仅显示姓名
			"total":     total,
			//"bazi":      parseBazi(f.BaZi),
			"zodiac": map[string]string{
				"name":      f.Zodiac.Name,
				"xiRadical": f.Zodiac.XiRadical,
			},
			"detail":    req,
			"lunarDate": lunarDate,
			"solarDate": solarDate,
			"surname":   surnameText,
			"lastChar":  parseCharacters(f.LastChar),
			"star":      f.Star,
			//"sanCai":    f.SanCai(),
			//"guaXiang": guaXiang,
		},
	})
	//
	//var ms runtime.MemStats
	//runtime.ReadMemStats(&ms)
	//fmt.Println("mem usage", ms.Alloc/1024/1024)
}

func parseBazi(bazi *fate.BaZi) map[string]interface{} {
	baziData := map[string]interface{}{
		"result":  bazi.String(),
		"shiChen": bazi.ShiChen,
		"tiangan": bazi.RiZhuWuXing(),
		//"bazi": []string{
		//	bazi.GetBaZi(0),
		//	bazi.GetBaZi(1),
		//	bazi.GetBaZi(2),
		//	bazi.GetBaZi(3),
		//},
		//"wuxing": []string{
		//	bazi.GetWuXing(0),
		//	bazi.GetWuXing(1),
		//	bazi.GetWuXing(2),
		//	bazi.GetWuXing(3),
		//},
		//"nayin": []string{
		//	bazi.GetNaYin(0),
		//	bazi.GetNaYin(1),
		//	bazi.GetNaYin(2),
		//	bazi.GetNaYin(3),
		//},
		//"wuxing_percent": map[string]int{
		//	"金": bazi.XiYong().GetWuXing("金"),
		//	"木": bazi.XiYong().GetWuXing("木"),
		//	"水": bazi.XiYong().GetWuXing("水"),
		//	"火": bazi.XiYong().GetWuXing("火"),
		//	"土": bazi.XiYong().GetWuXing("土"),
		//},
		//"wuxing_number": map[string]int{
		//	"金": bazi.XiYong().GetWuXingNum("金"),
		//	"木": bazi.XiYong().GetWuXingNum("木"),
		//	"水": bazi.XiYong().GetWuXingNum("水"),
		//	"火": bazi.XiYong().GetWuXingNum("火"),
		//	"土": bazi.XiYong().GetWuXingNum("土"),
		//},
		//"xiyong": map[string]interface{}{
		//	"wang":       bazi.XiYong().GetWang(),
		//	"que":        bazi.XiYong().GetQue(),
		//	"tong":       bazi.XiYong().GetSimilar(),
		//	"tong_point": bazi.XiYong().GetSimilarPoint(),
		//	"yi":         bazi.XiYong().GetHeterogeneous(),
		//	"yi_point":   bazi.XiYong().GetHeterogeneousPoint(),
		//	"zong_point": bazi.XiYong().GetZongPoint(),
		//	"qiangruo":   bazi.XiYong().GetQiangRuo(),
		//	"shen":       bazi.XiYongShen(),
		//},
		"wuxing_text": bazi.GetWuXingString(),
	}

	return baziData
}

func parseName(name *fate.Name, full bool) map[string]interface{} {
	nameData := map[string]interface{}{
		"fullName":   name.FullName(),
		"gender":     name.Gender(),
		"totalScore": name.TotalScore(),
		"lastName":   parseCharacters(name.LastName),
		"firstName":  parseCharacters(name.FirstName),
		"zodiac": map[string]string{
			"name":      name.Zodiac().Name,
			"xiRadical": name.Zodiac().XiRadical,
		},
		"ciyu": name.CiYu(),
		"scores": map[string]int{
			"good":   name.GetScore(0),
			"bazi":   name.GetScore(1),
			"zodiac": name.GetScore(2),
			"star":   name.GetScore(3),
			"wuge":   name.GetScore(4),
			"yi":     name.GetScore(5),
		},
	}

	if full {
		//	nameData["daYan"] = name.DaYan()
	}

	return nameData
}

func parseCharacters(chs []*model.Character) []map[string]interface{} {
	var data []map[string]interface{}
	for _, ch := range chs {
		datum := map[string]interface{}{
			"ch":             ch.Ch,
			"science_stroke": ch.ScienceStroke,
			"pin_yin":        ch.GetPinYin(),
			"radical":        ch.Radical,
			"stroke":         ch.Stroke,
			"kang_xi":        ch.KangXi,
			"wu_xing":        ch.WuXing,
			"lucky":          ch.Lucky,
			"regular":        ch.Regular,
		}
		if len([]string(ch.Comment)) > 0 {
			datum["description"] = ch.Comment[0]
		}
		data = append(data, datum)
	}

	return data
}
