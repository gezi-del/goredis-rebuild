package lib

// time.go 可以理解为一个“统一时间包”，后续redis过期时间
// 持久化恢复等地方都会涉及时间，因此设计一个封装好的时间包
import "time"

// 定义一个时间的输出格式
const (
	YYYY_MM_DD_HH_MM_SS = "2006-01-02 15:04:05"
)

func TimeNow() time.Time { // 虽然只是套层外壳，但是为了后面统一用lib调用，又封装了一层
	return time.Now()
}

func TimeSecondFormat(t time.Time) string {
	return t.Format(YYYY_MM_DD_HH_MM_SS)
}

// time.Time = 年 + 月 + 日 + 时 + 分 + 秒 + 纳秒 + 时区 等信息
func ParseTimeSecondFormat(timeStr string) (time.Time, error) {
	return time.ParseInLocation( // 按照规定格式解析字符串，并按照本地时区解释这个时间
		YYYY_MM_DD_HH_MM_SS,
		timeStr,
		time.Local,
	)
}
