package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

var configLock sync.Mutex

type Plugin struct {
	Id     string   `json:"id"`
	Prefix string   `json:"prefix"`
	Group  []string `json:"group"`
}

type AppConfig struct {
	Port           uint16                    `json:"port"`
	AppId          string                    `json:"appid"`
	AppSecret      string                    `json:"secret"`
	Plugins        []Plugin                  `json:"plugins"`
	ProxyAPI       string                    `json:"proxy"`
	Uin            uint64                    `json:"uin"`
	Uid            string                    `json:"uid"`
	Database       string                    `json:"database"`
	AdminPassword  string                    `json:"admin_password"`
	PluginSettings map[string]map[string]any `json:"plugin_settings"`
	PluginAccess   map[string]AccessConfig   `json:"plugin_access"`
}

type AccessRule struct {
	Mode   string   `json:"mode"`
	Users  []string `json:"users,omitempty"`
	Groups []string `json:"groups,omitempty"`
}

type AccessConfig struct {
	Default  AccessRule            `json:"default"`
	Commands map[string]AccessRule `json:"commands,omitempty"`
}

func InitConfig() AppConfig {
	file, err := os.ReadFile("./config.json")
	if err != nil {
		fmt.Println("请正确配置config.json")
		os.Exit(1)
	}

	var appConfig AppConfig
	err = json.Unmarshal(file, &appConfig)
	if err != nil {
		fmt.Println("请正确配置config.json")
		os.Exit(1)
	}
	if appConfig.Database == "" {
		appConfig.Database = "bot.db"
	}
	return appConfig
}

func SavePluginSettings(pluginID string, settings map[string]any) error {
	configLock.Lock()
	defer configLock.Unlock()

	file, err := os.ReadFile("./config.json")
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(file, &raw); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	var pluginSettings map[string]map[string]any
	if encoded, ok := raw["plugin_settings"]; ok {
		if err := json.Unmarshal(encoded, &pluginSettings); err != nil {
			return fmt.Errorf("decode plugin settings: %w", err)
		}
	}
	if pluginSettings == nil {
		pluginSettings = make(map[string]map[string]any)
	}
	pluginSettings[pluginID] = settings
	encoded, err := json.Marshal(pluginSettings)
	if err != nil {
		return fmt.Errorf("encode plugin settings: %w", err)
	}
	raw["plugin_settings"] = encoded

	updated, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	updated = append(updated, '\n')
	if err := os.WriteFile("./config.json", updated, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func SavePluginAccess(pluginID string, access AccessConfig) error {
	configLock.Lock()
	defer configLock.Unlock()

	file, err := os.ReadFile("./config.json")
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(file, &raw); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}

	pluginAccess := make(map[string]AccessConfig)
	if encoded, ok := raw["plugin_access"]; ok {
		if err := json.Unmarshal(encoded, &pluginAccess); err != nil {
			return fmt.Errorf("decode plugin access: %w", err)
		}
	}
	pluginAccess[pluginID] = access
	encoded, err := json.Marshal(pluginAccess)
	if err != nil {
		return fmt.Errorf("encode plugin access: %w", err)
	}
	raw["plugin_access"] = encoded

	updated, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	updated = append(updated, '\n')
	if err := os.WriteFile("./config.json", updated, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
