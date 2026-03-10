package services

import (
	"backend/client"
	"backend/dto"
	"backend/models"
	"backend/utils"
	"log"

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

func (p *AuctionService) placeBid(bidDto *dto.BidDto) (*models.Auction, *utils.AppError) {
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

func (p *AuctionService) HandleClientAuctionCreatedEvent(clientAuctionCreated interface{}) {
	var auction models.Auction
	event := clientAuctionCreated.(*client.ClientAuctionCreated)
	log.Printf("新拍卖: ID=%v", event.AuctionId)
	if err := p.db.First(&auction, event.AuctionId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {

		}
	}
}

func (p *AuctionService) HandleClientAuctionEndedEvent(clientAuctionEnded interface{}) {
	var auction models.Auction
	event := clientAuctionEnded.(*client.ClientAuctionEnded)
	log.Printf("拍卖结束: ID=%v, 赢家=%v", event.AuctionId, event.Winner)
	if err := p.db.First(&auction, event.AuctionId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {

		}
	}
}

func (p *AuctionService) HandleClientBidPlacedEvent(clientBidPlaced interface{}) {

}
