package datastore

// AOF Rewrite的重要基础
// 遍历 KVStore 中的数据，取出能够恢复这些数据的命令表示，并把它们交给指定的函数处理
import (
	"time"

	"github.com/xiaoxuxiansheng/goredis/database"
	"github.com/xiaoxuxiansheng/goredis/lib"
)

func (k *KVStore) ForEach(f func(key string, adapter database.CmdAdapter, expireAt *time.Time)) {
	for key, data := range k.data {
		expiredAt, ok := k.expiredAt[key]
		if ok && expiredAt.Before(lib.TimeNow()) {
			continue
		}
		_adapter, _ := data.(database.CmdAdapter)
		if ok {
			f(key, _adapter, &expiredAt)
		} else {
			f(key, _adapter, nil)
		}
	}
}
