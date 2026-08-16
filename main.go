package main

import (
	"Plrx/lib/admin"
	"Plrx/lib/config"
	"Plrx/lib/constant"
	"Plrx/lib/middleware"
	"Plrx/lib/plugin"
	"Plrx/lib/qqapi"
	"Plrx/lib/requests"
	"Plrx/lib/schedule"
	"Plrx/lib/storage"
	"Plrx/lib/structers"
	"Plrx/lib/templates"
	_ "Plrx/plugins"
	plugins_push "Plrx/plugins/push"
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var requestsClient *requests.Client = requests.Init(5)

func main() {
	// 初始化相关配置
	appConfig := config.InitConfig()
	if err := plugin.LoadConfigurations(appConfig.PluginSettings); err != nil {
		log.Fatalf("Failed to load plugin configuration: %v", err)
	}
	accessConfigs := make(map[string]plugin.AccessConfig, len(appConfig.PluginAccess))
	for id, access := range appConfig.PluginAccess {
		commands := make(map[string]plugin.AccessRule, len(access.Commands))
		for path, rule := range access.Commands {
			commands[path] = plugin.AccessRule{Mode: rule.Mode, Users: rule.Users, Groups: rule.Groups}
		}
		accessConfigs[id] = plugin.AccessConfig{
			Default:  plugin.AccessRule{Mode: access.Default.Mode, Users: access.Default.Users, Groups: access.Default.Groups},
			Commands: commands,
		}
	}
	if err := plugin.LoadAccessConfigurations(accessConfigs); err != nil {
		log.Fatalf("Failed to load plugin access configuration: %v", err)
	}
	if err := storage.Open(appConfig.Database); err != nil {
		log.Fatalf("Failed to initialize SQLite storage: %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			log.Printf("Failed to close SQLite storage: %v", err)
		}
	}()
	client := qqapi.Init(appConfig.AppId, appConfig.AppSecret, appConfig.ProxyAPI, requestsClient)
	plugins_push.SetClient(&client)
	schedule.Start(&client)
	r := gin.Default()
	admin.Register(r, appConfig.AdminPassword)

	// 主动推送接口 (不经过QQ签名校验)
	r.POST("/push/:scope/:openid", plugins_push.HTTPHandle)

	// 仅 webhook 需要QQ签名校验
	webhook := r.Group("/")
	webhook.Use(middleware.VerifySignature(appConfig.AppSecret))
	webhook.POST("/webhook", func(c *gin.Context) {
		c.Status(http.StatusOK)
		// 中间件已提取
		var payload structers.Payload
		if err := c.ShouldBindJSON(&payload); err != nil {
			return
		}

		// Op = 13, 签名验证
		if payload.Op == 13 {
			// log.Printf("[Webhook] 收到平台网络探测/验证请求")

			// 再次利用相同的 seed 计算私钥用于回包签名
			seed := appConfig.AppSecret
			for len(seed) < ed25519.SeedSize {
				seed = strings.Repeat(seed, 2)
			}
			reader := strings.NewReader(seed[:ed25519.SeedSize])
			_, privateKey, _ := ed25519.GenerateKey(reader)

			var msg bytes.Buffer
			msg.WriteString(payload.Data.EventTs)
			msg.WriteString(payload.Data.PlainToken)

			signature := hex.EncodeToString(ed25519.Sign(privateKey, msg.Bytes()))
			c.JSON(http.StatusOK, gin.H{
				"plain_token": payload.Data.PlainToken,
				"signature":   signature,
			})
			return
		}

		if !constant.IsValidEventType(payload.T) {
			return
		}
		payload.EventType = constant.EventType(payload.T)
		go middleware.ProcessPayload(payload, &client)
	})

	log.Printf("Server running on %v", appConfig.Port)
	log.Printf("注册了%v个Markdown模板", templates.GetMarkdownTemplateCount())
	log.Printf("注册了%v个指令", plugin.GetCommandCount())
	log.Printf("注册了%v个定时任务", schedule.GetJobCount())
	r.Run(fmt.Sprintf(":%v", appConfig.Port))
}
