package model

import (
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        uint   `gorm:"primary_key" json:"id"`
	UserName  string `json:"userName"`
	Password  string `json:"password"`
	AddTime   int64  `json:"addTime"`
	LastLogin int64  `json:"lastLogin"`
	IsAdmin   int    `json:"isAdmin"`
}

func (user User) CheckPassword(password string) bool {
	if password == "" || user.Password == "" {
		return false
	}
	byteHash := []byte(user.Password)
	bytePass := []byte(password)
	err := bcrypt.CompareHashAndPassword(byteHash, bytePass)
	if err != nil {
		return false
	}

	return true
}

func (user User) EncryptPassword(password string) string {
	if password == "" {
		return ""
	}
	pass := []byte(password)
	hash, err := bcrypt.GenerateFromPassword(pass, bcrypt.MinCost)
	if err != nil {
		return ""
	}

	return string(hash)
}
