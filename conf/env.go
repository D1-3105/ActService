package conf

import (
	"github.com/caarlos0/env/v11"
)

type ActEnviron struct {
	ActBinaryPath     string `env:"ACT_BINARY_PATH"`
	DockerContextPath string `env:"ACT_DOCKER_CONTEXT_PATH"`
}

type ServerEnviron struct {
	GRPCAddr string `env:"GRPC_ADDR" envDefault:":50051"`
}

type StorageEnviron struct {
	LogFileStorageRoot string `env:"LOG_FILE_STORAGE"`
}

func NewEnviron(environ any) {
	if err := env.Parse(environ); err != nil {
		panic(err)
	}
}
