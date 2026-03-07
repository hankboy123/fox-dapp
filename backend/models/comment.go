package models

import "gorm.io/gorm"

type Comment struct {
	gorm.Model
	Content string `gorm:"not null"`
	UserId  uint
	User    User
	PostId  uint
	Post    Post
}
