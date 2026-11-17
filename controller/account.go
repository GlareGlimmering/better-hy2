package controller

import (
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"h-ui/model/constant"
	"h-ui/model/dto"
	"h-ui/model/entity"
	"h-ui/model/vo"
	"h-ui/service"
	"time"
)

func BindUnassignedAccountOutbounds(c *gin.Context) {
	var input struct {
		Bindings map[string]string `json:"bindings"`
	}
	if c.ShouldBindJSON(&input) != nil || len(input.Bindings) == 0 {
		vo.Fail(constant.InvalidError, c)
		return
	}
	result, err := service.BindUnassignedAccountOutbounds(input.Bindings)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(result, c)
}

func AccountOutboundOptions(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	options, err := service.GetAccountOutboundOptions()
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(options, c)
}

func Login(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var credentials dto.LoginDto
	if c.ShouldBindJSON(&credentials) != nil || validate.Struct(credentials) != nil {
		vo.Fail(constant.InvalidError, c)
		return
	}
	token, err := service.Login(*credentials.Username, *credentials.Pass)
	if errors.Is(err, service.ErrPanelPasswordChangeRequired) {
		vo.Success(vo.JwtVo{PasswordChangeRequired: true}, c)
		return
	}
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	jwtVo := vo.JwtVo{
		TokenType:   constant.TokenType,
		AccessToken: token,
	}
	vo.Success(jwtVo, c)
}

func ChangePanelPassword(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var credentials dto.PanelPasswordChangeDto
	if c.ShouldBindJSON(&credentials) != nil || validate.Struct(credentials) != nil {
		vo.Fail(constant.InvalidError, c)
		return
	}
	token, err := service.ChangePanelPassword(*credentials.Username, *credentials.Pass, *credentials.NewPassword)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(vo.JwtVo{TokenType: constant.TokenType, AccessToken: token}, c)
}

func PageAccount(c *gin.Context) {
	accountPageDto, err := validateField(c, dto.AccountPageDto{})
	if err != nil {
		return
	}
	accounts, total, err := service.PageAccount(accountPageDto)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}

	onlineUsers, err := service.Hysteria2Online()
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}

	var accountVos []vo.AccountVo
	for _, item := range accounts {
		accountVo := vo.AccountVo{
			Username:     *item.Username,
			Quota:        *item.Quota,
			UploadMbps:   *item.UploadMbps,
			DownloadMbps: *item.DownloadMbps,
			Download:     *item.Download,
			Upload:       *item.Upload,
			ExpireTime:   *item.ExpireTime,
			KickUtilTime: *item.KickUtilTime,
			DeviceNo:     *item.DeviceNo,
			Role:         *item.Role,
			Deleted:      *item.Deleted,
			BaseVo: vo.BaseVo{
				Id:         *item.Id,
				CreateTime: *item.CreateTime,
			},
			LoginAt:  *item.LoginAt,
			ConAt:    *item.ConAt,
			Remark:   *item.Remark,
			Outbound: service.AccountOutbound(item),
		}
		if value, exists := onlineUsers[*item.Username]; exists {
			accountVo.Online = true
			accountVo.Device = value
			delete(onlineUsers, *item.Username)
		}
		accountVos = append(accountVos, accountVo)
	}
	accountPageVo := vo.AccountPageVo{
		AccountVos: accountVos,
		Total:      total,
	}
	vo.Success(accountPageVo, c)
}

func SaveAccount(c *gin.Context) {
	var accountSaveDto dto.AccountSaveDto
	if c.ShouldBindJSON(&accountSaveDto) != nil || validate.Struct(&accountSaveDto) != nil {
		vo.Fail(constant.InvalidError, c)
		return
	}

	if service.ExistAccountUsername(*accountSaveDto.Username, 0) {
		vo.Fail(fmt.Sprintf("username %s already exists", *accountSaveDto.Username), c)
		return
	}

	var password string
	if accountSaveDto.ConPass == nil || *accountSaveDto.ConPass == "" {
		var err error
		password, err = generateConnectionPassword(rand.Reader)
		if err != nil {
			vo.Fail("could not generate connection password", c)
			return
		}
	} else {
		password = *accountSaveDto.ConPass
	}
	conPass := fmt.Sprintf("%s.%s", *accountSaveDto.Username, password)
	emptyLoginPassword := ""
	account := entity.Account{
		Username:     accountSaveDto.Username,
		Pass:         &emptyLoginPassword,
		ConPass:      &conPass,
		Quota:        accountSaveDto.Quota,
		UploadMbps:   accountSaveDto.UploadMbps,
		DownloadMbps: accountSaveDto.DownloadMbps,
		ExpireTime:   accountSaveDto.ExpireTime,
		DeviceNo:     accountSaveDto.DeviceNo,
		Deleted:      accountSaveDto.Deleted,
		Remark:       accountSaveDto.Remark,
		Outbound:     accountSaveDto.Outbound,
	}
	result, err := service.SaveAccountWithResult(account, accountSaveDto.ExistingBindings)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(result, c)
}

