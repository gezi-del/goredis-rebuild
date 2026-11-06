package handler

import (
	"strconv"
	"strings"
)

// reply.go 的核心工作是把Go中的结果转换成Redis客户端能识别的RESP字节

// 第一步：先定义 Reply 的共同标准
//       ↓
// 第二步：实现最简单的 OK
//       ↓
// 第三步：实现普通字符串
//       ↓
// 第四步：实现整数
//       ↓
// 第五步：实现错误
//       ↓
// 第六步：实现 Bulk String
//       ↓
// 第七步：实现数组 MultiBulk

// 第一步
// CRLF 是 redis 统一的行分隔符协议
const CRLF = "\r\n"

// 第二步
// 表示 Redis 固定成功响应 +OK\r\n
type OKReply struct{} // 因为OKReply永远只会返回一种结果，不需要内容
// 准备好提前准备发送的字节
var okBytes = []byte("+OK\r\n")

// 这个函数把准备好的OK确认发出去
func (o *OKReply) ToBytes() []byte {
	return okBytes
}

// 始终返回同一个结构体，每次reply ok不需要继续开辟内存空间，复用之前的ok即可
var theOkReply = new(OKReply)

func NewOKReply() *OKReply {
	return theOkReply
}

// 第三步
// 表示 RESP 的简单字符串类型： +字符串\r\n
type SimpleStringReply struct {
	Str string
}

// 构造函数
func NewSimpleStringReply(str string) *SimpleStringReply {
	return &SimpleStringReply{
		Str: str,
	}
}

// 拼接成 RESP 形式
func (s *SimpleStringReply) ToBytes() []byte {
	return []byte("+" + s.Str + CRLF)
}

// 第四步
type IntReply struct {
	Code int64
}

// 构造函数
func NewIntReply(code int64) *IntReply {
	return &IntReply{
		Code: code,
	}
}

// 拼接成 RESP形式
func (i *IntReply) ToBytes() []byte {
	return []byte(":" + strconv.FormatInt(i.Code, 10) + CRLF)
}

// 第五步
// 语法错误：客户端命令格式不符合要求时，返回一个固定的语法报错
type SyntaxErrReply struct{}

var (
	syntaxErrBytes    = []byte("-Err syntax error\r\n")
	theSyntaxErrReply = &SyntaxErrReply{}
)

func NewSyntaxErrReply() *SyntaxErrReply {
	return theSyntaxErrReply
}

func (s *SyntaxErrReply) ToBytes() []byte {
	return syntaxErrBytes
}

// Go 中内置了Error接口，实现这个函数使其成为一个go error
func (s *SyntaxErrReply) Error() string {
	return "Err syntax error"
}

// 类型错误：比如在string变量上执行list的操作
type WrongTypeErrReply struct{}

var (
	wrongTypeErrBytes    = []byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n")
	theWrongTypeErrReply = &WrongTypeErrReply{}
)

func NewWrongTypeErrReply() *WrongTypeErrReply {
	return theWrongTypeErrReply
}

func (w *WrongTypeErrReply) ToBytes() []byte {
	return wrongTypeErrBytes
}

func (w *WrongTypeErrReply) Error() string {
	return "WRONGTYPE Operation against a key holding the wrong kind of value"
}

// 设计通用err reply
type ErrReply struct {
	ErrStr string
}

func NewErrReply(errStr string) *ErrReply {
	return &ErrReply{
		ErrStr: errStr,
	}
}

func (e *ErrReply) ToBytes() []byte {
	return []byte("-" + e.ErrStr + CRLF)
}

// NilReply：表示Redis中没有这个值
// 比如指令：Get abc，但abc这个值不存在，返回一个提示
type NilReply struct{}

var (
	nilReply     = &NilReply{}
	nilBulkBytes = []byte("$-1\r\n")
)

func NewNilReply() *NilReply {
	return nilReply
}

func (n *NilReply) ToBytes() []byte {
	return nilBulkBytes
}

// BulkReply:返回一个带长度的二进制安全字符串
// 格式： $长度\r\n
//
//	内容\r\n
type BulkReply struct {
	Arg []byte
}

func NewBulkReply(arg []byte) *BulkReply {
	return &BulkReply{
		Arg: arg,
	}
}

func (b *BulkReply) ToBytes() []byte {
	if b.Arg == nil {
		return nilBulkBytes
	}

	return []byte(
		"$" + strconv.Itoa(len(b.Arg)) + CRLF +
			string(b.Arg) + CRLF,
	)
}

// MultiBulkReply
type MultiBulkReply struct {
	args [][]byte
}

func NewMultiBulkReply(args [][]byte) *MultiBulkReply {
	return &MultiBulkReply{
		args: args,
	}
}

// Args() 是为了让外部能够拿到内部数组
func (m *MultiBulkReply) Args() [][]byte {
	return m.args
}

// 转成RESP格式
func (m *MultiBulkReply) ToBytes() []byte {
	// 创建这个用于高效拼接字符串
	var strBuf strings.Builder
	strBuf.WriteString(
		"*" + strconv.Itoa(len(m.args)) + CRLF,
	)

	for _, arg := range m.args {
		if arg == nil {
			strBuf.WriteString(string(nilBulkBytes))
			continue
		}
		strBuf.WriteString(
			"$" + strconv.Itoa(len(arg)) + CRLF +
				string(arg) + CRLF,
		)
	}
	return []byte(strBuf.String())
}

// EmptyMultiBulkReply: 表示空数组
type EmptyMutiBulkReply struct{}

var emptyMultiBulkBytes = []byte("*0\r\n")

func NewEmptyMultiBulkReply() *EmptyMutiBulkReply {
	return &EmptyMutiBulkReply{}
}

func (e *EmptyMutiBulkReply) ToBytes() []byte {
	return emptyMultiBulkBytes
}
