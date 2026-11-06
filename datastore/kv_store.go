package datastore

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/xiaoxuxiansheng/goredis/database"
	"github.com/xiaoxuxiansheng/goredis/handler"
	"github.com/xiaoxuxiansheng/goredis/lib"
)

type KVStore struct {
	data            map[string]interface{} // 存储所有redis key和对应的数据实体
	expiredAt       map[string]time.Time   // 记录每个key的过期时间
	expireTimeWheel SortedSet              // 管理需要过期的key
	persister       handler.Persister      // 将命令持久化
}

func NewKVStore(persister handler.Persister) database.DataStore {
	return &KVStore{
		data:            make(map[string]interface{}),
		expiredAt:       make(map[string]time.Time),
		expireTimeWheel: newSkiplist("expireTimeWheel"),
		persister:       persister,
	}
}

// expire
func (k *KVStore) Expire(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	ttl, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return handler.NewSyntaxErrReply()
	}
	if ttl <= 0 {
		return handler.NewErrReply("ERR invalid expire time")
	}

	expireAt := lib.TimeNow().Add(time.Duration(ttl) * time.Second)
	_cmd := [][]byte{
		[]byte(database.CmdTypeExpireAt),
		[]byte(key),
		[]byte(lib.TimeSecondFormat(expireAt)),
	}
	return k.expireAt(cmd.Ctx(), _cmd, key, expireAt)
}

// 直接设置绝对过期时间
func (k *KVStore) ExpireAt(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	expiredAt, err := lib.ParseTimeSecondFormat(string(args[1]))
	if err != nil {
		return handler.NewSyntaxErrReply()
	}

	if expiredAt.Before(lib.TimeNow()) {
		if handler.IsLoadingPattern(cmd.Ctx()) {
			k.expireProcess(key)
			return handler.NewOKReply()
		}
		return handler.NewErrReply("ERR invalid expire time")
	}

	return k.expireAt(cmd.Ctx(), cmd.Cmd(), key, expiredAt)
}

func (k *KVStore) expireAt(ctx context.Context, cmd [][]byte, key string, expiredAt time.Time) handler.Reply {
	k.expire(key, expiredAt)
	k.persister.PersistCmd(ctx, cmd)
	return handler.NewOKReply()
}

// string 命令
// 读取一个string
func (k *KVStore) Get(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])

	v, err := k.getAsString(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if v == nil {
		return handler.NewNilReply()
	}

	return handler.NewBulkReply(v.Bytes())
}

// 一次读取多个key
func (k *KVStore) MGet(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	res := make([][]byte, 0, len(args))
	for _, arg := range args {
		v, error := k.getAsString(string(arg))
		if error != nil {
			return handler.NewErrReply(error.Error())
		}
		if v == nil {
			res = append(res, []byte("(nil)"))
			continue
		}
		res = append(res, v.Bytes())
	}
	return handler.NewMultiBulkReply(res)
}

// 写入一个key
func (k *KVStore) Set(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	value := string(args[1])

	// NX EX
	// NX: 只有key不存在才写入
	// EX xx: 表示设置xx秒的过期时间
	var (
		insertStrategy bool // 是否采用NX
		ttlStrategy    bool // 是否采用EX
		ttlSeconds     int64
		ttlIndex       = -1
	)

	for i := 2; i < len(args); i++ {
		flag := strings.ToLower(string(args[i]))
		switch flag {
		case "nx":
			insertStrategy = true
		case "ex":
			// 防止重复
			if ttlStrategy {
				return handler.NewSyntaxErrReply()
			}
			if i == len(args)-1 {
				return handler.NewSyntaxErrReply()
			}
			ttl, err := strconv.ParseInt(string(args[i+1]), 10, 64)
			if err != nil {
				return handler.NewSyntaxErrReply()
			}
			if ttl <= 0 {
				return handler.NewErrReply("ERR invalid expire time")
			}

			ttlStrategy = true
			ttlSeconds = ttl
			ttlIndex = i
			i++
		default:
			return handler.NewSyntaxErrReply()
		}
	}

	// 将 args 剔除 ex 部分，进行持久化
	if ttlIndex != -1 {
		args = append(args[:ttlIndex], args[ttlIndex+2:]...)
	}

	// 设置
	affected := k.put(key, value, insertStrategy)
	if affected > 0 {
		k.persister.PersistCmd(cmd.Ctx(), append([][]byte{[]byte(database.CmdTypeSet)}, args...))
	}
	if affected > 0 && ttlStrategy {
		expireAt := lib.TimeNow().Add(time.Duration(ttlSeconds) * time.Second)
		_cmd := [][]byte{[]byte(database.CmdTypeExpireAt), []byte(key), []byte(lib.TimeSecondFormat(expireAt))}
		_ = k.expireAt(cmd.Ctx(), _cmd, key, expireAt) // 其中会完成 ex 信息持久化
	}

	// 过期时间处理
	if affected > 0 {
		return handler.NewIntReply(affected)
	}
	return handler.NewNilReply()
}

