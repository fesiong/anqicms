package graphql

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/controller"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

// 查询解析器
func existQueryField(fieldName string, queryFields []*ast.Field) bool {
	for _, field := range queryFields {
		if field.Name.Value == fieldName {
			return true
		}
	}
	return false
}

func resolvePageMeta(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	ctx := rootValue["ctx"].(iris.Context)
	searchParams := p.Args["params"].(map[string]interface{})
	path, _ := p.Args["path"].(string)
	ctx.Params().Set("path", path)
	params, _ := controller.ParseRoute(ctx)
	for i, v := range params {
		if len(i) == 0 {
			continue
		}
		ctx.Params().Set(i, v)
	}
	for i, v := range searchParams {
		params[i] = fmt.Sprintf("%v", v)
		ctx.Params().Set(i, params[i])
	}
	currentPage := ctx.Params().GetIntDefault("page", 1)
	log.Printf("resolvePageMeta path:%s", path)
	log.Printf("resolvePageMeta params:%v", params)
	webInfo := &response.WebInfo{
		StatusCode: 200,
		Params:     params,
	}

	switch params["match"] {
	case "notfound":
		// 走到 not Found
		webInfo.StatusCode = 404
		webInfo.Title = "404 Not Found"
		break
	case provider.PatternArchive:
		id := ctx.Params().GetInt64Default("id", 0)
		urlToken := ctx.Params().GetString("filename")
		var archive *model.Archive
		var err error
		if urlToken != "" {
			//优先使用urlToken
			archive, err = currentSite.GetArchiveByUrlToken(urlToken)
		} else {
			archive, err = currentSite.GetArchiveById(id)
		}
		if err != nil {
			webInfo.StatusCode = 404
			webInfo.Title = "404 Not Found"
			break
		}
		archive.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
		webInfo.Title = archive.Title
		if archive.SeoTitle != "" {
			webInfo.Title = archive.SeoTitle
		}
		webInfo.Keywords = archive.Keywords
		webInfo.Description = archive.Description
		webInfo.NavBar = int64(archive.CategoryId)
		webInfo.PageId = archive.Id
		webInfo.ModuleId = int64(archive.ModuleId)
		webInfo.Image = archive.Logo
		//设置页面名称，方便tags识别
		webInfo.PageName = "archiveDetail"
		webInfo.CanonicalUrl = archive.CanonicalUrl
		if webInfo.CanonicalUrl == "" {
			webInfo.CanonicalUrl = currentSite.GetUrl("archive", archive, 0)
		}
		break
	case provider.PatternArchiveIndex:
		urlToken := ctx.Params().GetString("module")
		module := currentSite.GetModuleFromCacheByToken(urlToken)
		if module == nil {
			webInfo.StatusCode = 404
			webInfo.Title = "404 Not Found"
			break
		}
		webInfo.Title = module.Title
		webInfo.Keywords = module.Keywords
		webInfo.Description = module.Description

		//设置页面名称，方便tags识别
		webInfo.CurrentPage = currentPage
		webInfo.PageName = "archiveIndex"
		webInfo.NavBar = int64(module.Id)
		webInfo.PageId = int64(module.Id)
		webInfo.ModuleId = int64(module.Id)
		webInfo.CanonicalUrl = currentSite.GetUrl("archiveIndex", module, 0)
		break
	case provider.PatternCategory:
		categoryId := ctx.Params().GetUintDefault("id", 0)
		catId := ctx.Params().GetUintDefault("catid", 0)
		if catId > 0 {
			categoryId = catId
		}
		var category *model.Category
		urlToken := ctx.Params().GetString("filename")
		multiCatNames := ctx.Params().GetString("multicatname")
		if multiCatNames != "" {
			chunkCatNames := strings.Split(multiCatNames, "/")
			urlToken = chunkCatNames[len(chunkCatNames)-1]
			isErr := false
			for _, catName := range chunkCatNames {
				tmpCat := currentSite.GetCategoryFromCacheByToken(catName, category)
				if tmpCat == nil || (category != nil && tmpCat.ParentId != category.Id) {
					isErr = true
					break
				}
				category = tmpCat
			}
			if isErr {
				webInfo.StatusCode = 404
				webInfo.Title = "404 Not Found"
				break
			}
		} else {
			if urlToken != "" {
				//优先使用urlToken
				category = currentSite.GetCategoryFromCacheByToken(urlToken)
			} else {
				category = currentSite.GetCategoryFromCache(categoryId)
			}
		}
		if category == nil || category.Status != config.ContentStatusOK {
			webInfo.StatusCode = 404
			webInfo.Title = "404 Not Found"
			break
		}
		category.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
		webInfo.Title = category.Title
		if category.SeoTitle != "" {
			webInfo.Title = category.SeoTitle
		}
		webInfo.CurrentPage = currentPage
		webInfo.Keywords = category.Keywords
		webInfo.Description = category.Description
		webInfo.NavBar = int64(category.Id)
		webInfo.PageId = int64(category.Id)
		webInfo.ModuleId = int64(category.ModuleId)
		webInfo.PageName = "archiveList"
		webInfo.CanonicalUrl = currentSite.GetUrl("category", category, currentPage)
		break
	case provider.PatternPage:
		categoryId := ctx.Params().GetUintDefault("id", 0)
		urlToken := ctx.Params().GetString("filename")
		catId := ctx.Params().GetUintDefault("catid", 0)
		if catId > 0 {
			categoryId = catId
		}
		var category *model.Category
		if urlToken != "" {
			//优先使用urlToken
			category = currentSite.GetCategoryFromCacheByToken(urlToken)
		} else {
			category = currentSite.GetCategoryFromCache(categoryId)
		}
		if category == nil || category.Status != config.ContentStatusOK {
			webInfo.StatusCode = 404
			webInfo.Title = "404 Not Found"
			break
		}

		//修正，如果这里读到的的category，则跳到category中
		if category.Type != config.CategoryTypePage {
			webInfo.StatusCode = 301
			webInfo.Title = "301 Redirect"
			webInfo.CanonicalUrl = currentSite.GetUrl("category", category, 0)
			break
		}
		category.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
		webInfo.Title = category.Title
		if category.SeoTitle != "" {
			webInfo.Title = category.SeoTitle
		}
		webInfo.Keywords = category.Keywords
		webInfo.Description = category.Description
		webInfo.NavBar = int64(category.Id)
		webInfo.PageId = int64(category.Id)
		webInfo.PageName = "pageDetail"
		webInfo.CanonicalUrl = currentSite.GetUrl("page", category, 0)
		break
	case provider.PatternSearch:
		q := strings.TrimSpace(ctx.Params().GetString("q"))
		moduleToken := ctx.Params().GetString("module")
		var module *model.Module
		if len(moduleToken) > 0 {
			module = currentSite.GetModuleFromCacheByToken(moduleToken)
		}

		webInfo.Title = currentSite.TplTr("Search%s", "")
		if module != nil {
			webInfo.Title = module.Title + webInfo.Title
			webInfo.ModuleId = int64(module.Id)
		}
		webInfo.CurrentPage = currentPage
		webInfo.PageName = "search"
		webInfo.CanonicalUrl = currentSite.GetUrl(fmt.Sprintf("/search?q=%s(&page={page})", url.QueryEscape(q)), nil, currentPage)
		break
	case provider.PatternTagIndex:
		webInfo.Title = currentSite.TplTr("TagList")
		webInfo.CurrentPage = currentPage
		webInfo.PageName = "tagIndex"
		webInfo.CanonicalUrl = currentSite.GetUrl("tagIndex", nil, currentPage)
		break
	case provider.PatternTag:
		tagId := ctx.Params().GetUintDefault("id", 0)
		urlToken := ctx.Params().GetString("filename")
		var tag *model.Tag
		var err error
		if urlToken != "" {
			//优先使用urlToken
			tag, err = currentSite.GetTagByUrlToken(urlToken)
		} else {
			tag, err = currentSite.GetTagById(tagId)
		}
		if err != nil {
			webInfo.StatusCode = 404
			webInfo.Title = "404 Not Found"
			break
		}
		tag.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
		webInfo.Title = tag.Title
		if tag.SeoTitle != "" {
			webInfo.Title = tag.SeoTitle
		}
		webInfo.CurrentPage = currentPage
		webInfo.Keywords = tag.Keywords
		webInfo.Description = tag.Description
		webInfo.NavBar = int64(tag.Id)
		webInfo.PageId = int64(tag.Id)
		webInfo.PageName = "tag"
		webInfo.CanonicalUrl = currentSite.GetUrl("tag", tag, currentPage)
		break
	case "index":
		webTitle := currentSite.Index.SeoTitle
		webInfo.Title = webTitle
		webInfo.Keywords = currentSite.Index.SeoKeywords
		webInfo.Description = currentSite.Index.SeoDescription
		webInfo.Image = currentSite.System.SiteLogo
		//设置页面名称，方便tags识别
		webInfo.CurrentPage = currentPage
		webInfo.PageName = "index"
		webInfo.CanonicalUrl = currentSite.GetUrl("", nil, 0)
		break
	case provider.PatternPeople:
		id := ctx.Params().GetUintDefault("id", 0)
		urlToken := ctx.Params().GetString("filename")
		var user *model.User
		var err error
		if urlToken != "" {
			//优先使用urlToken
			user, err = currentSite.GetUserInfoByUrlToken(urlToken)
		} else {
			user, err = currentSite.GetUserInfoById(id)
		}
		if err != nil {
			webInfo.StatusCode = 404
			webInfo.Title = "404 Not Found"
			break
		}

		webInfo.Title = user.UserName
		webInfo.NavBar = int64(user.Id)
		webInfo.PageId = int64(user.Id)
		webInfo.PageName = "userDetail"
		webInfo.CanonicalUrl = currentSite.GetUrl(provider.PatternPeople, user, 0)
		break
	}

	return webInfo, nil
}

