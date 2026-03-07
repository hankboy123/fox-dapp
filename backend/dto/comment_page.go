package dto

type CommentPageDTO struct {
	BasePageQuery
	Content *string `form:"content" json:"content" query:"content"`
}
