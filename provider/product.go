package provider

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"gorm.io/gorm"
	"irisweb/config"
	"irisweb/library"
	"irisweb/model"
	"irisweb/request"
	"net/url"
	"strconv"
	"strings"
)

func SaveProduct(req *request.Product) (product *model.Product, err error) {
	var category *model.Category
	//检查分类
	if req.CategoryName != "" {
		category, err = GetCategoryByTitle(req.CategoryName)
		if err != nil {
			category = &model.Category{
				Title:  req.CategoryName,
				Status: 1,
			}
			err = category.Save(config.DB)
			if err != nil {
				return
			}
		}
		if category != nil {
			req.CategoryId = category.Id
		}
	}

	newPost := false
	if req.Id > 0 {
		product, err = GetProductById(req.Id)
		if err != nil {
			return nil, err
		}

		if product.ProductData == nil {
			product.ProductData = &model.ProductData{}
		}
		product.ProductData.Content = req.Content
	} else {
		newPost = true
		newToken := library.GetPinyin(req.Title)
		_, err := CheckProductByUrlToken(newToken)
		if err == nil {
			//增加随机
			newToken += library.GenerateRandString(3)
		}

		product = &model.Product{
			Status:   1,
			UrlToken: newToken,
			ProductData: &model.ProductData{
				Content: req.Content,
			},
		}
	}
	product.Title = req.Title
	product.Keywords = req.Keywords
	product.Description = req.Description
	product.CategoryId = req.CategoryId
	product.Price = req.Price
	product.Stock = req.Stock
	product.Images = req.Images

	//extra
	extraFields := map[string]interface{}{}
	if len(config.JsonData.ProductExtraFields) > 0 {
		for _, v := range config.JsonData.ProductExtraFields {
			//先检查是否有必填而没有填写的
			if v.Required && req.Extra[v.FieldName] == nil {
				return nil, fmt.Errorf("%s必填", v.Name)
			}
			if req.Extra[v.FieldName] != nil {
				if v.Type == config.CustomFieldTypeCheckbox {
					//只有这个类型的数据是数组,数组转成,分隔字符串
					if val, ok := req.Extra[v.FieldName].([]interface{}); ok {
						var val2 []string
						for _, v2 := range val {
							val2 = append(val2, v2.(string))
						}
						extraFields[v.FieldName] = strings.Join(val2, ",")
					}

				} else if v.Type == config.CustomFieldTypeNumber {
					//只有这个类型的数据是数字，转成数字
					extraFields[v.FieldName], _ = strconv.Atoi(req.Extra[v.FieldName].(string))
				} else {
					extraFields[v.FieldName] = req.Extra[v.FieldName]
				}
			} else {
				if v.Type == config.CustomFieldTypeNumber {
					//只有这个类型的数据是数字，转成数字
					extraFields[v.FieldName] = 0
				} else {
					extraFields[v.FieldName] = ""
				}
			}
		}
	}

	//goquery
	htmlR := strings.NewReader(req.Content)
	doc, err := goquery.NewDocumentFromReader(htmlR)
	if err == nil {
		baseHost := ""
		urls, err := url.Parse(config.JsonData.System.BaseUrl)
		if err == nil {
			baseHost = urls.Host
		}

		//提取描述
		if req.Description == "" {
			textRune := []rune(strings.TrimSpace(doc.Text()))
			if len(textRune) > 150 {
				product.Description = string(textRune[:150])
			} else {
				product.Description = string(textRune)
			}
		}
		//下载远程图片
		if config.JsonData.Content.RemoteDownload == 1 {
			doc.Find("img").Each(func(i int, s *goquery.Selection) {
				src, exists := s.Attr("src")
				if exists {
					alt := s.AttrOr("alt", "")
					imgUrl, err := url.Parse(src)
					if err == nil {
						if imgUrl.Host != "" && imgUrl.Host != baseHost {
							//外链
							attachment, err := DownloadRemoteImage(src, alt)
							if err == nil {
								s.SetAttr("src", attachment.Logo)
							}
						}
					}
				}
			})
		}
		//提取缩略图
		if len(product.Images) == 0 {
			imgSections := doc.Find("img")
			if imgSections.Length() > 0 {
				//获取第一条
				product.Images = append(product.Images, imgSections.Eq(0).AttrOr("src", ""))
			}
		}
		for i, v := range product.Images {
			product.Images[i] = strings.Replace(v, config.JsonData.System.BaseUrl, "", -1)
		}

		//过滤外链
		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if exists {
				aUrl, err := url.Parse(href)
				if err == nil {
					if aUrl.Host != "" && aUrl.Host != baseHost {
						//外链
						if config.JsonData.Content.FilterOutlink == 1 {
							//过滤外链
							s.Contents().Unwrap()
						} else {
							//增加nofollow
							s.SetAttr("rel", "nofollow")
						}
					}
				}
			}
		})

		//检查有多少个material
		var materialIds []uint
		doc.Find("div[data-material]").Each(func(i int, s *goquery.Selection) {
			tmpId, exists := s.Attr("data-material")
			if exists {
				//记录material
				materialId, err := strconv.Atoi(tmpId)
				if err == nil {
					materialIds = append(materialIds, uint(materialId))
				}
			}
		})
		go LogMaterialData(materialIds, "product", product.Id)
		//返回最终可用的内容
		product.ProductData.Content, _ = doc.Find("body").Html()
	}

	err = product.Save(config.DB)

	//extra
	if len(extraFields) > 0 {
		//入库
		config.DB.Model(product).Updates(extraFields)
	}

	link := GetUrl("product", product, 0)

	//添加锚文本
	if config.JsonData.PluginAnchor.ReplaceWay == 1 {
		go ReplaceContent(nil, "product", product.Id, link)
	}
	//提取锚文本
	if config.JsonData.PluginAnchor.KeywordWay == 1 {
		go AutoInsertAnchor(product.Keywords, link)
	}

	//新发布的产品，执行推送
	if newPost {
		go PushProduct(link)
		if config.JsonData.PluginSitemap.AutoBuild == 1 {
			_ = AddonSitemap("product", link)
		}
	}
	return
}

