package handler

import (
	"context"
	"io"
)

var loadingPersisterPattern int
var ctxKeyLoadingPersisterPattern = &loadingPersisterPattern

// 给当前ctx加一个正在加载持久化数据的标记
func SetLoadingPattern(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyLoadingPersisterPattern, true)
}

// 返回true说明此命令首次执行，需要AOF持久化
func IsLoadingPattern(ctx context.Context) bool {
	is, _ := ctx.Value(
		ctxKeyLoadingPersisterPattern,
	).(bool) // 如果存了is == true

	return is
}

// 不要直接说成是AOF的接口，我们持久化的东西可能会替换
type Persister interface {
	Reloader() (io.ReadCloser, error)             // 读
	PersistCmd(ctx context.Context, cmd [][]byte) // 写
	Close()                                       // 关
}

// 因为网络层会有Write的要求（满足接口），但AOF持久化又只需要重读后执行，不需要在AOF中重新写，所以”假写“
type fakeReadWriter struct {
	io.Reader
}

func NewFakeReaderWriter(reader io.Reader) io.ReadWriter {
	return &fakeReadWriter{
		Reader: reader,
	}
}

func (f *fakeReadWriter) Write(p []byte) (n int, err error) {
	return 0, nil
}
