package textcomponent

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// component 是 Java 1.21.4 的 JSON 文本组件。只输出转换需要的字段，
// 未设置的样式用 omitempty 省略。
type component struct {
	Text          string      `json:"text"`
	Color         string      `json:"color,omitempty"`
	Bold          bool        `json:"bold,omitempty"`
	Italic        bool        `json:"italic,omitempty"`
	Underlined    bool        `json:"underlined,omitempty"`
	Strikethrough bool        `json:"strikethrough,omitempty"`
	Obfuscated    bool        `json:"obfuscated,omitempty"`
	Extra         []component `json:"extra,omitempty"`
}

var colors = [16]string{
	"black", "dark_blue", "dark_green", "dark_aqua",
	"dark_red", "dark_purple", "gold", "gray",
	"dark_gray", "blue", "green", "aqua",
	"red", "light_purple", "yellow", "white",
}

type style struct {
	color         string
	bold          bool
	italic        bool
	underlined    bool
	strikethrough bool
	obfuscated    bool
}

// Styled 把 Bedrock 的 § 格式码转成 Java JSON 文本组件。
//
// 颜色码会重置粗体等格式，和原版的样式规则一致；§r 重置全部样式。
// 未知代码按原样保留，不吞掉后面的字符。换行保留在文本节点里。
func Styled(text string) string {
	var runs []component
	var current style
	var buffer strings.Builder
	flush := func() {
		if buffer.Len() == 0 {
			return
		}
		runs = append(runs, current.apply(buffer.String()))
		buffer.Reset()
	}
	for i := 0; i < len(text); {
		// § 在 UTF-8 里占两个字节，必须按 rune 读取，不能按字节比较。
		mark, size := utf8.DecodeRuneInString(text[i:])
		if mark == '§' && i+size < len(text) {
			code, codeSize := utf8.DecodeRuneInString(text[i+size:])
			if code < utf8.RuneSelf && known(byte(code)) {
				// 先结算上一段，否则它会带上新样式。
				flush()
				apply(&current, byte(code))
				i += size + codeSize
				continue
			}
		}
		buffer.WriteString(text[i : i+size])
		i += size
	}
	flush()

	switch len(runs) {
	case 0:
		return marshal(component{})
	case 1:
		return marshal(runs[0])
	default:
		return marshal(component{Extra: runs})
	}
}

// known 报告一个字符是否是支持的格式码。
func known(code byte) bool {
	switch {
	case code >= '0' && code <= '9', code >= 'a' && code <= 'f':
		return true
	case code >= 'k' && code <= 'o', code == 'r':
		return true
	}
	return false
}

// apply 处理一个格式码。
func apply(current *style, code byte) {
	switch {
	case code >= '0' && code <= '9':
		*current = style{color: colors[code-'0']}
	case code >= 'a' && code <= 'f':
		*current = style{color: colors[code-'a'+10]}
	case code == 'k':
		current.obfuscated = true
	case code == 'l':
		current.bold = true
	case code == 'm':
		current.strikethrough = true
	case code == 'n':
		current.underlined = true
	case code == 'o':
		current.italic = true
	case code == 'r':
		*current = style{}
	}
}

func (s style) apply(text string) component {
	value := component{Text: text}
	if s.color != "" {
		value.Color = s.color
	}
	value.Bold = s.bold
	value.Italic = s.italic
	value.Underlined = s.underlined
	value.Strikethrough = s.strikethrough
	value.Obfuscated = s.obfuscated
	return value
}

func marshal(value component) string {
	// 数据只含字符串和自行构造的树，JSON 编码不会失败。
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
