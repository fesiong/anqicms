package provider

import "kandaoni.com/anqicms/model"

func (w *Website) GetHoroscopeByHash(id string) (*model.Horoscope, error) {
	var horoscope model.Horoscope
	err := w.DB.Where("`hash` = ?", id).Take(&horoscope).Error
	if err != nil {
		return nil, err
	}
	return &horoscope, nil
}

func (w *Website) GetHoroscopeByBorn(born int) (*model.Horoscope, error) {
	var horoscope model.Horoscope
	err := w.DB.Where("`born` = ?", born).Take(&horoscope).Error
	if err != nil {
		return nil, err
	}
	return &horoscope, nil
}

func (w *Website) GetHoroscopeList(currentPage int, pageSize int, order string) ([]*model.Horoscope, int64, error) {
	var horoscopes []*model.Horoscope
	offset := (currentPage - 1) * pageSize

	var total int64

	builder := w.DB.Model(model.Horoscope{})

	if order != "" {
		builder = builder.Order(order)
	} else {
		builder = builder.Order("id DESC")
	}

	if err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&horoscopes).Error; err != nil {
		return nil, 0, err
	}

	return horoscopes, total, nil
}

func (w *Website) GetHoroscopeRelated(id uint, limit int) ([]*model.Horoscope, error) {
	var horoscopes []*model.Horoscope
	var horoscopes2 []*model.Horoscope

	if err := w.DB.Where("`id` > ?", id).Order("id ASC").Limit(limit / 2).Find(&horoscopes).Error; err != nil {
		//no
	}
	if err := w.DB.Where("`id` < ?", id).Order("id DESC").Limit(limit / 2).Find(&horoscopes2).Error; err != nil {
		//no
	}

	if len(horoscopes2) > 0 {
		for _, v := range horoscopes2 {
			horoscopes = append(horoscopes, v)
		}
	}

	return horoscopes, nil
}
