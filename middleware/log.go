package middleware

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"h-ui/model/constant"
	"h-ui/util"
	"net/http"
	"time"
)

func InitLog() {
	logrus.SetOutput(util.NewSynchronizedWriter(constant.SystemLogPath, &lumberjack.Logger{
		Filename:   constant.SystemLogPath,
		MaxSize:    1,
		MaxBackups: 2,
		MaxAge:     30,
		Compress:   true,
		LocalTime:  true,
	}))
	logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: "2006-01-02 15:04:05"})
	logrus.SetLevel(logrus.WarnLevel)
}

func LogHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		method := "OTHER"
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
			http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace:
			method = c.Request.Method
		}
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		// FullPath is the registered route template, never the requested URL.
		fmt.Fprintf(gin.DefaultWriter, "http method=%s route=%q status=%d duration=%s\n",
			method, route, c.Writer.Status(), time.Since(start))
	}
}
