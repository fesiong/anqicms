package provider

import (
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

func (w *Website) GetPlaces(ops func(tx *gorm.DB) *gorm.DB, parentId uint, showType int) ([]*model.Place, error) {
	var places []*model.Place
	err := ops(w.DB).Omit("content", "extra").Find(&places).Error
	if err != nil {
		return nil, err
	}
	for i := range places {
		places[i].GetThumb(w.PluginStorage.StorageUrl, w.GetDefaultThumb(int(places[i].Id)))
		places[i].Link = w.GetUrl("place", places[i], 0)
	}
	if showType == config.CategoryShowTypeList {
		return places, nil
	}
	placeTree := NewPlaceTree(places)

	if showType == config.CategoryShowTypeNode {
		places = placeTree.GetTreeNode(0, "")
	} else {
		places = placeTree.GetTree(parentId, "")
	}

	return places, nil
}

func (w *Website) GetPlaceByTitle(title string) (*model.Place, error) {
	return w.GetPlaceByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`title` = ?", title)
	})
}

func (w *Website) GetPlaceById(id uint) (*model.Place, error) {
	return w.GetPlaceByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`id` = ?", id)
	})
}

func (w *Website) GetPlaceByUrlToken(urlToken string) (*model.Place, error) {
	if urlToken == "" {
		return nil, errors.New("empty token")
	}
	return w.GetPlaceByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`url_token` = ?", urlToken)
	})
}

func (w *Website) GetPlaceByFunc(ops func(tx *gorm.DB) *gorm.DB) (*model.Place, error) {
	var place model.Place
	err := ops(w.DB).Take(&place).Error
	if err != nil {
		return nil, err
	}
	place.GetThumb(w.PluginStorage.StorageUrl, w.GetDefaultThumb(int(place.Id)))
	place.Link = w.GetUrl("place", &place, 0)

	return &place, nil
}

