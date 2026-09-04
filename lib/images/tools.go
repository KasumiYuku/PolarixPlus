package images

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// dimClient 探测尺寸专用客户端：限定超时与响应体上限，防慢速/超大响应拖垮协程。
var dimClient = &http.Client{
	Timeout: 5 * time.Second,
}

const maxProbeBytes = 1 << 20 // 探测尺寸只需头部，1MB 足够

// GetImageDimensions 请求 URL 并返回图片宽高，失败返回零值与错误。
// 用项目自研 Probe 字节嗅探（支持 PNG/JPEG/GIF/WebP/BMP），零外部依赖；
// 带浏览器 UA：部分图床/CDN 拒绝无 UA 或非浏览器 UA 的请求。
func GetImageDimensions(url string) (int, int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")

	resp, err := dimClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}

	buf, err := io.ReadAll(io.LimitReader(resp.Body, maxProbeBytes))
	if err != nil {
		return 0, 0, err
	}
	sz := Probe(buf)
	if sz == nil {
		return 0, 0, fmt.Errorf("无法识别图片格式")
	}
	return sz.Width, sz.Height, nil
}
