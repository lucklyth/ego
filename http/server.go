package http

import (
	"context"
	"github.com/ebar-go/ego/component/event"
	"github.com/ebar-go/ego/config"
	"github.com/ebar-go/ego/http/handler"
	"github.com/ebar-go/ego/http/middleware"
	"github.com/gin-gonic/gin"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Server Web服务管理器
type Server struct {
	// 并发锁,可导出结构体采用私有变量，而不采用内嵌的方式
	mu sync.Mutex

	// gin的路由
	Router *gin.Engine

	// not found handler
	NotFoundHandler func(ctx *gin.Context)
}


// NewServer 实例化server
func NewServer() *Server {
	router := gin.Default()

	// use global trace middleware
	router.Use(middleware.Trace)

	return &Server{
		Router:          router,
		NotFoundHandler: handler.NotFoundHandler,
	}
}

// Start run http server
// args must be less than one,
// eg: Start()
//	   Start(8080)
func (server *Server) Start(args ...int) error {
	// use lock
	server.mu.Lock()

	// 解析port
	port := getPortFromArgs(args...)

	// 404
	server.Router.NoRoute(server.NotFoundHandler)
	server.Router.NoMethod(server.NotFoundHandler)

	completeHost := net.JoinHostPort("", strconv.Itoa(port))

	// before start
	event.Trigger(event.BeforeHttpStart, nil)

	srv := &http.Server{
		Addr:    completeHost,
		Handler: server.Router,
	}
	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen:%s\n", err)
		}

		// after start
		event.Trigger(event.AfterHttpStart, nil)
	}()

	server.shutdown(srv)

	return nil
}

// getPortFromArgs
func getPortFromArgs(args ...int) int {
	port := config.Server().Port
	if len(args) == 1 {
		port = args[0]
		config.Server().Port = port
	}
	return port
}

//
func (server *Server) shutdown(srv *http.Server) {
	// wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 10 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown Server ...")
	event.Trigger(event.BeforeHttpShutdown, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	log.Println("Server exiting")
}
