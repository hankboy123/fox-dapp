package dto

type NftPageDTO struct {
	BasePageQuery
	Name *string `form:"name" json:"name" query:"name"`
}
