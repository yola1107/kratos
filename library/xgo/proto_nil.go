package xgo

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

/*
	protobuf容错处理,
	- FillNilMessage自动将所有内层结构为nil的proto字段设置为非nil零值字段
*/
// FillNilMessage 拷贝 proto.Message，并将所有 nil 字段填充为零值、空 slice、空 map、空 struct、。
func FillNilMessage(src proto.Message) proto.Message {
	if src == nil {
		return nil
	}
	dst := proto.Clone(src)
	fillZeroFields(dst.ProtoReflect())
	return dst
}

func fillZeroFields(m protoreflect.Message) {
	fields := m.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)

		if m.Has(field) {
			// 如果是 message，递归检查其内部
			if field.Kind() == protoreflect.MessageKind && !field.IsList() && !field.IsMap() {
				sub := m.Get(field).Message()
				fillZeroFields(sub)
			}
			continue
		}

		switch {
		case field.IsList():
			m.Set(field, protoreflect.ValueOfList(m.NewField(field).List()))

		case field.IsMap():
			m.Set(field, protoreflect.ValueOfMap(m.NewField(field).Map()))

		case field.Kind() == protoreflect.MessageKind:
			sub := m.NewField(field).Message()
			m.Set(field, protoreflect.ValueOfMessage(sub))
			fillZeroFields(sub)

		default:
			m.Set(field, m.NewField(field))
		}
	}
}
