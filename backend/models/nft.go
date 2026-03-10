package models

import (
	"math/big"

	"gorm.io/gorm"
)

type Nft struct {
	gorm.Model
	Name          *string  `gorm:"not null"`
	Url           *string  `gorm:"not null"`
	Description   *string  `json:"description,omitempty"`
	AuctionId     *big.Int `json:"auctionId,omitempty"`
	Seller        *string  `json:"seller,omitempty"`
	Nft           *string  `json:"nft,omitempty"`
	TokenId       *big.Int `json:"tokenId,omitempty"`
	EndTime       *big.Int `json:"endTime,omitempty"`
	MinBidUsd18   *big.Int `json:"minBidUsd18,omitempty"`
	AuctionStatus *string  `gorm:"not null"` //拍卖状态
	Category      *string  `gorm:"not null"` //分类
	Winner        *string
	PayToken      *string
	PayAmount     *big.Int
	PayUsd18      *big.Int
	UserId        uint
}