func resolveArchive(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	ctx := rootValue["ctx"].(iris.Context)

	id, _ := p.Args["id"].(int)
	urlToken, _ := p.Args["url_token"].(string)
	if p.Args["filename"] != nil {
		urlToken, _ = p.Args["filename"].(string)
	}
	password, _ := p.Args["password"].(string)
	render := currentSite.Content.Editor == "markdown"
	if p.Args["render"] != nil {
		render, _ = p.Args["render"].(bool)
	}
	userId := ctx.Values().GetUintDefault("userId", 0)
	userGroup, _ := ctx.Values().Get("userGroup").(*model.UserGroup)
	userInfo, _ := ctx.Values().Get("userInfo").(*model.User)

	req := &request.ApiArchiveRequest{
		Id:        int64(id),
		UrlToken:  urlToken,
		Render:    render,
		Password:  password,
		UserId:    userId,
		UserGroup: userGroup,
		UserInfo:  userInfo,
	}

	return currentSite.ApiGetArchive(req)
}

func resolveArchives(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	//ctx := rootValue["ctx"].(iris.Context)

	id, _ := p.Args["id"].(int)
	parentId, _ := p.Args["parent_id"].(int)
	categoryId, _ := p.Args["category_id"].(int)
	categoryIds, _ := p.Args["category_ids"].([]int)
	if categoryId > 0 {
		categoryIds = append(categoryIds, categoryId)
	}
	moduleId, _ := p.Args["module_id"].(int)
	userId, _ := p.Args["user_id"].(int)
	authorId, _ := p.Args["author_id"].(int)
	if authorId > 0 {
		userId = authorId
	}
	showFlag, _ := p.Args["show_flag"].(bool)
	showContent, _ := p.Args["show_content"].(bool)
	showExtra, _ := p.Args["show_extra"].(bool)
	draft, _ := p.Args["draft"].(bool)
	excludeCategoryId, _ := p.Args["exclude_category_id"].(int)
	excludeCategoryIds, _ := p.Args["exclude_category_ids"].([]int)
	if excludeCategoryId > 0 {
		excludeCategoryIds = append(excludeCategoryIds, excludeCategoryId)
	}
	order, _ := p.Args["order"].(string)
	listType, _ := p.Args["type"].(string)
	flag, _ := p.Args["flag"].(string)
	child := true
	if p.Args["child"] != nil {
		child, _ = p.Args["child"].(bool)
	}
	tag, _ := p.Args["tag"].(string)
	tagId, _ := p.Args["tag_id"].(int)
	q, _ := p.Args["q"].(string)
	like, _ := p.Args["like"].(string)
	keywords, _ := p.Args["keywords"].(string)
	render := currentSite.Content.Editor == "markdown"
	if p.Args["render"] != nil {
		render, _ = p.Args["render"].(bool)
	}
	page, _ := p.Args["page"].(int)
	limit, _ := p.Args["limit"].(int)
	offset, _ := p.Args["offset"].(int)
	if page < 1 {
		page = 1
	}

	//curUserId := ctx.Values().GetUintDefault("userId", 0)

	extraFields := map[string]interface{}{}
	if len(categoryIds) > 0 {
		categoryId = categoryIds[0]
		categoryDetail := currentSite.GetCategoryFromCache(uint(categoryId))
		if categoryDetail != nil {
			moduleId = int(categoryDetail.ModuleId)
		}
	}
	if moduleId > 0 {
		module := currentSite.GetModuleFromCache(uint(moduleId))
		if module != nil && len(module.Fields) > 0 {
			for _, field := range module.Fields {
				if p.Args[field.FieldName] != nil {
					extraFields[field.FieldName], _ = p.Args[field.FieldName]
				}
			}
		}
	}

	req := request.ApiArchiveListRequest{
		Id:                 int64(id),
		Render:             render,
		ParentId:           int64(parentId),
		CategoryIds:        categoryIds,
		ExcludeCategoryIds: excludeCategoryIds,
		ModuleId:           int64(moduleId),
		AuthorId:           int64(authorId),
		ShowFlag:           showFlag,
		ShowContent:        showContent,
		ShowExtra:          showExtra,
		ShowCategory:       existQueryField("category", p.Info.FieldASTs),
		ShowTag:            existQueryField("tags", p.Info.FieldASTs),
		Draft:              draft,
		Child:              child,
		Order:              order,
		Tag:                tag,
		TagId:              int64(tagId),
		Flag:               flag,
		Q:                  q,
		Like:               like,
		Keywords:           keywords,
		Type:               listType,
		Page:               page,
		Limit:              limit,
		Offset:             offset,
		UserId:             uint(userId),
		ExtraFields:        extraFields,
	}

	archives, total := currentSite.ApiGetArchives(&req)

	return iris.Map{
		"items": archives,
		"total": total,
	}, nil
}

