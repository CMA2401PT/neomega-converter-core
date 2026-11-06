package structure

import (
	rawnbt "neomega-nbt"
	goencode "neomega-nbt/encoder-go-object"
)

// discardValue 消费不参与转换的字段，避免未知大型数组被默认接收器构造成 Go 对象。
type discardValue struct{}

var discarded discardValue

func (d *discardValue) GetReciver(rawnbt.HighLevelValueType) (goencode.ValueReciver, error) {
	return goencode.ValueReciver{TrivialValReciver: func(any, int64) error { return nil }, CompoundConstructor: d, ListConstructor: discardList{}, ArrayConstructor: discardArray{}}, nil
}
func (d *discardValue) OnResetCompound() error                                       { return nil }
func (d *discardValue) OnNextElement() error                                         { return nil }
func (d *discardValue) OnCompoundFinish() error                                      { return nil }
func (d *discardValue) GetTagPointer(string, rawnbt.HighLevelValueType) (any, error) { return d, nil }
func (d *discardValue) ConsumeByteArray(length int, read func([]byte) error) (bool, error) {
	var data [64 * 1024]byte
	for length > 0 {
		n := min(length, len(data))
		if err := read(data[:n]); err != nil {
			return true, err
		}
		length -= n
	}
	return true, nil
}

type discardList struct{}

func (discardList) HasFlag(rawnbt.Flag) bool                       { return false }
func (discardList) NonTypedEmptyList() error                       { return nil }
func (discardList) TypedEmptyList(rawnbt.HighLevelValueType) error { return nil }
func (discardList) GetReusableElementPointer(rawnbt.HighLevelValueType, int, bool) (any, error) {
	return &discarded, nil
}
func (discardList) OnNextElement() error { return nil }
func (discardList) OnListFinish() error  { return nil }

type discardArray struct{}

func (discardArray) HasFlag(rawnbt.Flag) bool                   { return false }
func (discardArray) EmptyArray(rawnbt.HighLevelValueType) error { return nil }
func (discardArray) GetReusableElementPointer(rawnbt.HighLevelValueType, int) (any, error) {
	return &discarded, nil
}
func (discardArray) OnNextElement() error { return nil }
func (discardArray) OnArrayFinish() error { return nil }