func (w *Website) SavePlace(req *model.Place) (place *model.Place, err error) {
	newPost := false
	if req.Id > 0 {
		place, err = w.GetPlaceById(req.Id)
		if err != nil {
			// 表示不存在，则新建一个
			place = &model.Place{
				Status: 1,
			}
			place.Id = req.Id
			newPost = true
		}
	} else {
		place = &model.Place{
			Status: 1,
		}
		newPost = true
	}
	place.Title = req.Title
	place.SeoTitle = req.SeoTitle
	place.Keywords = req.Keywords
	place.Description = req.Description
	place.ParentId = req.ParentId
	place.Sort = req.Sort
	place.Status = req.Status
	place.Template = req.Template
	place.IsInherit = req.IsInherit
	place.Images = req.Images
	place.Logo = req.Logo
	place.Latitude = req.Latitude
	place.Longitude = req.Longitude
	place.Timezone = req.Timezone
	if req.Extra != nil {
		place.Extra = req.Extra
	}

	for i, v := range place.Images {
		place.Images[i] = strings.TrimPrefix(v, w.PluginStorage.StorageUrl)
	}
	if place.Logo != "" {
		place.Logo = strings.TrimPrefix(place.Logo, w.PluginStorage.StorageUrl)
	}
	// 判断重复
	req.UrlToken = library.ParseUrlToken(req.UrlToken)
	if req.UrlToken == "" {
		req.UrlToken = library.GetPinyin(req.Title, w.Content.UrlTokenType == config.UrlTokenTypeSort)
	}
	req.UrlToken = w.VerifyPlaceUrlToken(req.UrlToken, place.Id)
	place.UrlToken = req.UrlToken
	// 将单个&nbsp;替换为空格
	req.Content = library.ReplaceSingleSpace(req.Content)
	req.Content = w.ReplaceContentUrl(req.Content, false)
	if place.Extra != nil {
		placeFields := w.PluginPlace.Fields
		if len(placeFields) > 0 {
			for _, field := range placeFields {
				if (field.Type == config.CustomFieldTypeImage || field.Type == config.CustomFieldTypeFile || field.Type == config.CustomFieldTypeEditor) &&
					place.Extra[field.FieldName] != nil {
					value, ok := place.Extra[field.FieldName].(string)
					if ok {
						place.Extra[field.FieldName] = w.ReplaceContentUrl(value, false)
					}
				}
				if field.Type == config.CustomFieldTypeImages {
					if val, ok := place.Extra[field.FieldName].([]interface{}); ok {
						for j, v2 := range val {
							v2s, _ := v2.(string)
							val[j] = w.ReplaceContentUrl(v2s, false)
						}
						place.Extra[field.FieldName] = val
					}
				} else if field.Type == config.CustomFieldTypeTexts && place.Extra[field.FieldName] != nil {
					buf, _ := json.Marshal(place.Extra[field.FieldName])
					place.Extra[field.FieldName] = string(buf)
				} else if field.Type == config.CustomFieldTypeTimeline {
					// 存 json
					buf, _ := json.Marshal(place.Extra[field.FieldName])
					place.Extra[field.FieldName] = string(buf)
				}
			}
		}
	}
	baseHost := ""
	frontUrl := w.System.BaseUrl
	if w.System.FrontUrl != "" {
		frontUrl = w.System.FrontUrl
	}
	urls, err := url.Parse(frontUrl)
	if err == nil {
		baseHost = urls.Host
	}
	autoAddImage := false
	//提取描述
	if place.Description == "" {
		tmpContent := req.Content
		if w.Content.Editor == "markdown" {
			tmpContent = library.MarkdownToHTML(tmpContent)
		}
		place.Description = library.ParseDescription(strings.ReplaceAll(CleanTagsAndSpaces(tmpContent), "\n", " "), 250)
	}
	//提取缩略图
	if len(place.Logo) == 0 {
		re, _ := regexp.Compile(`(?i)<img.*?src="(.+?)".*?>`)
		match := re.FindStringSubmatch(req.Content)
		if len(match) > 1 {
			//提取缩略图
			place.Logo = match[1]
			autoAddImage = true
		} else {
			// 匹配Markdown ![新的图片](http://xxx/xxx.webp)
			re, _ = regexp.Compile(`!\[([^]]*)\]\(([^)]+)\)`)
			match = re.FindStringSubmatch(req.Content)
			if len(match) > 2 {
				place.Logo = match[2]
				autoAddImage = true
			}
		}
	}
	// 过滤外链
	if w.Content.FilterOutlink == 1 || w.Content.FilterOutlink == 2 {
		re, _ := regexp.Compile(`(?i)<a.*?href="(.+?)".*?>(.*?)</a>`)
		req.Content = re.ReplaceAllStringFunc(req.Content, func(s string) string {
			match := re.FindStringSubmatch(s)
			if len(match) < 3 {
				return s
			}
			aUrl, err2 := url.Parse(match[1])
			if err2 == nil {
				if aUrl.Host != "" && aUrl.Host != baseHost {
					//过滤外链
					if w.Content.FilterOutlink == 1 {
						return match[2]
					} else if !strings.Contains(match[0], "nofollow") {
						newUrl := match[1] + `" rel="nofollow`
						s = strings.Replace(s, match[1], newUrl, 1)
					}
				}
			}
			return s
		})
		// 匹配Markdown [link](url)
		// 由于不支持零宽断言，因此匹配所有
		re, _ = regexp.Compile(`!?\[([^]]*)\]\(([^)]+)\)`)
		req.Content = re.ReplaceAllStringFunc(req.Content, func(s string) string {
			// 过滤掉 ! 开头的
			if strings.HasPrefix(s, "!") {
				return s
			}
			match := re.FindStringSubmatch(s)
			if len(match) < 3 {
				return s
			}
			aUrl, err2 := url.Parse(match[2])
			if err2 == nil {
				if aUrl.Host != "" && aUrl.Host != baseHost {
					//过滤外链
					if w.Content.FilterOutlink == 1 {
						return match[1]
					}
					// 添加 nofollow 不在这里处理，因为md不支持
				}
			}
			return s
		})
	}
	place.Content = req.Content

	err = w.DB.Save(place).Error
	if err != nil {
		return
	}
	// 自动提取远程图片改成保存后处理
	if w.Content.RemoteDownload == 1 {
		hasChangeImg := false
		re, _ := regexp.Compile(`(?i)<img.*?src="(.+?)".*?>`)
		place.Content = re.ReplaceAllStringFunc(place.Content, func(s string) string {
			match := re.FindStringSubmatch(s)
			if len(match) < 2 {
				return s
			}
			imgUrl, err2 := url.Parse(match[1])
			if err2 == nil {
				if imgUrl.Host != "" && imgUrl.Host != baseHost && !strings.HasPrefix(match[1], w.PluginStorage.StorageUrl) {
					//外链
					attachment, err2 := w.DownloadRemoteImage(match[1], "", 0)
					if err2 == nil {
						// 下载完成
						hasChangeImg = true
						s = strings.Replace(s, match[1], attachment.Logo, 1)
					}
				}
			}
			return s
		})
		// 匹配Markdown ![新的图片](http://xxx/xxx.webp)
		re, _ = regexp.Compile(`!\[([^]]*)\]\(([^)]+)\)`)
		place.Content = re.ReplaceAllStringFunc(place.Content, func(s string) string {
			match := re.FindStringSubmatch(s)
			if len(match) < 3 {
				return s
			}
			imgUrl, err2 := url.Parse(match[2])
			if err2 == nil {
				if imgUrl.Host != "" && imgUrl.Host != baseHost && !strings.HasPrefix(match[2], w.PluginStorage.StorageUrl) {
					//外链
					attachment, err2 := w.DownloadRemoteImage(match[2], "", 0)
					if err2 == nil {
						// 下载完成
						hasChangeImg = true
						s = strings.Replace(s, match[2], attachment.Logo, 1)
					}
				}
			}
			return s
		})
		if hasChangeImg {
			w.DB.Model(place).UpdateColumn("content", place.Content)
			// 更新data
			if autoAddImage {
				//提取缩略图
				re, _ = regexp.Compile(`(?i)<img.*?src="(.+?)".*?>`)
				match := re.FindStringSubmatch(req.Content)
				if len(match) > 1 {
					place.Logo = match[1]
				} else {
					// 匹配Markdown ![新的图片](http://xxx/xxx.webp)
					re, _ = regexp.Compile(`!\[([^]]*)\]\(([^)]+)\)`)
					match = re.FindStringSubmatch(req.Content)
					if len(match) > 2 {
						place.Logo = match[2]
					}
				}
				w.DB.Model(place).UpdateColumn("logo", place.Logo)
			}
		}
	}
	// 如果隐藏的分类有下级，则下级也隐藏
	if place.Status == config.ContentStatusDraft {
		w.DB.Model(&model.Place{}).Where("`parent_id` = ?", place.Id).UpdateColumn("status", config.ContentStatusDraft)
	} else if place.Status == config.ContentStatusOK && place.ParentId > 0 {
		w.DB.Model(&model.Place{}).Where("`id` = ?", place.ParentId).UpdateColumn("status", config.ContentStatusOK)
	}

	if newPost && place.Status == config.ContentStatusOK {
		link := w.GetUrl("place", place, 0)
		go func() {
			w.PushArchive(link)
			if w.PluginSitemap.AutoBuild == 1 {
				_ = w.AddonSitemap("category", link, time.Unix(place.UpdatedTime, 0).Format("2006-01-02"), place)
			}
		}()
	}
	place.GetThumb(w.PluginStorage.StorageUrl, w.GetDefaultThumb(int(place.Id)))
	w.DeleteCachePlaces()
	w.DeleteCacheIndex()

	return
}

