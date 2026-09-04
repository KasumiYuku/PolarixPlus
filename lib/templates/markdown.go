package templates

import (
	"Plrx/lib/images"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// Args 模板参数，支持任意嵌套的 map/slice。
type Args map[string]any

var markdownTemplateCount uint

var MarkdownTemplates []*MarkdownTemplate

// ToMapString 把嵌套 Args 展开为扁平 map。
// 占位符规则：
//   - {{key}}           标量
//   - {{key.#0}}        列表第 0 项
//   - {{key.obj.prop}}  嵌套对象字段
func ToMapString(h Args) (map[string]string, error) {
	result := make(map[string]string)
	var walk func(prefix string, v any) error
	walk = func(prefix string, v any) error {
		switch val := v.(type) {
		case string:
			result[prefix] = val
		case bool:
			result[prefix] = strconv.FormatBool(val)
		case int:
			result[prefix] = strconv.Itoa(val)
		case int64:
			result[prefix] = strconv.FormatInt(val, 10)
		case float64:
			result[prefix] = strconv.FormatFloat(val, 'f', -1, 64)
		case map[string]any:
			for k, sub := range val {
				key := prefix + "." + k
				if err := walk(key, sub); err != nil {
					return err
				}
			}
		case []any:
			for i, sub := range val {
				if err := walk(prefix+".#"+strconv.Itoa(i), sub); err != nil {
					return err
				}
			}
		case nil:
		default:
			return fmt.Errorf("key %s has unsupported type: %T", prefix, v)
		}
		return nil
	}
	for k, v := range h {
		if err := walk(k, v); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// processTemplate 匹配 {{ 任意内容 }}，提取参数名并规范化为紧凑格式。
func processTemplate(input string) (string, []string) {
	re := regexp.MustCompile(`\{\{(.*?)\}\}`)
	var args []string
	seen := make(map[string]bool)
	result := re.ReplaceAllStringFunc(input, func(match string) string {
		trimmed := strings.TrimSpace(match[2 : len(match)-2])
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			args = append(args, trimmed)
		}
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

// ProcessMarkdownImages 处理 markdown 图片引用并附带尺寸。
func ProcessMarkdownImages(input string) string {
	re := regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)
	return re.ReplaceAllStringFunc(input, func(match string) string {
		submatch := re.FindStringSubmatch(match)
		alt, url := submatch[1], submatch[2]
		width, height, err := images.GetImageDimensions(url)
		if err != nil {
			return match
		}
		return fmt.Sprintf("![%s #%dpx #%dpx](%s)\n", alt, width, height, url)
	})
}

// FillMarkdownTemplate 填充模板参数并校验是否仍有未填充项。
func FillMarkdownTemplate(Id string, arg Args) (string, error) {
	args, err := ToMapString(arg)
	if err != nil {
		return "", err
	}
	for _, v := range MarkdownTemplates {
		if v.Id == Id {
			template := v.Template
			for key, value := range args {
				template = strings.ReplaceAll(template, "{{"+key+"}}", value)
			}
			_, after := processTemplate(template)
			if len(after) > 0 {
				return "", fmt.Errorf("Lost args: %s", strings.Join(after, ", "))
			}
			return template, nil
		}
	}
	return "", fmt.Errorf("Template %v not found", Id)
}

func init() {
	markdownTemplateCount = 0
	root := "templates/markdown"
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		fileName := strings.TrimSuffix(filepath.Base(path), ".md")
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		NewMarkdownTemplate(fileName, string(content))
		markdownTemplateCount++
		return nil
	})
	if err != nil {
		panic(err)
	}
}

func GetMarkdownTemplateCount() uint {
	return markdownTemplateCount
}
