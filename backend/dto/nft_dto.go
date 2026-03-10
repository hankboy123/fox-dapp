package dto

import (
	"backend/utils"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type NftDto struct {
	ID          *uint          `json:"id,omitempty"` // omitempty让nil不输出
	Name        *string        `json:"name" binding:"required"`
	Description *string        `json:"description,omitempty"`
	AuctionId   *big.Int       `json:"auctionId,omitempty"`
	Seller      common.Address `json:"seller,omitempty"`
	Nft         common.Address `json:"nft,omitempty"`
	TokenId     *big.Int       `json:"tokenId,omitempty"`
	EndTime     *big.Int       `json:"endTime,omitempty"`
	MinBidUsd18 *big.Int       `json:"minBidUsd18,omitempty"`
	Raw         types.Log      // Blockchain specific contextual infos
	Winner      common.Address `json:"seller,omitempty"`
	PayToken    common.Address `json:"seller,omitempty"`
	PayAmount   *big.Int
	PayUsd18    *big.Int
}

func (d *NftDto) Validate() *utils.AppError {
	if d.Name == nil || strings.TrimSpace(*d.Name) == "" {
		return utils.NewAppError(500, "名称不能为空")
	}
	return nil
}
