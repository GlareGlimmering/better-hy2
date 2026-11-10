package controller

import (
	"encoding/base64"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	"h-ui/model/constant"
	"h-ui/model/dto"
	"h-ui/model/entity"
	"h-ui/model/vo"
	"h-ui/service"
	"h-ui/util"
	"net/url"
	"strings"
	"time"
)

var hysteria2IsRunning = service.Hysteria2IsRunning
var downloadHysteria2Version = util.DownloadHysteria2
var startHysteria2 = func() error {
	return service.UpdateConfig(constant.Hysteria2Enable, "1")
}
var stopHysteria2 = func() error {
	return service.UpdateConfig(constant.Hysteria2Enable, "0")
}
var restartHysteria2 = service.RestartHysteria2
var authenticateHysteria2 = service.Hysteria2Auth
var updateHysteria2AuthAccount = service.UpdateAccount

func Hysteria2Auth(c *gin.Context) {
	var hysteria2AuthDto dto.Hysteria2AuthDto
	// Unlike the generic form helper, authentication must not ignore JSON
	// decoding errors (the official HTTP authenticator sends numeric tx).
	if err := c.ShouldBindJSON(&hysteria2AuthDto); err != nil {
		logrus.WithField("reason", "request_invalid").Warn("hysteria2_auth")
		vo.Hysteria2AuthFail("", c)
		return
	}
	if err := validate.Struct(hysteria2AuthDto); err != nil {
		logrus.WithField("reason", "request_invalid").Warn("hysteria2_auth")
		vo.Hysteria2AuthFail("", c)
		return
	}
	id, username, limit, err := authenticateHysteria2(*hysteria2AuthDto.Auth)
	if err != nil || username == "" {
		reason := "authentication_failed"
		if failure, ok := err.(*service.Hysteria2AuthError); ok {
			reason = failure.Reason
		}
		logrus.WithField("reason", reason).Warn("hysteria2_auth")
		vo.Hysteria2AuthFail("", c)
		return
	}

	// 更新最近连接时间
	now := time.Now().UnixMilli()
	if err = updateHysteria2AuthAccount(entity.Account{
		BaseEntity: entity.BaseEntity{Id: &id},
		ConAt:      &now,
	}); err != nil {
		logrus.WithFields(logrus.Fields{"reason": "connection_time_update_failed", "account_id": id}).Warn("hysteria2_auth")
		vo.Hysteria2AuthFail("", c)
		return
	}
	// The system logger defaults to Warn; keep successful diagnostic events
	// visible as well, using only the internal account ID for correlation.
	logrus.WithFields(logrus.Fields{"reason": "success", "account_id": id}).Warn("hysteria2_auth")
	vo.Hysteria2AuthSuccess(username, limit, id, c)
}

func Hysteria2Kick(c *gin.Context) {
	hysteria2KickDto, err := validateField(c, dto.Hysteria2KickDto{})
	if err != nil {
		return
	}
	err = service.Hysteria2Kick(hysteria2KickDto.Ids, *hysteria2KickDto.KickUtilTime)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func Hysteria2ChangeVersion(c *gin.Context) {
	hysteria2VersionDto, err := validateField(c, dto.Hysteria2VersionDto{})
	if err != nil {
		return
	}

	running := hysteria2IsRunning()
	if running {
		vo.Fail("please stop hysteria2 first", c)
		return
	}

	if err = downloadHysteria2Version(*hysteria2VersionDto.Version); err != nil {
		vo.Fail(err.Error(), c)
		return
	}

	vo.Success(nil, c)
}

func ListRelease(c *gin.Context) {
	vo.Success([]string{constant.Hysteria2CustomVersion}, c)
}

func Hysteria2RoutingStatus(c *gin.Context) {
	status, err := service.Hysteria2RoutingStatus()
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(status, c)
}

func Hysteria2Start(c *gin.Context) {
	hysteria2LifecycleResponse(c, startHysteria2)
}

func Hysteria2Stop(c *gin.Context) {
	hysteria2LifecycleResponse(c, stopHysteria2)
}

func Hysteria2Restart(c *gin.Context) {
	hysteria2LifecycleResponse(c, restartHysteria2)
}

func hysteria2LifecycleResponse(c *gin.Context, operation func() error) {
	if err := operation(); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func Hysteria2Url(c *gin.Context) {
	hysteria2UrlDto, err := validateField(c, dto.Hysteria2UrlDto{})
	if err != nil {
		return
	}

	url, err := service.Hysteria2Url(*hysteria2UrlDto.AccountId, *hysteria2UrlDto.Hostname)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	// 生成二维码
	qrCode, err := qrcode.Encode(url, qrcode.Medium, 300)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	hysteria2UrlVo := vo.Hysteria2UrlVo{
		Url:    url,
		QrCode: qrCode,
	}
	vo.Success(hysteria2UrlVo, c)
}

func Hysteria2SubscribeUrl(c *gin.Context) {
	hysteria2SubscribeUrlDto, err := validateField(c, dto.Hysteria2SubscribeUrlDto{})
	if err != nil {
		return
	}
	subscribeUrl, err := service.Hysteria2SubscribeUrl(*hysteria2SubscribeUrlDto.AccountId,
		*hysteria2SubscribeUrlDto.Protocol,
		*hysteria2SubscribeUrlDto.Host)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	qrCode, err := qrcode.Encode(subscribeUrl, qrcode.Medium, 300)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	subscribeVo := vo.Hysteria2SubscribeVo{
		Url:    subscribeUrl,
		QrCode: qrCode,
	}
	vo.Success(subscribeVo, c)
}

func Hysteria2Subscribe(c *gin.Context) {
	conPass := c.Param("conPass")
	conPass, err := url.QueryUnescape(conPass)
	if err != nil {
		vo.Fail("url decode err", c)
		return
	}
	userAgent := strings.ToLower(c.Request.Header.Get("User-Agent"))
	host := c.Request.Host

	if host == "" {
		vo.Fail("Host is empty", c)
		return
	}

	var clientType string
	if strings.Contains(userAgent, constant.Shadowrocket) {
		clientType = constant.Shadowrocket
	} else if strings.Contains(userAgent, constant.Clash) {
		clientType = constant.Clash
	} else if strings.Contains(userAgent, constant.V2rayN) {
		clientType = constant.V2rayN
	} else if strings.Contains(userAgent, constant.NekoBox) {
		clientType = constant.NekoBox
	} else {
		clientType = constant.Clash
	}

	userInfo, configStr, err := service.Hysteria2Subscribe(conPass, clientType, host)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}

	if clientType == constant.Shadowrocket || clientType == constant.Clash {
		c.Header("content-disposition", "attachment; filename=hui.yaml")
		c.Header("profile-update-interval", "12")
		c.Header("subscription-userinfo", userInfo)
	} else if clientType == constant.V2rayN {
		configStr = base64.StdEncoding.EncodeToString([]byte(configStr))
	}

	c.String(200, configStr)
}
