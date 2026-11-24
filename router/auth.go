package router

import (
	"github.com/gin-gonic/gin"
	"h-ui/controller"
	"h-ui/middleware"
)

func initAuthRouter(authApi *gin.RouterGroup) {
	auth := authApi.Group("/auth", middleware.PanelLoginLimiter())
	{
		auth.POST("/login", controller.Login)
	}
}
