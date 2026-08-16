package admin

import (
	"Plrx/lib/config"
	"Plrx/lib/plugin"
	"crypto/subtle"
	_ "embed"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed panel.html
var panelPage []byte

//go:embed plugin.html
var pluginPage []byte

func Register(router *gin.Engine, password string) {
	admin := router.Group("/admin")
	admin.Use(access(password))
	admin.GET("", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", panelPage)
	})
	admin.GET("/plugins/:id", func(c *gin.Context) {
		if _, ok := plugin.ManagedPluginByID(c.Param("id")); !ok {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", pluginPage)
	})
	admin.GET("/api/plugins", func(c *gin.Context) {
		c.JSON(http.StatusOK, plugin.ManagedPlugins())
	})
	admin.GET("/api/plugins/:id", func(c *gin.Context) {
		managed, ok := plugin.ManagedPluginByID(c.Param("id"))
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "插件不存在"})
			return
		}
		c.JSON(http.StatusOK, managed)
	})
	admin.PUT("/api/plugins/:id", func(c *gin.Context) {
		var input map[string]any
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式无效"})
			return
		}
		prepared, err := plugin.PrepareConfiguration(c.Param("id"), input)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := config.SavePluginSettings(c.Param("id"), prepared); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败"})
			return
		}
		if err := plugin.ApplyConfiguration(c.Param("id"), prepared); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	admin.PUT("/api/plugins/:id/access", func(c *gin.Context) {
		var input plugin.AccessConfig
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式无效"})
			return
		}
		prepared, err := plugin.PrepareAccessConfiguration(c.Param("id"), input)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		persisted := config.AccessConfig{
			Default:  toConfigAccessRule(prepared.Default),
			Commands: make(map[string]config.AccessRule, len(prepared.Commands)),
		}
		for path, rule := range prepared.Commands {
			persisted.Commands[path] = toConfigAccessRule(rule)
		}
		if err := config.SavePluginAccess(c.Param("id"), persisted); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存访问控制失败"})
			return
		}
		plugin.ApplyAccessConfiguration(c.Param("id"), prepared)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}

func toConfigAccessRule(rule plugin.AccessRule) config.AccessRule {
	return config.AccessRule{Mode: rule.Mode, Users: rule.Users, Groups: rule.Groups}
}

func access(password string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if password == "" {
			host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
			if err == nil && net.ParseIP(host).IsLoopback() {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "远程管理未启用，请在 config.json 中设置 admin_password"})
			return
		}
		username, provided, ok := c.Request.BasicAuth()
		validUser := subtle.ConstantTimeCompare([]byte(username), []byte("admin")) == 1
		validPassword := subtle.ConstantTimeCompare([]byte(provided), []byte(password)) == 1
		if !ok || !validUser || !validPassword {
			c.Header("WWW-Authenticate", `Basic realm="Bot admin", charset="UTF-8"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}
