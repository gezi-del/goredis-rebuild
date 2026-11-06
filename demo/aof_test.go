package demo

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiaoxuxiansheng/goredis/handler"
)

func TestAOFViewReadsActualFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	first := handler.NewMultiBulkReply([][]byte{[]byte("SET"), []byte("name"), []byte("Tom")}).ToBytes()
	second := handler.NewMultiBulkReply([][]byte{[]byte("EXPIRE"), []byte("name"), []byte("60")}).ToBytes()
	if err := os.WriteFile(path, append(first, second...), 0600); err != nil {
		t.Fatal(err)
	}
	web := New(Options{AppendOnly: true, AOFPath: path, AppendFsync: "everysec", RewriteAfter: 1000}, nil)
	recorder := httptest.NewRecorder()
	web.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/aof", nil))
	if recorder.Code != 200 {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	var result aofStatus
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Enabled || !result.Exists || result.SyncPolicy != "everysec" || result.Total != 2 || len(result.Commands) != 2 {
		t.Fatalf("unexpected AOF status: %+v", result)
	}
	if strings.Join(result.Commands[1].Args, " ") != "EXPIRE name 60" || result.SizeBytes == 0 {
		t.Fatalf("unexpected AOF records: %+v", result)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("*2\r\n$3\r\nSET\r\n$4\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	recorder = httptest.NewRecorder()
	web.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/aof", nil))
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.PartialTail || result.Total != 2 {
		t.Fatalf("partial AOF tail was not reported: %+v", result)
	}
}
