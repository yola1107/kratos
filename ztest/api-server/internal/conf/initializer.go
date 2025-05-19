package conf

import (
	"reflect"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/runtime/protoimpl"
)

/*
	protobuf容错处理,
	- FillNilMessage自动将所有内层结构为nil的字段设置为非nil零值字段
*/
// FillNilMessage recursively fills nil embedded messages in a proto.Message
func FillNilMessage(msg proto.Message) {
	if msg == nil {
		return
	}
	v := reflect.ValueOf(msg).Elem() // pointer to struct
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		structField := t.Field(i)

		// Skip internal protoimpl fields
		if structField.Anonymous && structField.Type == reflect.TypeOf(protoimpl.MessageState{}) {
			continue
		}

		// Only process pointer to struct (i.e., nested message)
		if field.Kind() == reflect.Ptr && field.IsNil() {
			fieldType := field.Type()

			// Check if it's a proto.Message
			if reflect.PointerTo(fieldType.Elem()).Implements(reflect.TypeOf((*proto.Message)(nil)).Elem()) ||
				fieldType.Implements(reflect.TypeOf((*proto.Message)(nil)).Elem()) {
				// create new instance
				newField := reflect.New(fieldType.Elem())
				field.Set(newField)
				FillNilMessage(newField.Interface().(proto.Message)) // recursive
			}
		}

		// Recurse into non-nil pointer fields
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			if pm, ok := field.Interface().(proto.Message); ok {
				FillNilMessage(pm)
			}
		}
	}
}