func DeleteAccount(c *gin.Context) {
	idDto, err := validateField(c, dto.IdDto{})
	if err != nil {
		return
	}
	account, err := service.GetAccount(*idDto.Id)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	if *account.Role == "admin" {
		vo.Fail("admin cannot be deleted", c)
		return
	}
	result, err := service.DeleteAccountWithResult([]int64{*idDto.Id})
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	service.ReconcileAccountConnections()
	vo.Success(result, c)
}

func UpdateAccount(c *gin.Context) {
	var accountUpdateDto dto.AccountUpdateDto
	if c.ShouldBindJSON(&accountUpdateDto) != nil || validate.Struct(&accountUpdateDto) != nil {
		vo.Fail(constant.InvalidError, c)
		return
	}

	if accountUpdateDto.Username != nil && *accountUpdateDto.Username != "" {
		if service.ExistAccountUsername(*accountUpdateDto.Username, *accountUpdateDto.Id) {
			vo.Fail(fmt.Sprintf("username %s already exists", *accountUpdateDto.Username), c)
			return
		}
	}

	if accountUpdateDto.Deleted != nil && *accountUpdateDto.Deleted == 1 {
		account, err := service.GetAccount(*accountUpdateDto.Id)
		if err != nil {
			vo.Fail(err.Error(), c)
			return
		}
		if *account.Role == "admin" {
			vo.Fail("the admin account cannot be deleted", c)
			return
		}
	}

	account := entity.Account{
		Username:     accountUpdateDto.Username,
		ConPass:      accountUpdateDto.ConPass,
		Quota:        accountUpdateDto.Quota,
		UploadMbps:   accountUpdateDto.UploadMbps,
		DownloadMbps: accountUpdateDto.DownloadMbps,
		ExpireTime:   accountUpdateDto.ExpireTime,
		DeviceNo:     accountUpdateDto.DeviceNo,
		Deleted:      accountUpdateDto.Deleted,
		Remark:       accountUpdateDto.Remark,
		Outbound:     accountUpdateDto.Outbound,
		BaseEntity: entity.BaseEntity{
			Id: accountUpdateDto.Id,
		},
	}
	result, err := service.UpdateAccountWithResult(account)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	service.ReconcileAccountConnections()
	vo.Success(result, c)
}

func ResetTraffic(c *gin.Context) {
	idDto, err := validateField(c, dto.IdDto{})
	if err != nil {
		return
	}
	if err = service.ResetTraffic(*idDto.Id); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func GetAccountInfo(c *gin.Context) {
	accountInfoVo, err := service.GetAccountInfo(c)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	// 更新最近登录时间
	now := time.Now().UnixMilli()
	if err = service.UpdateAccount(entity.Account{
		BaseEntity: entity.BaseEntity{Id: &accountInfoVo.Id},
		LoginAt:    &now,
	}); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(accountInfoVo, c)
}

func GetAccount(c *gin.Context) {
	idDto, err := validateField(c, dto.IdDto{})
	if err != nil {
		return
	}
	account, err := service.GetAccount(*idDto.Id)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	accountVo := vo.AccountVo{
		BaseVo: vo.BaseVo{
			Id:         *account.Id,
			CreateTime: *account.CreateTime,
		},
		Username:     *account.Username,
		Quota:        *account.Quota,
		UploadMbps:   *account.UploadMbps,
		DownloadMbps: *account.DownloadMbps,
		Download:     *account.Download,
		Upload:       *account.Upload,
		ExpireTime:   *account.ExpireTime,
		DeviceNo:     *account.DeviceNo,
		Role:         *account.Role,
		Deleted:      *account.Deleted,
		Remark:       *account.Remark,
		Outbound:     service.AccountOutbound(account),
	}
	vo.Success(accountVo, c)
}

func ReleaseKickAccount(c *gin.Context) {
	idDto, err := validateField(c, dto.IdDto{})
	if err != nil {
		return
	}
	if err = service.ReleaseKickAccount(*idDto.Id); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}
