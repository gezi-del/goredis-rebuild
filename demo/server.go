package demo

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/xiaoxuxiansheng/goredis/database"
	"github.com/xiaoxuxiansheng/goredis/handler"
)

//go:embed static/*
var assets embed.FS

type Options struct {
	RedisAddress string
	LogPath      string
	AppendOnly   bool
	AOFPath      string
	AppendFsync  string
	RewriteAfter int
}

type CommandResult struct {
	ID         uint64    `json:"id"`
	Time       time.Time `json:"time"`
	Args       []string  `json:"args"`
	OK         bool      `json:"ok"`
	Type       string    `json:"type"`
	Value      any       `json:"value"`
	Raw        string    `json:"raw"`
	DurationMS float64   `json:"durationMs"`
}

type Server struct {
	options   Options
	snapshots database.Snapshotter
	mu        sync.Mutex
	history   []CommandResult
	nextID    uint64
}

func New(options Options, snapshots database.Snapshotter) http.Handler {
	s := &Server{options: options, snapshots: snapshots, history: make([]CommandResult, 0)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/command", s.command)
	mux.HandleFunc("GET /api/storage", s.storage)
	mux.HandleFunc("GET /api/logs", s.logs)
	mux.HandleFunc("GET /api/aof", s.aof)
	mux.HandleFunc("GET /api/history", s.getHistory)
	mux.HandleFunc("DELETE /api/history", s.clearHistory)
	root, _ := fs.Sub(assets, "static")
	mux.Handle("/", http.FileServer(http.FS(root)))
	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	conn, err := net.DialTimeout("tcp", s.options.RedisAddress, time.Second)
	if err == nil {
		_ = conn.Close()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"connected": err == nil, "address": s.options.RedisAddress,
		"appendOnly": s.options.AppendOnly, "commands": 20,
	})
}

func (s *Server) command(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Args []string `json:"args"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Args) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请提供命令和参数"})
		return
	}
	started := time.Now()
	result := CommandResult{Time: started, Args: request.Args}
	result.Type, result.Value, result.Raw = s.execute(r.Context(), request.Args)
	result.OK = result.Type != "error"
	result.DurationMS = float64(time.Since(started).Microseconds()) / 1000
	s.mu.Lock()
	s.nextID++
	result.ID = s.nextID
	s.history = append(s.history, result)
	if len(s.history) > 200 {
		s.history = append([]CommandResult(nil), s.history[len(s.history)-200:]...)
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) execute(ctx context.Context, args []string) (string, any, string) {
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", s.options.RedisAddress)
	if err != nil {
		return "error", "无法连接 goredis：" + err.Error(), ""
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	command := make([][]byte, len(args))
	for i, arg := range args {
		command[i] = []byte(arg)
	}
	if _, err := conn.Write(handler.NewMultiBulkReply(command).ToBytes()); err != nil {
		return "error", err.Error(), ""
	}
	var raw strings.Builder
	typeName, value, err := readReply(bufio.NewReader(conn), &raw, 0)
	if err != nil {
		return "error", "读取响应失败：" + err.Error(), raw.String()
	}
	return typeName, value, raw.String()
}

func (s *Server) storage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	entries, err := s.snapshots.Snapshot(ctx)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "time": time.Now()})
}

func (s *Server) getHistory(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, http.StatusOK, s.history)
}

func (s *Server) clearHistory(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.history = make([]CommandResult, 0)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	file, err := os.Open(s.options.LogPath)
	if os.IsNotExist(err) {
		writeJSON(w, http.StatusOK, map[string]any{"lines": []string{}})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	const limit = 128 << 10
	truncated := info.Size() > limit
	if truncated {
		_, _ = file.Seek(-limit, io.SeekEnd)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if truncated && len(lines) > 0 {
		lines = lines[1:]
	}
	level := strings.ToUpper(r.URL.Query().Get("level"))
	filtered := make([]string, 0)
	for _, line := range lines {
		if line != "" && (level == "" || strings.Contains(line, "\t"+level+"\t")) {
			filtered = append(filtered, line)
		}
	}
	if len(filtered) > 300 {
		filtered = filtered[len(filtered)-300:]
	}
	writeJSON(w, http.StatusOK, map[string]any{"lines": filtered, "time": time.Now()})
}

// readReply decodes the server's RESP response, retaining its original wire form.
func readReply(reader *bufio.Reader, raw *strings.Builder, depth int) (string, any, error) {
	if depth > 8 {
		return "", nil, fmt.Errorf("response nesting is too deep")
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", nil, err
	}
	raw.WriteString(line)
	if len(line) < 3 || !strings.HasSuffix(line, "\r\n") {
		return "", nil, fmt.Errorf("invalid RESP header")
	}
	body := line[1 : len(line)-2]
	switch line[0] {
	case '+':
		return "string", body, nil
	case '-':
		return "error", body, nil
	case ':':
		var n int64
		_, err := fmt.Sscan(body, &n)
		return "integer", n, err
	case '$':
		var size int
		if _, err := fmt.Sscan(body, &size); err != nil || size < -1 || size > 1<<20 {
			return "", nil, fmt.Errorf("invalid bulk length")
		}
		if size == -1 {
			return "null", nil, nil
		}
		data := make([]byte, size+2)
		if _, err := io.ReadFull(reader, data); err != nil {
			return "", nil, err
		}
		raw.Write(data)
		if string(data[size:]) != "\r\n" {
			return "", nil, fmt.Errorf("invalid bulk delimiter")
		}
		return "string", string(data[:size]), nil
	case '*':
		var size int
		if _, err := fmt.Sscan(body, &size); err != nil || size < -1 || size > 10000 {
			return "", nil, fmt.Errorf("invalid array length")
		}
		if size == -1 {
			return "null", nil, nil
		}
		values := make([]any, 0, size)
		for i := 0; i < size; i++ {
			_, value, err := readReply(reader, raw, depth+1)
			if err != nil {
				return "", nil, err
			}
			values = append(values, value)
		}
		return "array", values, nil
	}
	return "", nil, fmt.Errorf("unknown RESP type")
}
