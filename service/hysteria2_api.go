package service

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/proxy"
	"net"
	"net/url"
	"strings"
	"time"
)

func Hysteria2Auth(conPass string) (int64, string, error) {
	if !Hysteria2IsRunning() {
		return 0, "", errors.New("hysteria2 is not running")
	}

	now := time.Now().UnixMilli()
	account, err := dao.GetAccount("con_pass = ? and deleted = 0 and (quota < 0 or quota > download + upload) and ? < expire_time and ? > kick_util_time", conPass, now, now)
	if err != nil {
		return 0, "", err
	}

	// 限制设备数
	onlineUsers, err := Hysteria2Online()
	if err != nil {
		return 0, "", err
	}
	device, exist := onlineUsers[*account.Username]
	if exist && *account.DeviceNo <= device {
		return 0, "", errors.New("device limited")
	}

	return *account.Id, *account.Username, nil
}

func Hysteria2Online() (map[string]int64, error) {
	if !Hysteria2IsRunning() {
		return map[string]int64{}, nil
	}
	apiPort, err := GetHysteria2ApiPort()
	if err != nil {
		return nil, errors.New("get hysteria2 apiPort err")
	}
	jwtSecretConfig, err := dao.GetConfig("key = ?", constant.JwtSecret)
	if err != nil {
		return nil, err
	}
	onlineUsers, err := proxy.NewHysteria2Api(apiPort).OnlineUsers(*jwtSecretConfig.Value)
	if err != nil {
		return nil, err
	}
	return onlineUsers, nil
}

func Hysteria2Kick(ids []int64, kickUtilTime int64) error {
	if !Hysteria2IsRunning() {
		return errors.New("hysteria2 is not running")
	}
	if err := dao.UpdateAccount(ids, map[string]interface{}{"kick_util_time": kickUtilTime}); err != nil {
		return err
	}

	accounts, err := dao.ListAccount("id in ?", ids)
	if err != nil {
		return err
	}
	var keys []string
	for _, item := range accounts {
		keys = append(keys, *item.Username)
	}
	apiPort, err := GetHysteria2ApiPort()
	if err != nil {
		return errors.New("get hysteria2 apiPort err")
	}
	jwtSecretConfig, err := dao.GetConfig("key = ?", constant.JwtSecret)
	if err != nil {
		return err
	}
	if err = proxy.NewHysteria2Api(apiPort).KickUsers(keys, *jwtSecretConfig.Value); err != nil {
		return err
	}
	return nil
}

func Hysteria2SubscribeUrl(accountId int64, protocol string, host string) (string, error) {
	account, err := dao.GetAccount("id = ?", accountId)
	if err != nil {
		return "", err
	}
	config, err := dao.GetConfig("key = ?", constant.HUIWebContext)
	if err != nil {
		return "", err
	}
	webContext := ""
	if config.Value != nil && *config.Value != "/" && strings.HasPrefix(*config.Value, "/") {
		webContext = *config.Value
	}
	return fmt.Sprintf("%s//%s%s/hui/%s", protocol, host, webContext, url.QueryEscape(*account.ConPass)), nil
}

