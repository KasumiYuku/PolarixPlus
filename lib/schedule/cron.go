package schedule

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// 简易 5 段 cron: 分 时 日 月 周 (0-6, 0=周日)
// 支持: *  n  n-m  */n  n-m/s  a,b,c
type cronExpr struct {
	minute, hour, day, month, weekday map[int]struct{}
}

func parseCron(expr string) (*cronExpr, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron 需要 5 个字段 (分 时 日 月 周), 得到 %d: %q", len(fields), expr)
	}
	c := &cronExpr{}
	var err error
	if c.minute, err = parseField(fields[0], 0, 59); err != nil {
		return nil, fmt.Errorf("分: %w", err)
	}
	if c.hour, err = parseField(fields[1], 0, 23); err != nil {
		return nil, fmt.Errorf("时: %w", err)
	}
	if c.day, err = parseField(fields[2], 1, 31); err != nil {
		return nil, fmt.Errorf("日: %w", err)
	}
	if c.month, err = parseField(fields[3], 1, 12); err != nil {
		return nil, fmt.Errorf("月: %w", err)
	}
	if c.weekday, err = parseField(fields[4], 0, 6); err != nil {
		return nil, fmt.Errorf("周: %w", err)
	}
	return c, nil
}

func parseField(field string, min, max int) (map[int]struct{}, error) {
	result := make(map[int]struct{})
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("空字段段")
		}
		step := 1
		rangePart := part
		if strings.Contains(part, "/") {
			bits := strings.SplitN(part, "/", 2)
			rangePart = bits[0]
			s, err := strconv.Atoi(bits[1])
			if err != nil || s <= 0 {
				return nil, fmt.Errorf("非法步长: %q", part)
			}
			step = s
		}
		var start, end int
		if rangePart == "*" {
			start, end = min, max
		} else if strings.Contains(rangePart, "-") {
			bits := strings.SplitN(rangePart, "-", 2)
			var err error
			start, err = strconv.Atoi(bits[0])
			if err != nil {
				return nil, fmt.Errorf("非法范围: %q", part)
			}
			end, err = strconv.Atoi(bits[1])
			if err != nil {
				return nil, fmt.Errorf("非法范围: %q", part)
			}
		} else {
			n, err := strconv.Atoi(rangePart)
			if err != nil {
				return nil, fmt.Errorf("非法值: %q", part)
			}
			start, end = n, n
		}
		if start < min || end > max || start > end {
			return nil, fmt.Errorf("值越界: %q (允许 %d-%d)", part, min, max)
		}
		for v := start; v <= end; v += step {
			result[v] = struct{}{}
		}
	}
	return result, nil
}

func (c *cronExpr) match(t time.Time) bool {
	if _, ok := c.minute[t.Minute()]; !ok {
		return false
	}
	if _, ok := c.hour[t.Hour()]; !ok {
		return false
	}
	if _, ok := c.day[t.Day()]; !ok {
		return false
	}
	if _, ok := c.month[int(t.Month())]; !ok {
		return false
	}
	// time.Weekday: Sunday=0 ... Saturday=6, 与常见 cron 一致
	if _, ok := c.weekday[int(t.Weekday())]; !ok {
		return false
	}
	return true
}
