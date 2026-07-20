package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Plugin struct {
	Id     string   `json:"id"`
	Prefix string   `json:"prefix"`
	Group  []string `json:"group"`
}

type AppConfig struct {
	Port      uint16   `json:"port"`
	AppId     string   `json:"appid"`
	AppSecret string   `json:"secret"`
	Plugins   []Plugin `json:"plugins"`
	ProxyAPI  string   `json:"proxy"`
	Uin       uint64   `json:"uin"`
	Uid       string   `json:"uid"`
	Database  string   `json:"database"`
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
