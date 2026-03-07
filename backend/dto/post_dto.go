package dto

import (
	"backend/utils"
	"strings"
)

type PostDto struct {
	ID      *uint   `json:"id,omitempty"` // omitempty让nil不输出
	Title   *string `json:"title" binding:"required"`
	Content *string `json:"content" binding:"required"`
}

func (d *PostDto) Validate() *utils.AppError {
	if d.Title == nil || strings.TrimSpace(*d.Title) == "" {
		return utils.NewAppError(500, "标题不能为空")
	}
	if len(*d.Title) > 100 {
		return utils.NewAppError(500, "标题长度不能超过100字符")
	}
	if d.Content == nil || strings.TrimSpace(*d.Content) == "" {
		return utils.NewAppError(500, "内容不能为空")
	}
	return nil
}
