package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func SurnameIndex(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	surnames, _ := currentSite.GetSurnameList()

	ctx.ViewData("surnames", surnames)

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = "百家姓有哪些_百家姓起名_百家姓"
		webInfo.Keywords = "百家姓有哪些,百家姓起名,百家姓排名"
		webInfo.Description = "《百家姓》是一部关于中文姓氏的作品。按文献记载，成文于北宋初。原收集姓氏411个，后增补到504个，其中单姓444个，复姓60个。《百家姓》采用四言体例，对姓氏进行了排列，而且句句押韵，虽然它的内容没有文理，但对于中国姓氏文化的传承、中国文字的认识等方面都起了巨大作用，这也是能够流传千百年的一个重要因素。《百家姓》与《三字经》、《千字文》并称“三百千”，是中国古代幼儿的启蒙读物。“赵钱孙李”成为《百家姓》前四姓是因为百家姓形成于宋朝，故而宋朝皇帝的赵氏、吴越国国王钱俶、正妃孙氏以及南唐国主李氏成为百家姓前四位"
		//设置页面名称，方便tags识别
		webInfo.PageName = "surname"
		webInfo.CanonicalUrl = currentSite.GetUrl("/surname", nil, 0)
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
	ctx.View("surname/index.html")
}

func SuanameDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := ctx.Params().GetString("id")
	if id == "" {
		NotFound(ctx)
		return
	}
	surname, err := currentSite.GetSurnameByHash(id)
	if err != nil {
		NotFound(ctx)
		return
	}

	relatedSurnames, err := currentSite.GetSurnameRelated(surname.Id, 20)
	ctx.ViewData("relatedSurnames", relatedSurnames)

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = fmt.Sprintf("姓%s男宝宝女宝宝取名周易起名_姓%s宝宝生辰八字起名", surname.Title, surname.Title)
		webInfo.Keywords = surname.Keywords
		webInfo.Description = surname.Description
		//设置页面名称，方便tags识别
		webInfo.PageName = "surname"
		webInfo.CanonicalUrl = currentSite.GetUrl("/surname/"+surname.Hash, nil, 0)
		ctx.ViewData("webInfo", webInfo)
	}

	ctx.ViewData("surname", surname)

	inApi, _ := ctx.Values().GetBool("inApi")
	if inApi {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": ctx.GetViewData(),
		})
		return
	}
	ctx.View("surname/detail.html")
}
