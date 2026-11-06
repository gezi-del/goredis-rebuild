package database

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/xiaoxuxiansheng/goredis/handler"
)

type DBTrigger struct {
	once     sync.Once // 保证关闭操作只执行一次
	executor Executor  // 保存真正的数据库执行器
	mu       sync.RWMutex
	closed   bool
}

func NewDBTrigger(executor Executor) handler.DB {
	return &DBTrigger{executor: executor}
}

// 接收一条redis命令，将其封装后提交给executor，等待完成，然后返回reply
func (d *DBTrigger) Do(ctx context.Context, cmdLine [][]byte) handler.Reply {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return handler.NewErrReply("ERR database is closed")
	}
	if len(cmdLine) < 2 {
		return handler.NewErrReply(fmt.Sprintf("invalid cmd line: %v", cmdLine))
	}

	cmdType := CmdType(strings.ToLower(string(cmdLine[0])))
	if !d.executor.ValidCommand(cmdType) {
		return handler.NewErrReply(fmt.Sprintf("unknown cmd '%s'", cmdLine[0]))
	}
	if !validArgs(cmdType, len(cmdLine)-1) {
		return handler.NewSyntaxErrReply()
	}

	cmd := Command{
		ctx:      ctx,
		cmd:      cmdType,
		args:     cmdLine[1:],
		receiver: make(CmdReceiver, 1),
	}

	// 投递给 executor
	select {
	case d.executor.Entrance() <- &cmd:
	case <-ctx.Done():
		return handler.NewErrReply(ctx.Err().Error())
	}
	// 监听 chan, 直到接收返回的 reply
	select {
	case reply := <-cmd.Receiver():
		return reply
	case <-ctx.Done():
		return handler.NewErrReply(ctx.Err().Error())
	}
}

func (d *DBTrigger) Close() {
	d.once.Do(func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.closed = true
		d.executor.Close()
	})
}

func validArgs(cmd CmdType, n int) bool {
	switch cmd {
	case CmdTypeGet:
		return n == 1
	case CmdTypeSet:
		return n >= 2
	case CmdTypeMGet:
		return n >= 1
	case CmdTypeMSet:
		return n >= 2 && n%2 == 0
	case CmdTypeLPush, CmdTypeRPush, CmdTypeSAdd, CmdTypeSRem, CmdTypeHDel, CmdTypeZRem:
		return n >= 2
	case CmdTypeLPop, CmdTypeRPop:
		return n == 1 || n == 2
	case CmdTypeLRange, CmdTypeZRangeByScore:
		return n == 3
	case CmdTypeSIsMember, CmdTypeHGet, CmdTypeExpire, CmdTypeExpireAt:
		return n == 2
	case CmdTypeHSet, CmdTypeZAdd:
		return n >= 3 && n%2 == 1
	}
	return false
}