func resolveFilters(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)

	moduleId, _ := p.Args["module_id"].(int)
	showAll, _ := p.Args["show_all"].(bool)
	allText, _ := p.Args["all_text"].(string)
	showPrice, _ := p.Args["show_price"].(bool)
	showCategory, _ := p.Args["show_category"].(bool)
	parentId, _ := p.Args["parent_id"].(int)
	categoryId, _ := p.Args["category_id"].(int)

	req := request.ApiFilterRequest{
		ModuleId:     int64(moduleId),
		ShowAll:      showAll,
		AllText:      allText,
		ShowPrice:    showPrice,
		ShowCategory: showCategory,
		ParentId:     int64(parentId),
		CategoryId:   int64(categoryId),
	}

	return currentSite.ApiGetFilters(&req)
}

func resolveArchiveParams(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	ctx := rootValue["ctx"].(iris.Context)
	id, _ := p.Args["id"].(int)
	urlToken, _ := p.Args["url_token"].(string)
	if p.Args["filename"] != nil {
		urlToken, _ = p.Args["filename"].(string)
	}
	render := currentSite.Content.Editor == "markdown"
	if p.Args["render"] != nil {
		render, _ = p.Args["render"].(bool)
	}
	userId := ctx.Values().GetUintDefault("userId", 0)
	userGroup, _ := ctx.Values().Get("userGroup").(*model.UserGroup)
	userInfo, _ := ctx.Values().Get("userInfo").(*model.User)
	req := &request.ApiArchiveRequest{
		Id:        int64(id),
		UrlToken:  urlToken,
		UserId:    userId,
		UserGroup: userGroup,
		UserInfo:  userInfo,
		Render:    render,
	}

	return currentSite.ApiGetArchiveParams(req)
}

