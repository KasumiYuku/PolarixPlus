package main

import (
	"Plrx/lib/admin"
	"Plrx/lib/assets"
	_ "Plrx/lib/assets/providers"
	"Plrx/lib/config"
	"Plrx/lib/constant"
	"Plrx/lib/gateway"
	"Plrx/lib/logx"
	"Plrx/lib/middleware"
	"Plrx/lib/plugin"
	"Plrx/lib/qqapi"
	"Plrx/lib/requests"
	"Plrx/lib/schedule"
	"Plrx/lib/state"
	"Plrx/lib/storage"
	"Plrx/lib/structers"
	"Plrx/lib/templates"
	_ "Plrx/plugins"
	plugins_push "Plrx/plugins/push"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	requestsClient *requests.Client = requests.Init(5)
	logger                          = logx.New("system")
)

// runOp 运行控制指令。
type runOp int

const (
	opStop runOp = iota
	opRestart
)

func main() {
	appConfig := config.InitConfig()
	logx.Open(logx.Options{Ring: logx.DefaultRing, Level: appConfig.LogLevel})

	// 初始化相关配置
	if err := plugin.LoadConfigurations(appConfig.PluginSettings); err != nil {
		logger.Errorf("加载插件配置失败: %v", err)
		os.Exit(1)
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
		logger.Errorf("加载插件访问控制失败: %v", err)
		os.Exit(1)
	}
	if err := storage.Open(appConfig.Database); err != nil {
		logger.Errorf("初始化 SQLite 存储失败: %v", err)
		os.Exit(1)
	}
	client := qqapi.Init(appConfig.AppId, appConfig.AppSecret, appConfig.ProxyAPI, requestsClient)

	// 图床聚合器：配置独立存放于 assets.json+
	assetsManager := assets.NewManager(assets.NewClient(30))
	assetsManager.OnReload(client.SetAssets)
	if err := assetsManager.Load(); err != nil {
		logger.Warnf("读取 assets.json 失败，使用空图床配置: %v", err)
	}
	if host := assetsManager.Host(); host.Size() > 0 {
		logger.Infof("已启用图床聚合，provider 数量: %d", host.Size())
	}
	client.SetMessageOptions(appConfig.GlobalMarkdown, appConfig.MarkdownVerifyImage, appConfig.RetryWhen, appConfig.UploadThreshold)

	plugins_push.SetClient(&client)
	schedule.Start(&client)
	state.Boot(appConfig.Protocol, int(appConfig.Port))

	// 运行控制通道, 首条指令生效, 防重复触发
	opCh := make(chan runOp, 1)
	var controlOnce atomic.Bool
	queue := func(op runOp) {
		if controlOnce.CompareAndSwap(false, true) {
			opCh <- op
		}
	}

	engine := gin.New()
	engine.Use(gin.Recovery())
	var gw *gateway.Client // websocket 模式在下方赋值, 探针闭包按请求时读取
	admin.Register(engine, admin.Deps{
		Assets: assetsManager,
		Client: &client,
		Gateway: func() any {
			if gw != nil {
				return gw.Status()
			}
			return nil
		},
		Control: admin.Control{
			Restart: func() { queue(opRestart) },
			Stop:    func() { queue(opStop) },
		},
	})
	engine.POST("/push/:scope/:openid", plugins_push.HTTPHandle)

	if appConfig.Protocol == "websocket" {
		gatewayURL, err := client.GatewayURL()
		if err != nil {
			logger.Errorf("获取网关地址失败: %v", err)
			os.Exit(1)
		}
		intents := gateway.Intents(appConfig.Intents)
		gw = gateway.New(&client, gatewayURL, intents, [2]int{0, 1})
		go gw.Start()
	} else {
		// 仅 webhook 需要QQ签名校验
		webhook := engine.Group("/")
		webhook.Use(middleware.VerifySignature(appConfig.AppSecret))
		webhook.POST("/webhook", webhookHandler(&client, appConfig))
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", appConfig.Port),
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	logger.Infof("管理台: http://127.0.0.1:%d/admin", appConfig.Port)
	logger.Infof("注册了%v个Markdown模板", templates.GetMarkdownTemplateCount())
	logger.Infof("注册了%v个指令", plugin.GetCommandCount())
	logger.Infof("注册了%v个定时任务", schedule.GetJobCount())
	if gw != nil {
		logger.Infof("WebSocket 网关已启动（intents=%d）", len(appConfig.Intents))
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("HTTP 服务异常退出: %v", err)
			queue(opStop)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	op := opStop
	select {
	case <-ctx.Done():
		logger.Infof("收到退出信号")
	case op = <-opCh:
	}
	shutdown(srv, gw, op == opRestart)
}

// webhookHandler QQ 回调入口。
func webhookHandler(client *qqapi.Client, appConfig config.AppConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Status(http.StatusOK)
		var payload structers.Payload
		if err := c.ShouldBindJSON(&payload); err != nil {
			return
		}
		// Op = 13, 签名验证
		if payload.Op == 13 {
			_, privateKey := middleware.DeriveEd25519Key(appConfig.AppSecret)

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
		go middleware.ProcessPayload(payload, client)
	}
}

// shutdown 优雅退出; restart 为真时重新拉起自身。
func shutdown(srv *http.Server, gw *gateway.Client, restart bool) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warnf("HTTP 关闭超时: %v", err)
	}
	if gw != nil {
		gw.Stop()
	}
	schedule.Stop()
	if err := storage.Close(); err != nil {
		logger.Warnf("SQLite 关闭失败: %v", err)
	}
	if restart {
		spawnSelf()
	}
	logger.Infof("进程退出")
}

// spawnSelf 以相同参数重新拉起自身。外部守护管理时直接退出交由守护拉起。
func spawnSelf() {
	if os.Getenv("POLARIX_SUPERVISED") != "" {
		logger.Infof("外部守护接管重启, 直接退出")
		return
	}
	exe, err := os.Executable()
	if err != nil {
		logger.Errorf("获取自身路径失败: %v", err)
		return
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = os.Environ()
	if dir, err := os.Getwd(); err == nil {
		cmd.Dir = dir
	}
	if err := cmd.Start(); err != nil {
		logger.Errorf("自重启失败: %v", err)
		return
	}
	logger.Infof("已拉起新进程 PID=%d", cmd.Process.Pid)
}
