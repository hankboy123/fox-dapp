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

type CommentService struct {
	// 这里可以添加数据库连接等依赖
	db          *gorm.DB
	context     *gin.Context
	userService *UserService
}

func NewCommentService(db *gorm.DB, userService *UserService, c *gin.Context) *CommentService {
	return &CommentService{db: db, userService: userService, context: c}
}

func (p *CommentService) CreateComment(post *dto.CommentDto) (*models.Comment, *utils.AppError) {
	if post == nil {
		return nil, utils.NewAppError(500, "参数不能为空")
	}

	if err := post.Validate(); err != nil {
		return nil, err
	}

	commentModel := &models.Comment{
		Content: *post.Content,
		UserId:  utils.GetCurrentUserID(p.context),
	}

	if err := p.db.Create(&commentModel).Error; err != nil {
		return nil, utils.NewAppError(500, "Failed to create comment")
	}
	return commentModel, nil

}
func (p *CommentService) GetCommentByID(commentID uint) (*models.Comment, *utils.AppError) {
	var comment models.Comment
	if err := p.db.First(&comment, commentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, utils.NewAppError(404, "comment not found")
		}
		return nil, utils.NewAppError(500, "Failed to retrieve comment")
	}
	return &comment, nil
}

func (p *CommentService) GetCommentByPage(commentPageDTO *dto.CommentPageDTO) (*dto.PageResult[models.Comment], *utils.AppError) {

	db := p.db.Model(&models.Comment{})
	if commentPageDTO.Content != nil || strings.TrimSpace(*commentPageDTO.Content) != "" {
		db.Where("", commentPageDTO.Content)
	}
	// 执行分页查询
	var comments []models.Comment
	return tools.Paginate(db, commentPageDTO.BasePageQuery, &comments)
}

func (p *CommentService) UpdateComment(comment *dto.CommentDto) (*models.Comment, *utils.AppError) {
	if comment == nil {
		return nil, utils.NewAppError(500, "参数不能为空")
	}
	if comment.ID == nil {
		return nil, utils.NewAppError(500, "参数ID不能为空")
	}
	existComment, err := p.GetCommentByID(*comment.ID)
	if err != nil {
		return nil, err
	}

	if err := comment.Validate(); err != nil {
		return nil, err
	}

	existComment.Content = *comment.Content

	if err := p.db.Save(&existComment).Error; err != nil {
		return nil, utils.NewAppError(500, "Failed to create Comment")
	}
	return existComment, nil

}

func (p *CommentService) DeleteByID(postID uint) *utils.AppError {
	if err := p.db.Delete(&models.Comment{}, postID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NewAppError(404, "Comment not found")
		}
		return utils.NewAppError(500, "Failed to delete Comment")
	}
	return nil
}
