// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v3.6.1
// source: metadata/metadata.proto

package metadata

import (
	_ "google.golang.org/genproto/googleapis/api/annotations"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GameCommand int32

const (
	GameCommand_Ping              GameCommand = 0    //
	GameCommand_ListServices      GameCommand = 1001 //
	GameCommand_ListServicesRsp   GameCommand = 1002 //
	GameCommand_GetServiceDesc    GameCommand = 1003 //
	GameCommand_GetServiceDescRsp GameCommand = 1004 //
	GameCommand_ChatPush          GameCommand = 1005 //push
)

// Enum value maps for GameCommand.
var (
	GameCommand_name = map[int32]string{
		0:    "Ping",
		1001: "ListServices",
		1002: "ListServicesRsp",
		1003: "GetServiceDesc",
		1004: "GetServiceDescRsp",
		1005: "ChatPush",
	}
	GameCommand_value = map[string]int32{
		"Ping":              0,
		"ListServices":      1001,
		"ListServicesRsp":   1002,
		"GetServiceDesc":    1003,
		"GetServiceDescRsp": 1004,
		"ChatPush":          1005,
	}
)

func (x GameCommand) Enum() *GameCommand {
	p := new(GameCommand)
	*p = x
	return p
}

func (x GameCommand) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (GameCommand) Descriptor() protoreflect.EnumDescriptor {
	return file_metadata_metadata_proto_enumTypes[0].Descriptor()
}

func (GameCommand) Type() protoreflect.EnumType {
	return &file_metadata_metadata_proto_enumTypes[0]
}

func (x GameCommand) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use GameCommand.Descriptor instead.
func (GameCommand) EnumDescriptor() ([]byte, []int) {
	return file_metadata_metadata_proto_rawDescGZIP(), []int{0}
}

type ListServicesRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListServicesRequest) Reset() {
	*x = ListServicesRequest{}
	mi := &file_metadata_metadata_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListServicesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListServicesRequest) ProtoMessage() {}

func (x *ListServicesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_metadata_metadata_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListServicesRequest.ProtoReflect.Descriptor instead.
func (*ListServicesRequest) Descriptor() ([]byte, []int) {
	return file_metadata_metadata_proto_rawDescGZIP(), []int{0}
}

type ListServicesReply struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Services      []string               `protobuf:"bytes,1,rep,name=services,proto3" json:"services,omitempty"`
	Methods       []string               `protobuf:"bytes,2,rep,name=methods,proto3" json:"methods,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListServicesReply) Reset() {
	*x = ListServicesReply{}
	mi := &file_metadata_metadata_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListServicesReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListServicesReply) ProtoMessage() {}

func (x *ListServicesReply) ProtoReflect() protoreflect.Message {
	mi := &file_metadata_metadata_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListServicesReply.ProtoReflect.Descriptor instead.
func (*ListServicesReply) Descriptor() ([]byte, []int) {
	return file_metadata_metadata_proto_rawDescGZIP(), []int{1}
}

func (x *ListServicesReply) GetServices() []string {
	if x != nil {
		return x.Services
	}
	return nil
}

func (x *ListServicesReply) GetMethods() []string {
	if x != nil {
		return x.Methods
	}
	return nil
}

type GetServiceDescRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetServiceDescRequest) Reset() {
	*x = GetServiceDescRequest{}
	mi := &file_metadata_metadata_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetServiceDescRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetServiceDescRequest) ProtoMessage() {}

func (x *GetServiceDescRequest) ProtoReflect() protoreflect.Message {
	mi := &file_metadata_metadata_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetServiceDescRequest.ProtoReflect.Descriptor instead.
func (*GetServiceDescRequest) Descriptor() ([]byte, []int) {
	return file_metadata_metadata_proto_rawDescGZIP(), []int{2}
}

func (x *GetServiceDescRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type GetServiceDescReply struct {
	state         protoimpl.MessageState          `protogen:"open.v1"`
	FileDescSet   *descriptorpb.FileDescriptorSet `protobuf:"bytes,1,opt,name=file_desc_set,json=fileDescSet,proto3" json:"file_desc_set,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetServiceDescReply) Reset() {
	*x = GetServiceDescReply{}
	mi := &file_metadata_metadata_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetServiceDescReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetServiceDescReply) ProtoMessage() {}

