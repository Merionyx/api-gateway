package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"merionyx/api-gateway/internal/shared/etcd"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server              ServerConfig               `mapstructure:"server" validate:"required" json:"server"`
	Etcd                etcd.EtcdConfig            `mapstructure:"etcd"`
	Environments        []EnvironmentConfig        `mapstructure:"environments" json:"environments"`
	Services            ServicesConfig             `mapstructure:"services" json:"services"`
	APIServer           APIServerConfig            `mapstructure:"api_server" json:"api_server"`
	Tenant              string                     `mapstructure:"tenant" json:"tenant"`
	HA                  HAConfig                   `mapstructure:"ha" json:"ha"`
	LeaderElection      LeaderElectionConfig       `mapstructure:"leader_election" json:"leader_election"`
	KubernetesDiscovery *KubernetesDiscoveryConfig `mapstructure:"kubernetes_discovery" json:"kubernetes_discovery"`
}

// KubernetesDiscoveryConfig enables building environments from gateway.merionyx.io CRs and annotated Services.
type KubernetesDiscoveryConfig struct {
	Enabled                bool              `mapstructure:"enabled" json:"enabled"`
	NamespaceLabelSelector map[string]string `mapstructure:"namespace_label_selector" json:"namespace_label_selector"`
	ResourceLabelSelector  map[string]string `mapstructure:"resource_label_selector" json:"resource_label_selector"`
	WatchNamespaces        []string          `mapstructure:"watch_namespaces" json:"watch_namespaces"`
}

// HAConfig groups settings shared by all replicas of one logical Gateway Controller pool.
type HAConfig struct {
	// ControllerID is the id registered with API Server. Set the same value on every replica (default: hostname).
	ControllerID string `mapstructure:"controller_id" json:"controller_id"`
}

// LeaderElectionConfig elects a single replica to stream from API Server and write contract snapshots to etcd.
type LeaderElectionConfig struct {
	Enabled           bool   `mapstructure:"enabled" json:"enabled"`
	KeyPrefix         string `mapstructure:"key_prefix" json:"key_prefix"`
	Identity          string `mapstructure:"identity" json:"identity"`
	SessionTTLSeconds int    `mapstructure:"session_ttl_seconds" json:"session_ttl_seconds"`
}

type APIServerConfig struct {
	Address string `mapstructure:"address" json:"address"`
}

type ServerConfig struct {
	HTTP1Port string `mapstructure:"http1_port" validate:"required" json:"http1_port"`
	HTTP2Port string `mapstructure:"http2_port" validate:"required" json:"http2_port"`
	GRPCPort  string `mapstructure:"grpc_port" validate:"required" json:"grpc_port"`
	XDSPort   string `mapstructure:"xds_port" validate:"required" json:"xds_port"`
	Host      string `mapstructure:"host" json:"host"`
}

type EnvironmentConfig struct {
	Name     string         `mapstructure:"name" validate:"required" json:"name"`
	Bundles  BundlesConfig  `mapstructure:"bundles" validate:"required" json:"bundles"`
	Services ServicesConfig `mapstructure:"services" validate:"required" json:"services"`
}

type BundlesConfig struct {
	Static []StaticBundleConfig `mapstructure:"static" validate:"required" json:"static"`
}

type StaticBundleConfig struct {
	Name       string `mapstructure:"name" validate:"required" json:"name"`
	Repository string `mapstructure:"repository" validate:"required" json:"repository"`
	Ref        string `mapstructure:"ref" validate:"required" json:"ref"`
	Path       string `mapstructure:"path" validate:"required" json:"path"`
}

type ServicesConfig struct {
	Static []StaticServiceConfig `mapstructure:"static" validate:"required" json:"static"`
}

type StaticServiceConfig struct {
	Name     string `mapstructure:"name" validate:"required" json:"name"`
	Upstream string `mapstructure:"upstream" validate:"required" json:"upstream"`
}

