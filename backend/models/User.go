package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Name     string `gorm:"no null;index" json:"name"`
	Email    string `gorm:"not null;uniqueIndex" json:"email"`
	Username string `gorm:"not null;uniqueIndex" json:"username"`
	Password string `gorm:"not null" json:"password"`
	IsAdmin  bool   `gorm:"default:false" json:"isAdmin"`
	Thumb    string `gorm:"default:'no_thumb.png'" json:"thumb"`
}
