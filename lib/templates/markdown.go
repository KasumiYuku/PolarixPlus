package templates

import (
	"Plrx/lib/images"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Markdown struct {
	Content string `json:"content"`
}

// 实现omitzero
func (m Markdown) IsZero() bool {
	return m.Content == ""
}

type MarkdownTemplate struct {
	Id       string
	Template string
	args     []string
}

type Args map[string]any

var markdownTemplateCount uint

func ToMapString(h Args) (map[string]string, error) {
	result := make(map[string]string, len(h))
	for k, v := range h {
		switch val := v.(type) {
		case string:
			result[k] = val
		case int, int64, float64:
			result[k] = fmt.Sprintf("%v", val)
		default:
			return nil, fmt.Errorf("key %s has unsupported type: %T", k, v)
		}
	}
	return result, nil
}

var MarkdownTemplates []*MarkdownTemplate

func processTemplate(input string) (string, []string) {
	// 匹配 {{ 任意内容 }}，使用非贪婪匹配
	re := regexp.MustCompile(`\{\{(.*?)\}\}`)

	var args []string
	seen := make(map[string]bool) // 用于判断参数是否重复

	// 动态替换并收集参数
	result := re.ReplaceAllStringFunc(input, func(match string) string {
		// 提取出参数并去掉首尾空格
		content := match[2 : len(match)-2]
		trimmed := strings.TrimSpace(content)

		// 如果参数不为空且之前没收集过，则加入 args 列表
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			args = append(args, trimmed)
		}

		// 返回替换后的标准格式
		return "{{" + trimmed + "}}"
	})

	return result, args
}

func NewMarkdownTemplate(Id string, Template string) {
	template, args := processTemplate(Template)
	MarkdownTemplates = append(MarkdownTemplates, &MarkdownTemplate{
		Id:       Id,
		Template: template,
		args:     args,
	})
}

func IsMarkdownTemplateExit(Id string) bool {
	for _, v := range MarkdownTemplates {
		if v.Id == Id {
			return true
		}
	}
	return false
}

// 适配QQ的图片显示
func ProcessMarkdownImages(input string) string {
	re := regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)

	return re.ReplaceAllStringFunc(input, func(match string) string {
		submatch := re.FindStringSubmatch(match)
		alt := submatch[1]
		url := submatch[2]

		// 获取图片尺寸
		width, height, err := images.GetImageDimensions(url)
		if err != nil {
			fmt.Printf("获取图片失败 [%s]: %v\n", url, err)
			return match // 失败时保持原样
		}

		// 格式化输出
		return fmt.Sprintf("![%s #%dpx #%dpx](%s)", alt, width, height, url)
	})
}

func FillMarkdownTemplate(Id string, arg Args) (string, error) {
	args, err := ToMapString(arg)
	if err != nil {
		return "", err
	}
	for _, v := range MarkdownTemplates {
		if v.Id == Id {
			template := v.Template
			for key, value := range args {
				template = strings.ReplaceAll(template, fmt.Sprintf("{{%v}}", key), value)
			}
			_, afterDo := processTemplate(template)
			if len(afterDo) > 0 {
				var lostArgs strings.Builder
				lostArgs.WriteString(afterDo[0])
				for k, i := range afterDo {
					if k == 0 {
						continue
					}
					fmt.Fprintf(&lostArgs, ", %v", i)
				}
				return "", fmt.Errorf("Lost args: %v", lostArgs.String())
			} else {
				return template, nil
			}
		}
	}
	return "", fmt.Errorf("Template %v not found", Id)
}

func init() {
	markdownTemplateCount = 0
	// Markdown模板
	root := "templates/markdown"
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		// 处理遍历过程中的错误
		if err != nil {
			panic(err)
		}

		// 忽略目录，只处理文件
		if d.IsDir() {
			return nil
		}

		// 检查文件后缀是否为 .md
		if filepath.Ext(path) == ".md" {
			fileName := strings.TrimSuffix(filepath.Base(path), ".md")
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			NewMarkdownTemplate(fileName, string(content))
			markdownTemplateCount++
		}

		return nil
	})

	if err != nil {
		panic(err)
	}
}

func GetMarkdownTemplateCount() uint {
	return markdownTemplateCount
}
