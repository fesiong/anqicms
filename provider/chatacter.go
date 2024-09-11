package provider

import "kandaoni.com/anqicms/model"

func (w *Website) GetCharacterByHash(hash string) (*model.Character, error) {
	var character model.Character
	err := w.DB.Where("`hash` = ?", hash).First(&character).Error
	if err != nil {
		return nil, err
	}

	return &character, nil
}

func (w *Website) GetCharacterByCh(hash string) (*model.Character, error) {
	var character model.Character
	err := w.DB.Where("`ch` = ?", hash).First(&character).Error
	if err != nil {
		return nil, err
	}

	return &character, nil
}

func (w *Website) GetCharacterList(order string, limit int) ([]*model.Character, error) {
	var characters []*model.Character
	err := w.DB.Limit(limit).Order(order + " DESC").Find(&characters).Error
	if err != nil {
		return nil, err
	}

	return characters, nil
}

func (w *Website) GetCharacterRelated(id uint, limit int) ([]*model.Character, error) {
	var characters []*model.Character
	var characters2 []*model.Character

	if err := w.DB.Where("`id` > ?", id).Order("id ASC").Limit(limit / 2).Find(&characters).Error; err != nil {
		//no
	}
	if err := w.DB.Where("`id` < ?", id).Order("id DESC").Limit(limit / 2).Find(&characters2).Error; err != nil {
		//no
	}

	if len(characters2) > 0 {
		for _, v := range characters2 {
			characters = append(characters, v)
		}
	}

	return characters, nil
}
