---
title: Logstream API (go version) 解析
date: 2024-08-15 18:53:29
toc: true
mathjax: true
categories:
- Remote Execution
tags:
- remote execution api
- 构建系统
- grpc
---

## 结构体
### 日志流
```go
// 一个日志的句柄（有序的字节序列）。
type LogStream struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 资源的结构化名称，其格式为：
    //
    // {parent=**}/logstreams/{logstream_id}
    // 例如: projects/123/logstreams/456-def
    //
    // 尝试将 LogStream 的 `name` 作为 `ByteStream.Write.resource_name`
    // 的值调用 Byte Stream API 的 `Write` RPC 是会出错的。
    Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
    // 用于传递给 `ByteStream.Write` 的资源名称，其格式为：
    //
    // {parent=**}/logstreams/{logstream_id}/{write_token}
    // 例如: projects/123/logstreams/456-def/789-ghi
    //
    // 尝试将 LogStream 的 `write_resource_name` 作为 `ByteStream.Write.resource_name`
    // 的值调用 Byte Stream API 的 `Read` RPC 是会出错的。
    //
    // `write_resource_name` 与 `name` 分开，以确保只有预期的写入者可以写入特定的 LogStream。
    // 写入者必须将写操作定向到 `write_resource_name`，而不是 `name`，并且必须具有写入 LogStreams
    // 的权限。`write_resource_name` 包含一个秘密令牌，应得到相应的保护；
    // 错误处理 `write_resource_name` 可能导致非预期的写入者破坏 LogStream。因此，该字段应从
    // 任何检索 LogStream 元数据的调用（例如：`GetLogStream`）中排除。
    //
    // 写入到该资源的字节在使用 `name` 资源调用 `ByteStream.Read` 时必须是可读的。
    // 读取 `write_resource_name` 必须返回 INVALID_ARGUMENT 错误。
    WriteResourceName string `protobuf:"bytes,2,opt,name=write_resource_name,json=writeResourceName,proto3" json:"write_resource_name,omitempty"`
}
```

- `Name`: 为**读取**日志流提供的唯一标识符，。
- `WriteResourceName`: 为**写入**日志流提供的唯一标识符（write_token）。该标识符应保密，以防止未经授权的写入。
### 请求
```go
// 包含创建新的 LogStream 资源所需的所有信息。
type CreateLogStreamRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
	// 必需的。创建的 LogStream 的父资源。
	// LogStream 的父资源类型列表由实现服务器决定。
	// 示例：projects/123
	Parent string `protobuf:"bytes,1,opt,name=parent,proto3" json:"parent,omitempty"`
}
```

- `parent`: 创建的 LogStream 资源的父资源。在构建系统中，父资源可以是构建任务或构建配置中包含的更高级别的资源或配置。
### 响应
## 接口
```go
// LogStreamServiceClient 是 LogStreamService 服务的客户端 API。
//
// 有关上下文 (ctx) 使用和关闭/结束流式 RPC 的语义信息，请参阅 https://godoc.org/google.golang.org/grpc#ClientConn.NewStream。
type LogStreamServiceClient interface {
    // 创建一个可以写入的 LogStream。
    //
    // 返回的 LogStream 资源名称将包含一个 `write_resource_name`，
    // 这是用于写入 LogStream 的资源。
    // 调用 CreateLogStream 的调用方应避免公开 `write_resource_name`。
    CreateLogStream(ctx context.Context, in *CreateLogStreamRequest, opts ...grpc.CallOption) (*LogStream, error)
}

```
该接口由日志的生产者调用，生产者获取LogStream返回的WriteResourceName后，使用ByteStream写入数据。
消费者持有同一个LogStream的Name，使用ByteSteam读取数据。
需要解决两个问题：

- LogSteam的Name如何传递给消费者？
- 多个生产者如何共享WriteResourceName？
## BuildGrid 实现
在BuildGrid的文档中，关于LogStream的描述如下：
> LogStream服务实现了LogStream API。在BuildGrid上下文中，这为Worker提供了一种机制，可以在构建过程中向感兴趣的客户端传输日志。客户端不一定是发出执行请求的工具;用于读取流的资源名可以使用Operations API获得。
> LogStream服务只处理创建实际的流资源，使用ByteStream API读取和写入流。这意味着任何包含LogStream服务的配置也需要ByteStream服务才能正常工作。
> LogStream服务的使用并不局限于从BuildBox工作器流化构建日志，BuildBox -tools存储库提供了将日志写入流的工具，这些流可以用于其他目的。LogStream服务也完全独立于BuildGrid的其余部分(除了用于读/写访问的ByteStream)，因此可以在不需要其他远程执行/缓存功能的情况下使用。在这个docker-compose示例中提供了一个仅使用logstream的部署示例

