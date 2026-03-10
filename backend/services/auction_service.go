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
	nftService  *NftService
	userService *UserService
}

func NewAuctionService(db *gorm.DB, nftService *NftService, userService *UserService, c *gin.Context) *AuctionService {
	return &AuctionService{db: db, nftService: nftService, userService: userService, context: c}
}

func (p *AuctionService) placeBid(bidDto *dto.BidDto) (*models.Auction, *utils.AppError) {
	if bidDto == nil {
		return nil, utils.NewAppError(500, "参数不能为空")
	}

	if err := bidDto.Validate(); err != nil {
		return nil, err
	}

	bidderHex := bidDto.Bidder.Hex()
	bidTokenHex := bidDto.BidToken.Hex()
	auctionModel := &models.Auction{
		AuctionId: bidDto.AuctionId,
		Bidder:    &bidderHex,
		BidToken:  &bidTokenHex,
		BidAmount: bidDto.BidAmount,
		BidUsd18:  bidDto.BidUsd18,
		UserId:    utils.GetCurrentUserID(p.context),
	}

	if err := p.db.Create(&auctionModel).Error; err != nil {
		return nil, utils.NewAppError(500, "Failed to create auction")
	}
	return auctionModel, nil
}

func (p *AuctionService) HandleClientAuctionCreatedEvent(clientAuctionCreated interface{}) {
	event := clientAuctionCreated.(*client.ClientAuctionCreated)
	log.Printf("新拍卖: ID=%v", event.AuctionId)
	nftDto, err := p.constructNftDto(event)
	if err != nil {
		log.Printf("Failed to construct NftDto: %v", err)
		return
	}
	if _, err := p.nftService.CreateNft(nftDto); err != nil {
		log.Printf("Failed to create NFT from auction event: %v", err)
		return
	}
}
func (p *AuctionService) constructNftDto(event *client.ClientAuctionCreated) (*dto.NftDto, *utils.AppError) {
	nftDto := &dto.NftDto{
		AuctionId:   event.AuctionId,
		Seller:      event.Seller,
		Nft:         event.Nft,
		TokenId:     event.TokenId,
		EndTime:     event.EndTime,
		MinBidUsd18: event.MinBidUsd18,
		Raw:         event.Raw,
	}
	return nftDto, nil
}

func (p *AuctionService) HandleClientAuctionEndedEvent(clientAuctionEnded interface{}) {
	event := clientAuctionEnded.(*client.ClientAuctionEnded)
	log.Printf("拍卖结束: ID=%v, 赢家=%v", event.AuctionId, event.Winner)

	nftDto := &dto.NftDto{
		AuctionId: event.AuctionId,
		Winner:    event.Winner,
		PayToken:  event.PayToken,
		PayAmount: event.PayAmount,
		PayUsd18:  event.PayUsd18,
		Raw:       event.Raw,
	}

	if _, err := p.nftService.UpdateNft(nftDto); err != nil {
		log.Printf("Failed to create NFT from auction event: %v", err)
		return
	}
}

func (p *AuctionService) HandleClientBidPlacedEvent(clientBidPlaced interface{}) {
	event := clientBidPlaced.(*client.ClientBidPlaced)
	log.Printf("出价已提交: ID=%v, 用户=%v, 金额=%v", event.AuctionId, event.Bidder, event.BidAmount)
	bidDto := &dto.BidDto{
		AuctionId: event.AuctionId,
		Bidder:    event.Bidder,
		BidToken:  event.BidToken,
		BidAmount: event.BidAmount,
		BidUsd18:  event.BidUsd18,
		Raw:       event.Raw,
	}
	if _, err := p.placeBid(bidDto); err != nil {
		log.Printf("Failed to place bid from event: %v", err)
		return
	}
}
