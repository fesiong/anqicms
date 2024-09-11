package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func CharacterIndex(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	totalCharacters, _ := currentSite.GetCharacterList("total", 100)
	maleCharacters, _ := currentSite.GetCharacterList("male", 100)
	femaleCharacters, _ := currentSite.GetCharacterList("female", 100)

	ctx.ViewData("totalCharacters", totalCharacters)
	ctx.ViewData("maleCharacters", maleCharacters)
	ctx.ViewData("femaleCharacters", femaleCharacters)

	lastDetails, _, _ := currentSite.GetNameDetailList(1, 22, "")
	ctx.ViewData("lastDetails", lastDetails)

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = "起名用字字典_男宝宝起名常用字_女宝宝起名常用字_汉字五行解析"
		webInfo.Keywords = "起名用字字典,男宝宝起名常用字,女宝宝起名常用字,汉字五行解析"
		webInfo.Description = "明泽起名网收集了汉语字典中适合取名使用的汉字，供大家在取名的时候，阅读并参考汉字意义、汉字五行属性等寓意。"
		//设置页面名称，方便tags识别
		webInfo.PageName = "character"
		webInfo.CanonicalUrl = currentSite.GetUrl("/character", nil, 0)
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
	ctx.View("character/index.html")
}

func CharacterDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := ctx.Params().GetString("id")
	if id == "" {
		NotFound(ctx)
		return
	}
	character, err := currentSite.GetCharacterByHash(id)
	if err != nil {
		NotFound(ctx)
		return
	}

	relatedCharacters, err := currentSite.GetCharacterRelated(character.Id, 20)
	ctx.ViewData("relatedCharacters", relatedCharacters)

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = fmt.Sprintf("%s字取名用字解释_%s字五行属性_有%d人使用了%s字取名", character.Ch, character.Ch, character.Total, character.Ch)
		webInfo.Keywords = fmt.Sprintf("%s字取名用字解释,%s字五行属性,有%d人使用了%s字取名", character.Ch, character.Ch, character.Total, character.Ch)
		webInfo.Description = character.GetComment(false)
		//设置页面名称，方便tags识别
		webInfo.PageName = "character"
		webInfo.CanonicalUrl = currentSite.GetUrl("/character/"+character.Hash, nil, 0)
		ctx.ViewData("webInfo", webInfo)
	}

	ctx.ViewData("character", character)

	inApi, _ := ctx.Values().GetBool("inApi")
	if inApi {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": ctx.GetViewData(),
		})
		return
	}
	ctx.View("character/detail.html")
}
