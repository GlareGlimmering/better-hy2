package router

import (
	"github.com/gin-gonic/gin"
	"h-ui/controller"
)

func initXrayRouter(api *gin.RouterGroup) {
	xray := api.Group("/xray")
	{
		xray.GET("/settings", controller.GetXraySettings)
		xray.POST("/draft", controller.SaveXrayDraft)
		xray.POST("/validate", controller.ValidateXrayDraft)
		xray.POST("/apply", controller.ApplyXrayDraft)
		xray.POST("/install", controller.InstallXrayVersion)
		xray.POST("/ca", controller.UploadXrayCA)
		xray.GET("/status", controller.GetXrayStatus)
		xray.POST("/start", controller.StartXray)
		xray.POST("/stop", controller.StopXray)
		xray.POST("/restart", controller.RestartXray)
		xray.POST("/test/tcp", controller.TestXrayTCP)
		xray.POST("/test/udp", controller.TestXrayUDP)
		xray.POST("/test", controller.TestXrayTCP)
	}
}
