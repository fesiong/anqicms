package provider

import (
	"kandaoni.com/anqicms/model"
)

func (w *Website) GetNameHistoryByHash(id string) (*model.NameHistory, error) {
	var nameHistory model.NameHistory
	if err := w.DB.Where("`hash` = ?", id).Take(&nameHistory).Error; err != nil {
		return nil, err
	}

	return &nameHistory, nil
}

func (w *Website) GetNameHistoryList(currentPage int, pageSize int, order string) ([]*model.NameHistory, int64, error) {
	var histories []*model.NameHistory
	offset := (currentPage - 1) * pageSize

	var total int64

	builder := w.DB.Model(model.NameHistory{})

	if order != "" {
		builder = builder.Order(order)
	} else {
		builder = builder.Order("id DESC")
	}

	if err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&histories).Error; err != nil {
		return nil, 0, err
	}

	return histories, total, nil
}

func (w *Website) GetNameHistoryRelated(id uint, limit int) ([]*model.NameHistory, error) {
	var histories []*model.NameHistory
	var histories2 []*model.NameHistory

	if err := w.DB.Where("`id` > ?", id).Order("id ASC").Limit(limit / 2).Find(&histories).Error; err != nil {
		//no
	}
	if err := w.DB.Where("`id` < ?", id).Order("id DESC").Limit(limit / 2).Find(&histories2).Error; err != nil {
		//no
	}

	if len(histories2) > 0 {
		for _, v := range histories2 {
			histories = append(histories, v)
		}
	}

	return histories, nil
}

func (w *Website) GetSurnameByTitle(title string) (*model.Surname, error) {
	var surname model.Surname
	if err := w.DB.Where("`title` = ?", title).Take(&surname).Error; err != nil {
		return nil, err
	}

	return &surname, nil
}

func (w *Website) GetSurnameByHash(hash string) (*model.Surname, error) {
	var surname model.Surname
	if err := w.DB.Where("`hash` = ?", hash).Take(&surname).Error; err != nil {
		return nil, err
	}

	return &surname, nil
}

func (w *Website) GetSurnameList() ([]*model.Surname, error) {
	var surnames []*model.Surname
	if err := w.DB.Omit("content").Find(&surnames).Error; err != nil {
		return nil, err
	}

	return surnames, nil
}

func (w *Website) GetSurnameRelated(id uint, limit int) ([]*model.Surname, error) {
	var surnames []*model.Surname
	var surnames2 []*model.Surname

	if err := w.DB.Omit("content").Where("`id` > ?", id).Order("id ASC").Limit(limit / 2).Find(&surnames).Error; err != nil {
		//no
	}
	if err := w.DB.Omit("content").Where("`id` < ?", id).Order("id DESC").Limit(limit / 2).Find(&surnames2).Error; err != nil {
		//no
	}

	if len(surnames2) > 0 {
		for _, v := range surnames2 {
			surnames = append(surnames, v)
		}
	}

	return surnames, nil
}

func (w *Website) GetNameDetailByHash(id string) (*model.NameDetail, error) {
	var nameDetail model.NameDetail
	if err := w.DB.Where("`hash` = ?", id).Take(&nameDetail).Error; err != nil {
		return nil, err
	}

	return &nameDetail, nil
}

func (w *Website) GetNameDetailList(currentPage int, pageSize int, order string) ([]*model.NameDetail, int64, error) {
	var histories []*model.NameDetail
	offset := (currentPage - 1) * pageSize

	var total int64

	builder := w.DB.Model(model.NameDetail{})

	if order != "" {
		builder = builder.Order(order)
	} else {
		builder = builder.Order("id DESC")
	}

	if err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&histories).Error; err != nil {
		return nil, 0, err
	}

	return histories, total, nil
}

func (w *Website) GetNameDetailRelated(id uint, limit int) ([]*model.NameDetail, error) {
	var details []*model.NameDetail
	var details2 []*model.NameDetail

	if err := w.DB.Where("`id` > ?", id).Order("id ASC").Limit(limit / 2).Find(&details).Error; err != nil {
		//no
	}
	if err := w.DB.Where("`id` < ?", id).Order("id DESC").Limit(limit / 2).Find(&details2).Error; err != nil {
		//no
	}

	if len(details2) > 0 {
		for _, v := range details2 {
			details = append(details, v)
		}
	}

	return details, nil
}
