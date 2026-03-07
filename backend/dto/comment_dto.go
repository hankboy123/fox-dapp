package dto

import (
	"backend/utils"
	"strings"
)

type CommentDto struct {
	ID      *uint   `json:"id,omitempty"` // omitempty让nil不输出
	Content *string `json:"content" binding:"required"`
}

func (d *CommentDto) Validate() *utils.AppError {
	if d.Content == nil || strings.TrimSpace(*d.Content) == "" {
		return utils.NewAppError(500, "内容不能为空")
	}
	return nil
}
