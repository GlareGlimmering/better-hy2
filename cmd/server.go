package cmd

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"h-ui/controller"
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
	"strings"
)

var initializeHysteria2 = service.InitHysteria2
var installManagedXray = service.InstallXrayVersion
var managedXrayRequired = service.ManagedXrayKernelRequired
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
		required, err := managedXrayRequired()
		if err != nil {
			logrus.Errorf("inspect managed Xray requirements; Xray installation skipped: %v", err)
			return
		}
		if !required {
			return
		}
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
	if err := protectHY2Configuration(constant.Hysteria2ConfigPath); err != nil {
		return err
	}
	if err := dao.InitSql(port); err != nil {
		return err
	}
	if err := middleware.InitCron(); err != nil {
		return err
	}
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

	r := newPanelRouter(config.Value)
	if err := configurePanelTrustedProxies(r, os.Getenv("HUI_TRUSTED_PROXIES")); err != nil {
		return fmt.Errorf("invalid HUI_TRUSTED_PROXIES: %w", err)
	}

	serverPort, crtPath, keyPath, err := service.GetServerPortAndCert()
	if err != nil {
		return err
	}

	address := panelListenAddress(bindAddress, serverPort)
	panelListener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("bind panel listener: %w", err)
	}
	defer panelListener.Close()
	stopAuth, err := service.StartHysteria2AuthListener(newHysteria2AuthRouter())
	if err != nil {
		return fmt.Errorf("bind Hysteria2 authentication listener: %w", err)
	}
	defer stopAuth()
	service.InitServer(address, r)

	// The callback is reachable before the background process generates YAML
	// and starts HY2. An incompatible HY2 kernel still leaves the panel available.
	initializeManagedHysteria2()
	initializeManagedXray()
	if bindAddress == constant.PanelBindAddress {
		// Standard installations terminate TLS at a local reverse proxy. Keep
		// the loopback panel itself on plain HTTP for deterministic health checks.
		crtPath, keyPath = "", ""
	}
	if err := service.StartServer(panelListener, crtPath, keyPath); err != nil && err != http.ErrServerClosed {
		logrus.Errorf("start server err: %v", err)
		return errors.New("start server err")
	}
	return nil
}

func newPanelRouter(contextPath *string) *gin.Engine {
	// Gin's debug redirect diagnostics include the raw URL.
	if gin.Mode() == gin.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	_ = configurePanelTrustedProxies(r, "")
	r.Use(middleware.LogHandler(), gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, _ interface{}) {
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
	router.Router(r, contextPath)
	return r
}

func configurePanelTrustedProxies(engine *gin.Engine, setting string) error {
	var proxies []string
	for _, address := range strings.Split(setting, ",") {
		if address = strings.TrimSpace(address); address != "" {
			proxies = append(proxies, address)
		}
	}
	engine.RemoteIPHeaders = []string{"X-Forwarded-For"}
	return engine.SetTrustedProxies(proxies)
}

func newHysteria2AuthRouter() http.Handler {
	r := gin.New()
	r.Use(gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, _ interface{}) {
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
	r.GET("/ready", func(c *gin.Context) {
		if !service.Hysteria2IsRunning() {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		c.Status(http.StatusNoContent)
	})
	r.POST(service.Hysteria2AuthPath, func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		controller.Hysteria2Auth(c)
	})
	return r
}

func protectHY2Configuration(path string) error {
	for _, file := range []string{path, path + ".candidate"} {
		if err := util.ProtectPrivateFile(file); err != nil {
			return fmt.Errorf("protect HY2 configuration: %w", err)
		}
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
