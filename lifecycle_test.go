package main

import (
	"Plrx/lib/constant"
	appcontext "Plrx/lib/context"
	"Plrx/lib/middleware"
	"Plrx/lib/plugin"
	"Plrx/lib/qqapi"
	"Plrx/lib/structers"
	"errors"
	"testing"
	"time"
)

func TestPermissionDeniedLifecycle(t *testing.T) {
	called := make(chan struct{}, 1)
	plugin.Register(&plugin.Plugin{
		Id: "test-permission-lifecycle",
		Commands: []*plugin.Command{{
			Prefix: "/permission-lifecycle",
			Role:   constant.RoleOwner,
			PermissionDenied: func(_ *appcontext.MessageContext) error {
				called <- struct{}{}
				return nil
			},
		}},
	})
	payload := structers.Payload{EventType: constant.GROUP_MESSAGE_CREATE}
	payload.Data.Content = "/permission-lifecycle"
	payload.Data.Author.Role = constant.RoleMember
	middleware.ProcessPayload(payload, &qqapi.Client{})
	select {
	case <-called:
	default:
		t.Fatal("permission denied lifecycle was not called")
	}
}

func TestHandleErrorLifecycle(t *testing.T) {
	want := errors.New("handler failed")
	called := make(chan error, 1)
	plugin.Register(&plugin.Plugin{
		Id: "test-error-lifecycle",
		Commands: []*plugin.Command{{
			Prefix: "/error-lifecycle",
			Handle: func(_ *appcontext.MessageContext) error {
				return want
			},
			HandleError: func(_ *appcontext.MessageContext, err error) error {
				called <- err
				return nil
			},
		}},
	})
	payload := structers.Payload{EventType: constant.C2C_MESSAGE_CREATE}
	payload.Data.Content = "/error-lifecycle"
	middleware.ProcessPayload(payload, &qqapi.Client{})
	select {
	case got := <-called:
		if !errors.Is(got, want) {
			t.Fatalf("unexpected lifecycle error: %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("handle error lifecycle was not called")
	}
}
