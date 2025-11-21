CURRENT_APP_VERSION := $(shell git describe --tags --long --always)

REGISTRY_USER := d13105
REGISTRY_PASSWORD ?=

DOCKER_IMAGE_NAME := ${REGISTRY_USER}/actor:${CURRENT_APP_VERSION}

compile_grpc:
	protoc \
	  --proto_path=api \
	  --go_out=api/gen --go_opt=paths=source_relative \
	  --go-grpc_out=api/gen --go-grpc_opt=paths=source_relative \
	  api/ActService/Job.proto

install_act:
	curl --proto '=https' --tlsv1.2 -sSf https://raw.githubusercontent.com/nektos/act/master/install.sh | bash
	mv bin/act /tmp/act

install_act_persistent:
	curl --proto '=https' --tlsv1.2 -sSf https://raw.githubusercontent.com/nektos/act/master/install.sh | bash
	mv bin/act /bin/act


docker_final:
	docker build -t ${DOCKER_IMAGE_NAME} .

registry_login:
	echo "${REGISTRY_PASSWORD}" | docker login -u ${REGISTRY_USER} --password-stdin 2>/dev/null || true

push_image:
	docker push ${DOCKER_IMAGE_NAME}

upload_docker_artifacts: registry_login docker_final push_image

test:
	go test -v ./tests -args -logtostderr=true -v=2

