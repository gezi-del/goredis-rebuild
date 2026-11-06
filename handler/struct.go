package handler

import (
	"context"
	"io"
	"strings"
)

// struct.go 可以理解为是项目骨架，他会解决Parser、Database、Handler这些
// 模块之间用什么类型和接口来交流

// 在实现 Parser 和 Database 之前必须“标准接口”定义好，就像
// 造电脑前要规定 USB接口、HDMI插口、电源接口规格一样

// 之前reply写的结构体都有这个方法，那么都是属于Reply接口
type Reply interface {
	ToBytes() []byte
}

type MultiReply interface {
	Reply
	Args() [][]byte
}

// Parser 解析一段网络数据后，会把结果或是错误传给后面的Handler
// 把返回结果解析出来，不然就返回error
// 相当于channel中的信息单位
type Droplet struct {
	Reply Reply
	Err   error
}

// 这个函数用来判断当前连接是不是已经结束了，如果读取TCP时
// client断开，会遇到io.EOF，表示数据流结束，如果异常中断：
// 会返回io.ErrUnexpectedEOF,表示本来还有数据，但连接提
// 结束了
func (d *Droplet) Terminated() bool {
	if d.Err == io.EOF || d.Err == io.ErrUnexpectedEOF {
		return true
	}
	// 表示连接已经关闭
	return d.Err != nil && strings.Contains(d.Err.Error(), "use of closed network connection")
}

// 数据库层接口
// 要求数据库至少支持 Do 和 Close 两个函数
type DB interface {
	Do(ctx context.Context, cmdLine [][]byte) Reply
	Close()
}

// 协议解析器
// 把一个字节流不断解析成 Redis RESP 对象
type Parser interface { // 表示只读channel
	ParseStream(reader io.Reader) <-chan *Droplet
}

var UnknownErrReplyBytes = []byte("-ERR unknown\r\n")
