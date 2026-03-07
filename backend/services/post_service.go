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

type PostService struct {
	// 这里可以添加数据库连接等依赖
	db          *gorm.DB
	context     *gin.Context
	userService *UserService
}

func NewPostService(db *gorm.DB, userService *UserService, c *gin.Context) *PostService {
	return &PostService{db: db, userService: userService, context: c}
}

func (p *PostService) CreatePost(post *dto.PostDto) (*models.Post, *utils.AppError) {
	if post == nil {
		return nil, utils.NewAppError(500, "参数不能为空")
	}

	if err := post.Validate(); err != nil {
		return nil, err
	}

	postModel := &models.Post{
		Title:   *post.Title,
		Content: *post.Content,
		UserId:  utils.GetCurrentUserID(p.context),
	}

	if err := p.db.Create(&postModel).Error; err != nil {
		return nil, utils.NewAppError(500, "Failed to create post")
	}
	return postModel, nil

}
func (p *PostService) GetPostByID(postID uint) (*models.Post, *utils.AppError) {
	var post models.Post
	if err := p.db.First(&post, postID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, utils.NewAppError(404, "Post not found")
		}
		return nil, utils.NewAppError(500, "Failed to retrieve Post")
	}
	return &post, nil
}

func (p *PostService) GetPostByPage(postPageDTO *dto.PostPageDTO) (*dto.PageResult[models.Post], *utils.AppError) {

	db := p.db.Model(&models.Post{})
	if postPageDTO.Title != nil || strings.TrimSpace(*postPageDTO.Title) != "" {
		db.Where("", postPageDTO.Title)
	}
	if postPageDTO.Content != nil || strings.TrimSpace(*postPageDTO.Content) != "" {
		db.Where("", postPageDTO.Content)
	}
	// 执行分页查询
	var posts []models.Post
	return tools.Paginate(db, postPageDTO.BasePageQuery, &posts)
}

func (p *PostService) UpdatePost(post *dto.PostDto) (*models.Post, *utils.AppError) {
	if post == nil {
		return nil, utils.NewAppError(500, "参数不能为空")
	}
	if post.ID == nil {
		return nil, utils.NewAppError(500, "参数ID不能为空")
	}
	existPost, err := p.GetPostByID(*post.ID)
	if err != nil {
		return nil, err
	}

	if err := post.Validate(); err != nil {
		return nil, err
	}

	existPost.Title = *post.Title
	existPost.Content = *post.Content

	if err := p.db.Save(&existPost).Error; err != nil {
		return nil, utils.NewAppError(500, "Failed to create post")
	}
	return existPost, nil

}

func (p *PostService) DeleteByID(postID uint) *utils.AppError {
	if err := p.db.Delete(&models.Post{}, postID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.NewAppError(404, "Post not found")
		}
		return utils.NewAppError(500, "Failed to delete Post")
	}
	return nil
}