func resolveUser(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	ctx := rootValue["ctx"].(iris.Context)

	id, ok := p.Args["id"].(int)
	if !ok {
		userId := ctx.Values().GetUintDefault("userId", 0)
		if userId == 0 {
			return nil, nil
		}
		id = int(userId)
	}

	user, err := currentSite.GetUserInfoById(uint(id))
	if err != nil {
		return nil, err
	}

	return user, nil
}

func resolveCategory(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	id, _ := p.Args["id"].(int)
	urlToken, _ := p.Args["url_token"].(string)
	if p.Args["filename"] != nil {
		urlToken, _ = p.Args["filename"].(string)
	}
	if p.Args["catname"] != nil {
		urlToken, _ = p.Args["catname"].(string)
	}
	render := currentSite.Content.Editor == "markdown"
	if p.Args["render"] != nil {
		render, _ = p.Args["render"].(bool)
	}

	req := &request.ApiCategoryRequest{
		Id:       int64(id),
		UrlToken: urlToken,
		Render:   render,
	}

	return currentSite.ApiGetCategory(req)
}

func resolveCategories(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	moduleId, _ := p.Args["module_id"].(int)
	parentId, _ := p.Args["parent_id"].(int)
	all, _ := p.Args["all"].(bool)
	limit, _ := p.Args["limit"].(int)
	offset, _ := p.Args["offset"].(int)

	req := &request.ApiCategoryListRequest{
		ModuleId: int64(moduleId),
		ParentId: int64(parentId),
		All:      all,
		Limit:    limit,
		Offset:   offset,
	}

	categories, _ := currentSite.ApiGetCategories(req)

	return categories, nil
}

