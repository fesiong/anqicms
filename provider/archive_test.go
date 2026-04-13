package provider

import (
	"testing"

	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// TestSaveArchiveOptions 测试保存商品选项和SKU信息
func TestSaveArchiveOptions(t *testing.T) {
	dbSite, _ := GetDBWebsiteInfo(1)
	dbSite.Status = 1
	InitWebsite(dbSite)
	w := GetWebsite(1)

	// 创建测试数据
	archive := &model.ArchiveDraft{
		Archive: model.Archive{
			Id:         1,
			UrlToken:   "test-product",
			OptionType: 1, // 组合SKU模式
		},
	}
	// 创建选项数据
	options := []*request.ArchiveOption{
		{
			Title:       "Color",
			IsMain:      1,
			IsAttribute: false,
			Values: []*request.ArchiveOptionValue{
				{
					Title:     "Stainless",
					Color:     "#ff0000",
					IsDefault: true,
				},
				{
					Title:     "Gold",
					Color:     "#ffff00",
					IsDefault: false,
				},
			},
		},
		{
			Title:       "Size",
			IsMain:      0,
			IsAttribute: false,
			Values: []*request.ArchiveOptionValue{
				{
					Title:     "S",
					IsDefault: true,
				},
				{
					Title:     "M",
					IsDefault: false,
				},
			},
		},
	}

	// 创建SKU数据
	archiveSkus := []*request.ArchiveSku{
		{
			Title:       "Stainless-S",
			Stock:       100,
			Price:       1000,
			OriginPrice: 1200,
			CostPrice:   800,
			Weight:      0.5,
			WeightUnit:  "kg",
			Values: []request.ArchiveOptionValue{
				{
					OptionTitle: "Color",
					Title:       "Stainless",
				},
				{
					OptionTitle: "Size",
					Title:       "S",
				},
			},
		},
		{
			Title:       "Stainless-M",
			Stock:       150,
			Price:       1100,
			OriginPrice: 1300,
			CostPrice:   850,
			Weight:      0.6,
			WeightUnit:  "kg",
			Values: []request.ArchiveOptionValue{
				{
					OptionTitle: "Color",
					Title:       "Stainless",
				},
				{
					OptionTitle: "Size",
					Title:       "M",
				},
			},
		},
		{
			Title:       "Gold-S",
			Stock:       200,
			Price:       1050,
			OriginPrice: 1250,
			CostPrice:   820,
			Weight:      0.5,
			WeightUnit:  "kg",
			Values: []request.ArchiveOptionValue{
				{
					OptionTitle: "Color",
					Title:       "Gold",
				},
				{
					OptionTitle: "Size",
					Title:       "S",
				},
			},
		},
		{
			Title:       "Gold-M",
			Stock:       200,
			Price:       1050,
			OriginPrice: 1250,
			CostPrice:   820,
			Weight:      0.5,
			WeightUnit:  "kg",
			Values: []request.ArchiveOptionValue{
				{
					OptionTitle: "Color",
					Title:       "Gold",
				},
				{
					OptionTitle: "Size",
					Title:       "M",
				},
			},
		},
	}

	// 测试SaveArchiveOptions函数
	err := w.SaveArchiveOptions(archive, options, archiveSkus)
	if err != nil {
		t.Errorf("SaveArchiveOptions failed: %v", err)
	}

	// 验证结果
	// 注意：实际测试中需要查询数据库验证数据是否正确保存
	t.Log("SaveArchiveOptions test passed")
}
