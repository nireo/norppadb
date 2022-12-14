// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.7
// source: messages/messages.proto

package messages

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Action_ACTION_TYPE int32

const (
	Action_GET    Action_ACTION_TYPE = 0
	Action_NEW    Action_ACTION_TYPE = 1
	Action_DELETE Action_ACTION_TYPE = 2
)

// Enum value maps for Action_ACTION_TYPE.
var (
	Action_ACTION_TYPE_name = map[int32]string{
		0: "GET",
		1: "NEW",
		2: "DELETE",
	}
	Action_ACTION_TYPE_value = map[string]int32{
		"GET":    0,
		"NEW":    1,
		"DELETE": 2,
	}
)

func (x Action_ACTION_TYPE) Enum() *Action_ACTION_TYPE {
	p := new(Action_ACTION_TYPE)
	*p = x
	return p
}

func (x Action_ACTION_TYPE) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Action_ACTION_TYPE) Descriptor() protoreflect.EnumDescriptor {
	return file_messages_messages_proto_enumTypes[0].Descriptor()
}

func (Action_ACTION_TYPE) Type() protoreflect.EnumType {
	return &file_messages_messages_proto_enumTypes[0]
}

func (x Action_ACTION_TYPE) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Action_ACTION_TYPE.Descriptor instead.
func (Action_ACTION_TYPE) EnumDescriptor() ([]byte, []int) {
	return file_messages_messages_proto_rawDescGZIP(), []int{0, 0}
}

type Action struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key   []byte             `protobuf:"bytes,1,opt,name=key,proto3,oneof" json:"key,omitempty"`
	Value []byte             `protobuf:"bytes,2,opt,name=value,proto3,oneof" json:"value,omitempty"`
	Type  Action_ACTION_TYPE `protobuf:"varint,3,opt,name=type,proto3,enum=store.Action_ACTION_TYPE" json:"type,omitempty"`
}

func (x *Action) Reset() {
	*x = Action{}
	if protoimpl.UnsafeEnabled {
		mi := &file_messages_messages_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Action) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Action) ProtoMessage() {}

func (x *Action) ProtoReflect() protoreflect.Message {
	mi := &file_messages_messages_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Action.ProtoReflect.Descriptor instead.
func (*Action) Descriptor() ([]byte, []int) {
	return file_messages_messages_proto_rawDescGZIP(), []int{0}
}

func (x *Action) GetKey() []byte {
	if x != nil {
		return x.Key
	}
	return nil
}

func (x *Action) GetValue() []byte {
	if x != nil {
		return x.Value
	}
	return nil
}

func (x *Action) GetType() Action_ACTION_TYPE {
	if x != nil {
		return x.Type
	}
	return Action_GET
}

type KVPair struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key   []byte `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Value []byte `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *KVPair) Reset() {
	*x = KVPair{}
	if protoimpl.UnsafeEnabled {
		mi := &file_messages_messages_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *KVPair) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*KVPair) ProtoMessage() {}

func (x *KVPair) ProtoReflect() protoreflect.Message {
	mi := &file_messages_messages_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use KVPair.ProtoReflect.Descriptor instead.
func (*KVPair) Descriptor() ([]byte, []int) {
	return file_messages_messages_proto_rawDescGZIP(), []int{1}
}

func (x *KVPair) GetKey() []byte {
	if x != nil {
		return x.Key
	}
	return nil
}

func (x *KVPair) GetValue() []byte {
	if x != nil {
		return x.Value
	}
	return nil
}

type Server struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id       string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	RpcAddr  string `protobuf:"bytes,2,opt,name=rpc_addr,json=rpcAddr,proto3" json:"rpc_addr,omitempty"`
	IsLeader bool   `protobuf:"varint,3,opt,name=is_leader,json=isLeader,proto3" json:"is_leader,omitempty"`
}

func (x *Server) Reset() {
	*x = Server{}
	if protoimpl.UnsafeEnabled {
		mi := &file_messages_messages_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Server) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Server) ProtoMessage() {}

func (x *Server) ProtoReflect() protoreflect.Message {
	mi := &file_messages_messages_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Server.ProtoReflect.Descriptor instead.
func (*Server) Descriptor() ([]byte, []int) {
	return file_messages_messages_proto_rawDescGZIP(), []int{2}
}

func (x *Server) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Server) GetRpcAddr() string {
	if x != nil {
		return x.RpcAddr
	}
	return ""
}