func Hysteria2Subscribe(conPass string, clientType string, host string) (string, string, error) {
	hysteria2Config, err := GetHysteria2Config()
	if err != nil {
		return "", "", err
	}
	if hysteria2Config.Listen == nil || *hysteria2Config.Listen == "" {
		return "", "", errors.New("hysteria2 config is empty")
	}

	account, err := dao.GetAccount("con_pass = ?", conPass)
	if err != nil {
		return "", "", err
	}

	node, err := subscriptionNodeForAccount(account)
	if err != nil {
		return "", "", err
	}

	hysteria2ConfigPortHopping, err := dao.GetConfig("key = ?", constant.Hysteria2ConfigPortHopping)
	if err != nil {
		return "", "", err
	}

	userInfo := ""
	configStr := ""
	if clientType == constant.Shadowrocket || clientType == constant.Clash {
		userInfo = fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d",
			*account.Upload,
			*account.Download,
			*account.Quota,
			*account.ExpireTime/1000)

		hysteria2 := bo.Hysteria2{
			Name:     node.Name,
			Type:     "hysteria2",
			Server:   node.Address,
			Port:     hysteria2ListenPort(*hysteria2Config.Listen),
			Ports:    *hysteria2ConfigPortHopping.Value,
			Password: conPass,
		}

		if hysteria2Config.Bandwidth != nil {
			if hysteria2Config.Bandwidth.Up != nil &&
				*hysteria2Config.Bandwidth.Up != "" {
				hysteria2.Up = *hysteria2Config.Bandwidth.Up
			}
			if hysteria2Config.Bandwidth.Down != nil &&
				*hysteria2Config.Bandwidth.Down != "" {
				hysteria2.Down = *hysteria2Config.Bandwidth.Down
			}
		}

		if hysteria2Config.Obfs != nil &&
			hysteria2Config.Obfs.Type != nil &&
			*hysteria2Config.Obfs.Type == "salamander" &&
			hysteria2Config.Obfs.Salamander != nil &&
			hysteria2Config.Obfs.Salamander.Password != nil &&
			*hysteria2Config.Obfs.Salamander.Password != "" {
			if clientType == constant.Shadowrocket {
				hysteria2.Obfs = *hysteria2Config.Obfs.Salamander.Password
			} else {
				hysteria2.Obfs = "salamander"
				hysteria2.ObfsPassword = *hysteria2Config.Obfs.Salamander.Password
			}
		}

		if hysteria2Config.ACME != nil &&
			hysteria2Config.ACME.Domains != nil &&
			len(hysteria2Config.ACME.Domains) > 0 {
			hysteria2.Sni = hysteria2Config.ACME.Domains[0]
		}

		hysteria2.SkipCertVerify = false

		proxyGroup := bo.ProxyGroup{
			Name:    "PROXY",
			Type:    "select",
			Proxies: []string{node.Name},
		}

		clashConfig := bo.ClashConfig{
			ProxyGroups: []bo.ProxyGroup{
				proxyGroup,
			},
			Proxies: []interface{}{hysteria2},
		}
		clashConfigYaml, err := yaml.Marshal(&clashConfig)
		if err != nil {
			return "", "", err
		}
		configStr = string(clashConfigYaml)
		if clientType == constant.Clash {
			clashExtension, err := GetConfig(constant.ClashExtension)
			if err != nil {
				return "", "", err
			}
			if clashExtension.Value != nil && *clashExtension.Value != "" {
				configStr = fmt.Sprintf("%s%s", configStr, *clashExtension.Value)
			}
		}
	} else if clientType == constant.V2rayN || clientType == constant.NekoBox {
		hysteria2Url, err := Hysteria2Url(*account.Id, host)
		if err != nil {
			return "", "", err
		}
		configStr = hysteria2Url
	}

	return userInfo, configStr, nil
}

func Hysteria2Url(accountId int64, hostname string) (string, error) {
	// hostname is retained for API compatibility. NodeAddress deliberately
	// keeps the HTTP request host separate from the HY2 connection address.
	_ = hostname
	hysteria2Config, err := GetHysteria2Config()
	if err != nil {
		return "", err
	}
	if hysteria2Config.Listen == nil || *hysteria2Config.Listen == "" {
		return "", errors.New("hysteria2 config is empty")
	}

	account, err := dao.GetAccount("id = ?", accountId)
	if err != nil {
		return "", err
	}

	node, err := subscriptionNodeForAccount(account)
	if err != nil {
		return "", err
	}
	query := url.Values{}
	if hysteria2Config.Obfs != nil &&
		hysteria2Config.Obfs.Type != nil &&
		*hysteria2Config.Obfs.Type == "salamander" &&
		hysteria2Config.Obfs.Salamander != nil &&
		hysteria2Config.Obfs.Salamander.Password != nil &&
		*hysteria2Config.Obfs.Salamander.Password != "" {
		query.Set("obfs", "salamander")
		query.Set("obfs-password", *hysteria2Config.Obfs.Salamander.Password)
	}

	if hysteria2Config.ACME != nil &&
		hysteria2Config.ACME.Domains != nil &&
		len(hysteria2Config.ACME.Domains) > 0 {
		query.Set("sni", hysteria2Config.ACME.Domains[0])
		// shadowrocket
		query.Set("peer", hysteria2Config.ACME.Domains[0])
	}
	query.Set("insecure", "0")

	if hysteria2Config.Bandwidth != nil &&
		hysteria2Config.Bandwidth.Down != nil &&
		*hysteria2Config.Bandwidth.Down != "" {
		// shadowrocket
		query.Set("downmbps", *hysteria2Config.Bandwidth.Down)
	}

	hysteria2ConfigPortHopping, err := dao.GetConfig("key = ?", constant.Hysteria2ConfigPortHopping)
	if err != nil {
		return "", err
	}
	if *hysteria2ConfigPortHopping.Value != "" {
		// shadowrocket
		query.Set("mport", *hysteria2ConfigPortHopping.Value)
	}
	connectionHost := net.JoinHostPort(node.Address, hysteria2ListenPort(*hysteria2Config.Listen))
	result := &url.URL{
		Scheme:   "hysteria2",
		User:     url.User(*account.ConPass),
		Host:     connectionHost,
		Path:     "/",
		RawQuery: query.Encode(),
		Fragment: node.Name,
	}
	return result.String(), nil
}

func hysteria2ListenPort(listen string) string {
	if _, port, err := net.SplitHostPort(listen); err == nil {
		return port
	}
	return strings.TrimPrefix(listen, ":")
}