func (k *KVStore) MSet(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	if len(args)&1 == 1 { // 按位与，，如果是奇数说明key value不一一对应
		return handler.NewSyntaxErrReply()
	}
	for i := 0; i < len(args); i += 2 {
		_ = k.put(string(args[i]), string(args[i+1]), false)
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd())
	return handler.NewIntReply(int64(len(args) >> 1)) // 右移一位，相当于除2
}

// List
func (k *KVStore) LPush(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	list, err := k.getAsList(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if list == nil {
		list = newListEntity(key)
		k.putAsList(key, list)
	}
	for i := 1; i < len(args); i++ {
		list.LPush(args[i])
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd())
	return handler.NewIntReply(list.Len())
}

func (k *KVStore) LPop(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	var cnt int64
	if len(args) > 1 {
		rawCnt, err := strconv.ParseInt(string(args[1]), 10, 64)
		if err != nil {
			return handler.NewSyntaxErrReply()
		}
		if rawCnt < 1 {
			return handler.NewSyntaxErrReply()
		}
		cnt = rawCnt
	}

	list, err := k.getAsList(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}
	if list == nil {
		return handler.NewNilReply()
	}
	if cnt == 0 {
		cnt = 1
	}
	poped := list.LPop(cnt)
	if poped == nil {
		return handler.NewNilReply()
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd())

	if len(poped) == 1 {
		return handler.NewBulkReply(poped[0])
	}

	return handler.NewMultiBulkReply(poped)
}

func (k *KVStore) RPush(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	list, err := k.getAsList(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}

	if list == nil {
		list = newListEntity(key)
		k.putAsList(key, list)
	}

	for i := 1; i < len(args); i++ {
		list.RPush(args[i])
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd())
	return handler.NewIntReply(list.Len())
}

func (k *KVStore) RPop(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	var cnt int64

	if len(args) > 1 {
		rawCnt, err := strconv.ParseInt(string(args[1]), 10, 64)
		if err != nil {
			return handler.NewSyntaxErrReply()
		}
		if rawCnt < 1 {
			return handler.NewSyntaxErrReply()
		}
		cnt = rawCnt
	}

	list, err := k.getAsList(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}
	if list == nil {
		return handler.NewNilReply()
	}

	if cnt == 0 {
		cnt = 1
	}

	poped := list.RPop(cnt)
	if poped == nil {
		return handler.NewNilReply()
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd())
	if len(poped) == 1 {
		return handler.NewBulkReply(poped[0])
	}

	return handler.NewMultiBulkReply(poped)
}

func (k *KVStore) LRange(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	if len(args) != 3 {
		return handler.NewSyntaxErrReply()
	}

	key := string(args[0])
	start, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return handler.NewSyntaxErrReply()
	}

	stop, err := strconv.ParseInt(string(args[2]), 10, 64)
	if err != nil {
		return handler.NewSyntaxErrReply()
	}

	list, err := k.getAsList(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}
	if list == nil {
		return handler.NewNilReply()
	}

	if got := list.Range(start, stop); got != nil {
		return handler.NewMultiBulkReply(got)
	}

	return handler.NewNilReply()
}

// set
func (k *KVStore) SAdd(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	set, err := k.getAsSet(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}
	if set == nil {
		set = newSetEntity(key)
		k.putAsSet(key, set)
	}

	var added int64 // 统计成功加入几个元素
	for _, arg := range args[1:] {
		added += set.Add(string(arg)) // add返回成功加入否（成功1）
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd())
	return handler.NewIntReply(added)
}

