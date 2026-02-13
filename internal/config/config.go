package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// FileshareConfig is the IRC + fileshare config (Marvin-compatible subset + relay).
type FileshareConfig struct {
	Host         string `json:"Host"`
	Port         string `json:"Port"`
	Nick         string `json:"Nick"`
	Password     string `json:"Password"`
	Channel      string `json:"Channel"`
	Name         string `json:"Name"`
	Version      string `json:"Version"`
	Quit         string `json:"Quit"`
	ProxyEnabled bool   `json:"ProxyEnabled"`
	Proxy        string `json:"Proxy"`
	SASL         bool   `json:"SASL"`
	SlackAPIToken string `json:"SlackAPIToken,omitempty"`
	SharedDir    string `json:"SharedDir"`
	RelayTURNURL string `json:"RelayTURNURL"`
	RelayAuthUsername string `json:"RelayAuthUsername,omitempty"`
	RelayAuthSecret   string `json:"RelayAuthSecret,omitempty"`
	MaxUploadBytes int64 `json:"MaxUploadBytes,omitempty"`
	MaxFileBytes   int64 `json:"MaxFileBytes,omitempty"`
}

// LoadFileshareConfigs loads all *.json files from dir and returns valid fileshare configs (skips Slack).
func LoadFileshareConfigs(dir string) ([]*FileshareConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var configs []*FileshareConfig
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var c FileshareConfig
		if err := json.Unmarshal(data, &c); err != nil {
			continue
		}
		if c.SlackAPIToken != "" {
			continue
		}
		if c.Host == "" || c.SharedDir == "" || c.RelayTURNURL == "" {
			continue
		}
		configs = append(configs, &c)
	}
	return configs, nil
}
