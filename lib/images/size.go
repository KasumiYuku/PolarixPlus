package images

import "encoding/binary"

// Size 图片宽高，均为零表示未探测到。
type Size struct{ Width, Height int }

// IsImage 魔数嗅探字节是否为常见图片格式（PNG/JPEG/GIF/WebP）。
func IsImage(buf []byte) bool {
	if len(buf) < 12 {
		return false
	}
	switch {
	case buf[0] == 0x89 && buf[1] == 'P' && buf[2] == 'N' && buf[3] == 'G':
		return true
	case buf[0] == 0xFF && buf[1] == 0xD8:
		return true
	case buf[0] == 'G' && buf[1] == 'I' && buf[2] == 'F' && buf[3] == '8':
		return true
	case buf[0] == 'R' && buf[1] == 'I' && buf[2] == 'F' && buf[3] == 'F' && buf[8] == 'W' && buf[9] == 'E' && buf[10] == 'B' && buf[11] == 'P':
		return true
	case buf[0] == 'B' && buf[1] == 'M':
		return true
	}
	return false
}

// probe 从字节切片头部探测图片尺寸，O(1) 内存，不依赖解码库。
// 支持 PNG / JPEG / GIF / WebP，覆盖 QQ 消息常见图片格式。
func Probe(buf []byte) *Size {
	if len(buf) < 4 {
		return nil
	}
	n := len(buf)
	_ = buf[n-1] // 边界保护

	// PNG: 8 字节签名 + IHDR 13 字节（宽高各 4 字节大端）
	if n >= 24 && buf[0] == 0x89 && buf[1] == 'P' && buf[2] == 'N' && buf[3] == 'G' {
		return &Size{
			Width:  int(binary.BigEndian.Uint32(buf[16:20])),
			Height: int(binary.BigEndian.Uint32(buf[20:24])),
		}
	}

	// GIF: 6 字节签名后第 6-9 字节小端宽高
	if n >= 10 && buf[0] == 'G' && buf[1] == 'I' && buf[2] == 'F' {
		return &Size{
			Width:  int(binary.LittleEndian.Uint16(buf[6:8])),
			Height: int(binary.LittleEndian.Uint16(buf[8:10])),
		}
	}

	// JPEG: 扫描 SOF 标记，0xFF 0xC0~0xCF 排除 DHT(0xC4) 与 JPG(0xC8)
	if buf[0] == 0xFF && buf[1] == 0xD8 {
		for off := 2; off+9 < n; {
			if buf[off] != 0xFF {
				off++
				continue
			}
			mk := buf[off+1]
			// 独立标记（无数据段）
			if mk == 0xD8 || mk == 0xD9 || mk == 0x01 || (mk >= 0xD0 && mk <= 0xD7) {
				off += 2
				continue
			}
			segLen := int(binary.BigEndian.Uint16(buf[off+2 : off+4]))
			if (mk >= 0xC0 && mk <= 0xCF) && mk != 0xC4 && mk != 0xC8 {
				return &Size{
					Height: int(binary.BigEndian.Uint16(buf[off+5 : off+7])),
					Width:  int(binary.BigEndian.Uint16(buf[off+7 : off+9])),
				}
			}
			off += 2 + segLen
		}
		return nil
	}

	// WebP: RIFF + WEBP 容器，三种变体
	if n >= 30 && buf[0] == 'R' && buf[1] == 'I' && buf[2] == 'F' && buf[3] == 'F' &&
		buf[8] == 'W' && buf[9] == 'E' && buf[10] == 'B' && buf[11] == 'P' {
		chunk := string(buf[12:16])
		switch chunk {
		case "VP8 ":
			if n >= 30 {
				return &Size{
					Width:  int(binary.LittleEndian.Uint16(buf[26:28]) & 0x3FFF),
					Height: int(binary.LittleEndian.Uint16(buf[28:30]) & 0x3FFF),
				}
			}
		case "VP8L":
			if n >= 25 {
				bits := binary.LittleEndian.Uint32(buf[21:25])
				return &Size{
					Width:  int(bits&0x3FFF) + 1,
					Height: int((bits>>14)&0x3FFF) + 1,
				}
			}
		case "VP8X":
			if n >= 30 {
				w := uint32(buf[24]) | uint32(buf[25])<<8 | uint32(buf[26])<<16
				h := uint32(buf[27]) | uint32(buf[28])<<8 | uint32(buf[29])<<16
				return &Size{
					Width:  int(w) + 1,
					Height: int(h) + 1,
				}
			}
		}
	}

	return nil
}
