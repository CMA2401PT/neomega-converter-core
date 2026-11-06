package structure

import (
	"fmt"
	rawnbt "neomega-nbt"
	goencode "neomega-nbt/encoder-go-object"
	"reflect"
	"strings"
)

// streamCompound 保留调用方绑定的流式字段；普通 Go struct 接收器会先清零整个对象。
// 元数据字段仍交给现成的 Go 接收器，不重复实现 NBT 类型转换。
type streamCompound struct {
	target   any
	children map[string]any
	fields   goencode.CompoundConstructor
	known    map[string]bool
}

func (c *streamCompound) GetReciver(t rawnbt.HighLevelValueType) (goencode.ValueReciver, error) {
	if t != rawnbt.ComplexCompound {
		return goencode.ValueReciver{}, fmt.Errorf("structure: expected compound, got %v", t)
	}
	vr, err := goencode.ExpandReciver(c.target, t, goencode.EncodeOption{TagName: "nbt"})
	c.known = map[string]bool{}
	typ := reflect.TypeOf(c.target).Elem()
	for i := range typ.NumField() {
		field := typ.Field(i)
		name := strings.Split(field.Tag.Get("nbt"), ",")[0]
		if name == "" {
			name = field.Name
		}
		if name != "-" {
			c.known[name] = true
		}
	}
	c.fields = vr.CompoundConstructor
	return goencode.ValueReciver{CompoundConstructor: c}, err
}
func (c *streamCompound) OnResetCompound() error  { return nil }
func (c *streamCompound) OnNextElement() error    { return c.fields.OnNextElement() }
func (c *streamCompound) OnCompoundFinish() error { return c.fields.OnCompoundFinish() }
func (c *streamCompound) GetTagPointer(tag string, t rawnbt.HighLevelValueType) (any, error) {
	if child, ok := c.children[tag]; ok {
		return child, nil
	}
	if !c.known[tag] {
		return &discarded, nil
	}
	return c.fields.GetTagPointer(tag, t)
}
