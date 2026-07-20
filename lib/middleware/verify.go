package middleware

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func VerifySignature(botSecret string) gin.HandlerFunc {
	// 提前计算好公钥，避免每次请求重复计算
	seed := botSecret
	for len(seed) < ed25519.SeedSize {
		seed = strings.Repeat(seed, 2)
	}
	rand := strings.NewReader(seed[:ed25519.SeedSize])
	publicKey, _, err := ed25519.GenerateKey(rand)
	if err != nil {
		log.Fatalf("初始化公钥失败: %v", err)
	}

	return func(c *gin.Context) {
		// 主动推送接口不走QQ签名校验
		if strings.HasPrefix(c.Request.URL.Path, "/push/") {
			c.Next()
			return
		}

		// 获取 Header 参数
		signature := c.GetHeader("X-Signature-Ed25519")
		timestamp := c.GetHeader("X-Signature-Timestamp")

		if signature == "" || timestamp == "" {
			log.Println("[签名校验失败] 缺少签名字段")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// 解码签名
		sig, err := hex.DecodeString(signature)
		if err != nil || len(sig) != ed25519.SignatureSize || sig[63]&224 != 0 {
			log.Println("[签名校验失败] 签名格式不合法")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// 读取Body并重写
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		// log.Printf("[Debug]Raw request: %v", string(bodyBytes))
		// 拼接签名体
		var msg bytes.Buffer
		msg.WriteString(timestamp)
		msg.Write(bodyBytes)

		// 校验签名
		if !ed25519.Verify(publicKey, msg.Bytes(), sig) {
			log.Println("[签名校验失败] 签名验证不通过，可能遭遇伪造请求")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// 校验通过，继续后面的路由逻辑
		c.Next()
	}
}