func GetProductById(id uint) (*model.Product, error) {
	var product model.Product
	db := config.DB
	err := db.Where("`id` = ?", id).First(&product).Error
	if err != nil {
		return nil, err
	}
	//加载内容
	product.ProductData = &model.ProductData{}
	db.Where("`id` = ?", product.Id).First(product.ProductData)
	//加载分类
	var category model.Category
	err = db.Where("`id` = ?", product.CategoryId).First(&category).Error
	if err == nil {
		product.Category = &category
	}
	product.Extra = GetProductExtra(product.Id)

	return &product, nil
}

func GetProductDataById(id uint) (*model.ProductData, error) {
	var data model.ProductData
	err := config.DB.Where("`id` = ?", id).First(&data).Error
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func CheckProductByUrlToken(urlToken string) (*model.Product, error) {
	var product model.Product
	db := config.DB
	err := db.Where("`url_token` = ?", urlToken).First(&product).Error
	if err != nil {
		return nil, err
	}

	return &product, nil
}

func GetProductByUrlToken(urlToken string) (*model.Product, error) {
	var product model.Product
	db := config.DB
	err := db.Where("`url_token` = ?", urlToken).First(&product).Error
	if err != nil {
		return nil, err
	}
	//加载内容
	product.ProductData = &model.ProductData{}
	db.Where("`id` = ?", product.Id).First(product.ProductData)
	//加载分类
	var category model.Category
	err = db.Where("`id` = ?", product.CategoryId).First(&category).Error
	if err == nil {
		product.Category = &category
	}
	product.Extra = GetProductExtra(product.Id)

	return &product, nil
}

func GetProductList(categoryId uint, order string, currentPage int, pageSize int) ([]*model.Product, int64, error) {
	var products []*model.Product
	offset := (currentPage - 1) * pageSize
	var total int64

	extraFields := map[uint]map[string]*model.CustomField{}
	var results []map[string]interface{}
	var fields []string
	fields = append(fields, "id")
	if len(config.JsonData.ProductExtraFields) > 0 {
		for _, v := range config.JsonData.ProductExtraFields {
			fields = append(fields, v.FieldName)
		}
	}

	builder := config.DB.Model(&model.Product{}).Where("`status` = 1")
	if categoryId > 0 {
		builder = builder.Where("`category_id` = ?", categoryId)
	}
	if order != "" {
		builder = builder.Order(order)
	}
	builder = builder.Count(&total).Limit(pageSize).Offset(offset)
	if err := builder.Find(&products).Error; err != nil {
		return nil, 0, err
	}
	if len(fields) > 0 {
		builder.Select(strings.Join(fields, ",")).Scan(&results)
		for i, field := range results {
			item := map[string]*model.CustomField{}
			for _, v := range config.JsonData.ProductExtraFields {
				if v.Type == config.CustomFieldTypeCheckbox {
					if val, ok := field[v.FieldName].(string); ok {
						results[i][v.FieldName] = strings.Split(val, ",")
					}
				}
				item[v.FieldName] = &model.CustomField{
					Name:      v.Name,
					Value:     field[v.FieldName],
				}
			}
			if id, ok := field["id"].(uint32); ok {
				extraFields[uint(id)] = item
			}
		}
		for i := range products {
			if extraFields[products[i].Id] != nil {
				products[i].Extra = extraFields[products[i].Id]
			}
		}
	}

	return products, total, nil
}

func GetRelationProductList(categoryId uint, id uint, limit int) ([]*model.Product, error) {
	var products []*model.Product
	var products2 []*model.Product
	db := config.DB
	if err := db.Model(&model.Product{}).Where("`status` = 1").Where("`id` > ?", id).Where("`category_id` = ?", categoryId).Order("id ASC").Limit(limit / 2).Find(&products).Error; err != nil {
		//no
	}
	if err := db.Model(&model.Product{}).Where("`status` = 1").Where("`id` < ?", id).Where("`category_id` = ?", categoryId).Order("id DESC").Limit(limit / 2).Find(&products2).Error; err != nil {
		//no
	}
	//列表不返回content
	if len(products2) > 0 {
		for _, v := range products2 {
			products = append(products, v)
		}
	}

	return products, nil
}

func GetPrevProductById(categoryId uint, id uint) (*model.Product, error) {
	var product model.Product
	db := config.DB
	if err := db.Model(&model.Product{}).Where("`category_id` = ?", categoryId).Where("`id` < ?", id).Where("`status` = 1").Last(&product).Error; err != nil {
		return nil, err
	}

	return &product, nil
}

func GetNextProductById(categoryId uint, id uint) (*model.Product, error) {
	var product model.Product
	db := config.DB
	if err := db.Model(&model.Product{}).Where("`category_id` = ?", categoryId).Where("`id` > ?", id).Where("`status` = 1").First(&product).Error; err != nil {
		return nil, err
	}

	return &product, nil
}

func SaveProductExtraFields(reqFields []*config.CustomField) error {
	var diffFields []*config.CustomField
	for _, v := range config.JsonData.ProductExtraFields {
		exists := false
		for _, f := range reqFields {
			if v == f {
				exists = true
			}
		}
		if !exists {
			diffFields = append(diffFields, v)
		}
	}

	//对于需要去除的fields，进行删除操作
	if len(diffFields) > 0 {
		for _, v := range diffFields {
			if config.DB.Migrator().HasColumn(&model.Product{}, v.FieldName) {
				config.DB.Migrator().DropColumn(&model.Product{}, v.FieldName)
			}
		}
	}
	//然后再追加
	stmt := &gorm.Statement{DB: config.DB}
	stmt.Parse(&model.Product{})
	for _, v := range reqFields {
		column := v.GetFieldColumn()

		if !config.DB.Migrator().HasColumn(&model.Product{}, v.FieldName) {
			//创建语句
			config.DB.Exec("ALTER TABLE ? ADD COLUMN ?", gorm.Expr(stmt.Table), gorm.Expr(column))
		} else {
			//更新语句
			config.DB.Exec("ALTER TABLE ? MODIFY COLUMN ?", gorm.Expr(stmt.Table), gorm.Expr(column))
		}
	}

	//记录到内容
	config.JsonData.ProductExtraFields = reqFields

	err := config.WriteConfig()

	return err
}

func GetProductExtra(id uint) map[string]*model.CustomField {
	//读取extra
	result := map[string]interface{}{}
	extraFields := map[string]*model.CustomField{}
	if len(config.JsonData.ProductExtraFields) > 0 {
		var fields []string
		for _, v := range config.JsonData.ProductExtraFields {
			fields = append(fields, v.FieldName)
		}
		//从数据库中取出来
		config.DB.Model(&model.Product{}).Where("`id` = ?", id).Select(strings.Join(fields, ",")).Scan(&result)
		//extra的CheckBox的值
		for _, v := range config.JsonData.ProductExtraFields {
			if v.Type == config.CustomFieldTypeCheckbox {
				if val, ok := result[v.FieldName].(string); ok {
					result[v.FieldName] = strings.Split(val, ",")
				}
			}
			extraFields[v.FieldName] = &model.CustomField{
				Name:      v.Name,
				Value:     result[v.FieldName],
			}
		}
	}

	return extraFields
}