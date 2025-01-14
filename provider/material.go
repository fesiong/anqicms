package provider

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

func (w *Website) GetMaterialList(categoryId uint, keyword string, currentPage, pageSize int) ([]*model.Material, int64, error) {
	var materials []*model.Material
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := w.DB.Model(&model.Material{}).Order("id desc")
	if keyword != "" {
		//模糊搜索
		builder = builder.Where("(`title` like ?)", "%"+keyword+"%")
	}
	if categoryId != 0 {
		//模糊搜索
		builder = builder.Where("`category_id` = ?", categoryId)
	}

	err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&materials).Error
	if err != nil {
		return nil, 0, err
	}
	for i, v := range materials {
		materials[i].Content = w.ReplaceContentUrl(v.Content, true)
	}

	//增加分类名称
	categories, err := w.GetMaterialCategories()
	if err == nil {
		for i, v := range materials {
			for _, c := range categories {
				if v.CategoryId == c.Id {
					materials[i].CategoryTitle = c.Title
				}
			}
		}
	}

	return materials, total, nil
}

func (w *Website) SaveMaterial(req *request.PluginMaterial) (material *model.Material, err error) {
	if req.Id > 0 {
		material, err = w.GetMaterialById(req.Id)
		if err != nil {
			return nil, err
		}
	} else {
		material = &model.Material{
			Status: 1,
		}
	}
	oldContent := material.Content
	material.Title = req.Title
	material.Status = 1
	material.CategoryId = req.CategoryId
	material.AutoUpdate = req.AutoUpdate
	md5Str := library.Md5(material.Content)
	material.Md5 = md5Str

	// 将单个&nbsp;替换为空格
	req.Content = library.ReplaceSingleSpace(req.Content)
	req.Content = w.ReplaceContentUrl(req.Content, false)
	baseHost := ""
	urls, err := url.Parse(w.System.BaseUrl)
	if err == nil {
		baseHost = urls.Host
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
						s = strings.Replace(match[0], match[1], newUrl, 1)
					}
				}
			}
			return s
		})
	}
	if w.Content.RemoteDownload == 1 {
		re, _ := regexp.Compile(`(?i)<img.*?src="(.+?)".*?>`)
		req.Content = re.ReplaceAllStringFunc(req.Content, func(s string) string {
			match := re.FindStringSubmatch(s)
			if len(match) < 2 {
				return s
			}
			imgUrl, err2 := url.Parse(match[1])
			if err2 == nil {
				if imgUrl.Host != "" && imgUrl.Host != baseHost && !strings.HasPrefix(match[1], w.PluginStorage.StorageUrl) {
					//外链
					attachment, err2 := w.DownloadRemoteImage(match[1], "")
					if err2 == nil {
						// 下载完成
						s = strings.Replace(s, match[1], attachment.Logo, 1)
					}
				}
			}
			return s
		})
	}
	material.Content = req.Content

	err = w.DB.Save(material).Error

	if err != nil {
		return
	}
	//增加素材的时候，更新素材计数
	var materialCount int64
	w.DB.Model(&model.Material{}).Where("`category_id` = ?", material.CategoryId).Count(&materialCount)
	w.DB.Model(&model.MaterialCategory{}).Where("`id` = ?", material.CategoryId).Update("material_count", materialCount)

	//如果素材是自动更新，则自动
	if material.AutoUpdate == 1 && strings.Compare(oldContent, material.Content) != 0 {
		go w.AutoUpdateMaterial(material)
	}

	return
}

func (w *Website) GetMaterialById(id uint) (*model.Material, error) {
	var material model.Material
	if err := w.DB.Where("id = ?", id).First(&material).Error; err != nil {
		return nil, err
	}

	return &material, nil
}

func (w *Website) DeleteMaterial(id uint) error {
	material, err := w.GetMaterialById(id)
	if err != nil {
		return err
	}

	//删除素材，删除记录
	w.DB.Unscoped().Where("`material_id` = ?", material.Id).Delete(model.MaterialData{})

	//执行删除操作
	err = w.DB.Delete(material).Error

	if err != nil {
		return err
	}
	//删除素材的时候，更新素材计数
	var materialCount int64
	w.DB.Model(&model.Material{}).Where("`category_id` = ?", material.CategoryId).Count(&materialCount)
	w.DB.Model(&model.MaterialCategory{}).Where("`id` = ?", material.CategoryId).Update("material_count", materialCount)

	return nil
}

// 获取所有分类
func (w *Website) GetMaterialCategories() ([]*model.MaterialCategory, error) {
	var categories []*model.MaterialCategory

	err := w.DB.Where("`status` = 1").Find(&categories).Error
	if err != nil {
		return nil, err
	}

	return categories, nil
}

func (w *Website) GetMaterialCategoryById(id uint) (*model.MaterialCategory, error) {
	var category model.MaterialCategory
	if err := w.DB.Where("id = ?", id).First(&category).Error; err != nil {
		return nil, err
	}

	return &category, nil
}

func (w *Website) DeleteMaterialCategory(id uint) error {
	category, err := w.GetMaterialCategoryById(id)
	if err != nil {
		return err
	}

	//如果存在内容，则不能删除
	var materialCount int64
	w.DB.Model(&model.Material{}).Where("`category_id` = ?", category.Id).Count(&materialCount)
	if materialCount > 0 {
		return errors.New(w.Tr("PleaseDeleteTheMaterialsUnderTheCategoryBeforeDeletingTheCategory"))
	}

	//执行删除操作
	err = w.DB.Delete(category).Error

	return err
}

