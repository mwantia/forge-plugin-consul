package main

import (
	"github.com/mwantia/forge-plugin-consul/plugin/consul"
	"github.com/mwantia/forge-sdk/pkg/plugins/grpc"
)

func main() {
	grpc.Serve(consul.NewConsulDriver)
}
