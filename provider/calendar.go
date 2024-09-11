package provider

import "kandaoni.com/anqicms/model"

func (w *Website) GetCalendarByLunar(str string, leap int) (*model.Calendar, error) {
	var calendar model.Calendar
	if err := w.DB.Where("lunar_date = ? and lunar_leap = ?", str, leap).First(&calendar).Error; err != nil {
		if leap == 1 {
			if err := w.DB.Where("lunar_date = ? and lunar_leap = 0", str).First(&calendar).Error; err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return &calendar, nil
}
