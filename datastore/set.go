package datastore

import (
	"github.com/xiaoxuxiansheng/goredis/database"
	"github.com/xiaoxuxiansheng/goredis/handler"
)

func (k *KVStore) getAsSet(key string) (Set, error) {
	v, ok := k.data[key]
	if !ok {
		return nil, nil
	}

	set, ok := v.(Set)
	if !ok {
		return nil, handler.NewWrongTypeErrReply()
	}

	return set, nil
}

func (k *KVStore) putAsSet(key string, set Set) {
	k.data[key] = set
}

type Set interface {
	Add(value string) int64   // 加元素
	Exist(value string) int64 // 判断元素是否存在
	Rem(value string) int64   // 删除元素
	database.CmdAdapter       // ToCmd()
}

type setEntity struct {
	key       string
	container map[string]struct{} // 用map的key的非重复性来实现，不用value
}

func newSetEntity(key string) Set {
	return &setEntity{
		key:       key,
		container: make(map[string]struct{}),
	}
}

func (s *setEntity) Add(value string) int64 {
	if _, ok := s.container[value]; ok {
		return 0 // 以及在Set中的key
	}

	s.container[value] = struct{}{}
	return 1
}

func (s *setEntity) Exist(value string) int64 {
	if _, ok := s.container[value]; ok {
		return 1
	}

	return 0
}

// 删除元素
func (s *setEntity) Rem(value string) int64 {
	if _, ok := s.container[value]; ok {
		delete(s.container, value)
		return 1
	}

	return 0
}

func (s *setEntity) ToCmd() [][]byte {
	args := make([][]byte, 0, 2+len(s.container))

	args = append(
		args,
		[]byte(database.CmdTypeSAdd),
		[]byte(s.key),
	)

	for k := range s.container {
		args = append(args, []byte(k))
	}

	return args
}
