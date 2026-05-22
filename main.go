package main

import (
	"crypto/tls"
	"crypto/x509"
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
	"google.golang.org/grpc/credentials"
)

func main() {
	flag.Parse()
	var grpcEnvriron conf.ServerEnviron
	conf.NewEnviron(&grpcEnvriron)
	lis, err := net.Listen("tcp", grpcEnvriron.GRPCAddr)
	if err != nil {
		glog.Fatalf("failed to listen: %v", err)
	}

	var grpcOpts []grpc.ServerOption
	if grpcEnvriron.GRPSCertFile != "" && grpcEnvriron.GRPCKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(grpcEnvriron.GRPSCertFile, grpcEnvriron.GRPCKeyFile)
		if err != nil {
			glog.Fatalf("failed to load TLS keypair: %v", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		if grpcEnvriron.GRPCClientCAFile != "" {
			clientCAPool := x509.NewCertPool()
			clientCABytes, err := os.ReadFile(grpcEnvriron.GRPCClientCAFile)
			if err != nil {
				glog.Fatalf("failed to read client CA file: %v", err)
			}
			if !clientCAPool.AppendCertsFromPEM(clientCABytes) {
				glog.Fatalf("failed to parse client CA")
			}
			tlsConfig.ClientCAs = clientCAPool

			if grpcEnvriron.GRPCRequireClientAuth {
				tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
				glog.Warningf("mTLS enabled (strict)")
			} else {
				tlsConfig.ClientAuth = tls.RequestClientCert
				glog.Warningf("mTLS optional")
			}
		} else {
			tlsConfig.ClientAuth = tls.NoClientCert
			glog.Warningf("TLS only (no client auth)")
		}

		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}

	grpcServer := grpc.NewServer(grpcOpts...)

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
