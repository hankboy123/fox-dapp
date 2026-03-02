package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"msg"`
	Data    interface{} `json:"data,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "success",
		Data:    data,
	})
}

func Error(c *gin.Context, code int, message string) {
	c.JSON(code, Response{
		Code:    code,
		Message: message,
		Error:   message,
	})
}

func ValidationError(c *gin.Context, errors map[string]string) {
	c.JSON(http.StatusUnprocessableEntity, Response{
		Code:    422,
		Message: "validation failed",
		Error:   errors,
	})
}
