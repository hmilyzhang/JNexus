// JNexus 运维平台 — By JJ Zhang, Version 1.0

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"jnexus/internal/config"
	"jnexus/internal/handler"
	"jnexus/internal/model"
	"jnexus/internal/service"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	genkey := flag.Bool("genkey", false, "生成 32 字节 base64 主密钥后退出")
	flag.Parse()

	if *genkey {
		fmt.Println(model.GenAESKey())
		return
	}

	if err := config.Load(*configPath); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	if _, err := config.AESKeyBytes(); err != nil {
		log.Fatalf("AES 主密钥配置错误: %v", err)
	}

	if err := model.Connect(config.Cfg.Database.DSN); err != nil {
		log.Fatalf("%v", err)
	}
	if err := model.Seed(); err != nil {
		log.Fatalf("初始化种子数据失败: %v", err)
	}
	if err := model.SeedConfig(); err != nil {
		log.Fatalf("初始化系统配置失败: %v", err)
	}
	if err := service.LoadDangerRules(); err != nil {
		log.Fatalf("加载危险命令规则失败: %v", err)
	}
	service.StartScheduler() // 计划任务调度循环
	service.StartMonitorLoop() // 监控调度循环（应用监控 + 主机资源采集）

	if err := os.MkdirAll(config.Cfg.Storage.UploadDir, 0755); err != nil {
		log.Fatalf("创建上传目录失败: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	if config.Cfg.Server.Mode == "debug" {
		gin.SetMode(gin.DebugMode)
	}

	r := handler.SetupRouter()
	addr := fmt.Sprintf(":%d", config.Cfg.Server.Port)
	log.Printf("JNexus 服务已启动: http://0.0.0.0%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