func LoadConfig(configFile ...string) (*Config, error) {
	// Isolated instance: global viper can be mutated by other imports; Unmarshal must see the same tree as Set after patch.
	v := viper.New()
	v.SetDefault("server.http1_port", "8080")
	v.SetDefault("server.http2_port", "8443")
	v.SetDefault("server.grpc_port", "19090")
	v.SetDefault("server.xds_port", "19091")
	v.SetDefault("server.host", "localhost")
	v.SetDefault("logging.enabled", false)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "[${time}] ${status} - ${method} ${path}")
	v.SetDefault("leader_election.enabled", true)
	v.SetDefault("leader_election.key_prefix", "/api-gateway/controller/election/leader")
	v.SetDefault("leader_election.session_ttl_seconds", 5)

	v.AutomaticEnv()
	v.SetEnvPrefix("CP_")

	var explicitPath string
	if len(configFile) > 0 && configFile[0] != "" {
		explicitPath = configFile[0]
		slog.Info(fmt.Sprintf("Loading config from %s", explicitPath))
		v.SetConfigFile(explicitPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("./configs/control-plane")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			slog.Info("Config file not found, using defaults and environment variables")
		} else {
			slog.Error(fmt.Sprintf("Error reading config file: %v", err))
			return nil, err
		}
	} else {
		slog.Info(fmt.Sprintf("Using config file %s", v.ConfigFileUsed()))
		patchPath := explicitPath
		if patchPath == "" {
			patchPath = v.ConfigFileUsed()
		}
		if patchPath != "" {
			if err := patchViperKubernetesDiscoverySelectors(v, patchPath); err != nil {
				slog.Warn("kubernetes_discovery label selectors: normalize skipped", "err", err)
			}
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	if strings.TrimSpace(config.APIServer.Address) == "" {
		return nil, fmt.Errorf("api_server.address is required (gRPC address of API Server)")
	}

	k8sOn := config.KubernetesDiscovery != nil && config.KubernetesDiscovery.Enabled
	if !k8sOn {
		if len(config.Environments) == 0 {
			return nil, fmt.Errorf("environments are required when kubernetes_discovery.enabled is false")
		}
		if len(config.Services.Static) == 0 {
			return nil, fmt.Errorf("services.static is required when kubernetes_discovery.enabled is false")
		}
		for _, e := range config.Environments {
			if e.Name == "" {
				return nil, fmt.Errorf("environment name is required")
			}
			if len(e.Services.Static) == 0 {
				return nil, fmt.Errorf("environment %q: services.static is required when kubernetes_discovery is off", e.Name)
			}
		}
	}

	return &config, nil
}

// patchViperKubernetesDiscoverySelectors fixes YAML where dotted label keys (e.g. gateway.merionyx.io/team)
// were parsed as nested maps; viper/mapstructure then cannot decode into map[string]string.
func patchViperKubernetesDiscoverySelectors(v *viper.Viper, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}
	kd, _ := root["kubernetes_discovery"].(map[string]interface{})
	if kd == nil {
		return nil
	}
	// Use Set on the same Viper instance as ReadInConfig/Unmarshal (not global viper).
	// Replace, not merge, so stale nested keys like resource_label_selector.gateway are dropped.
	if raw, ok := kd["resource_label_selector"]; ok {
		if flat := flattenYAMLStringMap(raw); len(flat) > 0 {
			v.Set("kubernetes_discovery.resource_label_selector", flat)
		}
	}
	if raw, ok := kd["namespace_label_selector"]; ok {
		if flat := flattenYAMLStringMap(raw); len(flat) > 0 {
			v.Set("kubernetes_discovery.namespace_label_selector", flat)
		}
	}
	return nil
}

func flattenYAMLStringMap(v interface{}) map[string]string {
	out := make(map[string]string)
	switch t := v.(type) {
	case map[string]string:
		for k, s := range t {
			if strings.TrimSpace(s) != "" {
				out[k] = s
			}
		}
	case map[string]interface{}:
		flattenYAMLStringMapMerge("", t, out)
	case map[interface{}]interface{}:
		m2 := make(map[string]interface{}, len(t))
		for k, val := range t {
			m2[fmt.Sprint(k)] = val
		}
		flattenYAMLStringMapMerge("", m2, out)
	}
	return out
}

func flattenYAMLStringMapMerge(prefix string, m map[string]interface{}, out map[string]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch t := v.(type) {
		case string:
			out[key] = t
		case map[string]interface{}:
			flattenYAMLStringMapMerge(key, t, out)
		case map[interface{}]interface{}:
			m2 := make(map[string]interface{}, len(t))
			for kk, vv := range t {
				m2[fmt.Sprint(kk)] = vv
			}
			flattenYAMLStringMapMerge(key, m2, out)
		default:
			// skip non-scalar leaves
		}
	}
}