从中可以提炼几个重点：

1. 日志流向：Worker -> Client
2. LogStream服务基于ByteStream
3. 使用Operations API传递Name

进一步查看文档
> 如果作为更大的 BuildGrid 远程执行服务的一部分部署，BuildBox Worker 将使用 LogStream 服务流式传输 stdout/stderr 合并日志。资源名称包含在响应 Execute 或 WaitExecution 请求而发送的 Operation 消息的元数据字段中。
> 然后，ByteStream客户端可以使用资源名向LogStream服务发出资源的Read请求。一旦Action完成执行，就不能保证流还会存在，最好使用ActionResult中内联的stdout/stderr(或从CAS获取日志的相关摘要)。

1. Name包含在响应Execute或WaitExecution请求的Operation中。
2. LogStream只在执行时可用。

下图来自BuildBox文档
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/0fc3b93119b7ea112566ca7d729eef36/b57ef2be4938eae6f0c2a86cc3221884.png)
> LogStream API  定义了一种传输日志的机制，它依赖于 ByteStream API  进行数据的实际传输。
> execution服务会向 Worker 提供该 Worker 可独占访问的 write_resource_name，并向有兴趣接收这些输出的客户端广播相关的 read_resource_name。
> 由于日志是在任务运行结束后上传到 CAS 的，因此只有在命令执行过程中，流才是有效的。一旦最后一个读取器关闭连接，流就会结束，输出应通过存储在 CAS 中的 stdout/err blobs 进行访问。

## 工作流程
Log Stream API 管理用于流写入和读取未知最终长度有序字节序列的 LogStream 资源。请注意，根据这个定义 [https://cloud.google.com/apis/design/glossary](https://cloud.google.com/apis/design/glossary)，这是一个 API 接口而不是 API 服务。Log Stream API 支持通过 seeking 或“tail”模式读取未终结的 LogStreams，例如终端用户浏览构建结果 UI，希望尽快查看构建操作日志。
通过 Byte Stream API 进行 LogStreams 的读取和写入:
[https://cloud.google.com/dataproc/docs/reference/rpc/google.bytestream](https://cloud.google.com/dataproc/docs/reference/rpc/google.bytestream)
[https://github.com/googleapis/googleapis/blob/master/google/bytestream/bytestream.proto](https://github.com/googleapis/googleapis/blob/master/google/bytestream/bytestream.proto)
### 写入 LogStreams
通过 Byte Stream API 的 `Write` RPC 写入 LogStreams。写入 LogStreams 的字节应在合理的时间内提交并可供读取（实现定义）。提交到 LogStream 的字节不能被覆盖，并且终结的 LogStreams - 通过在最后一个 WriteRequest 中设置 `finish_write` 字段指示 - 也不能再追加。
调用 Byte Stream API 的 `Write` RPC 来写入 LogStreams 时，写入者必须将 LogStream 的 `write_resource_name` 作为 `ByteStream.WriteRequest.resource_name`，而不是 LogStream 的 `name`。读取和写入的资源名称分离允许广泛发布读取资源名称，同时确保只有了解写入资源名称的作者才能将字节写入 LogStream。
### 读取 LogStreams
使用 Byte Stream API 的 `Read` RPC 读取 LogStreams。读取终结的 LogStreams 时，服务器会从 `ByteStream.ReadRequest.read_offset` 开始流式传输 LogStream 的所有内容。
读取未终结的 LogStreams 时，服务器必须保持流式 `ByteStream.Read` RPC 打开，并在有更多字节可用或 LogStream 终结时发送 `ByteStream.ReadResponse` 消息。
### 示例多方读/写流程

1. LogStream 作者调用`CreateLogStream`
2. LogStream 作者发布`LogStream.name`
3. LogStream 作者调用`ByteStream.Write`，并将`LogStream.write_resource_name`作为`ByteStream.WriteRequest.resource_name`，`ByteStream.WriteRequest.finish_write`=false。
4. LogStream 读者调用`ByteStream.Read`，并将已发布的`LogStream.name`作为`ByteStream.ReadRequest.resource_name`。
5. LogStream 服务将所有提交的字节流式传输给 LogStream 读者，保持流打开。
6. LogStream 作者调用`ByteStream.Write`，并将`LogStream.write_resource_name`作为`ByteStream.WriteRequest.resource_name`，`ByteStream.WriteRequest.finish_write`=true。
7. LogStream 服务将所有剩余字节流式传输给 LogStream 读者，终止流。
