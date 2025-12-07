package provider

import (
	"kandaoni.com/anqicms/model"
)

func (w *Website) CheckFavorites(userId int64, archiveIds []int64) []*model.ArchiveFavorite {
	var favorites []*model.ArchiveFavorite
	w.DB.Model(&model.ArchiveFavorite{}).Where("archive_id IN(?) and user_id = ?", archiveIds, userId).Find(&favorites)

	return favorites
}

func (w *Website) AddFavorite(userId int64, archiveId, skuId int64) (bool, error) {
	_, err := w.GetArchiveById(archiveId)
	if err != nil {
		return false, err
	}

	favorite := model.ArchiveFavorite{
		ArchiveId: archiveId,
		SkuId:     skuId,
		UserId:    userId,
	}

	// 如果已经收藏的，则移除
	var exist model.ArchiveFavorite
	err = w.DB.Where("archive_id = ? and user_id = ?", archiveId, userId).First(&exist).Error
	if err == nil {
		w.DB.Delete(&exist)
		return false, nil
	}

	w.DB.Model(&model.ArchiveFavorite{}).Create(&favorite)

	return true, nil
}

func (w *Website) DeleteFavorite(userId int64, archiveId int64) error {
	w.DB.Where("archive_id = ? and user_id = ?", archiveId, userId).Delete(&model.ArchiveFavorite{})

	return nil
}

func (w *Website) GetFavoriteList(userId int64, currentPage int, pageSize int) ([]*model.Archive, int64) {
	var archiveFavorites []*model.ArchiveFavorite
	var total int64
	offset := (currentPage - 1) * pageSize

	builder := w.DB.Model(&model.ArchiveFavorite{})
	if userId > 0 {
		builder = builder.Where("user_id = ?", userId).Order("id desc")
	} else {
		builder = builder.Order("id desc")
	}
	builder.Count(&total)
	builder.Limit(pageSize).Offset(offset).Find(&archiveFavorites)
	if len(archiveFavorites) == 0 {
		return nil, total
	}
	var userIds []int64
	var archiveIds []int64
	for _, v := range archiveFavorites {
		userIds = append(userIds, v.UserId)
		archiveIds = append(archiveIds, v.ArchiveId)
	}
	var archives []*model.Archive
	w.DB.Where("id IN(?)", archiveIds).Find(&archives)
	var favorites = make([]*model.Archive, 0, len(archives))
	for _, v := range archiveFavorites {
		var archive *model.Archive
		for _, a := range archives {
			if a.Id == v.ArchiveId {
				a.Link = w.GetUrl(PatternArchive, a, 0)
				a.Thumb = a.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
				archive = a
				break
			}
		}
		if archive == nil {
			continue
		}
		favorites = append(favorites, archive)
	}

	return favorites, total
}
