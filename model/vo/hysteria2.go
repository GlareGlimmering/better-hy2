package vo

import (
	"github.com/gin-gonic/gin"
	"h-ui/model/bo"
	"net/http"
)

type Hysteria2RoutingStatusVo struct {
	Enabled          bool     `json:"enabled"`
	Ready            bool     `json:"ready"`
	ActiveUsers      int      `json:"activeUsers"`
	BoundUsers       int      `json:"boundUsers"`
	UnboundUsers     []string `json:"unboundUsers"`
	Errors           []string `json:"errors"`
	KernelVersion    string   `json:"kernelVersion"`
	KernelCompatible bool     `json:"kernelCompatible"`
}

type hysteria2Result struct {
	Ok          bool                 `json:"ok"`
	Id          string               `json:"id"`
	DeviceLimit int64                `json:"deviceLimit,omitempty"`
	Bandwidth   *bo.AccountBandwidth `json:"bandwidth,omitempty"`
	AccountID   int64                `json:"accountId,omitempty"`
}

func Hysteria2AuthSuccess(id string, deviceLimit, accountID int64, bandwidth bo.AccountBandwidth, c *gin.Context) {
	var policy *bo.AccountBandwidth
	if bandwidth.Version != 0 {
		policy = &bandwidth
	}
	c.JSON(http.StatusOK, hysteria2Result{
		Bandwidth:   policy,
		Ok:          true,
		Id:          id,
		DeviceLimit: deviceLimit,
		AccountID:   accountID,
	})
}

func Hysteria2AuthFail(id string, c *gin.Context) {
	c.JSON(http.StatusOK, hysteria2Result{
		Ok: false,
		Id: id,
	})
}

type Hysteria2SubscribeVo struct {
	Url    string `json:"url"`
	QrCode []byte `json:"qrCode"`
}

type Hysteria2UrlVo struct {
	Url    string `json:"url"`
	QrCode []byte `json:"qrCode"`
}

type Hysteria2AcmePathVo struct {
	CrtPath string `json:"crtPath"`
	KeyPath string `json:"keyPath"`
}
