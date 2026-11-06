package database

import (
	"context"
	"strings"
	"time"

	"github.com/xiaoxuxiansheng/goredis/handler"
)

// 从database文件夹中，这个文件开始真正定义一些Redis命令的执行链
// 定义handler怎么和数据库交流

type CmdType string // 给所有redis命令单独定义一个类型，虽然实际上就是string，但是方便统一管理
// 将CmdType转换成普通字符串，并统一小写
func (c CmdType) String() string {
	return strings.ToLower(string(c))
}

// 常量命令,即这个简易的Redis支持哪些命令
const (
	CmdTypeExpire   CmdType = "expire"
	CmdTypeExpireAt CmdType = "expireat"

	// string
	CmdTypeGet  CmdType = "get"
	CmdTypeSet  CmdType = "set"
	CmdTypeMGet CmdType = "mget"
	CmdTypeMSet CmdType = "mset"

	// list
	CmdTypeLPush  CmdType = "lpush"
	CmdTypeLPop   CmdType = "lpop"
	CmdTypeRPush  CmdType = "rpush"
	CmdTypeRPop   CmdType = "rpop"
	CmdTypeLRange CmdType = "lrange"

	// hash
	CmdTypeHSet CmdType = "hset"
	CmdTypeHGet CmdType = "hget"
	CmdTypeHDel CmdType = "hdel"

	// set
	CmdTypeSAdd      CmdType = "sadd"
	CmdTypeSIsMember CmdType = "sismember"
	CmdTypeSRem      CmdType = "srem"

	// sorted set
	CmdTypeZAdd          CmdType = "zadd"
	CmdTypeZRangeByScore CmdType = "zrangebyscore"
	CmdTypeZRem          CmdType = "zrem"
)

type CmdHandler func(*Command) handler.Reply

// Command是Handler和Executor之间传递的任务对象
type Command struct {
	ctx      context.Context // 当前请求上下文
	cmd      CmdType         // CmdTypeSet
	args     [][]byte        // ["name", "Tom"]
	receiver CmdReceiver     // 等待接收执行结果
}

// 构造函数
func NewCommand(cmd CmdType, args [][]byte) *Command {
	return &Command{
		cmd:  cmd,
		args: args,
	}
}

// get Command的上下文
func (c *Command) Ctx() context.Context {
	return c.ctx
}

// receive获取结果接收通道
func (c *Command) Receiver() CmdReceiver {
	return c.receiver
}

// 获取命令参数
func (c *Command) Args() [][]byte {
	return c.args
}

// 获取命令类型,并把Cmd重新拼回args,AOF需要完整命令
func (c *Command) Cmd() [][]byte {
	return append(
		[][]byte{[]byte(c.cmd.String())},
		c.args...,
	)
}

// 适配器，将数据库的数据对象转换成一条Redis命令
type CmdAdapter interface {
	ToCmd() [][]byte
}

// 接口，定义 Redis 数据存储层必须提供的功能
type DataStore interface {
	// string
	Get(*Command) handler.Reply
	MGet(*Command) handler.Reply
	Set(*Command) handler.Reply
	MSet(*Command) handler.Reply
	// list
	LPush(*Command) handler.Reply
	LPop(*Command) handler.Reply
	RPush(*Command) handler.Reply
	RPop(*Command) handler.Reply
	LRange(*Command) handler.Reply
	// set
	SAdd(*Command) handler.Reply
	SIsMember(*Command) handler.Reply
	SRem(*Command) handler.Reply
	// hash
	HSet(*Command) handler.Reply
	HGet(*Command) handler.Reply
	HDel(*Command) handler.Reply
	// sorted set
	ZAdd(*Command) handler.Reply
	ZRangeByScore(*Command) handler.Reply
	ZRem(*Command) handler.Reply
	// 遍历所有key
	ForEach(task func(key string, adapter CmdAdapter, expireAt *time.Time))
	// 操作key前检查是不是过期
	ExpirePreprocess(key string)
	// 定期清理 key
	GC()

	Expire(*Command) handler.Reply
	ExpireAt(*Command) handler.Reply
}

// 数据库命令执行接口
type Executor interface {
	Entrance() chan<- *Command     // 返回一个只能写入Command的channel，只能发送，不能查看
	ValidCommand(cmd CmdType) bool // 有无命令
	Close()                        // 关闭Executor
}

type CmdReceiver chan handler.Reply // 专门传输handler.Reply的channel类型
