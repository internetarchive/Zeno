// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.27.1
// source: item.proto

package protobufv1

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

type ProtoItem struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Url             []byte `protobuf:"bytes,1,opt,name=url,proto3" json:"url,omitempty"`
	ParentUrl       []byte `protobuf:"bytes,2,opt,name=parentUrl,proto3" json:"parentUrl,omitempty"`
	ID              string `protobuf:"bytes,3,opt,name=ID,proto3" json:"ID,omitempty"`
	Hop             uint64 `protobuf:"varint,4,opt,name=hop,proto3" json:"hop,omitempty"`
	Hash            uint64 `protobuf:"varint,5,opt,name=hash,proto3" json:"hash,omitempty"`
	Type            string `protobuf:"bytes,6,opt,name=type,proto3" json:"type,omitempty"`
	BypassSeencheck bool   `protobuf:"varint,7,opt,name=bypass_seencheck,json=bypassSeencheck,proto3" json:"bypass_seencheck,omitempty"`
	Redirect        uint64 `protobuf:"varint,9,opt,name=redirect,proto3" json:"redirect,omitempty"`
	LocallyCrawled  uint64 `protobuf:"varint,10,opt,name=locally_crawled,json=locallyCrawled,proto3" json:"locally_crawled,omitempty"`
}

func (x *ProtoItem) Reset() {
	*x = ProtoItem{}
	if protoimpl.UnsafeEnabled {
		mi := &file_item_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ProtoItem) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProtoItem) ProtoMessage() {}

func (x *ProtoItem) ProtoReflect() protoreflect.Message {
	mi := &file_item_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProtoItem.ProtoReflect.Descriptor instead.
func (*ProtoItem) Descriptor() ([]byte, []int) {
	return file_item_proto_rawDescGZIP(), []int{0}
}

func (x *ProtoItem) GetUrl() []byte {
	if x != nil {
		return x.Url
	}
	return nil
}

func (x *ProtoItem) GetParentUrl() []byte {
	if x != nil {
		return x.ParentUrl
	}
	return nil
}

func (x *ProtoItem) GetID() string {
	if x != nil {
		return x.ID
	}
	return ""
}

func (x *ProtoItem) GetHop() uint64 {
	if x != nil {
		return x.Hop
	}
	return 0
}

func (x *ProtoItem) GetHash() uint64 {
	if x != nil {
		return x.Hash
	}
	return 0
}

func (x *ProtoItem) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *ProtoItem) GetBypassSeencheck() bool {
	if x != nil {
		return x.BypassSeencheck
	}
	return false
}

func (x *ProtoItem) GetRedirect() uint64 {
	if x != nil {
		return x.Redirect
	}
	return 0
}

func (x *ProtoItem) GetLocallyCrawled() uint64 {
	if x != nil {
		return x.LocallyCrawled
	}
	return 0
}

var File_item_proto protoreflect.FileDescriptor

var file_item_proto_rawDesc = []byte{
	0x0a, 0x0a, 0x69, 0x74, 0x65, 0x6d, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x71, 0x75,
	0x65, 0x75, 0x65, 0x22, 0xf5, 0x01, 0x0a, 0x09, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x49, 0x74, 0x65,
	0x6d, 0x12, 0x10, 0x0a, 0x03, 0x75, 0x72, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x03,
	0x75, 0x72, 0x6c, 0x12, 0x1c, 0x0a, 0x09, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x55, 0x72, 0x6c,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x09, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x55, 0x72,
	0x6c, 0x12, 0x0e, 0x0a, 0x02, 0x49, 0x44, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x49,
	0x44, 0x12, 0x10, 0x0a, 0x03, 0x68, 0x6f, 0x70, 0x18, 0x04, 0x20, 0x01, 0x28, 0x04, 0x52, 0x03,
	0x68, 0x6f, 0x70, 0x12, 0x12, 0x0a, 0x04, 0x68, 0x61, 0x73, 0x68, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x04, 0x52, 0x04, 0x68, 0x61, 0x73, 0x68, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18,
	0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x29, 0x0a, 0x10, 0x62,
	0x79, 0x70, 0x61, 0x73, 0x73, 0x5f, 0x73, 0x65, 0x65, 0x6e, 0x63, 0x68, 0x65, 0x63, 0x6b, 0x18,
	0x07, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0f, 0x62, 0x79, 0x70, 0x61, 0x73, 0x73, 0x53, 0x65, 0x65,
	0x6e, 0x63, 0x68, 0x65, 0x63, 0x6b, 0x12, 0x1a, 0x0a, 0x08, 0x72, 0x65, 0x64, 0x69, 0x72, 0x65,
	0x63, 0x74, 0x18, 0x09, 0x20, 0x01, 0x28, 0x04, 0x52, 0x08, 0x72, 0x65, 0x64, 0x69, 0x72, 0x65,
	0x63, 0x74, 0x12, 0x27, 0x0a, 0x0f, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x6c, 0x79, 0x5f, 0x63, 0x72,
	0x61, 0x77, 0x6c, 0x65, 0x64, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0e, 0x6c, 0x6f, 0x63,
	0x61, 0x6c, 0x6c, 0x79, 0x43, 0x72, 0x61, 0x77, 0x6c, 0x65, 0x64, 0x42, 0x4b, 0x5a, 0x49, 0x67,
	0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e,
	0x65, 0x74, 0x61, 0x72, 0x63, 0x68, 0x69, 0x76, 0x65, 0x2f, 0x5a, 0x65, 0x6e, 0x6f, 0x2f, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x71, 0x75, 0x65, 0x75,
	0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x76, 0x31, 0x3b, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x76, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_item_proto_rawDescOnce sync.Once
	file_item_proto_rawDescData = file_item_proto_rawDesc
)

func file_item_proto_rawDescGZIP() []byte {
	file_item_proto_rawDescOnce.Do(func() {
		file_item_proto_rawDescData = protoimpl.X.CompressGZIP(file_item_proto_rawDescData)
	})
	return file_item_proto_rawDescData
}

var file_item_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_item_proto_goTypes = []any{
	(*ProtoItem)(nil), // 0: queue.ProtoItem
}
var file_item_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_item_proto_init() }
func file_item_proto_init() {
	if File_item_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_item_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*ProtoItem); i {
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
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_item_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_item_proto_goTypes,
		DependencyIndexes: file_item_proto_depIdxs,
		MessageInfos:      file_item_proto_msgTypes,
	}.Build()
	File_item_proto = out.File
	file_item_proto_rawDesc = nil
	file_item_proto_goTypes = nil
	file_item_proto_depIdxs = nil
}
