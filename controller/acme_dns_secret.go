package controller

import (
	"slices"

	"github.com/gin-gonic/gin"
	"h-ui/model/constant"
	"h-ui/model/vo"
	"h-ui/service"
)

func ReadACMEDNSValue(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	claims, err := service.ParseToken(service.GetToken(c))
	if err != nil || claims.AccountBo.Deleted != 0 || !slices.Contains(claims.AccountBo.Roles, "admin") {
		vo.Fail(constant.ForbiddenError, c)
		return
	}
	var request struct {
		Provider string `json:"provider"`
		Key      string `json:"key"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Provider == "" || len(request.Provider) > 128 ||
		request.Key == "" || len(request.Key) > 256 {
		vo.Fail(constant.InvalidError, c)
		return
	}
	value, err := service.ACMEDNSValue(request.Provider, request.Key)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(value, c)
}