func (w *Website) DeletePlace(place *model.Place) (err error) {
	err = w.DB.Delete(place).Error
	if err != nil {
		return
	}

	return
}

// GetPlaceTemplate 获取模板，如果检测到不继承，则停止获取
func (w *Website) GetPlaceTemplate(place *model.Place) string {
	if place == nil {
		return ""
	}

	if place.Template != "" {
		return place.Template
	}

	//查找上级
	if place.ParentId > 0 {
		parent := w.GetPlaceFromCache(place.ParentId)
		if parent != nil {
			// 如果上级存在模板，并且选择不继承，从这里阻止
			if parent.Template != "" && parent.IsInherit == 0 {
				return ""
			}
		}
		return w.GetPlaceTemplate(parent)
	}

	//不存在，则返回空
	return ""
}

func (w *Website) GetParentPlaces(parentId uint) []*model.Place {
	var places []*model.Place
	if parentId == 0 {
		return nil
	}
	for {
		place := w.GetPlaceFromCache(parentId)
		if place == nil {
			break
		}
		places = append(places, place)
		parentId = place.ParentId
	}
	// 将 places 翻转
	for i, j := 0, len(places)-1; i < j; i, j = i+1, j-1 {
		places[i], places[j] = places[j], places[i]
	}

	return places
}

