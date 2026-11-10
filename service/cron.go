package service

import (
	"github.com/sirupsen/logrus"
	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/model/entity"
	"h-ui/proxy"
	"h-ui/util"
	"strconv"
	"sync"
	"time"
)

var trafficMutex sync.Mutex
var kickMutex sync.Mutex

func CronHandleAccount() {
	go func() {
		hysteriaEnable, err := dao.GetConfig("key = ?", constant.Hysteria2Enable)
		if err != nil {
			return
		}
		if hysteriaEnable.Value != nil && *hysteriaEnable.Value == "1" && Hysteria2IsRunning() {
			apiPort := hysteria2RuntimeStatsPort.Load()

			secret, err := trafficStatsSecret()
			if err != nil {
				return
			}

			if err := saveAccountTraffic(apiPort, secret); err != nil {
				logrus.Warnf("traffic settlement deferred: %v", err)
			}
			kickAccount(apiPort, secret)
		}
	}()
}

func CronResetTraffic() {
	accounts, err := dao.ListAccount(nil, nil)
	if err != nil {
		return
	}
	var ids []int64
	for _, item := range accounts {
		ids = append(ids, *item.Id)
	}
	idsList := util.SplitArr(ids, 100)
	for _, item := range idsList {
		if err := dao.UpdateAccount(item, map[string]interface{}{"download": 0, "upload": 0}); err != nil {
			continue
		}
	}
}

func saveAccountTraffic(apiPort int64, secret string) error {
	if !trafficMutex.TryLock() {
		return nil
	}
	defer trafficMutex.Unlock()
	return settleManagedTraffic(apiPort, secret)
}

func settleManagedTraffic(apiPort int64, secret string) error {
	hysteria2TrafficTime, err := dao.GetConfig("key = ?", constant.Hysteria2TrafficTime)
	if err != nil {
		return err
	}
	hysteria2TrafficTimeFloat, err := strconv.ParseFloat(*hysteria2TrafficTime.Value, 64)
	if err != nil {
		return err
	}
	return settleTraffic(proxy.NewHysteria2Api(apiPort), secret, hysteria2TrafficTimeFloat)
}

// ReconcileAccountConnections handles committed administrative changes
// without waiting for the next scheduled quota/expiry reconciliation.
func ReconcileAccountConnections() {
	if !Hysteria2IsRunning() {
		return
	}
	port := hysteria2RuntimeStatsPort.Load()
	secret, err := trafficStatsSecret()
	if err != nil {
		return
	}
	if err := proxy.NewHysteria2Api(port).FenceAdmissions(secret); err != nil {
		logrus.Warnf("account admission fence failed; reconciliation will retry: %v", err)
	}
	kickAccount(port, secret)
}

type trafficBatchAPI interface {
	TrafficBatch(string) (bo.Hysteria2TrafficBatch, error)
	AcknowledgeTrafficBatch(string, bo.Hysteria2TrafficReceipt) error
}

func settleTraffic(api trafficBatchAPI, secret string, multiplier float64) error {
	batch, err := api.TrafficBatch(secret)
	if err != nil {
		return err
	}
	if err := dao.SettleTrafficBatch(batch, multiplier); err != nil {
		return err
	}
	return api.AcknowledgeTrafficBatch(secret, batch.Hysteria2TrafficReceipt)
}

func kickAccount(apiPort int64, jwtSecret string) {
	if !kickMutex.TryLock() {
		return
	}
	defer kickMutex.Unlock()

	users, err := proxy.NewHysteria2Api(apiPort).OnlineUsers(jwtSecret)
	if err != nil {
		return
	}
	if len(users) > 0 {
		i := 0
		usernames := make([]string, len(users))
		for k := range users {
			usernames[i] = k
			i++
		}
		usernameLists := util.SplitArr(usernames, 10)
		for _, usernameList := range usernameLists {
			accounts, err := dao.ListAccount("username in ?", usernameList)
			if err != nil {
				logrus.Warn("account reconciliation could not read account state")
				continue
			}
			toKick := connectionsToKick(usernameList, accounts, time.Now().UnixMilli())
			if len(toKick) != 0 {
				if err := proxy.NewHysteria2Api(apiPort).KickUsers(toKick, jwtSecret); err != nil {
					logrus.Warnf("account connection reconciliation deferred: %v", err)
				}
			}
		}
	}
}

func connectionsToKick(online []string, accounts []entity.Account, now int64) []string {
	allowed := make(map[string]bool, len(accounts))
	for _, account := range accounts {
		allowed[*account.Username] = accountTrafficPolicy(account, now) == ""
	}
	var result []string
	for _, username := range online {
		if !allowed[username] {
			result = append(result, username)
		}
	}
	return result
}
