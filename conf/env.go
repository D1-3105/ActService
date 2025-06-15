package conf

import (
	"github.com/caarlos0/env/v11"
	"log"
)

type ActEnviron struct {
	ActBinaryPath     string `env:"ACT_BINARY_PATH"`
	DockerContextPath string `env:"ACT_DOCKER_CONTEXT_PATH"`
}

type StorageEnviron struct {
	LogFileStorageRoot string `env:"LOG_FILE_STORAGE"`
}

func NewActEnviron() *ActEnviron {
	var actEnviron ActEnviron
	if err := env.Parse(&actEnviron); err != nil {
		log.Fatal(err)
	}
	return &actEnviron
}
