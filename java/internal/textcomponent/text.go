// Package textcomponent 把 Java 的 JSON 文本组件转回 Bedrock 的 § 文本。
// 它只处理转换需要的基础字段；事件、字体、参数等无法用 § 表达的字段会丢失。
package textcomponent

import (
	"encoding/json"
	"strings"
)

var colorCodes = map[string]byte{
	"black": '0', "dark_blue": '1', "dark_green": '2', "dark_aqua": '3',
	"dark_red": '4', "dark_purple": '5', "gold": '6', "gray": '7',
	"dark_gray": '8', "blue": '9', "green": 'a', "aqua": 'b',
	"red": 'c', "light_purple": 'd', "yellow": 'e', "white": 'f',
}

type style struct {
	color         string
	bold          bool
	italic        bool
	underlined    bool
	strikethrough bool
	obfuscated    bool
}

type renderer struct {
	builder strings.Builder
	current style
	started bool
}

// ToSection 把 JSON 字符串、已解码的组件或纯文本转成 § 文本。
func ToSection(value any) string {
	var r renderer
	r.node(value, style{})
	return r.builder.String()
}

func (r *renderer) node(value any, parent style) {
	switch value := value.(type) {
	case string:
		trimmed := strings.TrimSpace(value)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			var decoded any
			if json.Unmarshal([]byte(trimmed), &decoded) == nil {
				r.node(decoded, parent)
				return
			}
		}
		r.text(value, parent)
	case []any:
		for _, item := range value {
			r.node(item, parent)
		}
	case []string:
		for _, item := range value {
			r.node(item, parent)
		}
	case map[string]any:
		own := styleOf(parent, value)
		if text, ok := value["text"].(string); ok && text != "" {
			r.text(text, own)
		} else if fallback, ok := value["fallback"].(string); ok {
			r.text(fallback, own)
		}
		r.extra(value["extra"], own)
	}
}

func (r *renderer) extra(value any, parent style) {
	switch value := value.(type) {
	case []any:
		for _, item := range value {
			r.node(item, parent)
		}
	case []map[string]any:
		// nbt.Compound 是 map[string]any 的别名，这条覆盖原生 NBT 列表。
		for _, item := range value {
			r.node(item, parent)
		}
	case []string:
		for _, item := range value {
			r.node(item, parent)
		}
	}
}

func (r *renderer) text(text string, next style) {
	if text == "" {
		return
	}
	if !r.started {
		r.emit(next)
	} else if next != r.current {
		r.builder.WriteString("§r")
		r.emit(next)
	}
	r.current = next
	r.builder.WriteString(text)
}

func (r *renderer) emit(value style) {
	if code, ok := colorCodes[value.color]; ok {
		r.builder.WriteString("§")
		r.builder.WriteByte(code)
	}
	if value.obfuscated {
		r.builder.WriteString("§k")
	}
	if value.bold {
		r.builder.WriteString("§l")
	}
	if value.strikethrough {
		r.builder.WriteString("§m")
	}
	if value.underlined {
		r.builder.WriteString("§n")
	}
	if value.italic {
		r.builder.WriteString("§o")
	}
	r.started = true
}

func styleOf(parent style, value map[string]any) style {
	out := parent
	if color, ok := value["color"].(string); ok {
		if _, known := colorCodes[color]; known {
			out.color = color
		}
	}
	if field, ok := value["bold"]; ok {
		out.bold = truthy(field)
	}
	if field, ok := value["italic"]; ok {
		out.italic = truthy(field)
	}
	if field, ok := value["underlined"]; ok {
		out.underlined = truthy(field)
	}
	if field, ok := value["strikethrough"]; ok {
		out.strikethrough = truthy(field)
	}
	if field, ok := value["obfuscated"]; ok {
		out.obfuscated = truthy(field)
	}
	return out
}

func truthy(value any) bool {
	switch value := value.(type) {
	case bool:
		return value
	case int8:
		return value != 0
	case int32:
		return value != 0
	case string:
		return value == "true"
	default:
		return false
	}
}
