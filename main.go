package main

import (
	"context"
	"encoding/json"
	"gbox/common"
	"gbox/common/config"
	"gbox/runner"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	p = atomic.Value{}
)

func Password() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !strings.EqualFold(req.Method, http.MethodPost) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		var pass = struct {
			P string `json:"p"`
		}{}
		if err := json.NewDecoder(req.Body).Decode(&pass); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if pass.P == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		p.Store(pass.P)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		DisableSorting:  true,
		TimestampFormat: "2006-01-02 15:04:05",
		PadLevelText:    true,
		FullTimestamp:   true,
	})
	ctx, cancel := context.WithCancel(context.TODO())
	server := &http.Server{Addr: common.GetEnv("SERVER_ADDR", ":8080"), Handler: Password()}
	go func() {
		if err := server.ListenAndServe(); err != nil && !(errors.Is(err, http.ErrServerClosed) || errors.Is(err, context.Canceled)) {
			logrus.Panicf("http server error: %s", err)
		}
	}()
	logrus.SetLevel(logrus.DebugLevel)
	for {
		if p.Load() == nil || p.Load().(string) == "" {
			logrus.Debugf("waiting for input...")
			time.Sleep(time.Second)
			continue
		}
		break
	}
	go func() {
		if err := runner.Run(ctx, config.Config(), p.Load().(string), new(runner.Reload)); err != nil {
			logrus.Panicf("runner run error: %s", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()
	_ = server.Shutdown(context.TODO())
	logrus.Debug("waiting for shutdown...")
	time.Sleep(time.Second * 5)
}