func (x *GetServiceDescReply) ProtoReflect() protoreflect.Message {
	mi := &file_metadata_metadata_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetServiceDescReply.ProtoReflect.Descriptor instead.
func (*GetServiceDescReply) Descriptor() ([]byte, []int) {
	return file_metadata_metadata_proto_rawDescGZIP(), []int{3}
}

func (x *GetServiceDescReply) GetFileDescSet() *descriptorpb.FileDescriptorSet {
	if x != nil {
		return x.FileDescSet
	}
	return nil
}

type ChatReq struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Age           int32                  `protobuf:"varint,2,opt,name=age,proto3" json:"age,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ChatReq) Reset() {
	*x = ChatReq{}
	mi := &file_metadata_metadata_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ChatReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChatReq) ProtoMessage() {}

func (x *ChatReq) ProtoReflect() protoreflect.Message {
	mi := &file_metadata_metadata_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ChatReq.ProtoReflect.Descriptor instead.
func (*ChatReq) Descriptor() ([]byte, []int) {
	return file_metadata_metadata_proto_rawDescGZIP(), []int{4}
}

func (x *ChatReq) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *ChatReq) GetAge() int32 {
	if x != nil {
		return x.Age
	}
	return 0
}

type ChatRsp struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Age           int32                  `protobuf:"varint,2,opt,name=age,proto3" json:"age,omitempty"`
	Code          int32                  `protobuf:"varint,3,opt,name=code,proto3" json:"code,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ChatRsp) Reset() {
	*x = ChatRsp{}
	mi := &file_metadata_metadata_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ChatRsp) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChatRsp) ProtoMessage() {}

func (x *ChatRsp) ProtoReflect() protoreflect.Message {
	mi := &file_metadata_metadata_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ChatRsp.ProtoReflect.Descriptor instead.
func (*ChatRsp) Descriptor() ([]byte, []int) {
	return file_metadata_metadata_proto_rawDescGZIP(), []int{5}
}

func (x *ChatRsp) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *ChatRsp) GetAge() int32 {
	if x != nil {
		return x.Age
	}
	return 0
}

func (x *ChatRsp) GetCode() int32 {
	if x != nil {
		return x.Code
	}
	return 0
}

var File_metadata_metadata_proto protoreflect.FileDescriptor

