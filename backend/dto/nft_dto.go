package dto

import (
	"backend/utils"
	"strings"
)

type NftDto struct {
	ID          *uint   `json:"id,omitempty"` // omitempty让nil不输出
	Name        *string `json:"name" binding:"required"`
	Description *string `json:"description,omitempty"`
}

func (d *NftDto) Validate() *utils.AppError {
	if d.Name == nil || strings.TrimSpace(*d.Name) == "" {
		return utils.NewAppError(500, "名称不能为空")
	}
	return nil
}