func (w *Website) SaveMaterialCategory(req *request.PluginMaterialCategory) (category *model.MaterialCategory, err error) {
	if req.Id > 0 {
		category, err = w.GetMaterialCategoryById(req.Id)
		if err != nil {
			return nil, err
		}
	} else {
		category = &model.MaterialCategory{
			Status: 1,
		}
	}
	category.Title = req.Title
	category.Status = 1

	err = w.DB.Save(category).Error

	if err != nil {
		return
	}
	return
}

func (w *Website) LogMaterialData(materialIds []uint, itemType string, itemId int64) {
	//清理不存在的
	w.DB.Unscoped().Model(&model.MaterialData{}).Where("`material_id` not in(?) and `item_type` = ? and `item_id` = ?", materialIds, itemType, itemId).Delete(model.MaterialData{})
	//先检查是否存在
	var dataCount int64
	for _, materialId := range materialIds {
		material, err := w.GetMaterialById(materialId)
		if err != nil {
			//素材被删了，不再入库
			continue
		}
		w.DB.Model(&model.MaterialData{}).Where("`material_id` = ? and `item_type` = ? and `item_id` = ?", material.Id, itemType, itemId).Count(&dataCount)
		if dataCount > 0 {
			continue
		}
		//插入
		data := model.MaterialData{
			MaterialId: materialId,
			ItemType:   itemType,
			ItemId:     itemId,
		}

		w.DB.Save(&data)

		//更新素材使用计数
		var useCount int64
		w.DB.Model(&model.MaterialData{}).Where("`material_id` = ?", material.Id).Count(&useCount)
		w.DB.Model(&model.Material{}).Where("`id` = ?", material.Id).Update("use_count", useCount)
	}
}

// 自动更新素材
func (w *Website) AutoUpdateMaterial(material *model.Material) {
	if material.AutoUpdate != 1 {
		return
	}

	//检查有多少个内容使用了这个素材
	var materialData []*model.MaterialData
	w.DB.Where("`material_id` = ?", material.Id).Find(&materialData)
	for _, datum := range materialData {
		archiveData, err := w.GetArchiveDataById(datum.ItemId)
		if err == nil {
			//可以操作
			htmlR := strings.NewReader(archiveData.Content)
			doc, err := goquery.NewDocumentFromReader(htmlR)
			if err == nil {
				doc.Find(fmt.Sprintf("div[data-material=\"%d\"]", material.Id)).Each(func(i int, s *goquery.Selection) {
					s.ReplaceWithHtml(fmt.Sprintf("<div data-material=\"%d\">%s</div>", material.Id, material.Content))
				})

				//如果有替换，则更新
				content, _ := doc.Find("body").Html()
				if strings.Compare(archiveData.Content, content) != 0 {
					archiveData.Content = content
					w.DB.Save(archiveData)
				}
			}
		}
	}
}

func (w *Website) SaveMaterials(materials []*request.PluginMaterial) error {
	var exists model.Material
	var err error
	var categoryIds = map[uint]struct{}{}
	for i := range materials {
		var material model.Material
		if materials[i].Id > 0 {
			err = w.DB.Where("id = ?", materials[i].Id).Take(&material).Error
			if err != nil {
				//不存在，跳过
				continue
			}
		}
		md5Str := library.Md5(materials[i].Content)
		err = w.DB.Where("md5 = ?", md5Str).Take(&exists).Error
		if err == nil {
			//已存在，更新
			material = exists
		}
		//开始操作数据
		material.Content = materials[i].Content
		material.CategoryId = materials[i].CategoryId
		material.Md5 = md5Str
		if materials[i].Title == "" {
			runeContent := []rune(materials[i].Content)
			if len(runeContent) > 30 {
				materials[i].Title = string(runeContent[:30]) + "..."
			} else {
				materials[i].Title = materials[i].Content
			}
		}
		material.Title = materials[i].Title
		material.AutoUpdate = materials[i].AutoUpdate
		material.Status = 1
		//入库
		w.DB.Save(&material)

		categoryIds[material.CategoryId] = struct{}{}
	}

	// 更新category
	for categoryId := range categoryIds {
		var total int64
		w.DB.Model(&model.Material{}).Where("category_id = ?", categoryId).Count(&total)
		w.DB.Model(&model.MaterialCategory{}).Where("id = ?", categoryId).UpdateColumn("material_count", total)
	}

	return nil
}

func (w *Website) GetMaterialByTitle(title string) (*model.Material, error) {
	// title 可能包含标签
	title = library.StripTags(title)
	title = strings.ReplaceAll(title, "\n", "")
	// 取前30个字符
	if utf8.RuneCountInString(title) > 30 {
		title = string([]rune(title)[:30])
	}
	var material model.Material
	err := w.DB.Where("`title` like ?", title+"%").Take(&material).Error
	if err != nil {
		return nil, err
	}

	return &material, nil
}

func (w *Website) GetMaterialByOriginUrl(originUrl string) (*model.Material, error) {
	var material model.Material
	err := w.DB.Where("`origin_url` = ?", originUrl).Take(&material).Error
	if err != nil {
		return nil, err
	}

	return &material, nil
}

func (w *Website) GetMaterialsByKeyword(keyword string) []*model.Material {
	var materials []*model.Material
	w.DB.Where("`keyword` = ?", keyword).Find(&materials)

	return materials
}
