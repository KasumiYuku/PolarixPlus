package main

import (
	"Plrx/lib/plugin"
	"testing"
)

func TestPrepareConfigurationPreservesPassword(t *testing.T) {
	id := "test-password-preservation"
	plugin.Register(&plugin.Plugin{
		Id: id,
		Config: []plugin.ConfigField{
			{Key: "enabled", Type: "boolean"},
			{Key: "token", Type: "password"},
		},
	})
	if err := plugin.LoadConfigurations(map[string]map[string]any{id: {"enabled": true, "token": "secret"}}); err != nil {
		t.Fatal(err)
	}

	prepared, err := plugin.PrepareConfiguration(id, map[string]any{"enabled": false, "token": ""})
	if err != nil {
		t.Fatal(err)
	}
	if prepared["token"] != "secret" {
		t.Fatalf("password was not preserved: %#v", prepared["token"])
	}
}

func TestConfiguredPluginsMasksPassword(t *testing.T) {
	id := "test-password-masking"
	plugin.Register(&plugin.Plugin{
		Id: id,
		Config: []plugin.ConfigField{
			{Key: "token", Type: "password"},
		},
	})
	if err := plugin.LoadConfigurations(map[string]map[string]any{id: {"token": "secret"}}); err != nil {
		t.Fatal(err)
	}

	for _, configured := range plugin.ConfiguredPlugins() {
		if configured.ID == id && configured.Values["token"] != true {
			t.Fatalf("password was exposed or marked unset: %#v", configured.Values["token"])
		}
	}
}

func TestPluginAccessRules(t *testing.T) {
	id := "test-access-rules"
	plugin.Register(&plugin.Plugin{
		Id: id,
		Commands: []*plugin.Command{{
			Prefix: "/access-test",
			SubCommand: []*plugin.Command{{
				Prefix: "private",
			}},
		}},
	})
	err := plugin.LoadAccessConfigurations(map[string]plugin.AccessConfig{
		id: {
			Default: plugin.AccessRule{Mode: "blacklist", Users: []string{"blocked-user"}, Groups: []string{"blocked-group"}},
			Commands: map[string]plugin.AccessRule{
				"/access-test private": {Mode: "whitelist", Users: []string{"allowed-user"}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if plugin.CanUse(id, "/access-test", "blocked-user", "") {
		t.Fatal("blacklisted user was allowed")
	}
	if plugin.CanUse(id, "/access-test", "other-user", "blocked-group") {
		t.Fatal("user in blacklisted group was allowed")
	}
	if !plugin.CanUse(id, "/access-test", "other-user", "other-group") {
		t.Fatal("unlisted user was denied by blacklist")
	}
	if !plugin.CanUse(id, "/access-test private", "allowed-user", "") {
		t.Fatal("whitelisted user was denied")
	}
	if plugin.CanUse(id, "/access-test private", "other-user", "") {
		t.Fatal("unlisted user was allowed by command whitelist")
	}
}

func TestResolveCommandPath(t *testing.T) {
	command := &plugin.Command{
		Prefix: "/root",
		SubCommand: []*plugin.Command{{
			Prefix: "child",
			SubCommand: []*plugin.Command{{
				Prefix: "leaf",
			}},
		}},
	}
	if path := plugin.ResolveCommandPath(command, "/root child leaf argument"); path != "/root child leaf" {
		t.Fatalf("unexpected command path: %s", path)
	}
}

func TestPrepareAccessRejectsUnknownCommand(t *testing.T) {
	id := "test-access-validation"
	plugin.Register(&plugin.Plugin{Id: id, Commands: []*plugin.Command{{Prefix: "/known"}}})
	_, err := plugin.PrepareAccessConfiguration(id, plugin.AccessConfig{
		Commands: map[string]plugin.AccessRule{"/unknown": {Mode: "blacklist"}},
	})
	if err == nil {
		t.Fatal("unknown command path was accepted")
	}
}
