package router

import (
	"github.com/gin-gonic/gin"
	"h-ui/controller"
)

func initLogRouter(accountApi *gin.RouterGroup) {
	account := accountApi.Group("/log")
	{
		account.GET("/logSystem", controller.LogSystem)
		account.GET("/logHysteria2", controller.LogHysteria2)
		account.GET("/stream", controller.StreamLog)
		account.GET("/usage", controller.LogUsage)
		account.POST("/clear", controller.ClearLog)
		account.POST("/exportLog", controller.ExportLog)
	}
}