func resolvePage(p graphql.ResolveParams) (interface{}, error) {
	// 复用 category
	return resolveCategory(p)
}

func resolvePages(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	limit, _ := p.Args["limit"].(int)
	offset, _ := p.Args["offset"].(int)

	pageList := currentSite.GetCategoriesFromCache(0, 0, config.CategoryTypePage, true)
	var resultList []*model.Category

	for i := range pageList {
		if offset > i {
			continue
		}
		if limit > 0 && i >= (limit+offset) {
			break
		}
		pageList[i].Link = currentSite.GetUrl("page", pageList[i], 0)
		pageList[i].Thumb = pageList[i].GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)

		resultList = append(resultList, pageList[i])
	}

	return resultList, nil
}

func resolveTag(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	id, _ := p.Args["id"].(int)
	urlToken, _ := p.Args["url_token"].(string)
	if p.Args["filename"] != nil {
		urlToken, _ = p.Args["filename"].(string)
	}
	render := currentSite.Content.Editor == "markdown"
	if p.Args["render"] != nil {
		render, _ = p.Args["render"].(bool)
	}

	req := &request.ApiTagRequest{
		Id:       int64(id),
		UrlToken: urlToken,
		Render:   render,
	}

	return currentSite.ApiGetTag(req)
}

func resolveTags(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	itemId, _ := p.Args["item_id"].(int)
	categoryId, _ := p.Args["category_id"].(int)
	categoryIds, _ := p.Args["category_ids"].([]int)
	if categoryId > 0 {
		categoryIds = append(categoryIds, categoryId)
	}
	listType, _ := p.Args["list_type"].(string)
	letter, _ := p.Args["letter"].(string)
	order, _ := p.Args["order"].(string)
	limit, _ := p.Args["limit"].(int)
	offset, _ := p.Args["offset"].(int)
	page, _ := p.Args["page"].(int)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	req := &request.ApiTagListRequest{
		ItemId:      int64(itemId),
		CategoryIds: categoryIds,
		Type:        listType,
		Letter:      letter,
		Order:       order,
		Limit:       limit,
		Offset:      offset,
		Page:        page,
	}

	tags, total := currentSite.ApiGetTags(req)

	return iris.Map{
		"items": tags,
		"total": total,
	}, nil
}

