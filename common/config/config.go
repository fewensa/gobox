package config

import (
	"bytes"
	"flag"
	"fmt"
	"gbox/common"
	"github.com/ilyakaznacheev/cleanenv"
	"strings"
	"time"
)

var config Conf

type Conf struct {
	Runner struct {
		ServerAddr        string `json:"server_addr" yaml:"server_addr" env:"SERVER_ADDR" env-default:":8080" help:"server address"`
		FetchConfigUrl    string `json:"fetch_config_url" yaml:"fetch_config_url" env:"FETCH_CONFIG_URL" help:"fetch config url, If not set, local files will be used and will not automatically restart when configuration changes."`
		CheckInterval     string `json:"check_interval" yaml:"check_interval" env:"CHECK_INTERVAL" env-default:"1m" help:"check interval,like:1s or 1m or 1h"`
		ConfigPlaceHolder string `json:"config_place_holder" yaml:"config_place_holder" env:"CONFIG_PLACE_HOLDER" help:"it matches the placeholder and replaces it with the corresponding value, like:{{HELIX_RELAYER_PASSWORD}}=123,{{test}}=456. {{HELIX_RELAYER_PASSWORD}} and {{test}} will be replaced with 123 and 456. if FETCH_CONFIG_URL is not set, the local file will not be replaced."`
	}
	Program struct {
		RootDir    string `json:"root_dir" yaml:"root_dir" env:"HELIX_ROOT_DIR" env-default:"./relayer" help:"helix relayer root directory"`
		ConfigPath string `json:"config_path" yaml:"config_path" env:"CONFIG_PATH" env-default:"./.maintain/configure.json" help:"config path, HELIX_ROOT_DIR+CONFIG_PATH"`
		Env        string `json:"env" yaml:"env" env:"HELIX_ENV" env-default:"LP_BRIDGE_PATH=./.maintain/configure.json,LP_BRIDGE_STORE_PATH=./.maintain/db" help:"helix relayer environment variables, like:env1=true,env2=false"`
		Command    string `json:"command" yaml:"command" env:"HELIX_COMMAND" env-default:"node ./dist/src/main"`
	}
}

func (c Conf) IsNeedConfigUpdate() bool {
	return c.Runner.FetchConfigUrl != ""
}

func (c Conf) GetProgramEnv() map[string]string {
	envs := strings.Split(c.Program.Env, ",")
	env := make(map[string]string)
	for _, v := range envs {
		kv := strings.Split(v, "=")
		if len(kv) < 2 {
			continue
		}
		env[kv[0]] = strings.Join(kv[1:], "=")
	}
	return env
}

func (c Conf) GetHelixCommand() (name string, args []string) {
	command := strings.Split(c.Program.Command, " ")
	name = command[0]
	args = command[1:]
	return
}

func (c Conf) GetRunnerCheckInterval() time.Duration {
	d, err := time.ParseDuration(c.Runner.CheckInterval)
	if err != nil {
		return time.Minute
	}
	return d
}

func init() {
	flag.Usage = Help
	flag.Parse()
	err := cleanenv.ReadEnv(&config)
	if err != nil {
		panic(err)
	}
}

func Help() {
	c := &Conf{}
	runnerHelp := common.ExtractTagFromStruct(&c.Runner, "env", "env-default", "help")
	helixHelp := common.ExtractTagFromStruct(&c.Program, "env", "env-default", "help")
	fmt.Println("Runner Config:")
	for _, v := range runnerHelp {
		fmt.Printf("\t%s: %s. default=%s\n", v["env"], v["help"], v["env-default"])
	}

	fmt.Println("Program Config:")
	for _, v := range helixHelp {
		fmt.Printf("\t%s: %s. default=%s\n", v["env"], v["help"], v["env-default"])
	}
}

func Config() Conf {
	return config
}

func (c Conf) ReplacePlaceHolder(content []byte) []byte {
	if c.Runner.ConfigPlaceHolder == "" {
		return content
	}
	for _, v := range strings.Split(c.Runner.ConfigPlaceHolder, ",") {
		if v == "" || !strings.Contains(v, "=") {
			continue
		}
		data := strings.Split(v, "=")
		if len(data) < 2 {
			continue
		}
		content = bytes.ReplaceAll(content, []byte(data[0]), []byte(data[1]))
	}
	return content
}
