package service

import (
	"errors"
	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/entity"
	"h-ui/proxy"
	"sync/atomic"
)

var hysteria2BandwidthReady atomic.Bool

func accountBandwidth(account entity.Account) bo.AccountBandwidth {
	policy := bo.AccountBandwidth{Version: account.BandwidthVersion}
	if account.UploadMbps != nil {
		policy.UploadMbps = *account.UploadMbps
	}
	if account.DownloadMbps != nil {
		policy.DownloadMbps = *account.DownloadMbps
	}
	return policy
}

func validateBandwidth(account entity.Account) error {
	for _, rate := range []*int64{account.UploadMbps, account.DownloadMbps} {
		if rate != nil && (*rate < 0 || *rate > 100000) {
			return errors.New("bandwidth must be an integer between 0 and 100000 Mbps")
		}
	}
	policy := accountBandwidth(account)
	if (policy.UploadMbps != 0 || policy.DownloadMbps != 0) && Hysteria2IsRunning() && !hysteria2BandwidthReady.Load() {
		return errors.New("running Hysteria2 kernel does not support account bandwidth")
	}
	return nil
}

func applyAccountBandwidth(id int64, result *AccountUpdateResult) {
	if !Hysteria2IsRunning() {
		result.BandwidthStatus = "stopped"
		return
	}
	result.BandwidthStatus = "unconfirmed"
	if !hysteria2BandwidthReady.Load() {
		result.BandwidthStatus = "unsupported"
		return
	}
	account, err := dao.GetAccount("id = ?", id)
	if err != nil {
		return
	}
	secret, err := trafficStatsSecret()
	if err != nil {
		return
	}
	api := proxy.NewHysteria2Api(hysteria2RuntimeStatsPort.Load())
	if api.UpdateBandwidth(id, accountBandwidth(account), secret) == nil {
		result.BandwidthStatus = "applied"
	}
}
