package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/fate"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func ZodiacIndex(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	lastDetails, _, _ := currentSite.GetNameDetailList(1, 22, "")
	ctx.ViewData("lastDetails", lastDetails)

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = "十二生肖_十二生肖运程_十二生肖起名_十二生肖有哪些"
		webInfo.Keywords = "十二生肖,十二生肖运程,十二生肖起名,十二生肖有哪些"
		webInfo.Description = "十二生肖，即鼠、牛、虎、兔、龙、蛇、马、羊、猴、鸡、狗、猪，用于记年。是中国传统文化的重要部分，顺序排列为子鼠、丑牛、寅虎、卯兔、辰龙、巳蛇、午马、未羊、申猴、酉鸡、戌狗、亥猪。每个人都知道自己的生肖是什么，但未必是准确的。中国黄历以立春确定生肖。“农历”是汉代开始使用的“太阴历”，新年是以立春为界的，立春是二十四节气之首，生肖以立春为准。传统的命理学、占卜学等民俗学，均以“立春”作为生肖计算的依据。包括现在的民间占星卜卦先生也一直沿用传统的生肖定法。因为生肖本与地支同源，不能以当前的春节来定。"
		//设置页面名称，方便tags识别
		webInfo.PageName = "zodiac"
		webInfo.CanonicalUrl = currentSite.GetUrl("/zodiac", nil, 0)
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
	ctx.View("zodiac/index.html")
}

func ZodiacDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := ctx.Params().GetString("id")
	zodiac := fate.GetZodiacById(id)
	if zodiac == nil {
		NotFound(ctx)
		return
	}

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = fmt.Sprintf("属%s的人取名用什么字_生肖属%s的人宜用什么字_生肖属%s的人不能用什么字", zodiac.Name, zodiac.Name, zodiac.Name)
		webInfo.Keywords = fmt.Sprintf("属%s的人取名用什么字,生肖属%s的人宜用什么字,生肖属%s的人不能用什么字", zodiac.Name, zodiac.Name, zodiac.Name)
		webInfo.Description = fmt.Sprintf("生肖属%s的人取名宜用字解释，忌用字解释", zodiac.Name)
		//设置页面名称，方便tags识别
		webInfo.PageName = "zodiac"
		webInfo.CanonicalUrl = currentSite.GetUrl("/zodiac/"+zodiac.Id, nil, 0)
		ctx.ViewData("webInfo", webInfo)
	}

	ctx.ViewData("zodiac", zodiac)

	inApi, _ := ctx.Values().GetBool("inApi")
	if inApi {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": ctx.GetViewData(),
		})
		return
	}
	ctx.View("zodiac/detail.html")
}
