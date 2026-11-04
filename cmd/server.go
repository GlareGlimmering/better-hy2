package cmd

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"h-ui/dao"
	"h-ui/middleware"
	"h-ui/model/constant"
	"h-ui/router"
	"h-ui/service"
	"h-ui/util"
	"net"
	"net/http"
	"os"
	"strconv"
)

var initializeHysteria2 = service.InitHysteria2
var installManagedXray = service.InstallXrayVersion
var launchBackground = func(operation func()) { go operation() }

func initializeManagedHysteria2() {
	// RC.16's upgrader allows only about ten seconds for the replacement
	// process to expose the panel. HY2 readiness can legitimately take longer,
	// so it must not delay the management listener of the downloaded candidate.
	launchBackground(func() {
		if err := initializeHysteria2(); err != nil {
			logrus.Errorf("Hysteria2 initialization failed; management panel will continue: %v", err)
		}
	})
}

func initializeManagedXray() {
	launchBackground(func() {
		if err := installManagedXray(constant.XrayPinnedVersion); err != nil {
			logrus.Errorf("pinned Xray installation failed; panel will continue and Xray remains stopped: %v", err)
		}
	})
}

func runServer(port, bindAddress string) error {
	defer releaseResource()

	middleware.InitLog()
	service.InitForward()
	if err := initFile(); err != nil {
		return err
	}
	if err := dao.InitSql(port); err != nil {
		return err
	}
	if err := middleware.InitCron(); err != nil {
		return err
	}
	// A missing or incompatible Hysteria2 kernel must not lock administrators
	// out of the panel that is needed to install the managed kernel. Keep HY2
	// stopped, report the reason, and continue starting the management server.
	initializeManagedHysteria2()
	// Install and verify the pinned kernel in the background. Installation does
	// not apply a configuration or start Xray, preserving fail-closed behavior.
	initializeManagedXray()
	if err := service.InitTableAndChain(); err != nil {
		logrus.Errorf(err.Error())
	}
	if err := service.InitPortHopping(); err != nil {
		logrus.Errorf(err.Error())
	}
	config, err := dao.GetConfig("key = ?", constant.HUIWebContext)
	if err != nil {
		return err
	}

	r := gin.Default()
	router.Router(r, config.Value)

	serverPort, crtPath, keyPath, err := service.GetServerPortAndCert()
	if err != nil {
		return err
	}

	service.InitServer(panelListenAddress(bindAddress, serverPort), r)
	if bindAddress == constant.PanelBindAddress {
		// Standard installations terminate TLS at a local reverse proxy. Keep
		// the loopback panel itself on plain HTTP for deterministic health checks.
		crtPath, keyPath = "", ""
	}
	if err := service.StartServer(crtPath, keyPath); err != nil && err != http.ErrServerClosed {
		logrus.Errorf("start server err: %v", err)
		return errors.New("start server err")
	}
	return nil
}

func panelListenAddress(bindAddress string, port int64) string {
	if bindAddress == "" {
		bindAddress = constant.PanelBindAddress
	}
	return net.JoinHostPort(bindAddress, strconv.FormatInt(port, 10))
}

func releaseResource() {
	if err := dao.CloseSqliteDB(); err != nil {
		logrus.Errorf(err.Error())
	}
	if err := service.ReleaseHysteria2(); err != nil {
		logrus.Errorf(err.Error())
	}
	if err := service.RemoveByComment(); err != nil {
		logrus.Errorf(err.Error())
	}
}

func initFile() error {
	var dirs = []string{constant.LogDir, constant.SqliteDBDir, constant.BinDir, constant.ExportPathDir}
	for _, item := range dirs {
		if !util.Exists(item) {
			if err := os.Mkdir(item, os.ModePerm); err != nil {
				logrus.Errorf("%s create err: %v", item, err)
				return errors.New(fmt.Sprintf("%s create err", item))
			}
		}
	}
	return nil
}
