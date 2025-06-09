package wasm

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"log"
	"net"
	"sync"
)

var (
	cfg  *config
	once sync.Once
)

type config struct {
	ErmesSecret     string `envconfig:"ERMES_SECRET" required:"true"`
	CentralRepoIP   string `envconfig:"CENTRAL_REPO_IP" default:""`
	CentralRepoPort int    `envconfig:"CENTRAL_REPO_PORT" default:"3000"`
}

func loadConfig() {
	cfg = &config{}
	err := envconfig.Process("", cfg)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if cfg.CentralRepoIP == "" {
		cfg.CentralRepoIP = GetDockerHost()
	}
	fmt.Println(cfg)
}

func ErmesSecret() string {
	once.Do(loadConfig)
	return cfg.ErmesSecret
}

func CentralRepoIP() string {
	once.Do(loadConfig)
	return cfg.CentralRepoIP
}

func CentralRepoPort() int {
	once.Do(loadConfig)
	return cfg.CentralRepoPort
}

func GetDockerHost() string {
	addrs, err := net.LookupHost("host.docker.internal")
	if err == nil && len(addrs) > 0 {
		return "host.docker.internal"
	}

	return "172.17.0.1"
}