func (w *Website) GetTopPlaceId(placeId uint) uint {
	if placeId == 0 {
		return 0
	}
	for {
		place := w.GetPlaceFromCache(placeId)
		if place == nil {
			break
		}
		if place.ParentId == 0 {
			break
		}
		placeId = place.ParentId
	}

	return placeId
}

func (w *Website) DeleteCachePlaces() {
	w.Cache.Delete("places")
}

func (w *Website) GetCachePlaces() []*model.Place {
	if w.DB == nil {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	var places []*model.Place

	err := w.Cache.Get("places", &places)

	if err == nil && len(places) > 0 {
		return places
	}

	err = w.DB.Model(model.Place{}).Order("sort asc").Find(&places).Error
	if err != nil {
		return nil
	}
	for i := range places {
		places[i].GetThumb(w.PluginStorage.StorageUrl, w.GetDefaultThumb(int(places[i].Id)))
	}
	placeTree := NewPlaceTree(places)
	places = placeTree.GetTree(0, "")

	if len(places) > 0 {
		_ = w.Cache.Set("places", places, 0)
	}

	return places
}

func (w *Website) GetCachePlacesByIds(ids []uint) []*model.Place {
	places := w.GetCachePlaces()
	var tmpPlaces = make([]*model.Place, 0, len(ids))
	for _, place := range places {
		if place.Link == "" {
			place.Link = w.GetUrl("place", place, 0)
		}
		for _, id := range ids {
			if place.Id == id {
				tmpPlaces = append(tmpPlaces, place)
			}
		}
	}

	return tmpPlaces
}

func (w *Website) GetSubPlaceIds(placeId uint, places []*model.Place) []uint {
	var subIds []uint
	if places == nil {
		places = w.GetCachePlaces()
	}

	for i := range places {
		if places[i].Status != config.ContentStatusOK {
			continue
		}
		if places[i].ParentId == placeId {
			subIds = append(subIds, places[i].Id)
			subIds = append(subIds, w.GetSubPlaceIds(places[i].Id, places)...)
		}
	}

	return subIds
}

func (w *Website) GetPlaceFromCache(placeId uint) *model.Place {
	if placeId == 0 {
		return nil
	}
	places := w.GetCachePlaces()
	for i := range places {
		if places[i].Id == placeId {
			if places[i].Link == "" {
				places[i].Link = w.GetUrl(PatternPlace, places[i], 0)
			}
			return places[i]
		}
	}

	return nil
}

func (w *Website) GetPlaceFromCacheByToken(urlToken string, parents ...*model.Place) *model.Place {
	places := w.GetCachePlaces()
	var parent *model.Place
	if len(parents) > 0 {
		parent = parents[0]
	}
	if parent != nil {
		for i := range places {
			if places[i].UrlToken == urlToken && parent.Id == places[i].ParentId {
				return places[i]
			}
		}
	} else {
		for i := range places {
			if places[i].Link == "" {
				places[i].Link = w.GetUrl("place", places[i], 0)
			}
			if places[i].UrlToken == urlToken {
				return places[i]
			}
		}
	}

	return nil
}

func (w *Website) GetPlacesFromCache(parentId uint, all bool) []*model.Place {
	places := w.GetCachePlaces()
	var tmpPlaces = make([]*model.Place, 0, len(places))
	for i := range places {
		if places[i].Status != config.ContentStatusOK {
			// 跳过隐藏的
			continue
		}
		if all || places[i].ParentId == parentId {
			if places[i].Link == "" {
				places[i].Link = w.GetUrl("place", places[i], 0)
			}
			tmpPlaces = append(tmpPlaces, places[i])
		}
	}

	return tmpPlaces
}

func (w *Website) VerifyPlaceUrlToken(urlToken string, id uint) string {
	index := 0
	// 防止超出长度
	if len(urlToken) > 150 {
		urlToken = urlToken[:150]
	}
	urlToken = strings.ToLower(urlToken)
	for {
		tmpToken := urlToken
		if index > 0 {
			tmpToken = urlToken + "-" + strconv.Itoa(index)
		}
		// 判断URLToken
		tmpPlace, err := w.GetPlaceByUrlToken(tmpToken)
		if err == nil && tmpPlace.Id != id {
			index++
			continue
		}
		urlToken = tmpToken
		break
	}

	return urlToken
}
