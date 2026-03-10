package models

import (
	"math/big"

	"gorm.io/gorm"
)

type Auction struct {
	gorm.Model
	BidderTime string `gorm:"not null"`
	AuctionId  *big.Int
	Bidder     *string `gorm:"not null"`
	BidToken   *string `gorm:"not null"`
	BidAmount  *big.Int
	BidUsd18   *big.Int
	UserId     uint
}
