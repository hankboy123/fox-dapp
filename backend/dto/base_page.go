package dto

import (
	"backend/utils"
	"fmt"
	"strings"
)

// BasePageQuery 基础分页查询DTO
type BasePageQuery struct {
	Page     int    `form:"page" json:"page" query:"page" binding:"min=1"`                     // 页码
	PageSize int    `form:"pageSize" json:"pageSize" query:"pageSize" binding:"min=1,max=100"` // 每页大小
	OrderBy  string `form:"orderBy" json:"orderBy" query:"orderBy"`                            // 排序字段
	Order    string `form:"order" json:"order" query:"order"`                                  // 排序方式 asc/desc
}

// NewBasePageQuery 创建默认分页查询
func NewBasePageQuery() *BasePageQuery {
	return &BasePageQuery{
		Page:     1,
		PageSize: 10,
		OrderBy:  "id",
		Order:    "desc",
	}
}

// Validate 验证分页参数
func (q *BasePageQuery) Validate() *utils.AppError {
	if q.Page < 1 {
		return utils.NewAppError(500, "页码不能小于1")
	}
	if q.PageSize < 1 || q.PageSize > 1000 {
		return utils.NewAppError(500, "每页大小必须在1-1000之间")
	}
	return nil
}

// GetOffset 获取偏移量
func (q *BasePageQuery) GetOffset() int {
	if q.Page <= 0 {
		q.Page = 1
	}
	return (q.Page - 1) * q.GetLimit()
}

// GetLimit 获取限制数
func (q *BasePageQuery) GetLimit() int {
	if q.PageSize <= 0 {
		q.PageSize = 10
	}
	if q.PageSize > 1000 {
		q.PageSize = 1000
	}
	return q.PageSize
}

// GetOrderClause 获取排序子句
func (q *BasePageQuery) GetOrderClause() string {
	if q.OrderBy == "" {
		return ""
	}

	order := "ASC"
	if strings.ToUpper(q.Order) == "DESC" {
		order = "DESC"
	}

	// 安全过滤字段名（防止SQL注入）
	safeField := getSafeFieldName(q.OrderBy)
	if safeField == "" {
		return ""
	}

	return fmt.Sprintf("%s %s", safeField, order)
}

// 安全字段名验证
func getSafeFieldName(field string) string {
	// 定义允许的字段名白名单
	safeFields := map[string]bool{
		"id": true, "created_at": true, "updated_at": true,
		"name": true, "title": true, "sort": true,
	}

	// 移除可能的SQL注入字符
	field = strings.TrimSpace(field)
	field = strings.ToLower(field)

	// 只允许字母、数字和下划线
	for _, ch := range field {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_') {
			return ""
		}
	}

	if safeFields[field] {
		return field
	}
	return ""
}