func resolveModule(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	id, _ := p.Args["id"].(int)
	urlToken, _ := p.Args["url_token"].(string)
	if p.Args["filename"] != nil {
		urlToken, _ = p.Args["filename"].(string)
	}
	module := currentSite.GetModuleFromCache(uint(id))
	if module == nil {
		if urlToken != "" {
			module = currentSite.GetModuleFromCacheByToken(urlToken)
		}
	}
	if module == nil {
		return nil, errors.New("no module found")
	}

	return module, nil
}

func resolveModules(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)

	modules := currentSite.GetCacheModules()

	return modules, nil
}

func resolveComments(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	itemId, _ := p.Args["id"].(int)
	userId, _ := p.Args["user_id"].(int)
	order, _ := p.Args["order"].(string)
	limit, _ := p.Args["limit"].(int)
	if limit <= 0 {
		limit = 10
	}
	offset, _ := p.Args["offset"].(int)
	page, _ := p.Args["page"].(int)
	if page < 1 {
		page = 1
	}

	comments, total, _ := currentSite.GetCommentList(int64(itemId), uint(userId), order, page, limit, offset)

	return iris.Map{
		"items": comments,
		"total": total,
	}, nil
}

func resolveSystemSetting(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)

	return currentSite.ApiGetSystemSetting(), nil
}

func resolveContactSetting(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)

	return currentSite.ApiGetContactSetting(), nil
}

func resolveDiyFields(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	render := currentSite.Content.Editor == "markdown"
	if p.Args["render"] != nil {
		render, _ = p.Args["render"].(bool)
	}

	return currentSite.ApiGetDiyFields(render), nil
}

func resolveIndexSetting(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)

	return currentSite.ApiGetIndexSetting(), nil
}

func resolveGuestbookFields(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)

	return currentSite.ApiGetGuestbookFields(), nil
}

func resolveBanners(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	bannerType, _ := p.Args["type"].(string)
	if bannerType == "" {
		bannerType = "default"
	}
	banners, _ := currentSite.ApiGetBanners(bannerType)

	return banners, nil
}

func resolveLanguages(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)

	languages, _ := currentSite.ApiGetLanguages()

	return languages, nil
}

func resolveFriendLinks(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)

	friendLinks, _ := currentSite.GetLinkList()

	return friendLinks, nil
}

func resolveNavs(p graphql.ResolveParams) (interface{}, error) {
	rootValue := p.Info.RootValue.(map[string]interface{})
	currentSite := rootValue["site"].(*provider.Website)
	typeId, _ := p.Args["type_id"].(int)
	showType, _ := p.Args["show_type"].(string)
	if showType == "" {
		showType = "children"
	}
	if typeId <= 0 {
		typeId = 1
	}

	navs := currentSite.GetNavsFromCache(uint(typeId), showType)

	return navs, nil
}

func createReview(p graphql.ResolveParams) (interface{}, error) {
	// todo
	return nil, nil
}