func (x *Server) GetIsLeader() bool {
	if x != nil {
		return x.IsLeader
	}
	return false
}

var File_messages_messages_proto protoreflect.FileDescriptor

var file_messages_messages_proto_rawDesc = []byte{
	0x0a, 0x17, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61,
	0x67, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x73, 0x74, 0x6f, 0x72, 0x65,
	0x22, 0xa8, 0x01, 0x0a, 0x06, 0x41, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x15, 0x0a, 0x03, 0x6b,
	0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x48, 0x00, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x88,
	0x01, 0x01, 0x12, 0x19, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0c, 0x48, 0x01, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x88, 0x01, 0x01, 0x12, 0x2d, 0x0a,
	0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x19, 0x2e, 0x73, 0x74,
	0x6f, 0x72, 0x65, 0x2e, 0x41, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x41, 0x43, 0x54, 0x49, 0x4f,
	0x4e, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x22, 0x2b, 0x0a, 0x0b,
	0x41, 0x43, 0x54, 0x49, 0x4f, 0x4e, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x12, 0x07, 0x0a, 0x03, 0x47,
	0x45, 0x54, 0x10, 0x00, 0x12, 0x07, 0x0a, 0x03, 0x4e, 0x45, 0x57, 0x10, 0x01, 0x12, 0x0a, 0x0a,
	0x06, 0x44, 0x45, 0x4c, 0x45, 0x54, 0x45, 0x10, 0x02, 0x42, 0x06, 0x0a, 0x04, 0x5f, 0x6b, 0x65,
	0x79, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x22, 0x30, 0x0a, 0x06, 0x4b,
	0x56, 0x50, 0x61, 0x69, 0x72, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x22, 0x50, 0x0a,
	0x06, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x19, 0x0a, 0x08, 0x72, 0x70, 0x63, 0x5f, 0x61,
	0x64, 0x64, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x72, 0x70, 0x63, 0x41, 0x64,
	0x64, 0x72, 0x12, 0x1b, 0x0a, 0x09, 0x69, 0x73, 0x5f, 0x6c, 0x65, 0x61, 0x64, 0x65, 0x72, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x69, 0x73, 0x4c, 0x65, 0x61, 0x64, 0x65, 0x72, 0x42,
	0x24, 0x5a, 0x22, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6e, 0x69,
	0x72, 0x65, 0x6f, 0x2f, 0x6e, 0x6f, 0x72, 0x70, 0x70, 0x61, 0x64, 0x62, 0x2f, 0x6d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_messages_messages_proto_rawDescOnce sync.Once
	file_messages_messages_proto_rawDescData = file_messages_messages_proto_rawDesc
)

func file_messages_messages_proto_rawDescGZIP() []byte {
	file_messages_messages_proto_rawDescOnce.Do(func() {
		file_messages_messages_proto_rawDescData = protoimpl.X.CompressGZIP(file_messages_messages_proto_rawDescData)
	})
	return file_messages_messages_proto_rawDescData
}

var file_messages_messages_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_messages_messages_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_messages_messages_proto_goTypes = []interface{}{
	(Action_ACTION_TYPE)(0), // 0: store.Action.ACTION_TYPE
	(*Action)(nil),          // 1: store.Action
	(*KVPair)(nil),          // 2: store.KVPair
	(*Server)(nil),          // 3: store.Server
}
var file_messages_messages_proto_depIdxs = []int32{
	0, // 0: store.Action.type:type_name -> store.Action.ACTION_TYPE
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_messages_messages_proto_init() }
func file_messages_messages_proto_init() {
	if File_messages_messages_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_messages_messages_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Action); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_messages_messages_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*KVPair); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_messages_messages_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Server); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_messages_messages_proto_msgTypes[0].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_messages_messages_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_messages_messages_proto_goTypes,
		DependencyIndexes: file_messages_messages_proto_depIdxs,
		EnumInfos:         file_messages_messages_proto_enumTypes,
		MessageInfos:      file_messages_messages_proto_msgTypes,
	}.Build()
	File_messages_messages_proto = out.File
	file_messages_messages_proto_rawDesc = nil
	file_messages_messages_proto_goTypes = nil
	file_messages_messages_proto_depIdxs = nil
}
