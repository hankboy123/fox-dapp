package utils

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
)

type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func NewAppError(code int, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		Error(c, appErr.Code, appErr.Message)
		return
	}

	// 未知错误，记录日志但不暴露给客户端
	Error(c, 500, "Internal server error")
}
