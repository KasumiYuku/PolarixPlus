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

// processTemplate 规范化占位符并提取参数名; {{#each}} 段内的占位符跳过收集。
func processTemplate(input string) (string, []string) {
	re := regexp.MustCompile(`\{\{(.*?)\}\}`)
	var args []string
	seen := make(map[string]bool)
	inEach := false
	result := re.ReplaceAllStringFunc(input, func(match string) string {
		trimmed := strings.TrimSpace(match[2 : len(match)-2])
		switch {
		case strings.HasPrefix(trimmed, "#each"):
			inEach = true
			return "{{" + trimmed + "}}"
		case trimmed == "/each":
			inEach = false
			return "{{/each}}"
		}
		if trimmed != "" && !seen[trimmed] && !inEach {
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

// processEach 展开 {{#each key}}...{{/each}} 段。
// key 对应 Args 中的 []any 数组, 段内 {{field}} 指向当前项字段; 不支持嵌套。
// 缺失/类型不符报错, 显式传空数组则整段输出为空。
func processEach(template string, arg Args, flat map[string]string) (string, error) {
	if !strings.Contains(template, "{{#each") {
		return template, nil
	}
	var out strings.Builder
	rest := template
	for {
		start := strings.Index(rest, "{{#each")
		if start < 0 {
			out.WriteString(rest)
			break
		}
		headEnd := strings.Index(rest[start:], "}}")
		if headEnd < 0 {
			return "", fmt.Errorf("invalid each tag: %s", rest[start:])
		}
		head := rest[start : start+headEnd+2]
		key := strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(head, "}}"), "{{#each"))
		if key == "" {
			return "", fmt.Errorf("invalid each tag: missing key")
		}
		bodyStart := start + len(head)
		tailIdx := strings.Index(rest[bodyStart:], "{{/each}}")
		if tailIdx < 0 {
			return "", fmt.Errorf("each %s: missing {{/each}}", key)
		}
		body := rest[bodyStart : bodyStart+tailIdx]
		if strings.Contains(body, "{{#each") {
			return "", fmt.Errorf("each %s: nested each not supported", key)
		}
		arr, ok := arg[key].([]any)
		if !ok {
			return "", fmt.Errorf("each %s: arg must be []any, got %T", key, arg[key])
		}
		out.WriteString(rest[:start])
		for i, item := range arr {
			if _, ok := item.(map[string]any); !ok {
				return "", fmt.Errorf("each %s: item %d must be map[string]any, got %T", key, i, item)
			}
			seg := body
			prefix := key + ".#" + strconv.Itoa(i) + "."
			for fk, fv := range flat {
				if strings.HasPrefix(fk, prefix) {
					seg = strings.ReplaceAll(seg, "{{"+fk[len(prefix):]+"}}", fv)
				}
			}
			out.WriteString(seg)
		}
		rest = rest[bodyStart+tailIdx+len("{{/each}}"):]
	}
	return out.String(), nil
}

// FillMarkdownTemplate 填充模板参数并校验是否仍有未填充项。
func FillMarkdownTemplate(Id string, arg Args) (string, error) {
	flat, err := ToMapString(arg)
	if err != nil {
		return "", err
	}
	for _, v := range MarkdownTemplates {
		if v.Id == Id {
			template := v.Template
			template, err = processEach(template, arg, flat)
			if err != nil {
				return "", err
			}
			for key, value := range flat {
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
