package plugin

import (
	"github.com/mwantia/forge-plugin-consul/internal/consul"
	"github.com/mwantia/forge-sdk/pkg/plugins"
)

func init() {
	plugins.Register(consul.PluginName, consul.PluginDescription, consul.NewConsulDriver)
}
