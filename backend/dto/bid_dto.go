package dto

import (
	"backend/utils"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type BidDto struct {
	ID         *uint  `json:"id,omitempty"` // omitempty让nil不输出
	BidderTime string `gorm:"not null"`
	AuctionId  *big.Int
	Bidder     common.Address `gorm:"not null"`
	BidToken   common.Address `gorm:"not null"`
	BidAmount  *big.Int
	BidUsd18   *big.Int
	Raw        types.Log
}

func (d *BidDto) Validate() *utils.AppError {
	if d.AuctionId == nil {
		return utils.NewAppError(500, "拍卖ID不能为空")
	}
	return nil
}
