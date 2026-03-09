package services

import (
	"backend/dto"
	"backend/models"
	"backend/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuctionService struct {
	// 这里可以添加数据库连接等依赖
	db          *gorm.DB
	context     *gin.Context
	userService *UserService
}

func NewAuctionService(db *gorm.DB, userService *UserService, c *gin.Context) *AuctionService {
	return &AuctionService{db: db, userService: userService, context: c}
}

func (p *AuctionService) bidPrice(bidDto *dto.BidDto) (*models.Auction, *utils.AppError) {
	if bidDto == nil {
		return nil, utils.NewAppError(500, "参数不能为空")
	}

	if err := bidDto.Validate(); err != nil {
		return nil, err
	}

	auctionModel := &models.Auction{
		UserId: utils.GetCurrentUserID(p.context),
	}

	if err := p.db.Create(&auctionModel).Error; err != nil {
		return nil, utils.NewAppError(500, "Failed to create auction")
	}
	return auctionModel, nil
}

func (p *AuctionService) endAuction(auctionID uint) (*models.Auction, *utils.AppError) {
	var auction models.Auction
	if err := p.db.First(&auction, auctionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, utils.NewAppError(404, "Auction not found")
		}
		return nil, utils.NewAppError(500, "Failed to retrieve Auction")
	}
	//TODO: 结束拍卖的逻辑
	return &auction, nil
}
