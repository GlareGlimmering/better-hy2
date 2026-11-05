package controller

import (
	"github.com/gin-gonic/gin"
	"h-ui/model/bo"
	"h-ui/model/dto"
	"h-ui/model/vo"
	"h-ui/service"
	"io"
	"strings"
)

func GetXraySettings(c *gin.Context) {
	settings, err := service.GetXraySettings()
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(settings, c)
}

func SaveXrayDraft(c *gin.Context) {
	input, err := validateField(c, dto.XraySettingsDto{})
	if err != nil {
		return
	}
	settings := bo.XraySettings{
		Version: strings.TrimSpace(*input.Version),
	}
	for _, item := range *input.Outbounds {
		outbound := bo.XrayManagedOutbound{
			Name: strings.TrimSpace(*item.Name), Protocol: strings.TrimSpace(*item.Protocol), Enabled: *item.Enabled,
			Address: strings.TrimSpace(*item.Address), Port: *item.Port,
			UUID: strings.TrimSpace(valueOrEmpty(item.UUID)), Network: strings.TrimSpace(valueOrEmpty(item.Network)),
			Username: strings.TrimSpace(valueOrEmpty(item.Username)), Password: valueOrEmpty(item.Password),
			TLSSNI: strings.TrimSpace(valueOrEmpty(item.TLSSNI)), UseCustomCA: boolOrFalse(item.UseCustomCA),
			SOCKSListen: strings.TrimSpace(valueOrEmpty(item.SOCKSListen)), SOCKSPort: uint16OrZero(item.SOCKSPort),
			OutboundTag: strings.TrimSpace(valueOrEmpty(item.OutboundTag)),
		}
		if item.ID != nil {
			outbound.ID = strings.TrimSpace(*item.ID)
		}
		settings.Outbounds = append(settings.Outbounds, outbound)
	}
	if err = service.SaveXrayDraft(settings); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func ValidateXrayDraft(c *gin.Context) {
	result, err := service.ValidateXrayDraft()
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(result, c)
}

func ApplyXrayDraft(c *gin.Context) {
	if err := service.ApplyXrayDraft(); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func InstallXrayVersion(c *gin.Context) {
	input, err := validateField(c, dto.XrayVersionDto{})
	if err != nil {
		return
	}
	if err = service.InstallXrayVersion(strings.TrimSpace(*input.Version)); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func UploadXrayCA(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		vo.Fail("CA certificate file is required", c)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 2<<20))
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	if len(data) == 0 || len(data) >= 2<<20 {
		vo.Fail("CA certificate file must be smaller than 2 MB", c)
		return
	}
	status, err := service.SaveXrayCA(data)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(status, c)
}

func GetXrayStatus(c *gin.Context) { vo.Success(service.GetXrayStatus(), c) }

func StartXray(c *gin.Context) {
	if err := service.StartXray(); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func StopXray(c *gin.Context) {
	if err := service.StopXray(); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func RestartXray(c *gin.Context) {
	if err := service.RestartXray(); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func TestXrayTCP(c *gin.Context) {
	input, err := validateField(c, dto.XrayOutboundTestDto{})
	if err != nil {
		return
	}
	vo.Success(service.TestXrayOutbound(*input.ID, *input.Protocol), c)
}

func TestXrayUDP(c *gin.Context) {
	TestXrayTCP(c)
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func boolOrFalse(value *bool) bool {
	return value != nil && *value
}

func uint16OrZero(value *uint16) uint16 {
	if value == nil {
		return 0
	}
	return *value
}
