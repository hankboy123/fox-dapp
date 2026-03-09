package models

import (
	"gorm.io/gorm"
)

type Nft struct {
	gorm.Model
	Name          string `gorm:"not null"`
	Url           string `gorm:"not null"`
	TokenId       uint
	ReservePrice  float64 `gorm:"not null"` //地板价格
	AuctionStatus string  `gorm:"not null"` //拍卖状态
	Category      string  `gorm:"not null"` //分类
	User          User
}
