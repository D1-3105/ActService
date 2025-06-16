

compile_grpc:
	protoc \
	  --proto_path=api \
	  --go_out=api/gen --go_opt=paths=source_relative \
	  --go-grpc_out=api/gen --go-grpc_opt=paths=source_relative \
	  api/ActService/Job.proto

install_act:
	curl --proto '=https' --tlsv1.2 -sSf https://raw.githubusercontent.com/nektos/act/master/install.sh | bash
	mv bin/act /tmp/act

test:
	go test -v ./tests -args -logtostderr=true -v=1

