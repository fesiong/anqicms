package provider

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

func (w *Website) DeleteCacheIndex() {
	w.RemoveHtmlCache("/")
}

func init() {
	// check what if this server can visit google
	go func() {
		resp, err := library.GetURLData("https://www.google.com", "", 5)
		if err != nil {
			config.GoogleValid = false
		} else {
			config.GoogleValid = true
			log.Println("google-status", resp.StatusCode)
		}
	}()
}

func ProcessExtra(extraData model.ExtraData, fields []config.CustomField, currentSite *Website, render bool, fieldName string) model.ExtraData {
	var extra = model.ExtraData{}
	for _, field := range fields {
		if field.FieldName == "" && field.Name != "" {
			field.FieldName = field.Name
		}
		if fieldName != "" && field.FieldName != fieldName {
			continue
		}
		extra[field.FieldName] = extraData[field.FieldName]
		if (extra[field.FieldName] == nil || extra[field.FieldName] == "" || extra[field.FieldName] == 0) &&
			field.Type != config.CustomFieldTypeRadio &&
			field.Type != config.CustomFieldTypeCheckbox &&
			field.Type != config.CustomFieldTypeSelect {
			// default
			extra[field.FieldName] = field.Content
		}
		if (field.Type == config.CustomFieldTypeImage || field.Type == config.CustomFieldTypeFile || field.Type == config.CustomFieldTypeEditor) &&
			extra[field.FieldName] != nil {
			value, ok2 := extra[field.FieldName].(string)
			if ok2 {
				if field.Type == config.CustomFieldTypeEditor && render {
					value = library.MarkdownToHTML(value, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
				}
				extra[field.FieldName] = currentSite.ReplaceContentUrl(value, true)
			}
		} else if field.Type == config.CustomFieldTypeImages && extra[field.FieldName] != nil {
			if val, ok := extra[field.FieldName].([]interface{}); ok {
				for j, v2 := range val {
					v2s, _ := v2.(string)
					val[j] = currentSite.ReplaceContentUrl(v2s, true)
				}
				extra[field.FieldName] = val
			} else if value, ok := extra[field.FieldName].(string); ok {
				// json 还原
				var images []string
				err := json.Unmarshal([]byte(value), &images)
				if err == nil {
					for i := range images {
						images[i] = currentSite.ReplaceContentUrl(images[i], true)
					}
					extra[field.FieldName] = images
				}
			}
		} else if field.Type == config.CustomFieldTypeTexts && extra[field.FieldName] != nil {
			var texts []config.CustomFieldTexts
			_ = json.Unmarshal([]byte(fmt.Sprint(extra[field.FieldName])), &texts)
			extra[field.FieldName] = texts
		} else if field.Type == config.CustomFieldTypeTimeline && extra[field.FieldName] != nil {
			var val config.TimelineField
			_ = json.Unmarshal([]byte(fmt.Sprint(extra[field.FieldName])), &val)
			extra[field.FieldName] = val
		} else if field.Type == config.CustomFieldTypeNumber {
			if value, ok := extra[field.FieldName].(string); ok {
				extra[field.FieldName], _ = strconv.ParseInt(value, 10, 64)
			}
		} else if field.Type == config.CustomFieldTypeArchive && extra[field.FieldName] != nil {
			// 列表
			var arcIds []int64
			if val, ok := extra[field.FieldName].(string); ok {
				err := json.Unmarshal([]byte(val), &arcIds)
				if err == nil {
					extra[field.FieldName] = arcIds
				}
			} else {
				buf, _ := json.Marshal(extra[field.FieldName])
				_ = json.Unmarshal(buf, &arcIds)
				if len(arcIds) == 0 && field.Content != "" {
					value, _ := strconv.ParseInt(fmt.Sprint(field.Content), 10, 64)
					if value > 0 {
						arcIds = append(arcIds, value)
					}
				}
				if len(arcIds) > 0 {
					archives, _, _ := currentSite.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
						return tx.Where("archives.`id` IN (?)", arcIds)
					}, "archives.id ASC", 0, len(arcIds))
					extra[field.FieldName] = archives
				} else {
					extra[field.FieldName] = nil
				}
			}
		} else if field.Type == config.CustomFieldTypeCategory {
			value, err := strconv.ParseInt(fmt.Sprint(extra[field.FieldName]), 10, 64)
			if err != nil && field.Content != "" {
				value, _ = strconv.ParseInt(fmt.Sprint(field.Content), 10, 64)
			}
			if value > 0 {
				extra[field.FieldName] = currentSite.GetCategoryFromCache(uint(value))
			} else {
				extra[field.FieldName] = nil
			}
		}
	}
	return extra
}
