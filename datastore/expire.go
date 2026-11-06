package datastore

import (
	"time"

	"github.com/xiaoxuxiansheng/goredis/lib"
)

// 找出当前所有已过期的key，批量回收
func (k *KVStore) GC() {
	nowUnix := lib.TimeNow().Unix()
	for _, expiredKey := range k.expireTimeWheel.Range(0, nowUnix) {
		k.ExpirePreprocess(expiredKey)
	}
}

// 为某个redis key登记过期时间，并加入到过期索引中
func (k *KVStore) expire(key string, expiredAt time.Time) {
	// 先检查key是否存在
	if _, ok := k.data[key]; !ok {
		return
	}
	k.expiredAt[key] = expiredAt
	k.expireTimeWheel.Add(expiredAt.Unix(), key)
}

// 访问key前检查过期
// 惰性过期检查lazy expiration
func (k *KVStore) ExpirePreprocess(key string) {
	expiredAt, ok := k.expiredAt[key]
	if !ok {
		return
	}
	if expiredAt.After(lib.TimeNow()) {
		return
	}

	k.expireProcess(key)
}

// 删除过期key
func (k *KVStore) expireProcess(key string) {
	delete(k.data, key)
	delete(k.expiredAt, key)
	k.expireTimeWheel.Rem(key)
}
