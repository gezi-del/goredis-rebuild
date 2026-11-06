package pool

// 这个package是负责创建协程池的，以应对极高并发的访问
// 这个并发池会限制goroutine的数量，防止因为访问量过高而无法管理或是占用资源超出范围
// Redis 服务器突然同时来了三个client A、B、C
// goroutine可以让三个客户端并发（注意不是并行）执行
import (
	"runtime/debug"
	"strings"

	"github.com/panjf2000/ants"
	"github.com/xiaoxuxiansheng/goredis/log"
)

// 定义全局协程池,一个协程池的地址对象
var pool *ants.Pool // 此时仅仅声明，pool==nil

// 然后写init函数，这个包被调用会自动执行
func init() {
	_pool, err := ants.NewPool(
		50000,
		ants.WithPanicHandler(
			func(i interface{}) {
				// 将所有”\n“换成“”，-1表示全部替换，为了让异常堆栈不会因为太多行不方便检索
				stackInfo := strings.Replace(string(debug.Stack()), "\n", "", -1) // debug.Stack()会得到当前goroutine的调用栈
				log.GetDefaultLogger().Errorf(
					"recover info: %v, stack info: %s",
					i,
					stackInfo,
				)
			},
		),
	) // 创建一个容量为50000的 goroutine pool
	if err != nil { // 如果有报错就输出报错
		panic(err) // 停止程序并报错
	}

	pool = _pool
}

// 在整个package中只有Submit函数是大写开头，也就是外部可以访问的。
// 这么做会使得Submit函数成为一个对外的接口，这就是一种封装。
// 外界只能作这个函数的访问，例如只能调用pool.Submit()
func Submit(task func()) {
	pool.Submit(task)
}
