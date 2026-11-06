package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xiaoxuxiansheng/goredis/app"
	"github.com/xiaoxuxiansheng/goredis/demo"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "演示服务启动失败：", err)
		os.Exit(1)
	}
}

func run() error {
	httpAddress := flag.String("http", "127.0.0.1:8080", "演示页面监听地址")
	flag.Parse()
	conf := app.SetUpConfig()
	tcp, snapshots, err := app.ConstructDemoServer()
	if err != nil {
		return err
	}
	redisHost := conf.Bind
	if redisHost == "" || redisHost == "0.0.0.0" || redisHost == "::" {
		redisHost = "127.0.0.1"
	}
	redisAddress := net.JoinHostPort(redisHost, fmt.Sprint(conf.Port))
	tcpDone := make(chan error, 1)
	go func() { tcpDone <- tcp.Serve(conf.Address()) }()
	// The browser is served only after the real TCP server has become available.
	ready := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		select {
		case err := <-tcpDone:
			return err
		default:
		}
		conn, err := net.DialTimeout("tcp", redisAddress, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			ready = true
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if !ready {
		tcp.Stop()
		return fmt.Errorf("等待 goredis TCP 服务超时")
	}
	web := &http.Server{
		Addr: *httpAddress,
		Handler: demo.New(demo.Options{
			RedisAddress: redisAddress,
			LogPath:      "app.log",
			AppendOnly:   conf.AppendOnly(),
			AOFPath:      conf.AppendFileName(),
			AppendFsync:  conf.AppendFsync(),
			RewriteAfter: conf.AutoAofRewriteAfterCmd(),
		}, snapshots),
		ReadHeaderTimeout: 5 * time.Second,
	}
	webDone := make(chan error, 1)
	go func() { webDone <- web.ListenAndServe() }()
	fmt.Printf("\n  goredis 演示面板\n  浏览器打开 http://%s\n  数据库地址 %s\n  按 Ctrl+C 退出\n\n", *httpAddress, redisAddress)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	select {
	case err = <-webDone:
		tcp.Stop()
		<-tcpDone
	case err = <-tcpDone:
	case <-ctx.Done():
		tcp.Stop()
		err = <-tcpDone
	}
	shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	_ = web.Shutdown(shutdown)
	return err
}
