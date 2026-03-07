package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求开始时间
		startTime := time.Now()

		// 处理请求
		c.Next()

		// 记录请求结束时间
		endTime := time.Now()
		latency := endTime.Sub(startTime)

		// 获取请求信息
		method := c.Request.Method
		path := c.Request.URL.Path
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()

		// 记录日志
		log.Printf("%s - [%s] \"%s %s\" %d %s\n",
			clientIP,
			endTime.Format(time.RFC1123),
			method,
			path,
			statusCode,
			latency,
		)
	}
}
