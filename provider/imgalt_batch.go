package provider

import (
	"strings"

	"kandaoni.com/anqicms/model"
)

// FillAllArchiveImgAlt 全站回填：为所有文档正文里没有 alt 的图片补充文档标题。
// 用于存量内容的一次性同步（新保存的文档由保存管线自动处理）。
// 只补空 alt，不覆盖已有值；每处理一篇即清理该篇的页面缓存。
// 返回：处理的文档数、填充的图片数。
func (w *Website) FillAllArchiveImgAlt() (docs int, imgs int, err error) {
	var datas []model.ArchiveData
	if err = w.DB.Select("id", "content").Find(&datas).Error; err != nil {
		return 0, 0, err
	}
	for _, data := range datas {
		if strings.TrimSpace(data.Content) == "" {
			continue
		}
		var draft model.ArchiveDraft
		if e := w.DB.Select("id", "title", "url_token", "link").
			Where("id = ?", data.Id).First(&draft).Error; e != nil {
			continue
		}
		if strings.TrimSpace(draft.Title) == "" {
			continue
		}
		newContent := FillContentImgAlt(data.Content, draft.Title)
		if newContent == data.Content {
			continue
		}
		if e := w.DB.Model(&model.ArchiveData{}).Where("id = ?", data.Id).
			Update("content", newContent).Error; e != nil {
			continue
		}
		docs++
		imgs += strings.Count(newContent, "alt=") - strings.Count(data.Content, "alt=")
		w.DeleteArchiveCache(draft.Id, draft.UrlToken, draft.Link)
	}
	if docs > 0 {
		w.DeleteCacheIndex()
	}
	return docs, imgs, nil
}
