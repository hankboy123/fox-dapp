package models

import (
	"gorm.io/gorm"
)

type Auction struct {
	gorm.Model
	TokenAddress string `gorm:"not null"`
	Amount       string `gorm:"not null"`
	BidderTime   string `gorm:"not null"`
	UserId       uint
}
