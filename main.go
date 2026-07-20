package main

import (
	"Plrx/lib/config"
	"Plrx/lib/constant"
	"Plrx/lib/middleware"
	"Plrx/lib/plugin"
	"Plrx/lib/qqapi"
	"Plrx/lib/requests"
	"Plrx/lib/storage"
	"Plrx/lib/structers"
	"Plrx/lib/templates"
	_ "Plrx/plugins"
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
	if err := storage.Open(appConfig.Database); err != nil {
		log.Fatalf("Failed to initialize SQLite storage: %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			log.Printf("Failed to close SQLite storage: %v", err)
		}
	}()
	client := qqapi.Init(appConfig.AppId, appConfig.AppSecret, appConfig.ProxyAPI, requestsClient)
	r := gin.Default()

	// 签名校验中间件
	r.Use(middleware.VerifySignature(appConfig.AppSecret))

	r.POST("/webhook", func(c *gin.Context) {
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
	r.Run(fmt.Sprintf(":%v", appConfig.Port))
}
