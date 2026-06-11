package consul

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/mapstructure"
	"github.com/mwantia/forge-sdk/pkg/plugins"
)

const (
	PluginName        = "consul"
	PluginAuthor      = "forge"
	PluginVersion     = "0.1.0"
	PluginDescription = "HashiCorp Consul service mesh and key-value store tools"
)

type ConsulDriver struct {
	plugins.UnimplementedDriver

	log          hclog.Logger
	config       *ConsulConfig
	client       *api.Client
	capabilities ConsulCapabilitySet
	enabled      []string
}

type ConsulConfig struct {
	Address    string     `mapstructure:"address"`
	Token      string     `mapstructure:"token"`
	Datacenter string     `mapstructure:"datacenter"`
	Namespace  string     `mapstructure:"namespace"`
	Partition  string     `mapstructure:"partition"`
	Timeout    int        `mapstructure:"timeout"`
	TLS        *TLSConfig `mapstructure:"tls"`
}

type TLSConfig struct {
	CAFile             string `mapstructure:"ca_file"`
	CertFile           string `mapstructure:"cert_file"`
	KeyFile            string `mapstructure:"key_file"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
}

func NewConsulDriver(log hclog.Logger) plugins.Driver {
	return &ConsulDriver{
		log: log.Named(PluginName),
	}
}

func (d *ConsulDriver) GetPluginInfo() plugins.PluginInfo {
	return plugins.PluginInfo{
		Name:        PluginName,
		Author:      PluginAuthor,
		Version:     PluginVersion,
		Description: PluginDescription,
	}
}

func (d *ConsulDriver) GetPluginHealth(ctx context.Context) (*plugins.PluginHealth, error) {
	if d.client == nil {
		return &plugins.PluginHealth{
			Status:  plugins.StatusUnhealthy,
			Code:    plugins.HealthCodeConfigInvalid,
			Message: "consul client not configured",
		}, nil
	}
	_, err := d.client.Status().Leader()
	if err != nil {
		return &plugins.PluginHealth{
			Status:  plugins.StatusUnhealthy,
			Code:    plugins.HealthCodeConnectionRefused,
			Message: fmt.Sprintf("consul unreachable: %v", err),
			Action:  "Ensure consul is running and the address in the config is correct.",
		}, nil
	}
	return &plugins.PluginHealth{
		Status:  plugins.StatusHealthy,
		Code:    plugins.HealthCodeOK,
		Message: "consul reachable",
	}, nil
}

func (d *ConsulDriver) GetCapabilities(ctx context.Context) (*plugins.DriverCapabilities, error) {
	return &plugins.DriverCapabilities{
		Types: []string{plugins.PluginTypeTools},
		Tools: &plugins.ToolsCapabilities{
			SupportsAsyncExecution: false,
		},
	}, nil
}

func (d *ConsulDriver) ConfigDriver(ctx context.Context, config plugins.PluginConfig) error {
	cfg := &ConsulConfig{}
	if err := mapstructure.Decode(config.ConfigMap, cfg); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	if cfg.Address == "" {
		cfg.Address = "http://localhost:8500"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30
	}

	d.config = cfg

	consulCfg := api.DefaultConfig()
	consulCfg.Address = cfg.Address
	consulCfg.Token = cfg.Token
	consulCfg.Datacenter = cfg.Datacenter
	consulCfg.Namespace = cfg.Namespace
	consulCfg.Partition = cfg.Partition

	if cfg.TLS != nil {
		tlsCfg, err := buildTLSConfig(cfg.TLS)
		if err != nil {
			return fmt.Errorf("failed to build TLS config: %w", err)
		}
		consulCfg.HttpClient = &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
			Timeout:   time.Duration(cfg.Timeout) * time.Second,
		}
	} else {
		consulCfg.HttpClient = &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		}
	}

	client, err := api.NewClient(consulCfg)
	if err != nil {
		return fmt.Errorf("failed to create consul client: %w", err)
	}
	d.client = client

	d.log.Info("Consul configured", "address", cfg.Address, "datacenter", cfg.Datacenter)
	return nil
}

func (d *ConsulDriver) OpenDriver(ctx context.Context) error {
	d.capabilities = ProbeCapabilities(ctx, d.client, d.log)
	d.enabled = BuildEnabledTools(d.capabilities)
	d.log.Info("Consul capabilities probed", "enabled", len(d.enabled), "capabilities", d.capabilities.Summary())

	return nil
}

func (d *ConsulDriver) CloseDriver(ctx context.Context) error {
	return nil
}

func (d *ConsulDriver) GetToolsPlugin(ctx context.Context) (plugins.ToolsPlugin, error) {
	return &ConsulToolsPlugin{driver: d}, nil
}

func buildTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec
	}

	if cfg.CAFile != "" {
		caPEM, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsCfg.RootCAs = pool
	}

	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}
