package provider

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"strings"
)

func GetMaterialList(categoryId uint, keyword string, currentPage, pageSize int) ([]*model.Material, int64, error) {
	var materials []*model.Material
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := dao.DB.Model(&model.Material{}).Order("id desc")
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

	//增加分类名称
	categories, err := GetMaterialCategories()
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

func SaveMaterial(req *request.PluginMaterial) (material *model.Material, err error) {
	if req.Id > 0 {
		material, err = GetMaterialById(req.Id)
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
	material.Content = req.Content
	material.AutoUpdate = req.AutoUpdate
	md5Str := library.Md5(material.Content)
	material.Md5 = md5Str

	err = dao.DB.Save(material).Error

	if err != nil {
		return
	}
	//增加素材的时候，更新素材计数
	var materialCount int64
	dao.DB.Model(&model.Material{}).Where("`category_id` = ?", material.CategoryId).Count(&materialCount)
	dao.DB.Model(&model.MaterialCategory{}).Where("`id` = ?", material.CategoryId).Update("material_count", materialCount)

	//如果素材是自动更新，则自动
	if material.AutoUpdate == 1 && strings.Compare(oldContent, material.Content) != 0 {
		go AutoUpdateMaterial(material)
	}

	return
}

func GetMaterialById(id uint) (*model.Material, error) {
	var material model.Material
	if err := dao.DB.Where("id = ?", id).First(&material).Error; err != nil {
		return nil, err
	}

	return &material, nil
}

func DeleteMaterial(id uint) error {
	material, err := GetMaterialById(id)
	if err != nil {
		return err
	}

	//删除素材，删除记录
	dao.DB.Unscoped().Where("`material_id` = ?", material.Id).Delete(model.MaterialData{})

	//执行删除操作
	err = dao.DB.Delete(material).Error

	if err != nil {
		return err
	}
	//删除素材的时候，更新素材计数
	var materialCount int64
	dao.DB.Model(&model.Material{}).Where("`category_id` = ?", material.CategoryId).Count(&materialCount)
	dao.DB.Model(&model.MaterialCategory{}).Where("`id` = ?", material.CategoryId).Update("material_count", materialCount)

	return nil
}

//获取所有分类
func GetMaterialCategories() ([]*model.MaterialCategory, error) {
	var categories []*model.MaterialCategory

	err := dao.DB.Where("`status` = 1").Find(&categories).Error
	if err != nil {
		return nil, err
	}

	return categories, nil
}

func GetMaterialCategoryById(id uint) (*model.MaterialCategory, error) {
	var category model.MaterialCategory
	if err := dao.DB.Where("id = ?", id).First(&category).Error; err != nil {
		return nil, err
	}

	return &category, nil
}

func DeleteMaterialCategory(id uint) error {
	category, err := GetMaterialCategoryById(id)
	if err != nil {
		return err
	}

	//如果存在内容，则不能删除
	var materialCount int64
	dao.DB.Model(&model.Material{}).Where("`category_id` = ?", category.Id).Count(&materialCount)
	if materialCount > 0 {
		return errors.New(config.Lang("请删除分类下的素材，才能删除分类"))
	}

	//执行删除操作
	err = dao.DB.Delete(category).Error

	return err
}

func SaveMaterialCategory(req *request.PluginMaterialCategory) (category *model.MaterialCategory, err error) {
	if req.Id > 0 {
		category, err = GetMaterialCategoryById(req.Id)
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

	err = dao.DB.Save(category).Error

	if err != nil {
		return
	}
	return
}

func LogMaterialData(materialIds []uint, itemType string, itemId uint) {
	//清理不存在的
	dao.DB.Unscoped().Model(&model.MaterialData{}).Where("`material_id` not in(?) and `item_type` = ? and `item_id` = ?", materialIds, itemType, itemId).Delete(model.MaterialData{})
	//先检查是否存在
	var dataCount int64
	for _, materialId := range materialIds {
		material, err := GetMaterialById(materialId)
		if err != nil {
			//素材被删了，不再入库
			continue
		}
		dao.DB.Model(&model.MaterialData{}).Where("`material_id` = ? and `item_type` = ? and `item_id` = ?", material.Id, itemType, itemId).Count(&dataCount)
		if dataCount > 0 {
			continue
		}
		//插入
		data := model.MaterialData{
			MaterialId: materialId,
			ItemType:   itemType,
			ItemId:     itemId,
		}

		dao.DB.Save(&data)

		//更新素材使用计数
		var useCount int64
		dao.DB.Model(&model.MaterialData{}).Where("`material_id` = ?", material.Id).Count(&useCount)
		dao.DB.Model(&model.Material{}).Where("`id` = ?", material.Id).Update("use_count", useCount)
	}
}

//自动更新素材
func AutoUpdateMaterial(material *model.Material) {
	if material.AutoUpdate != 1 {
		return
	}

	//检查有多少个内容使用了这个素材
	var materialData []*model.MaterialData
	dao.DB.Where("`material_id` = ?", material.Id).Find(&materialData)
	for _, datum := range materialData {
		archiveData, err := GetArchiveDataById(datum.ItemId)
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
					dao.DB.Save(archiveData)
				}
			}
		}
	}
}

func SaveMaterials(materials []*request.PluginMaterial) error {
	var exists model.Material
	var err error
	var categoryIds = map[uint]struct{}{}
	for i := range materials {
		var material model.Material
		if materials[i].Id > 0 {
			err = dao.DB.Where("id = ?", materials[i].Id).Take(&material).Error
			if err != nil {
				//不存在，跳过
				continue
			}
		}
		md5Str := library.Md5(materials[i].Content)
		err = dao.DB.Where("md5 = ?", md5Str).Take(&exists).Error
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
		dao.DB.Save(&material)

		categoryIds[material.CategoryId] = struct{}{}
	}

	// 更新category
	for categoryId := range categoryIds {
		var total int64
		dao.DB.Model(&model.Material{}).Where("category_id = ?", categoryId).Count(&total)
		dao.DB.Model(&model.MaterialCategory{}).Where("id = ?", categoryId).UpdateColumn("material_count", total)
	}

	return nil
}
