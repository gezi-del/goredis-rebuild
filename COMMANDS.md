# 支持的命令

共 20 条命令，命令名支持大小写。

| 类型 | 命令格式 | 用途 |
| --- | --- | --- |
| String | `SET key value [NX] [EX seconds]` | 设置字符串；NX 表示仅 key 不存在时写入，EX 设置过期秒数 |
| String | `GET key` | 获取字符串 |
| String | `MSET key value [key value ...]` | 批量设置字符串 |
| String | `MGET key [key ...]` | 批量获取字符串 |
| List | `LPUSH key value [value ...]` | 从列表左侧插入元素 |
| List | `RPUSH key value [value ...]` | 从列表右侧插入元素 |
| List | `LPOP key [count]` | 从左侧弹出元素，默认一个 |
| List | `RPOP key [count]` | 从右侧弹出元素，默认一个 |
| List | `LRANGE key start stop` | 获取下标闭区间内的元素，`0 -1` 表示整个列表 |
| Set | `SADD key member [member ...]` | 添加集合成员，自动去重 |
| Set | `SISMEMBER key member` | 判断成员是否存在，返回 1 或 0 |
| Set | `SREM key member [member ...]` | 删除集合成员 |
| Hash | `HSET key field value [field value ...]` | 设置一个或多个字段 |
| Hash | `HGET key field` | 获取字段值 |
| Hash | `HDEL key field [field ...]` | 删除字段 |
| Sorted Set | `ZADD key score member [score member ...]` | 添加成员或更新分数，分数为整数 |
| Sorted Set | `ZRANGEBYSCORE key min max` | 按分数升序获取闭区间内的成员，上界 -1 表示无上限 |
| Sorted Set | `ZREM key member [member ...]` | 删除有序集合成员 |
| 过期 | `EXPIRE key seconds` | 设置相对过期时间，单位为秒 |
| 过期 | `EXPIREAT key "YYYY-MM-DD HH:MM:SS"` | 设置绝对过期时间，使用本地时区的日期时间字符串 |
