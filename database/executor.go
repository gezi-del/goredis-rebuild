package database

import (
	"context"
	"fmt"
	"time"

	"github.com/xiaoxuxiansheng/goredis/handler"
	"github.com/xiaoxuxiansheng/goredis/lib/pool"
)

type DBExecutor struct {
	ctx       context.Context    // 控制执行器的生命周期
	cancel    context.CancelFunc // 通知执行器停止
	ch        chan *Command      // 接受等待执行的命令
	done      chan struct{}
	snapshots chan snapshotRequest

	cmdHandlers map[CmdType]CmdHandler // 命令类型和处理函数的对应表
	dataStore   DataStore              // 实际执行数据操作

	gcTicker *time.Ticker // 定时触发过期数据清理
}

func NewDBExecutor(dataStore DataStore) Executor {
	ctx, cancel := context.WithCancel(context.Background())
	e := DBExecutor{
		dataStore: dataStore,
		ch:        make(chan *Command),
		done:      make(chan struct{}),
		snapshots: make(chan snapshotRequest),
		ctx:       ctx,
		cancel:    cancel,
		gcTicker:  time.NewTicker(time.Minute), // 没一分钟产生一次信号
	}

	e.cmdHandlers = map[CmdType]CmdHandler{
		CmdTypeExpire:        e.dataStore.Expire,
		CmdTypeExpireAt:      e.dataStore.ExpireAt,
		CmdTypeGet:           e.dataStore.Get,
		CmdTypeSet:           e.dataStore.Set,
		CmdTypeHDel:          e.dataStore.HDel,
		CmdTypeHGet:          e.dataStore.HGet,
		CmdTypeHSet:          e.dataStore.HSet,
		CmdTypeLPop:          e.dataStore.LPop,
		CmdTypeLPush:         e.dataStore.LPush,
		CmdTypeLRange:        e.dataStore.LRange,
		CmdTypeMGet:          e.dataStore.MGet,
		CmdTypeMSet:          e.dataStore.MSet,
		CmdTypeRPop:          e.dataStore.RPop,
		CmdTypeRPush:         e.dataStore.RPush,
		CmdTypeSAdd:          e.dataStore.SAdd,
		CmdTypeSIsMember:     e.dataStore.SIsMember,
		CmdTypeSRem:          e.dataStore.SRem,
		CmdTypeZAdd:          e.dataStore.ZAdd,
		CmdTypeZRangeByScore: e.dataStore.ZRangeByScore,
		CmdTypeZRem:          e.dataStore.ZRem,
	}

	pool.Submit(e.run)
	return &e
}

// 只能单向发送 *Command
func (e *DBExecutor) Entrance() chan<- *Command {
	return e.ch
}

// 判断是否是合法命令
func (e *DBExecutor) ValidCommand(cmd CmdType) bool {
	_, valid := e.cmdHandlers[cmd]
	return valid
}

func (e *DBExecutor) Close() {
	e.cancel()
	<-e.done
}

func (e *DBExecutor) run() {
	defer close(e.done)
	defer e.gcTicker.Stop()
	for {
		select {
		case <-e.ctx.Done():
			return

		// 每隔 1 分钟批量一次过期的 key
		case <-e.gcTicker.C: // 每一分钟产生一次
			e.dataStore.GC()
		case request := <-e.snapshots:
			request.reply <- e.snapshot()

		case cmd := <-e.ch:
			cmdFunc, ok := e.cmdHandlers[cmd.cmd]
			if !ok {
				cmd.receiver <- handler.NewErrReply(fmt.Sprintf("unknown command '%s'", cmd.cmd))
				continue
			}

			e.dataStore.ExpirePreprocess(string(cmd.args[0]))
			cmd.receiver <- cmdFunc(cmd)
		}
	}
}
