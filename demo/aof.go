package demo

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type aofRecord struct {
	Number int64    `json:"number"`
	Args   []string `json:"args"`
}

type aofStatus struct {
	Enabled      bool        `json:"enabled"`
	FileName     string      `json:"fileName"`
	Exists       bool        `json:"exists"`
	SizeBytes    int64       `json:"sizeBytes"`
	SyncPolicy   string      `json:"syncPolicy"`
	RewriteAfter int         `json:"rewriteAfter"`
	UpdatedAt    *time.Time  `json:"updatedAt"`
	Total        int64       `json:"total"`
	Commands     []aofRecord `json:"commands"`
	PartialTail  bool        `json:"partialTail"`
	ReadError    string      `json:"readError,omitempty"`
}

func (s *Server) aof(w http.ResponseWriter, r *http.Request) {
	fileName := "未配置"
	if s.options.AOFPath != "" {
		fileName = filepath.Base(s.options.AOFPath)
	}
	result := aofStatus{
		Enabled:      s.options.AppendOnly,
		FileName:     fileName,
		SyncPolicy:   s.options.AppendFsync,
		RewriteAfter: s.options.RewriteAfter,
		Commands:     []aofRecord{},
	}
	if !result.Enabled || s.options.AOFPath == "" {
		writeJSON(w, http.StatusOK, result)
		return
	}
	file, err := os.Open(s.options.AOFPath)
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusOK, result)
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
	result.Exists = true
	result.SizeBytes = info.Size()
	updated := info.ModTime()
	result.UpdatedAt = &updated
	// Stop at the file size captured above so one refresh is a consistent view
	// even when another goroutine appends a new command during the read.
	result.Commands, result.Total, result.PartialTail, err = readAOF(io.LimitReader(file, info.Size()))
	if err != nil {
		result.ReadError = err.Error()
	}
	writeJSON(w, http.StatusOK, result)
}

func readAOF(source io.Reader) ([]aofRecord, int64, bool, error) {
	reader := bufio.NewReader(source)
	recent := make([]aofRecord, 0, 40)
	var total int64
	for {
		header, err := reader.ReadString('\n')
		if err == io.EOF && header == "" {
			return recent, total, false, nil
		}
		if err != nil {
			return recent, total, true, nil
		}
		if len(header) > 64 || !strings.HasPrefix(header, "*") || !strings.HasSuffix(header, "\r\n") {
			return recent, total, false, fmt.Errorf("AOF 中存在无效的命令头")
		}
		count, err := strconv.Atoi(header[1 : len(header)-2])
		if err != nil || count < 1 || count > 10000 {
			return recent, total, false, fmt.Errorf("AOF 命令参数数量无效")
		}
		args := make([]string, 0, count)
		for i := 0; i < count; i++ {
			line, err := reader.ReadString('\n')
			if err != nil {
				return recent, total, true, nil
			}
			if len(line) > 64 || !strings.HasPrefix(line, "$") || !strings.HasSuffix(line, "\r\n") {
				return recent, total, false, fmt.Errorf("AOF 中存在无效的参数头")
			}
			size, err := strconv.Atoi(line[1 : len(line)-2])
			if err != nil || size < 0 || size > 1<<20 {
				return recent, total, false, fmt.Errorf("AOF 参数长度无效")
			}
			body := make([]byte, size+2)
			if _, err := io.ReadFull(reader, body); err != nil {
				return recent, total, true, nil
			}
			if string(body[size:]) != "\r\n" {
				return recent, total, false, fmt.Errorf("AOF 参数结束符无效")
			}
			args = append(args, string(body[:size]))
		}
		total++
		recent = append(recent, aofRecord{Number: total, Args: args})
		if len(recent) > 40 {
			recent = recent[1:]
		}
	}
}
