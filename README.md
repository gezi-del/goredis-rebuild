# goredis-rebuild · goredis 药房

这是一个基于 [xiaoxuxiansheng/goredis](https://github.com/xiaoxuxiansheng/goredis) 的 Go Redis 学习复现项目。它重现了从 TCP 接收 RESP 命令、操作内存数据结构到 AOF 持久化的主要流程，并增加「goredis 药房」网页工作台，方便查看和演示真实的执行结果。

## 环境要求

- Go 1.26.5（见 [`go.mod`](go.mod)）；仓库已包含 `vendor/` 依赖。
- 数据库默认使用 `6379` 端口，网页默认使用 `127.0.0.1:8080`，启动前应确保端口可用。

## 安装

```bash
git clone https://github.com/gezi-del/goredis-rebuild.git
cd goredis-rebuild
```

## 配置

程序从**当前工作目录**读取 [`redis.conf`](redis.conf)，因此下列命令均应在仓库根目录执行。仓库自带配置的主要取值为：

| 配置项 | 当前值 | 用途 |
| --- | --- | --- |
| `bind` / `port` | `0.0.0.0` / `6379` | 数据库监听地址和端口 |
| `appendonly` / `appendfilename` | `yes` / `appendonly.aof` | 启用 AOF 并指定文件 |
| `appendfsync` | `everysec` | AOF 同步策略；代码还支持 `always` 和 `no` |
| `auto-aof-rewrite-after-cmds` | `1000` | 触发 AOF 重写的命令计数阈值 |

仅在本机演示时，可将 `bind` 改为 `127.0.0.1`。AOF 和日志文件也会按当前工作目录中的相对路径读写。

## 快速开始

```bash
go run ./cmd/demo
```

打开 <http://127.0.0.1:8080>，在命令选择器中执行 `SET name Tom`，再执行 `GET name`，即可看到返回值 `Tom`；页面的“存储信息”也会显示该 key。`cmd/demo` 在同一进程内启动数据库与网页，按 `Ctrl+C` 退出，无需另开数据库服务或安装前端依赖。

只启动数据库而不启动网页时，运行 `go run .`。如需改用其他网页端口，运行：

```bash
go run ./cmd/demo -http 127.0.0.1:8081
```

## 项目背景与技术栈

原项目用 Go 实现了简化版 Redis，覆盖 String、List、Set、Hash、Sorted Set 和 AOF。本项目以其源码为学习参照，复现主要处理链路，并在此基础上补充可视化与运行处理：

```text
网页工作台 → HTTP 接口 → RESP/TCP → 协议解析 → Handler
                                                ↓
                                       命令校验与分发
                                                ↓
                                     Executor → KVStore → AOF
```

后端使用 Go 标准库的 `net`、`net/http` 和 `embed`，并使用 `ants` 协程池、`dig` 依赖注入及 `zap` 日志。内存中以 `map` 管理 key：List 使用切片，Set 与 Hash 使用哈希表，Sorted Set 使用跳表及成员索引。项目还实现了惰性过期检查、定时清理，以及 AOF 追加、启动重放与重写。网页使用原生 HTML、CSS 和 JavaScript，无需前端构建工具。

## 相较于原项目的补充与修改

- **可视化 Demo**：新增 `cmd/demo` 和 `demo/`，把命令执行、存储信息、日志与 AOF 状态集中在浏览器展示；网页命令会经 RESP/TCP 交给实际数据库执行。
- **存储快照**：新增只读快照请求，让 `ForEach` 在 Executor 的执行循环中读取各类数据和过期时间，供网页使用，避免直接并发遍历底层 map。
- **命令与关闭处理**：命令名支持大小写，增加参数数量校验、请求取消与关闭状态处理。
- **AOF 生命周期处理**：关闭时等待缓冲写入完成并同步文件；重写时协调并发，检查临时文件同步、重命名和重新打开文件的结果。

这些改动服务于学习和演示，不表示本项目与官方 Redis 的命令语义和可靠性完全一致。

## 可视化 UI

「goredis 药房」使用卡通药房背景和毛玻璃卡片，提供五个主要入口：

- **命令选择器**：滚轮选择、搜索和筛选 [20 条支持的命令](COMMANDS.md)；动态填写参数，`SET` 可选择 `NX`、`EX`，按 Enter 执行。
- **执行结果**：显示返回值、错误、耗时和原始 RESP 响应。
- **存储信息**：查看 key 的类型、内容和过期时间，支持搜索、筛选与刷新。
- **命令记录与日志**：查看当前 Demo 进程内的页面命令记录，以及 `app.log` 中的运行日志。
- **AOF 视图**：读取配置的 AOF 文件，展示文件大小、同步策略和最近命令，辅助演示重启后的重放恢复。

页面功能和接口调用关系详见 [Demo 说明](demo/README.md)。

## 测试

以下测试覆盖存储快照和 Demo 接口，不使用根目录集成测试的固定 `6379` 端口：

```bash
go test ./database ./demo
```

**运行完整测试前请注意：**根目录的 `goredis_test.go` 会清理当前目录的 `appendonly.aof`，且使用固定的 `6379` 端口。请先停止正在运行的 Demo 并备份 AOF，再考虑执行 `go test ./...`。

## 原项目与许可证

原项目地址：[xiaoxuxiansheng/goredis](https://github.com/xiaoxuxiansheng/goredis)。本仓库是基于原项目的学习复现，不是其官方版本。

截至编写本文时，原项目仓库及随附的本地副本均未发现 `LICENSE` 文件或明确的许可证声明；本仓库也尚未添加许可证文件。因此这里不标注任何未经证实的许可证类型。如需超出学习和课程展示范围使用或再分发，请先向原作者确认授权。
