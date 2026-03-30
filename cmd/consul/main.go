package main

import (
	"github.com/mwantia/forge-plugin-consul/internal/consul"
	"github.com/mwantia/forge-sdk/pkg/plugins/grpc"
)

func main() {
	grpc.Serve(consul.NewConsulDriver)
}
