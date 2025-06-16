package main

import (
	"flag"
	"github.com/D1-3105/ActService/conf"
	"github.com/golang/glog"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	actservice "github.com/D1-3105/ActService/api/gen/ActService"
	"github.com/D1-3105/ActService/api/rpc"

	"google.golang.org/grpc"
)

func main() {
	flag.Parse()
	var grpcEnvriron conf.ServerEnviron
	conf.NewEnviron(&grpcEnvriron)
	lis, err := net.Listen("tcp", grpcEnvriron.GRPCAddr)
	if err != nil {
		glog.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	actSvc := rpc.NewActService()

	actservice.RegisterActServiceServer(grpcServer, actSvc)

	glog.Warningf("gRPC server listening on %s", grpcEnvriron.GRPCAddr)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	sig := <-sigCh
	glog.Warningf("Received signal %s, shutting down...", sig)

	grpcServer.GracefulStop()
	glog.Warning("Server stopped")
}
