package tools

import (
	"backend/dto"
	"backend/utils"
	"math"

	"gorm.io/gorm"
)

// Paginate 分页查询函数
func Paginate[T any](db *gorm.DB, query dto.BasePageQuery, result *[]T) (*dto.PageResult[T], *utils.AppError) {
	// 验证参数
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// 查询总数
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, utils.NewAppError(500, err.Error())
	}

	// 如果有总数才查询数据
	if total > 0 {
		// 应用排序
		if orderBy := query.GetOrderClause(); orderBy != "" {
			db = db.Order(orderBy)
		}

		// 执行分页查询
		if err := db.
			Offset(query.GetOffset()).
			Limit(query.GetLimit()).
			Find(result).Error; err != nil {
			return nil, utils.NewAppError(500, err.Error())
		}
	} else {
		*result = []T{}
	}

	// 计算分页信息
	totalPages := int(math.Ceil(float64(total) / float64(query.PageSize)))

	return &dto.PageResult[T]{
		Page:       query.Page,
		PageSize:   query.PageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    query.Page < totalPages,
		HasPrev:    query.Page > 1,
		Items:      *result,
	}, nil
}
