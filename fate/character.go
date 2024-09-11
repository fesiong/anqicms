package fate

import (
	"crypto/sha256"
	"fmt"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
	"strings"
)

var Characters = map[int][]*model.Character{}
var Character = map[string]*model.Character{}
var NameSources = map[uint][]*model.NameSourceData{}
var db *gorm.DB

func InitFate(tx *gorm.DB) {
	db = tx
	var chars []*model.Character

	db.Where("total > 0").Order("total desc").Find(&chars)
	for _, v := range chars {
		Character[v.Ch] = v
		Characters[v.ScienceStroke] = append(Characters[v.ScienceStroke], v)
	}

	// 加载sourceNames
	var nameDatas []*model.NameSourceData
	db.Order("source_id asc").Find(&nameDatas)
	for _, v := range nameDatas {
		NameSources[v.SourceId] = append(NameSources[v.SourceId], v)
	}
}

type TotalCharacters []*model.Character
type MaleCharacters []*model.Character
type FemaleCharacters []*model.Character

func (a TotalCharacters) Len() int           { return len(a) }
func (a TotalCharacters) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a TotalCharacters) Less(i, j int) bool { return a[i].Total > a[j].Total }

func (a MaleCharacters) Len() int           { return len(a) }
func (a MaleCharacters) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a MaleCharacters) Less(i, j int) bool { return a[i].Male > a[j].Male }

func (a FemaleCharacters) Len() int           { return len(a) }
func (a FemaleCharacters) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a FemaleCharacters) Less(i, j int) bool { return a[i].Female > a[j].Female }

func GetCharacter(fn func(engine *gorm.DB) *gorm.DB) (*model.Character, error) {
	eng := db
	s := fn(eng)
	var c model.Character
	e := s.First(&c).Error
	if e == nil {
		return &c, nil
	}
	return nil, fmt.Errorf("character get error:%w", e)
}

// CharacterOptions ...
type CharacterOptions func(session *gorm.DB) *gorm.DB

// Regular ...
func Regular() CharacterOptions {
	return func(session *gorm.DB) *gorm.DB {
		return session.Where("`regular` = ?", 1)
	}
}

// kangxi
func OnlyKangXi(only bool) CharacterOptions {
	return func(session *gorm.DB) *gorm.DB {
		if only {
			return session.Where("`is_kang_xi` = ?", 1)
		}

		return session
	}
}

// 性别
// gender = male | female
func FilterGender(gender string) CharacterOptions {
	return func(session *gorm.DB) *gorm.DB {
		if gender == "female" {
			return session.Order("female desc")
		} else {
			return session.Order("male desc")
		}
	}
}

func Taboo(char string, side string) CharacterOptions {
	return func(session *gorm.DB) *gorm.DB {
		if char != "" {
			chars := strings.Split(char, "")
			session = session.Where("`ch` NOT IN (?)", chars)
		}
		if side != "" {
			//忌讳部首
			sides := strings.Split(side, "")
			session = session.Where("(`simple_radical` NOT IN (?) AND `radical` NOT IN (?))", sides, sides)
		}

		return session
	}
}

// Stoker ...
func Stoker(s int, options ...CharacterOptions) func(engine *gorm.DB) *gorm.DB {
	return func(engine *gorm.DB) *gorm.DB {
		session := engine.Where("`science_stroke` = ?", s)
		for _, option := range options {
			session = option(session)
		}
		return session
	}

}

// Stoker ...
func Stokers(s []int, options ...CharacterOptions) func(engine *gorm.DB) *gorm.DB {
	return func(engine *gorm.DB) *gorm.DB {
		session := engine.Where("`science_stroke` IN (?)", s)
		for _, option := range options {
			session = option(session)
		}
		return session
	}

}

// Char ...
func Char(name string) func(engine *gorm.DB) *gorm.DB {
	return func(engine *gorm.DB) *gorm.DB {
		return engine.Where("`ch` = ? OR `kang_xi` = ?", name, name)
	}
}

// Hash ...
func Hash(url string) string {
	sum256 := sha256.Sum256([]byte(url))
	return fmt.Sprintf("%x", sum256)
}
