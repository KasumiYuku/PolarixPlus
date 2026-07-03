package config

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
}
