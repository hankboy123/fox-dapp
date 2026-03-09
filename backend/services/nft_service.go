package services

import (
	"backend/dto"
	"backend/models"
	"backend/tools"
	"backend/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NftService struct {
	// 这里可以添加数据库连接等依赖
	db          *gorm.DB
	context     *gin.Context
	userService *UserService
}

func NewNftService(db *gorm.DB, userService *UserService, c *gin.Context) *NftService {
	return &NftService{db: db, userService: userService, context: c}
}

func (p *NftService) CreateNft(nftDto *dto.NftDto) (*models.Nft, *utils.AppError) {
	if nftDto == nil {
		return nil, utils.NewAppError(500, "参数不能为空")
	}

	if err := nftDto.Validate(); err != nil {
		return nil, err
	}

	nftModel := &models.Nft{
		Name:   *nftDto.Name,
		UserId: utils.GetCurrentUserID(p.context),
	}

	if err := p.db.Create(&nftModel).Error; err != nil {
		return nil, utils.NewAppError(500, "Failed to create nft")
	}
	return nftModel, nil

}
func (p *NftService) GetNftByID(nftID uint) (*models.Nft, *utils.AppError) {
	var nft models.Nft
	if err := p.db.First(&nft, nftID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, utils.NewAppError(404, "Nft not found")
		}
		return nil, utils.NewAppError(500, "Failed to retrieve Nft")
	}
	return &nft, nil
}

func (p *NftService) GetNftByPage(nftPageDTO *dto.NftPageDTO) (*dto.PageResult[models.Nft], *utils.AppError) {

	db := p.db.Model(&models.Nft{})
	if nftPageDTO.Name != nil || strings.TrimSpace(*nftPageDTO.Name) != "" {
		db.Where("name LIKE ?", "%"+*nftPageDTO.Name+"%")
	}
	// 执行分页查询
	var nfts []models.Nft
	return tools.Paginate(db, nftPageDTO.BasePageQuery, &nfts)
}

func (p *NftService) UpdateNft(nft *dto.NftDto) (*models.Nft, *utils.AppError) {
	if nft == nil {
		return nil, utils.NewAppError(500, "参数不能为空")
	}
	if nft.ID == nil {
		return nil, utils.NewAppError(500, "参数ID不能为空")
	}
	existNft, err := p.GetNftByID(*nft.ID)
	if err != nil {
		return nil, err
	}

	if err := nft.Validate(); err != nil {
		return nil, err
	}

	existNft.Name = *nft.Name

	if err := p.db.Save(&existNft).Error; err != nil {
		return nil, utils.NewAppError(500, "Failed to update nft")
	}
	return existNft, nil

}

func (p *NftService) DeleteByID(nftID uint) *utils.AppError {
	if err := p.db.Delete(&models.Nft{}, nftID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NewAppError(404, "Nft not found")
		}
		return utils.NewAppError(500, "Failed to delete Nft")
	}
	return nil
}
