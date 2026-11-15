package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

const Hysteria2AuthPath = "/auth"

var hysteria2AuthAddress atomic.Pointer[string]

// StartHysteria2AuthListener publishes the callback only after binding a private
// socket. The already-bound panel socket is excluded by the OS.
func StartHysteria2AuthListener(handler http.Handler) (func(), error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	address := listener.Addr().String()
	if !hysteria2AuthAddress.CompareAndSwap(nil, &address) {
		_ = listener.Close()
		return nil, errors.New("Hysteria2 authentication listener is already running")
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			hysteria2AuthAddress.CompareAndSwap(&address, nil)
			logrus.Errorf("Hysteria2 authentication listener: %v", err)
		}
	}()
	return func() {
		hysteria2AuthAddress.CompareAndSwap(&address, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			_ = server.Close()
		}
	}, nil
}

func GetAuthHttpUrl() (string, error) {
	address := hysteria2AuthAddress.Load()
	if address == nil {
		return "", errors.New("Hysteria2 authentication listener is unavailable")
	}
	return "http://" + *address + Hysteria2AuthPath, nil
}
