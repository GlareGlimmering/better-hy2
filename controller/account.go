package controller

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"h-ui/model/constant"
	"h-ui/model/dto"
	"h-ui/model/entity"
	"h-ui/model/vo"
	"h-ui/service"
	"h-ui/util"
	"io"
	"strings"
	"time"
)

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
	loginDto, err := validateField(c, dto.LoginDto{})
	if err != nil {
		return
	}

	if !service.ExistAccountUsername(*loginDto.Username, 0) {
		vo.Fail("account not exist", c)
		return
	}

	token, err := service.Login(*loginDto.Username, util.SHA224String(*loginDto.Pass))
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
	result, err := service.SaveAccountWithResult(account)
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

	if accountUpdateDto.Username != nil && *accountUpdateDto.Username != "" && service.ExistAccountUsername(*accountUpdateDto.Username, *accountUpdateDto.Id) {
		vo.Fail(fmt.Sprintf("username %s already exists", *accountUpdateDto.Username), c)
		return
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

	var passEncrypt *string
	if accountUpdateDto.Pass != nil && *accountUpdateDto.Pass != "" {
		target, err := service.GetAccount(*accountUpdateDto.Id)
		if err != nil {
			vo.Fail(err.Error(), c)
			return
		}
		if target.Role == nil || *target.Role != "admin" {
			vo.Fail("regular users do not have a login password", c)
			return
		}
		hashed := util.SHA224String(*accountUpdateDto.Pass)
		passEncrypt = &hashed
	}

	account := entity.Account{
		Username:     accountUpdateDto.Username,
		Pass:         passEncrypt,
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

func ImportAccount(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	// 文件大小 2MB
	if header.Size > 1024*1024*2 {
		vo.Fail("the file is too big", c)
		return
	}
	// 文件后缀.json
	if !strings.HasSuffix(header.Filename, ".json") {
		vo.Fail("file format not supported", c)
		return
	}
	content, err := io.ReadAll(file)
	if err != nil {
		vo.Fail("json file read err", c)
		return
	}
	var accounts []entity.Account
	if err = json.Unmarshal(content, &accounts); err != nil {
		vo.Fail("content Unmarshal err", c)
		return
	}
	if err = service.UpsertAccount(accounts); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	service.ReconcileAccountConnections()
	vo.Success(nil, c)
}

func ExportAccount(c *gin.Context) {
	accountExports, err := service.ListExportAccount()
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}

	fileName := fmt.Sprintf("AccountExport-%s.json", time.Now().Format("20060102150405"))
	filePath := constant.ExportPathDir + fileName

	if err = util.ExportFile(filePath, accountExports, 0); err != nil {
		vo.Fail(err.Error(), c)
		return
	}

	// 下载
	if !util.Exists(filePath) {
		vo.Fail("file not exist", c)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.File(filePath)
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

func VerifyDefaultPass(c *gin.Context) {
	info, err := service.GetAccountInfo(c)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	account, err := service.GetAccount(info.Id)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(account.Pass != nil && *account.Pass == "02f382b76ca1ab7aa06ab03345c7712fd5b971fb0c0f2aef98bac9cd", c)
}
