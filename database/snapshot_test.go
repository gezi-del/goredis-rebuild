package database_test

import (
	"context"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xiaoxuxiansheng/goredis/database"
	"github.com/xiaoxuxiansheng/goredis/datastore"
)

type discardPersister struct{}

func (discardPersister) Reloader() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (discardPersister) PersistCmd(context.Context, [][]byte) {}
func (discardPersister) Close()                               {}

func TestSnapshotCopiesAllTypesAndSkipsExpired(t *testing.T) {
	executor := database.NewDBExecutor(datastore.NewKVStore(discardPersister{}))
	db := database.NewDBTrigger(executor)
	defer db.Close()
	command := func(args ...string) {
		line := make([][]byte, len(args))
		for i, arg := range args {
			line[i] = []byte(arg)
		}
		reply := db.Do(context.Background(), line).ToBytes()
		if len(reply) == 0 || reply[0] == '-' {
			t.Fatalf("%v: %s", args, reply)
		}
	}
	command("set", "s", "hello")
	command("rpush", "l", "a", "b")
	command("sadd", "members", "b", "a")
	command("hset", "h", "name", "Tom")
	command("zadd", "z", "2", "b", "1", "a")
	command("set", "ttl", "temporary", "EX", "1")
	snapshots := executor.(database.Snapshotter)
	entries, err := snapshots.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 6 {
		t.Fatalf("expected six keys, got %d", len(entries))
	}
	byKey := make(map[string]database.SnapshotEntry)
	for _, entry := range entries {
		byKey[entry.Key] = entry
	}
	if byKey["s"].Value != "hello" || byKey["s"].Type != "string" {
		t.Fatal(byKey["s"])
	}
	if !reflect.DeepEqual(byKey["l"].Value, []string{"a", "b"}) {
		t.Fatal(byKey["l"])
	}
	if !reflect.DeepEqual(byKey["members"].Value, []string{"a", "b"}) {
		t.Fatal(byKey["members"])
	}
	if !reflect.DeepEqual(byKey["h"].Value, map[string]string{"name": "Tom"}) {
		t.Fatal(byKey["h"])
	}
	if !reflect.DeepEqual(byKey["z"].Value, []database.ScoredMember{{Member: "a", Score: 1}, {Member: "b", Score: 2}}) {
		t.Fatal(byKey["z"])
	}
	if byKey["ttl"].ExpiresAt == nil {
		t.Fatal("missing expiration")
	}
	command("hset", "h", "name", "Jerry")
	command("rpush", "l", "c")
	if byKey["h"].Value.(map[string]string)["name"] != "Tom" || len(byKey["l"].Value.([]string)) != 2 {
		t.Fatal("snapshot shares mutable storage")
	}
	time.Sleep(1100 * time.Millisecond)
	entries, err = snapshots.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Key == "ttl" {
			t.Fatal("expired key in snapshot")
		}
	}
}

func TestSnapshotConcurrentCommandsAndClose(t *testing.T) {
	executor := database.NewDBExecutor(datastore.NewKVStore(discardPersister{}))
	db := database.NewDBTrigger(executor)
	snapshots := executor.(database.Snapshotter)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			db.Do(context.Background(), [][]byte{[]byte("set"), []byte("k"), []byte("v")})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			if _, err := snapshots.Snapshot(context.Background()); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	wg.Wait()
	db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := snapshots.Snapshot(ctx); err == nil {
		t.Fatal("closed executor accepted a snapshot")
	}
}
