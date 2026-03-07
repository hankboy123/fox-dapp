package dto

type PostPageDTO struct {
	BasePageQuery
	Title   *string `form:"title" json:"title" query:"title"`
	Content *string `form:"content" json:"content" query:"content"`
}