func (k *KVStore) SIsMember(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	if len(args) != 2 {
		return handler.NewSyntaxErrReply()
	}

	key := string(args[0])
	set, err := k.getAsSet(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}
	if set == nil {
		return handler.NewIntReply(0)
	}

	return handler.NewIntReply(set.Exist(string(args[1])))
}

func (k *KVStore) SRem(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	set, err := k.getAsSet(key)

	if err != nil {
		return handler.NewErrReply(err.Error())
	}
	if set == nil {
		return handler.NewIntReply(0)
	}

	var remed int64
	for _, arg := range args[1:] {
		remed += set.Rem(string(arg))
	}
	if remed > 0 {
		k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd())
	}
	return handler.NewIntReply(remed)
}

// hash
func (k *KVStore) HSet(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	if len(args)&1 != 1 {
		return handler.NewSyntaxErrReply()
	}
	key := string(args[0])
	hmap, err := k.getAsHashMap(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}
	if hmap == nil {
		hmap = newHashMapEntity(key)
		k.putAsHashMap(key, hmap)
	}

	for i := 0; i < len(args)-1; i += 2 {
		hkey := string(args[i+1])
		hval := args[i+2]
		hmap.Put(hkey, hval)
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd())
	return handler.NewIntReply(int64((len(args) - 1) >> 1))
}

func (k *KVStore) HGet(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	hmap, err := k.getAsHashMap(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}
	if hmap == nil {
		return handler.NewNilReply()
	}

	if v := hmap.Get(string(args[1])); v != nil {
		return handler.NewBulkReply(v)
	}

	return handler.NewNilReply()
}

func (k *KVStore) HDel(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	hmap, err := k.getAsHashMap(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}
	if hmap == nil {
		return handler.NewIntReply(0)
	}

	var remed int64
	for _, arg := range args[1:] {
		remed += hmap.Del(string(arg))
	}

	if remed > 0 {
		k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd())
	}

	return handler.NewIntReply(remed)
}

// sorted set
func (k *KVStore) ZAdd(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	if len(args)&1 != 1 {
		return handler.NewSyntaxErrReply()
	}

	key := string(args[0])
	var (
		scores  = make([]int64, 0, (len(args)-1)>>1)
		members = make([]string, 0, (len(args)-1)>>1)
	)

	for i := 0; i < len(args)-1; i += 2 {
		score, err := strconv.ParseInt(string(args[i+1]), 10, 64)
		if err != nil {
			return handler.NewSyntaxErrReply()
		}
		scores = append(scores, score)
		members = append(members, string(args[i+2]))
	}

	zset, err := k.getAsSortedSet(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}
	if zset == nil {
		zset = newSkiplist(key)
		k.putAsSortedSet(key, zset)
	}

	for i := 0; i < len(scores); i++ {
		zset.Add(scores[i], members[i])
	}

	k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd())
	return handler.NewIntReply(int64(len(scores)))
}

func (k *KVStore) ZRangeByScore(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	if len(args) < 3 {
		return handler.NewSyntaxErrReply()
	}

	key := string(args[0])
	score1, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		return handler.NewSyntaxErrReply()
	}
	score2, err := strconv.ParseInt(string(args[2]), 10, 64)
	if err != nil {
		return handler.NewSyntaxErrReply()
	}

	zset, err := k.getAsSortedSet(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}
	if zset == nil {
		return handler.NewNilReply()
	}

	rawRes := zset.Range(score1, score2)
	if len(rawRes) == 0 {
		return handler.NewNilReply()
	}

	res := make([][]byte, 0, len(rawRes))
	for _, item := range rawRes {
		res = append(res, []byte(item))
	}

	return handler.NewMultiBulkReply(res)
}

func (k *KVStore) ZRem(cmd *database.Command) handler.Reply {
	args := cmd.Args()
	key := string(args[0])
	zset, err := k.getAsSortedSet(key)
	if err != nil {
		return handler.NewErrReply(err.Error())
	}
	if zset == nil {
		return handler.NewIntReply(0)
	}

	var remed int64
	for _, arg := range args {
		remed += zset.Rem(string(arg))
	}

	if remed > 0 {
		k.persister.PersistCmd(cmd.Ctx(), cmd.Cmd())
	}
	return handler.NewIntReply(remed)
}
