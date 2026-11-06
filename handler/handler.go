package handler

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/xiaoxuxiansheng/goredis/log"
	"github.com/xiaoxuxiansheng/goredis/server"
)

// Handler 用于管理 Redis 客户端请求的对象
type Handler struct {
	sync.Once                       // 确保关闭逻辑只执行一次
	mu        sync.RWMutex          // 保护连接集合，相当于互斥锁
	conns     map[net.Conn]struct{} // 保存当前登记的客户端连接
	closed    atomic.Bool           // 标记Handler是否已经关闭，有原子性防止多个goroutine 竞争修改

	db        DB         // 调用数据库命令入口Do()
	parser    Parser     // 解析客户端发来的 RESP 数据
	persister Persister  // 负责数据持久化和恢复
	logger    log.Logger // 日志
}

func NewHandler(db DB, persister Persister, parser Parser, logger log.Logger) (server.Handler, error) {
	h := Handler{
		conns:     make(map[net.Conn]struct{}),
		persister: persister,
		logger:    logger,
		db:        db,
		parser:    parser,
	}

	return &h, nil
}

func (h *Handler) Start() error {
	// 加载持久化文件
	reloader, err := h.persister.Reloader()
	if err != nil {
		return err
	}
	defer reloader.Close()
	h.handle(SetLoadingPattern(context.Background()), NewFakeReaderWriter(reloader))
	return nil
}

// 接收一个客户端连接
func (h *Handler) Handle(ctx context.Context, conn net.Conn) {
	h.mu.Lock() // 先上锁
	// 先检查db有没有关闭
	if h.closed.Load() {
		h.mu.Unlock()
		_ = conn.Close()
		return
	}

	// 当前conn缓存起来
	h.conns[conn] = struct{}{}
	h.mu.Unlock()
	defer func() {
		_ = conn.Close()
		h.mu.Lock()
		delete(h.conns, conn)
		h.mu.Unlock()
	}()

	h.handle(ctx, conn)
}

// 持续处理客户端请求
func (h *Handler) handle(ctx context.Context, conn io.ReadWriter) {
	// 持续处理
	stream := h.parser.ParseStream(conn)
	for {
		select {
		case <-ctx.Done():
			h.logger.Warnf("[handler]handle ctx err: %s", ctx.Err().Error())
			return

		case droplet := <-stream:
			if err := h.handleDroplet(ctx, conn, droplet); err != nil {
				h.logger.Errorf("[handler]conn terminated, err: %s", droplet.Err.Error())
				return
			}
		}
	}
}

func (h *Handler) handleDroplet(ctx context.Context, conn io.ReadWriter, droplet *Droplet) error {
	// 判断连接是否终止
	if droplet.Terminated() {
		return droplet.Err
	}

	if droplet.Err != nil {
		_, _ = conn.Write(droplet.Reply.ToBytes())
		h.logger.Errorf("[handler]conn request, err: %s", droplet.Err.Error())
		return nil
	}

	if droplet.Reply == nil {
		h.logger.Errorf("[handler]conn empty request")
		return nil
	}

	multiReply, ok := droplet.Reply.(MultiReply)
	if !ok {
		h.logger.Errorf("[handler]conn invalid request: %s", droplet.Reply.ToBytes())
		return nil
	}

	if reply := h.db.Do(ctx, multiReply.Args()); reply != nil {
		_, _ = conn.Write(reply.ToBytes())
		return nil
	}

	_, _ = conn.Write(UnknownErrReplyBytes)
	return nil
}

func (h *Handler) Close() {
	// 保证只做Do内的函数一次
	h.Once.Do(func() {
		h.logger.Warnf("[handler]handler closing...")
		h.closed.Store(true) // 标记handle已关闭
		h.mu.Lock()
		for conn := range h.conns {
			if err := conn.Close(); err != nil {
				h.logger.Errorf("[handler]close conn err, local addr: %s, err: %s", conn.LocalAddr().String(), err.Error())
			}
		}
		h.conns = nil
		h.mu.Unlock()
		h.db.Close()
		h.persister.Close()
	})
}