const file_metadata_metadata_proto_rawDesc = "" +
	"\n" +
	"\x17metadata/metadata.proto\x12\n" +
	"kratos.api\x1a google/protobuf/descriptor.proto\x1a\x1cgoogle/api/annotations.proto\"\x15\n" +
	"\x13ListServicesRequest\"I\n" +
	"\x11ListServicesReply\x12\x1a\n" +
	"\bservices\x18\x01 \x03(\tR\bservices\x12\x18\n" +
	"\amethods\x18\x02 \x03(\tR\amethods\"+\n" +
	"\x15GetServiceDescRequest\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\"]\n" +
	"\x13GetServiceDescReply\x12F\n" +
	"\rfile_desc_set\x18\x01 \x01(\v2\".google.protobuf.FileDescriptorSetR\vfileDescSet\"/\n" +
	"\aChatReq\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\x12\x10\n" +
	"\x03age\x18\x02 \x01(\x05R\x03age\"C\n" +
	"\aChatRsp\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\x12\x10\n" +
	"\x03age\x18\x02 \x01(\x05R\x03age\x12\x12\n" +
	"\x04code\x18\x03 \x01(\x05R\x04code*|\n" +
	"\vGameCommand\x12\b\n" +
	"\x04Ping\x10\x00\x12\x11\n" +
	"\fListServices\x10\xe9\a\x12\x14\n" +
	"\x0fListServicesRsp\x10\xea\a\x12\x13\n" +
	"\x0eGetServiceDesc\x10\xeb\a\x12\x16\n" +
	"\x11GetServiceDescRsp\x10\xec\a\x12\r\n" +
	"\bChatPush\x10\xed\a2\xdd\x01\n" +
	"\bMetadata\x12a\n" +
	"\fListServices\x12\x1f.kratos.api.ListServicesRequest\x1a\x1d.kratos.api.ListServicesReply\"\x11\x82\xd3\xe4\x93\x02\v\x12\t/services\x12n\n" +
	"\x0eGetServiceDesc\x12!.kratos.api.GetServiceDescRequest\x1a\x1f.kratos.api.GetServiceDescReply\"\x18\x82\xd3\xe4\x93\x02\x12\x12\x10/services/{name}Bb\n" +
	"\x15com.github.kratos.apiP\x01Z;github.com/yola1107/kratos/v2/api/proto/kratos/api;metadata\xa2\x02\tKratosAPIb\x06proto3"

var (
	file_metadata_metadata_proto_rawDescOnce sync.Once
	file_metadata_metadata_proto_rawDescData []byte
)

func file_metadata_metadata_proto_rawDescGZIP() []byte {
	file_metadata_metadata_proto_rawDescOnce.Do(func() {
		file_metadata_metadata_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_metadata_metadata_proto_rawDesc), len(file_metadata_metadata_proto_rawDesc)))
	})
	return file_metadata_metadata_proto_rawDescData
}

var file_metadata_metadata_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_metadata_metadata_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_metadata_metadata_proto_goTypes = []any{
	(GameCommand)(0),                       // 0: kratos.api.GameCommand
	(*ListServicesRequest)(nil),            // 1: kratos.api.ListServicesRequest
	(*ListServicesReply)(nil),              // 2: kratos.api.ListServicesReply
	(*GetServiceDescRequest)(nil),          // 3: kratos.api.GetServiceDescRequest
	(*GetServiceDescReply)(nil),            // 4: kratos.api.GetServiceDescReply
	(*ChatReq)(nil),                        // 5: kratos.api.ChatReq
	(*ChatRsp)(nil),                        // 6: kratos.api.ChatRsp
	(*descriptorpb.FileDescriptorSet)(nil), // 7: google.protobuf.FileDescriptorSet
}
var file_metadata_metadata_proto_depIdxs = []int32{
	7, // 0: kratos.api.GetServiceDescReply.file_desc_set:type_name -> google.protobuf.FileDescriptorSet
	1, // 1: kratos.api.Metadata.ListServices:input_type -> kratos.api.ListServicesRequest
	3, // 2: kratos.api.Metadata.GetServiceDesc:input_type -> kratos.api.GetServiceDescRequest
	2, // 3: kratos.api.Metadata.ListServices:output_type -> kratos.api.ListServicesReply
	4, // 4: kratos.api.Metadata.GetServiceDesc:output_type -> kratos.api.GetServiceDescReply
	3, // [3:5] is the sub-list for method output_type
	1, // [1:3] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_metadata_metadata_proto_init() }
func file_metadata_metadata_proto_init() {
	if File_metadata_metadata_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_metadata_metadata_proto_rawDesc), len(file_metadata_metadata_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_metadata_metadata_proto_goTypes,
		DependencyIndexes: file_metadata_metadata_proto_depIdxs,
		EnumInfos:         file_metadata_metadata_proto_enumTypes,
		MessageInfos:      file_metadata_metadata_proto_msgTypes,
	}.Build()
	File_metadata_metadata_proto = out.File
	file_metadata_metadata_proto_goTypes = nil
	file_metadata_metadata_proto_depIdxs = nil
}
