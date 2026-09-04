package assets

import (
	"fmt"
	"maps"
	"slices"
)

type providerEntry struct {
	factory ProviderFactory
	schema  []ConfigField
}

var registry = make(map[string]providerEntry)

// Register 注册图床 provider，name 全局唯一。
func Register(name string, factory ProviderFactory, schema []ConfigField) {
	if _, ok := registry[name]; ok {
		panic("assets: duplicate provider: " + name)
	}
	registry[name] = providerEntry{factory: factory, schema: schema}
}

// Names 返回所有已注册 provider 名称（按名称排序，顺序稳定）。
func Names() []string {
	return slices.Sorted(maps.Keys(registry))
}

// ProviderSchema 返回 provider 的配置字段定义，供管理面板使用。
func ProviderSchema(name string) []ConfigField {
	if e, ok := registry[name]; ok {
		return e.schema
	}
	return nil
}

// Instantiate 从配置项实例化一个 provider。
func Instantiate(name string, cl *Client, config map[string]any) (ImageProvider, error) {
	e, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return e.factory(cl, config)
}

// ConfigField 图床 provider 配置字段定义，与 plugin.ConfigField 一致。
type ConfigField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
	Required    bool     `json:"required,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Default     any      `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
}
