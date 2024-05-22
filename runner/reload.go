package runner

import (
	"bytes"
	"context"
	"fmt"
	"gbox/common/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var (
	shellScriptName = ".gobox.sh"
)

type Reload struct {
	conf             config.Conf
	input            string
	cmd              *exec.Cmd
	shutdownCallback func()
}

func (r *Reload) Init(conf config.Conf, input string, shutdownCallback func()) error {
	r.conf = conf
	r.input = input
	r.shutdownCallback = shutdownCallback
	if r.input == "" {
		return errors.New("input is empty")
	}
	if _, err := r.saveRelayerConfig(); err != nil {
		return errors.Wrap(err, "save relayer config")
	}

	return nil
}

// IsNeedRestart check if the config file is different from the old one
func (r *Reload) IsNeedRestart() (bool, error) {
	if !r.conf.IsNeedConfigUpdate() {
		return false, nil
	}
	return r.saveRelayerConfig()
}

// saveRelayerConfig save the new config to the local file, if the new config is different from the old one, return true
func (r *Reload) saveRelayerConfig() (bool, error) {
	c := r.conf.Runner
	helixConf := r.conf.Program
	if !r.conf.IsNeedConfigUpdate() {
		return false, nil
	}
	resp, err := (&http.Client{Timeout: time.Second * 10}).Get(c.FetchConfigUrl)
	if err != nil {
		return false, errors.Wrap(err, "get config file")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return false, errors.Errorf("status code: %d", resp.StatusCode)
	}
	oldConfData, err := os.ReadFile(filepath.Join(helixConf.RootDir, helixConf.ConfigPath))
	if !errors.Is(err, os.ErrNotExist) && err != nil {
		return false, errors.Wrap(err, "read old config file")
	}

	newConfData, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, errors.Wrap(err, "read new config file")
	}
	newConfData = r.conf.ReplacePlaceHolder(newConfData)
	if bytes.Equal(newConfData, oldConfData) {
		return false, nil
	}
	logrus.Debug(fmt.Sprintf("save new config file: %s", filepath.Join(helixConf.RootDir, helixConf.ConfigPath)))
	return true, os.WriteFile(filepath.Join(helixConf.RootDir, helixConf.ConfigPath), newConfData, 0644)
}

func (r *Reload) Run(ctx context.Context) (err error) {
	r.cmd, err = run(ctx, r.conf, r.input, r.shutdownCallback)
	if err != nil {
		return errors.Wrap(err, "run helix relayer")
	}
	return nil
}

func (r *Reload) Shutdown(ctx context.Context) error {
	if r.cmd == nil || r.cmd.Process == nil || r.cmd.ProcessState == nil {
		return nil
	}
	return shutdown(ctx, r.cmd)
}

func (r *Reload) Restart(ctx context.Context) (err error) {
	r.cmd, err = restart(ctx, r.cmd, r.input, r.shutdownCallback)
	return err
}

func (r *Reload) HealthCheck(ctx context.Context) error {
	return healthCheck(ctx, r.cmd)
}
