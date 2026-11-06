package demo

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiaoxuxiansheng/goredis/database"
	"github.com/xiaoxuxiansheng/goredis/datastore"
	"github.com/xiaoxuxiansheng/goredis/handler"
	"github.com/xiaoxuxiansheng/goredis/log"
	"github.com/xiaoxuxiansheng/goredis/protocol"
)

type discardPersister struct{}

func (discardPersister) Reloader() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (discardPersister) PersistCmd(context.Context, [][]byte) {}
func (discardPersister) Close()                               {}

func TestDemoRealTCPCommandsStorageAndLogs(t *testing.T) {
	executor := database.NewDBExecutor(datastore.NewKVStore(discardPersister{}))
	db := database.NewDBTrigger(executor)
	logger := log.GetDefaultLogger()
	h, err := handler.NewHandler(db, discardPersister{}, protocol.NewParser(logger), logger)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.Start(); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go h.Handle(context.Background(), conn)
		}
	}()
	logPath := filepath.Join(t.TempDir(), "app.log")
	if err = os.WriteFile(logPath, []byte("2026-10-26\tINFO\tstarted\n2026-10-26\tERROR\ttest error\n"), 0600); err != nil {
		t.Fatal(err)
	}
	web := New(Options{RedisAddress: listener.Addr().String(), LogPath: logPath, AppendOnly: true}, executor.(database.Snapshotter))
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		web.ServeHTTP(r, httptest.NewRequest(method, path, strings.NewReader(body)))
		if r.Code != 200 {
			t.Fatalf("%s %s: %d %s", method, path, r.Code, r.Body.String())
		}
		return r
	}
	for _, path := range []string{"/", "/app.js", "/style.css"} {
		request("GET", path, "")
	}
	var health struct {
		Connected bool `json:"connected"`
	}
	json.Unmarshal(request("GET", "/api/health", "").Body.Bytes(), &health)
	if !health.Connected {
		t.Fatal("TCP health check failed")
	}
	var result CommandResult
	json.Unmarshal(request("POST", "/api/command", `{"args":["SET","name","Tom"]}`).Body.Bytes(), &result)
	if !result.OK || result.Type != "integer" {
		t.Fatalf("SET failed: %+v", result)
	}
	json.Unmarshal(request("POST", "/api/command", `{"args":["GET","name"]}`).Body.Bytes(), &result)
	if !result.OK || result.Value != "Tom" || result.Raw != "$3\r\nTom\r\n" {
		t.Fatalf("GET failed: %+v", result)
	}
	var storage struct {
		Entries []database.SnapshotEntry `json:"entries"`
	}
	json.Unmarshal(request("GET", "/api/storage", "").Body.Bytes(), &storage)
	if len(storage.Entries) != 1 || storage.Entries[0].Key != "name" || storage.Entries[0].Value != "Tom" {
		t.Fatalf("snapshot: %+v", storage)
	}
	var history []CommandResult
	json.Unmarshal(request("GET", "/api/history", "").Body.Bytes(), &history)
	if len(history) != 2 {
		t.Fatalf("history has %d records", len(history))
	}
	request("DELETE", "/api/history", "")
	if strings.TrimSpace(request("GET", "/api/history", "").Body.String()) != "[]" {
		t.Fatal("history was not cleared")
	}
	json.Unmarshal(request("POST", "/api/command", `{"args":["GET","name"]}`).Body.Bytes(), &result)
	if result.Value != "Tom" {
		t.Fatal("clearing history changed data")
	}
	logs := request("GET", "/api/logs?level=ERROR", "").Body.String()
	if !strings.Contains(logs, "test error") || strings.Contains(logs, "started") {
		t.Fatalf("log filter: %s", logs)
	}
	json.Unmarshal(request("POST", "/api/command", `{"args":["SET","missing-value"]}`).Body.Bytes(), &result)
	if result.OK || result.Type != "error" {
		t.Fatal("command error was not propagated")
	}
}
