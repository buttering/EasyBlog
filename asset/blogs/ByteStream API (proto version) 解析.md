---
title: ByteStream API (proto version) 解析
date: 2024-08-15 18:52:29
toc: true
mathjax: true
categories:
- Remote Execution
tags:
- bytestream api
- 构建系统
- grpc
---

ByteStream不由Remote APIs所定义，而是归于Google APIs的范畴。

ByteStream API允许客户端从资源中读写字节流。资源有名称，这些名称在下面的API调用中提供，以标识正在读取或写入的资源。
ByteStream API的所有实现导出这里定义的接口:
* Read() :读取资源的内容。
* Write() :写入资源的内容。客户端可以使用相同的资源多次调用 Write() ，并且可以通过调用QueryWriteStatus() 来检查写的状态。
## message
### 读取
```protobuf
// ByteStream.Read 的请求对象。
message ReadRequest {
  // 要读取的资源名称。
  string resource_name = 1;
  
  // 相对于资源起始位置的第一个字节的偏移量。
  //
  // `read_offset` 为负或大于资源大小将导致 `OUT_OF_RANGE` 错误。
  int64 read_offset = 2;
  
  // 服务器允许在所有 `ReadResponse` 消息的总和中返回的最大 `data` 字节数。
  // `read_limit` 为零表示没有限制，负的 `read_limit` 则会导致错误。
  //
  // 如果流返回的字节数少于 `read_limit` 允许的字节数且未发生错误，
  // 则流包括从 `read_offset` 到资源结束的所有数据。
  int64 read_limit = 3;
}
```
```protobuf
// ByteStream.Read 的响应对象。
message ReadResponse {
  // 资源数据的一部分。服务 **可能** 会在某个 `ReadResponse` 中留空 `data`。
  // 这使得服务能够在生成更多数据的操作过程中，告知客户端请求仍然有效。
  bytes data = 10;
}
```
### 写入
```protobuf
// ByteStream.Write 的请求对象。
message WriteRequest {
  // 要写入的资源名称。对于每次 `Write()` 操作的第一个 `WriteRequest`，这**必须**设置。
  // 如果在后续调用中设置，它**必须**与第一个请求的值匹配。
  string resource_name = 1;
  
  // 数据应写入的资源起始位置的偏移量。这个字段在所有 `WriteRequest` 请求中都是必需的。
  //
  // 在一次 `Write()` 操作的第一个 `WriteRequest` 中，它表示 `Write()` 调用的初始偏移量。
  // 该值**必须**等于调用 `QueryWriteStatus()` 会返回的 `committed_size`。
  //
  // 在后续调用中，这个值**必须**设置，并且**必须**等于第一个 `write_offset` 和之前发送的所有 `data` 数据块大小的总和。
  //
  // 不正确的值会导致错误。
  int64 write_offset = 2;
  
  // 如果为 `true`，表示写入已完成。在发送 `finish_write` 为 `true` 的请求后发送任何 `WriteRequest` 会导致错误。
  bool finish_write = 3;
  
  // 资源的数据部分。客户端**可以**在某些 `WriteRequest` 请求中不填 `data` 字段。
  // 这使得客户端能够在生成更多数据的操作过程中告知服务请求仍然有效。
  bytes data = 10;
}
```
```protobuf
// ByteStream.Write 的响应对象。
message WriteResponse {
  // 已处理的给定资源的字节数。
  int64 committed_size = 1;
}
```
## service
```protobuf
service ByteStream {
  // `Read()` 用于将资源内容作为字节序列检索。
  // 字节被一次性返回多个响应中，并作为服务器端流式 RPC 的结果交付。
  rpc Read(ReadRequest) returns (stream ReadResponse);

  // `Write()` 用于将资源内容作为字节序列发送。
  // 字节作为客户端流式 RPC 请求原型的序列发送。
  //
  // `Write()` 操作是可恢复的。如果在 `Write()` 过程中出现错误或连接中断，
  // 客户端应通过调用 `QueryWriteStatus()` 检查 `Write()` 的状态，并从返回的 
  // `committed_size` 继续写入。这可能小于客户端之前发送的数据量。
  //
  // 对一个已写入并完成的资源名称调用 `Write()` 可能会导致错误，
  // 这取决于底层服务是否允许覆盖已写入的资源。
  //
  // 当客户端关闭请求通道时，服务将响应 `WriteResponse`。
  // 只有当客户端发送的 `WriteRequest` 中 `finish_write` 设置为 `true` 时，
  // 服务才会将资源视为 `complete`。在发送 `finish_write` 设置为 `true` 的请求后
  // 发送任何请求都会导致错误。客户端 **应该** 检查收到的 `WriteResponse`，
  // 以确定服务能够提交的数据量以及服务是否将资源视为 `complete`。
  rpc Write(stream WriteRequest) returns (WriteResponse);

  // `QueryWriteStatus()` 用于查找正在写入的资源的 `committed_size`，
  // 然后可以用作下一次 `Write()` 调用的 `write_offset`。
  //
  // 如果资源不存在（例如，资源已被删除，或第一次 `Write()` 尚未到达服务），
  // 该方法返回错误 `NOT_FOUND`。
  //
  // 客户端 **可以** 随时调用 `QueryWriteStatus()` 以确定已处理多少数据。
  // 如果客户端正在缓冲数据并需要知道哪些数据可以安全删除，这非常有用。
  // 对于给定资源名称的任何一系列 `QueryWriteStatus()` 调用，
  // 返回的 `committed_size` 值的序列将是非递减的。
  rpc QueryWriteStatus(QueryWriteStatusRequest)
      returns (QueryWriteStatusResponse);
}
```
## go 实现
在`bytestream.pb.go`的实现中，`ByteStream`对应这样一个接口（需要服务端实现）：
```go
type ByteStreamServer interface {
	Read(*ReadRequest, ByteStream_ReadServer) error
	Write(ByteStream_WriteServer) error
	QueryWriteStatus(context.Context, *QueryWriteStatusRequest) (*QueryWriteStatusResponse, error)
}
```
结构体的三个函数均被Handler封装：
```go
func _ByteStream_Read_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ReadRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ByteStreamServer).Read(m, &byteStreamReadServer{stream})
}

func _ByteStream_Write_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(ByteStreamServer).Write(&byteStreamWriteServer{stream})
}

func _ByteStream_QueryWriteStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryWriteStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ByteStreamServer).QueryWriteStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/google.bytestream.ByteStream/QueryWriteStatus",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ByteStreamServer).QueryWriteStatus(ctx, req.(*QueryWriteStatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}
```
这三个Handler被注册到`grpc.ServiceDesc`结构体上：
```go
var _ByteStream_serviceDesc = grpc.ServiceDesc{
	ServiceName: "google.bytestream.ByteStream",
	HandlerType: (*ByteStreamServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "QueryWriteStatus",
			Handler:    _ByteStream_QueryWriteStatus_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Read",
			Handler:       _ByteStream_Read_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Write",
			Handler:       _ByteStream_Write_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "google/bytestream/bytestream.proto",
}
```
最后，服务端只需调用`RegisterByteStreamServer`即可将ByteStream注册到持有的grpc.Server上。
```go
func RegisterByteStreamServer(s *grpc.Server, srv ByteStreamServer) {
	s.RegisterService(&_ByteStream_serviceDesc, srv)
}
```
