package runner

import (
	"context"
	"gbox/common/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	SHELL_SCRIPT = `#!/usr/bin/expect
eval spawn node ./dist/src/main
expect "Password:"
send -- "$env(HELIX_RELAYER_PASSWORD)\r"
interact`
)

type Runner interface {
	Init(conf config.Conf, password string, waitErrorCallback func()) error
	Run(ctx context.Context) (err error)
	Shutdown(ctx context.Context) error
	Restart(ctx context.Context) (err error)
	HealthCheck(ctx context.Context) error
	IsNeedRestart() (bool, error)
}

type RelayerBuf struct {
	ReplaceFunc func(data []byte) string
}

func (s *RelayerBuf) Write(data []byte) (int, error) {
	if s.ReplaceFunc != nil {
		logrus.Infof("[RELAYER]: %s", s.ReplaceFunc(data))
		return len(data), nil
	}
	logrus.Infof("[RELAYER]: %s", data)
	return len(data), nil
}

func (s *RelayerBuf) String() string {
	return ""
}

func run(ctx context.Context, conf config.Conf, password string, waitErrorCallback func()) (*exec.Cmd, error) {
	err := os.WriteFile(filepath.Join(conf.Program.RootDir, shellScriptName), []byte(SHELL_SCRIPT), 0755)
	if err != nil {
		return nil, errors.Wrap(err, "write shell script")
	}
	subProcess := exec.CommandContext(ctx, filepath.Join(conf.Program.RootDir, shellScriptName))
	subProcess.Stderr = os.Stderr
	subProcess.Stdin = os.Stdin
	subProcess.Stdout = &RelayerBuf{
		ReplaceFunc: func(data []byte) string {
			return strings.ReplaceAll(string(data), password, "**********")
		},
	}

	_ = os.Setenv("HELIX_RELAYER_PASSWORD", password)
	for k, v := range conf.GetProgramEnv() {
		_ = os.Setenv(k, v)
	}

	subProcess.Env = os.Environ()
	subProcess.Dir = conf.Program.RootDir
	if err := subProcess.Start(); err != nil {
		return nil, errors.Wrap(err, "start helix relayer")
	}
	go func() {
		err := subProcess.Wait()
		if err != nil && (strings.Contains(err.Error(), "signal: killed") || errors.Is(err, context.Canceled)) {
			return
		}
		if err != nil {
			logrus.Errorf("helix relayer process exited: %s\n", err)
		}

		if waitErrorCallback != nil {
			waitErrorCallback()
		}
	}()
	return subProcess, nil
}

func healthCheck(ctx context.Context, cmd *exec.Cmd) error {
	serveHealthCheck := func() error {
		client := &http.Client{
			Timeout: time.Second * 2,
		}
		resp, err := client.Get("http://127.0.0.1:3000/")
		if err != nil {
			return errors.Wrap(err, "http.Get()")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return errors.Errorf("status code: %d", resp.StatusCode)
		}
		return nil
	}
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return errors.Errorf("helix relayer process exited")
	}
	if cmd.Process == nil {
		return errors.Errorf("helix relayer process is nil")
	}

	_, err := os.FindProcess(cmd.Process.Pid)
	if err != nil {
		return errors.Wrap(err, "find process")
	}

	if err := serveHealthCheck(); err != nil {
		return err
	}
	return nil
}

func shutdown(ctx context.Context, cmd *exec.Cmd) error {
	if cmd == nil || cmd.ProcessState == nil {
		return nil
	}
	if cmd.ProcessState.Exited() {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil {
		return errors.Wrap(err, "send interrupt signal")
	}
	for {
		if _, err := os.FindProcess(cmd.Process.Pid); err != nil {
			logrus.Debug(err.Error())
			return nil
		}
		time.Sleep(time.Second)
	}
}

func restart(ctx context.Context, cmd *exec.Cmd, password string, waitErrorCallback func()) (*exec.Cmd, error) {
	_ = shutdown(ctx, cmd)
	return run(ctx, config.Config(), password, waitErrorCallback)
}

func Run(ctx context.Context, conf config.Conf, input string, runner Runner) error {
	var waitErrorCallback = make(chan struct{})
	if err := runner.Init(conf, input, func() {
		return
	}); err != nil {
		return errors.Wrap(err, "init runner")
	}
	runnerCtx, cancel := context.WithCancel(context.Background())
	start := func(runFunc func(ctx context.Context) error) {
		err := runFunc(runnerCtx)
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.Errorf("run helix relayer failed: %s\n", err)
			return
		}
	}
	doCancelAndWait := func() {
		cancel()
		for runner.HealthCheck(ctx) == nil {
			logrus.Debug("waiting for helix relayer process exited...")
			time.Sleep(time.Second)
		}
		runnerCtx, cancel = context.WithCancel(context.Background())
	}

	start(runner.Run)
	var (
		t                     = time.NewTicker(conf.GetRunnerCheckInterval())
		healthCheckErrorCount = 0
	)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			logrus.Info("main process exited, shutting down helix relayer...")
			doCancelAndWait()
			return nil
		case <-waitErrorCallback:
			doCancelAndWait()
			start(runner.Restart)
			continue
		case <-t.C:
			ok, err := runner.IsNeedRestart()
			if err != nil {
				logrus.Warnf("check if need restart failed: %s\n", err)
			}
			if ok {
				logrus.Infof("helix relayer config changed, restarting...\n")
				doCancelAndWait()
				start(runner.Restart)
				healthCheckErrorCount = 0
				continue
			}
			if healthCheckErrorCount > 3 {
				logrus.Errorf("helix relayer health check failed more than 3 times, restarting...\n")
				doCancelAndWait()
				start(runner.Restart)
				continue
			}
			if err := runner.HealthCheck(ctx); err != nil {
				healthCheckErrorCount++
				logrus.Warnf("helix relayer health check failed: %s\n", err)
				continue
			}
			t.Reset(conf.GetRunnerCheckInterval())
			logrus.Infof("helix relayer still running, next check in %s\n", time.Now().Add(conf.GetRunnerCheckInterval()))
			healthCheckErrorCount = 0
			continue
		}
	}
}
