package main

import (
	"Plrx/lib/admin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAdminPluginPages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin.Register(router, "test-password")

	tests := []struct {
		path       string
		status     int
		bodyMarker string
	}{
		{path: "/admin", status: http.StatusOK, bodyMarker: "插件目录"},
		{path: "/admin/plugins/echo", status: http.StatusOK, bodyMarker: "返回插件目录"},
		{path: "/admin/plugins/not-registered", status: http.StatusNotFound},
	}
	for _, test := range tests {
		req := httptest.NewRequest(http.MethodGet, test.path, nil)
		req.SetBasicAuth("admin", "test-password")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != test.status {
			t.Fatalf("GET %s returned %d, want %d", test.path, response.Code, test.status)
		}
		if test.bodyMarker != "" && !strings.Contains(response.Body.String(), test.bodyMarker) {
			t.Fatalf("GET %s did not contain %q", test.path, test.bodyMarker)
		}
	}
}
