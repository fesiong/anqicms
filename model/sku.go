package model

import (
	"database/sql/driver"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lib/pq"
)

type ArchiveOption struct {
	Id          int64  `json:"id" gorm:"column:id;type:bigint(20) unsigned not null AUTO_INCREMENT;primaryKey"`
	ArchiveId   int64  `json:"archive_id" gorm:"column:archive_id;type:bigint(20) unsigned not null;default:0;index"`
	Title       string `json:"title" gorm:"column:title;type:varchar(100) not null;default:'';index"`
	Sort        uint   `json:"sort" gorm:"column:sort;type:int(10) unsigned not null;default:0;index:idx_sort"` // 排序
	IsMain      uint   `json:"is_main" gorm:"column:is_main;type:tinyint(1) unsigned not null;default:0"`       // 是否主规格
	IsAttribute bool   `json:"is_attribute" gorm:"column:is_attribute;type:tinyint(1) not null;default:0"`      // 是否是属性，属性不参与sku计算
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`

	Values []ArchiveOptionValue `json:"values" gorm:"-"` // values
}

type ArchiveOptionValue struct {
	Id          int64          `json:"id" gorm:"column:id;type:bigint(20) unsigned not null AUTO_INCREMENT;primaryKey"`
	ArchiveId   int64          `json:"archive_id" gorm:"column:archive_id;type:bigint(20) unsigned not null;default:0;index"`
	OptionId    int64          `json:"option_id" gorm:"column:option_id;type:bigint(20) unsigned not null;default:0;index"`
	Title       string         `json:"title" gorm:"column:title;type:varchar(100) not null;default:'';index"`
	Images      pq.StringArray `json:"images" gorm:"column:images;type:text default null"`
	Color       string         `json:"color" gorm:"column:color;type:varchar(100) not null;default:''"`
	Price       int64          `json:"price" gorm:"column:price;type:bigint(20) not null;default:0"`                    // 加价，sku加价模式使用
	IsDefault   bool           `json:"is_default" gorm:"column:is_default;type:tinyint(1) not null;default:0"`          // 是否默认选项
	Sort        uint           `json:"sort" gorm:"column:sort;type:int(10) unsigned not null;default:0;index:idx_sort"` // 排序
	CreatedTime int64          `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64          `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	Stock       int64          `json:"stock" gorm:"column:stock;type:bigint(20) unsigned not null;default:0"`
	SoldCount   int64          `json:"sold_count" gorm:"column:sold_count;type:bigint(20) unsigned not null;default:0"`

	OptionTitle string `json:"option_title" gorm:"-"`
	Logo        string `json:"logo" gorm:"-"`  //  logo
	Thumb       string `json:"thumb" gorm:"-"` // 这个缩略图在保存的时候就生成
}

type ArchiveSku struct {
	Id          int64               `json:"id" gorm:"column:id;type:bigint(20) unsigned not null AUTO_INCREMENT;primaryKey"`
	ArchiveId   int64               `json:"archive_id" gorm:"column:archive_id;type:bigint(20) unsigned not null;default:0;index"`
	Title       string              `json:"title" gorm:"column:title;type:varchar(100) not null;default:''"` // sku 组合名字
	UrlToken    string              `json:"url_token" gorm:"column:url_token;type:varchar(190) not null;default:'';index"`
	Images      pq.StringArray      `json:"images" gorm:"column:images;type:text default null"`
	Symbol      string              `json:"symbol" gorm:"column:symbol;type:varchar(100) not null;default:''"`   // option value id 组合
	Sku         string              `json:"sku" gorm:"column:sku;type:varchar(100) not null;default:'';index"`   // sku 货号
	Barcode     string              `json:"barcode" gorm:"column:barcode;type:varchar(100) not null;default:''"` //  条形码
	HsCode      string              `json:"hs_code" gorm:"column:hs_code;type:varchar(100) not null;default:''"` //  海关编码
	Stock       int64               `json:"stock" gorm:"column:stock;type:bigint(20) unsigned not null;default:0"`
	Price       int64               `json:"price" gorm:"column:price;type:bigint(20) unsigned not null;default:0"`
	OriginPrice int64               `json:"origin_price" gorm:"column:origin_price;type:bigint(20) not null;default:0"`                     // 原始价格
	CostPrice   int64               `json:"cost_price" gorm:"column:cost_price;type:bigint(20) not null;default:0"`                         // 成本价
	Weight      float64             `json:"weight" gorm:"column:weight;type:decimal(12,6) not null;default:0.0000000;comment:重量 (kg/g/lb)"` // 如果填写了长宽高，并且设置了体积重系数，则自动计算重量
	WeightUnit  string              `json:"weight_unit" gorm:"column:weight_unit;type:varchar(20);default:null;comment:重量单位 kg/g/lb"`       // 如果填写了长宽高，并且设置了体积重系数，则自动使用重量单位kg
	Length      float64             `json:"length" gorm:"column:length;type:decimal(12,2) not null;default:0.00;comment:长 (用户单位)"`
	Width       float64             `json:"width" gorm:"column:width;type:decimal(12,2) not null;default:0.00;comment:宽 (用户单位)"`
	Height      float64             `json:"height" gorm:"column:height;type:decimal(12,2) not null;default:0.00;comment:高 (用户单位)"`
	Volume      float64             `json:"volume" gorm:"column:volume;type:decimal(12,6) not null;default:0.000000;comment:体积 (m³)"` // 固定是 m³，自动换算
	LengthUnit  string              `json:"length_unit" gorm:"column:length_unit;type:varchar(20);default:null;comment:长度单位 m/cm/inch"`
	SoldCount   int64               `json:"sold_count" gorm:"column:sold_count;type:bigint(20) unsigned not null;default:0"`
	CreatedTime int64               `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64               `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	Values      ArchiveOptionValues `json:"values" gorm:"column:values;type:text default null"`                   // SKU 组合字段
	Status      int                 `json:"status" gorm:"column:status;type:int(10) unsigned not null;default:1"` // 状态,0:下架,1:上架
	Link        string              `json:"link" gorm:"-"`
	Logo        string              `json:"logo" gorm:"-"`  //  logo
	Thumb       string              `json:"thumb" gorm:"-"` // 这个缩略图在保存的时候就生成
}

type WholesaleRule struct {
	Id           int64 `json:"id" gorm:"column:id;type:bigint(20) unsigned not null AUTO_INCREMENT;primaryKey"`
	ArchiveId    int64 `json:"archive_id" gorm:"column:archive_id;type:bigint(20) unsigned not null;default:0;index;index:idx_archive_sku_quantity"`
	SkuId        int64 `json:"sku_id" gorm:"column:sku_id;type:bigint(20) unsigned not null;default:0;index;index:idx_archive_sku_quantity"` // SKU ID，为NULL则表示规则适用于整个商品
	MinQuantity  int   `json:"min_quantity" gorm:"column:min_quantity;type:int(10) unsigned not null;default:0;index:idx_archive_sku_quantity"`
	MaxQuantity  int   `json:"max_quantity" gorm:"column:max_quantity;type:int(10) unsigned not null;default:0"`
	DiscountRate int   `json:"discount_rate" gorm:"column:discount_rate;type:int(10) unsigned not null;default:0"` // 百分比
	CreatedTime  int64 `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime  int64 `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
}

type ArchiveOptionValues []ArchiveOptionValue

func (e ArchiveOptionValues) Value() (driver.Value, error) {
	return json.Marshal(e)
}

// Scan 这里不报错
func (e *ArchiveOptionValues) Scan(data interface{}) error {
	_ = json.Unmarshal(data.([]byte), &e)
	return nil
}

func (a *ArchiveOptionValue) GetThumb(storageUrl, defaultThumb string) string {
	//取第一张
	if len(a.Images) > 0 {
		for i := range a.Images {
			if !strings.HasPrefix(a.Images[i], "http") && !strings.HasPrefix(a.Images[i], "//") {
				a.Images[i] = storageUrl + "/" + strings.TrimPrefix(a.Images[i], "/")
			}
		}
		a.Logo = a.Images[0]
		if strings.HasPrefix(a.Logo, storageUrl) && !strings.HasSuffix(a.Logo, ".svg") {
			paths, fileName := filepath.Split(a.Logo)
			a.Thumb = paths + "thumb_" + fileName
		} else {
			a.Thumb = a.Logo
		}
	}

	return a.Thumb
}

func (a *ArchiveSku) GetThumb(storageUrl, defaultThumb string) string {
	//取第一张
	if len(a.Images) > 0 {
		for i := range a.Images {
			if !strings.HasPrefix(a.Images[i], "http") && !strings.HasPrefix(a.Images[i], "//") {
				a.Images[i] = storageUrl + "/" + strings.TrimPrefix(a.Images[i], "/")
			}
		}
		a.Logo = a.Images[0]
		if strings.HasPrefix(a.Logo, storageUrl) && !strings.HasSuffix(a.Logo, ".svg") {
			paths, fileName := filepath.Split(a.Logo)
			a.Thumb = paths + "thumb_" + fileName
		} else {
			a.Thumb = a.Logo
		}
	}

	return a.Thumb
}

func (a *ArchiveSku) BuildValues(options []*ArchiveOption) []ArchiveOptionValue {
	if a == nil {
		return nil
	}
	if options == nil {
		return a.Values
	}
	var values []ArchiveOptionValue
	symbols := strings.Split(a.Symbol, "-")
	for _, sv := range symbols {
		symbol, _ := strconv.ParseInt(sv, 10, 64)
		for _, opt := range options {
			for _, vv := range opt.Values {
				if vv.Id == symbol {
					values = append(values, vv)
					break
				}
			}
		}
	}
	a.Values = values

	return values
}
