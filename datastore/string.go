package datastore

import (
	"github.com/xiaoxuxiansheng/goredis/database"
	"github.com/xiaoxuxiansheng/goredis/handler"
)

type String interface {
	Bytes() []byte      // 把内部保存的字符串内容取出来，转成字节
	database.CmdAdapter // 实现适配器，需要能把string转换成cmd命令
}

type stringEntity struct {
	key string
	str string
}

// 实现Bytes()适配String接口
func (s *stringEntity) Bytes() []byte {
	return []byte(s.str)
}

// 实现Adapter
func (s *stringEntity) ToCmd() [][]byte {
	return [][]byte{
		[]byte(database.CmdTypeSet),
		[]byte(s.key),
		[]byte(s.str),
	}
}
func NewString(key string, str string) String {
	return &stringEntity{key: key, str: str}
}

// 从KVStore的data中取出某个key，并确认他是string类型
func (k *KVStore) getAsString(key string) (String, error) {
	v, ok := k.data[key]
	if !ok {
		return nil, nil
	}

	str, ok := v.(String) // 类型断言，判断是不是string
	if !ok {              // 如果不是String类型，返回空值和错误
		return nil, handler.NewWrongTypeErrReply()
	}
	return str, nil
}

// 向KVStore存入一个String
func (k *KVStore) put(key, value string, insertStrategy bool) int64 {
	if _, ok := k.data[key]; ok && insertStrategy {
		// 如果key已经存在，并且当前不允许插入不存在的key,直接返回空值
		return 0
	}

	k.data[key] = NewString(key, value)
	return 1
}
