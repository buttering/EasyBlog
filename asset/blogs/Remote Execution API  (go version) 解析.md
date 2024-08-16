---
title: Remote Execution API  (go version) 解析
date: 2024-08-15 18:54:29
toc: true
mathjax: true
tags:
- remote execution api
- buildfarm
- 构建系统
- grpc
---

> 基于编译出的的go文件进行说明

远程执行的目的是通过两个独立但相关的概念：远程缓存和远程执行来加快构建过程。远程缓存允许用户共享构建输出，而远程执行则允许在远程机器集群上运行操作，这些机器集群可能比用户本地使用的机器集群功能更强大（或配置不同）。
远程执行应用程序接口（REAPI）描述了一个 gRPC + 协议缓冲区接口，为远程缓存和执行提供了三种主要服务：

- 内容可寻址存储（ContentAddressableStorage，CAS）服务：远程存储端点，通过摘要对内容进行寻址，摘要是存储或检索数据的哈希值和大小对。
- 动作缓存（AC）服务：已执行的构建动作与相应的结果工件之间的映射（通常与 CAS 服务共存）。
- 执行服务：允许用户请求针对构建农场执行构建任务的主要端点。
## 结构体
### 基础字段
re中每个表示message的结构体都有3个基础字段：
**state：**
由 Protocol Buffers 自动生成，用于管理消息的内部状态，通常用于支持序列化、反序列化和其他内部操作。
**sizeCache：**
由 Protocol Buffers 自动生成，用于缓存消息的大小信息，以优化序列化性能。
**unknownFields：**
由 Protocol Buffers 自动生成，用于存储解析过程中遇到的未知字段信息。这有助于在结构变化时保持兼容性。
### 摘要
摘要是re中最常见的结构体。用于对文件/action/command等实体进行唯一标识。
digest 包含哈希值和数据块大小来表示文件或数据块的摘要信息。在分布式系统中，可以进行高效的缓存、验证和去重操作。
```go
// Digest 表示内容摘要。给定 blob 的摘要由该 blob 的大小和其哈希值组成。
// 要使用的哈希算法由服务器定义。
//
// 大小被认为是摘要的一个不可分割的部分。也就是说，
// 即使 `hash` 字段被正确指定，但 `size_bytes` 未被指定，
// 服务器也必须拒绝请求。
//
// 包含大小在摘要中的原因如下：在许多情况下，服务器在开始处理 blob 之前需要知道它的大小，
// 例如展平 Merkle 树结构或将其流式传输到工作节点。
// 从技术上讲，服务器可以实现一个独立的元数据存储，但这会导致实现复杂度大大增加，
// 相对于让客户端预先指定大小（或在每个嵌入摘要的消息中存储大小）。
// 这确实意味着 API 泄漏了一些服务器实现的细节（我们认为这是合理的服务器实现），
// 但我们认为这是值得的权衡。
//
// 当 `Digest` 用于引用 proto 消息时，它总是引用二进制编码形式的消息。
// 为确保哈希的一致性，客户端和服务器必须确保它们根据以下规则序列化消息，
// 即使对于相同消息也有其他有效编码：
//
// * 字段按标签顺序序列化。
// * 没有未知字段。
// * 没有重复字段。
// * 字段按照其类型的默认语义进行序列化。
//
// 大多数协议缓冲区实现通常都会遵循这些规则进行序列化，但应注意避免捷径。
// 例如，将两个消息连接以合并它们可能会产生重复字段。
type Digest struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 哈希值。在使用 SHA-256 算法时，它将始终是一个小写的十六进制字符串，
    // 长度正好为 64 个字符。
    Hash string `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
    // blob 的大小，以字节为单位。
    SizeBytes int64 `protobuf:"varint,2,opt,name=size_bytes,json=sizeBytes,proto3" json:"size_bytes,omitempty"`
}
```
### 文件
#### 文件夹
文件夹`Directory`，表示一个无名的目录，其中包含实际的子文件夹、文件和链接。
```go
// Directory 表示文件树中的一个目录节点，包含零个或多个
// 子文件节点（FileNode）、子目录节点（DirectoryNode）和
// 符号链接节点（SymlinkNode）。
// 每个 Node 在目录中包含其名称，内容的摘要
// （无论是文件 blob 还是 Directory proto）或者是符号链接目标，
// 以及可能的一些关于文件或目录的元数据。
//
// 为了确保两个等价的目录树生成相同的哈希值，在构建 Directory 时
// 必须遵守以下限制：
//
// * 目录中的每个子节点路径必须只有一个段。多个层级的目录层次结构不得被折叠。
// * 目录中的每个子节点必须有唯一的路径段（文件名）。
//   需要注意的是，尽管 API 本身是区分大小写的，但执行 Action 的环境
//   可能区分大小写，也可能不区分。这就是说，带有“Foo”和“foo”的 Directory 是合法的，
//   但在执行时可能会被远程系统拒绝。
// * 目录中的文件、子目录和符号链接必须按照路径的字典序排序。
//   路径字符串必须按代码点排序，等价于按 UTF-8 字节排序。
// * 文件、子目录和符号链接的 NodeProperties 必须按属性名称的字典序排序。
//
// 符合这些限制的 Directory 称为规范形式。
//
// 举个例子，下面是一个名为 `bar` 的文件和一个名为 `foo` 的目录，
// 其中包含一个可执行文件 `baz` 的规范形式（为便于阅读，哈希值被缩短）:
//
// ```json
// // (Directory proto)
// {
//   files: [
//     {
//       name: "bar",
//       digest: {
//         hash: "4a73bc9d03...",
//         size: 65534
//       },
//       node_properties: [
//         {
//           "name": "MTime",
//           "value": "2017-01-15T01:30:15.01Z"
//         }
//       ]
//     }
//   ],
//   directories: [
//     {
//       name: "foo",
//       digest: {
//         hash: "4cf2eda940...",
//         size: 43
//       }
//     }
//   ]
// }
//
// // (Directory proto with hash "4cf2eda940..." and size 43)
// {
//   files: [
//     {
//       name: "baz",
//       digest: {
//         hash: "b2c941073e...",
//         size: 1294,
//       },
//       is_executable: true
//     }
//   ]
// }
// ```
type Directory struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
	// 目录中的文件列表。
	Files []*FileNode `protobuf:"bytes,1,rep,name=files,proto3" json:"files,omitempty"`
	// 目录中的子目录列表。
	Directories []*DirectoryNode `protobuf:"bytes,2,rep,name=directories,proto3" json:"directories,omitempty"`
	// 目录中的符号链接列表。
	Symlinks []*SymlinkNode  `protobuf:"bytes,3,rep,name=symlinks,proto3" json:"symlinks,omitempty"`
	// 节点的附加属性。
	NodeProperties *NodeProperties `protobuf:"bytes,5,opt,name=node_properties,json=nodeProperties,proto3" json:"node_properties,omitempty"`
}
```
值得注意的是，Directory自身并不是嵌套关系。比如上面代码注释中给出的例子，表示的是如下目录结构：
```powershell
/
├── bar (文件, 哈希值: 4a73bc9d03..., 大小: 65534, 节点属性: { MTime: 2017-01-15T01:30:15.01Z })
└── foo (目录, 哈希值: 4cf2eda940..., 大小: 43)
    └── baz (可执行文件, 哈希值: b2c941073e..., 大小: 1294)

```
根目录包含了foo目录，分别用两个独立的`Directory`结构表示，嵌套关系则使用`DirectoryNode`成员（其中只记录foo目录的摘要）表示。
`Directory`不持有自身的名字和摘要。
这种办法适合于分布式文件存储系统（如IPFS），能通过摘要（哈希值）进行标识和检索数据。
#### 文件夹节点/文件节点/链接节点
文件夹节点（`DirectoryNode`）不同于文件夹（`Directory`），只包含了用于在分布式文件系统中检索文件夹的必要数据。
```go
// DirectoryNode 表示一个 [Directory][build.bazel.remote.execution.v2.Directory] 的子节点，
// 该子节点本身是一个 `Directory` 及其相关的元数据。
type DirectoryNode struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 目录的名称。
    Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
    // 表示的 [Directory][build.bazel.remote.execution.v2.Directory] 对象的摘要。
    // 参见 [Digest][build.bazel.remote.execution.v2.Digest] 了解如何生成 proto 消息的摘要。
    Digest *Digest `protobuf:"bytes,2,opt,name=digest,proto3" json:"digest,omitempty"`
}
```

- `Digest`:   采用 Merkle Tree（默克尔树）结构生成，这种结构可以为整个目录树生成一个唯一的摘要，确保内容的一致性和完整性。
```go
// FileNode 代表一个单一文件及其相关的元数据。
type FileNode struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
	// 文件的名称。
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// 文件内容的摘要。
	Digest *Digest `protobuf:"bytes,2,opt,name=digest,proto3" json:"digest,omitempty"`
	// 如果文件可执行，则为 true，否则为 false。
	IsExecutable   bool            `protobuf:"varint,4,opt,name=is_executable,json=isExecutable,proto3" json:"is_executable,omitempty"`
	// 节点的属性。
	NodeProperties *NodeProperties `protobuf:"bytes,6,opt,name=node_properties,json=nodeProperties,proto3" json:"node_properties,omitempty"`
}
```

- `Digest`:  文件自身内容的摘要。
```go
// SymlinkNode 表示一个符号链接。
type SymlinkNode struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 符号链接的名称。
    Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
    // 符号链接的目标路径。路径分隔符为正斜杠 `/`。
    // 目标路径可以是相对于符号链接父目录的相对路径，
    // 也可以是以 `/` 开头的绝对路径。
    // 可以使用 [Capabilities][build.bazel.remote.execution.v2.Capabilities] API 检查对于绝对路径的支持。
    // `..` 组件可以出现在目标路径的任何位置，因为逻辑规范化在存在目录符号链接时可能导致不同的行为
    // （例如，`foo/../bar` 可能与 `bar` 不同）。
    // 为了减少潜在的缓存未命中，仍然推荐在不影响正确性的情况下进行规范化。
    Target         string          `protobuf:"bytes,2,opt,name=target,proto3" json:"target,omitempty"`
    // 节点的属性。
    NodeProperties *NodeProperties `protobuf:"bytes,4,opt,name=node_properties,json=nodeProperties,proto3" json:"node_properties,omitempty"`
}
```
`FileNode`、`SymlinkNode`和`Directory`(而不是`DirectoryNode`) 表示了文件/目录/链接在文件系统中的全部信息。
#### 节点属性
```go
// NodeProperties 适用于 [FileNodes][build.bazel.remote.execution.v2.FileNode]、
// [DirectoryNodes][build.bazel.remote.execution.v2.DirectoryNode] 和
// [SymlinkNodes][build.bazel.remote.execution.v2.SymlinkNode] 的节点属性。
// 服务器负责指定其接受的属性。
//
type NodeProperties struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 一个基于字符串的 [NodeProperties][build.bazel.remote.execution.v2.NodeProperty] 列表。
    Properties []*NodeProperty `protobuf:"bytes,1,rep,name=properties,proto3" json:"properties,omitempty"`
    // 文件的最后修改时间戳。
    Mtime *timestamp.Timestamp `protobuf:"bytes,2,opt,name=mtime,proto3" json:"mtime,omitempty"`
    // UNIX 文件模式，例如 0755。
    UnixMode *wrappers.UInt32Value `protobuf:"bytes,3,opt,name=unix_mode,json=unixMode,proto3" json:"unix_mode,omitempty"`
}
```
`DirecotoyNode`实际上并没有节点属性的字段。文件夹对应的节点属性位于其摘要对应的`Direcory`结构中。
`NodeProperty`是一个键值对。
```go
// NodeProperty 表示文件节点、目录节点或符号链接节点的一个属性。
type NodeProperty struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 属性名称。
    Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
    // 属性值。
    Value string `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}
```
### 指令
```go
// Command 代表一个由 worker 执行的实际命令及其环境规范，
// 该命令是执行 [Action][build.bazel.remote.execution.v2.Action] 时运行的。
//
// 除非另有要求，环境（如可用的系统库或二进制文件，以及挂载的文件系统位置）
// 由远程执行 API 的具体实现定义。
type Command struct {
    state                  protoimpl.MessageState
    sizeCache              protoimpl.SizeCache
    unknownFields          protoimpl.UnknownFields
    // 命令的参数。
    //
    // 第一个参数指定要运行的命令，可以是绝对路径、相对于工作目录的路径，或无路径分隔符的非限定路径，
    // 将使用操作系统的 PATH 环境变量进行解析。应使用运行 worker 的操作系统的本地路径分隔符。
    // 如果 `environment_variables` 列表包含 PATH 环境变量的条目，则应遵循这些条目。
    // 否则，解析过程由实现定义。
    //
    // 在 v2.3 版本中进行了更改。v2.2 和更早版本要求不执行 PATH 查找，并且相对路径相对于输入根进行解析。
    // 然而，这种行为不能依赖，因为大多数实现已经遵循上面描述的规则。
    Arguments []string `protobuf:"bytes,1,rep,name=arguments,proto3" json:"arguments,omitempty"`
    
    // 运行程序时要设置的环境变量。worker 可以提供其自己的默认环境变量；
    // 这些默认值可以使用此字段覆盖。此外，还可以指定其他变量。
    //
    // 为了确保等效的 [Command][build.bazel.remote.execution.v2.Command] 始终生成相同的哈希值，
    // 环境变量必须按名称按字典顺序排序。字符串的排序按代码点进行，等效于按 UTF-8 字节进行排序。
    EnvironmentVariables []*Command_EnvironmentVariable `protobuf:"bytes,2,rep,name=environment_variables,json=environmentVariables,proto3" json:"environment_variables,omitempty"`
    
    // 客户端期望从动作中检索的输出文件列表。只有列出的文件以及 `output_directories` 中列出的目录
    // 将作为输出返回给客户端。命令执行过程中可能创建的其他文件或目录将被丢弃。
    //
    // 路径相对于动作执行的工作目录。路径使用单个正斜杠 (`/`) 作为路径分隔符，即使执行平台本地使用不同的分隔符。
    // 路径不得包含尾部斜杠或前导斜杠，应为相对路径。
    //
    // 为了确保相同 Action 的一致哈希值，输出路径必须按字典顺序按代码点进行排序（或等效于按 UTF-8 字节进行排序）。
    //
    // 输出文件不能重复、不能是另一个输出文件的父级，也不能与任何列出的输出目录路径相同。
    //
    // 在执行之前，由 worker 创建通向输出文件的目录，即使它们不是输入根的明确部分。
    //
    // 自 v2.1 版本起已弃用：请改用 `output_paths`。
    //
    // Deprecated: 不要使用。
    OutputFiles []string `protobuf:"bytes,3,rep,name=output_files,json=outputFiles,proto3" json:"output_files,omitempty"`
    
    // 客户端期望从动作中检索的输出目录列表。只有列出的目录（包括整个目录结构）
    // 将作为 [Tree][build.bazel.remote.execution.v2.Tree] 消息摘要返回，文件列在 `output_files` 中。
    // 命令执行过程中可能创建的其他文件或目录将被丢弃。
    //
    // 路径相对于动作执行的工作目录。路径使用单个正斜杠 (`/`) 作为路径分隔符，
    // 即使执行平台本地使用不同的分隔符。路径不得包含尾部斜杠或前导斜杠，应为相对路径。允许使用空字符串，但不推荐。
    //
    // 为了确保相同 Action 的一致哈希值，输出路径必须按字典顺序按代码点进行排序（或等效于按 UTF-8 字节进行排序）。
    //
    // 输出目录不能重复，也不能与任何列出的输出文件路径相同。
    // 允许一个输出目录作为另一个输出目录的父目录。
    //
    // 由 worker 创建通向输出目录的目录（但不是输出目录本身），即使它们不是输入根的明确部分。
    //
    // 自 v2.1 版本起已弃用：请改用 `output_paths`。
    //
    // Deprecated: 不要使用。
    OutputDirectories []string `protobuf:"bytes,4,rep,name=output_directories,json=outputDirectories,proto3" json:"output_directories,omitempty"`
    
    // 客户端期望从动作中检索的输出路径列表。只有列出的路径将作为输出返回给客户端。
    // 输出的类型（文件或目录）未指定，将在动作执行后由服务器确定。
    // 如果结果路径是文件，则将其作为 [OutputFile][build.bazel.remote.execution.v2.OutputFile] 类型字段返回。
    // 如果路径是目录，则整个目录结构将作为 [Tree][build.bazel.remote.execution.v2.Tree] 消息摘要返回。
    // 命令执行过程中可能创建的其他文件或目录将被丢弃。
    //
    // 路径相对于动作执行的工作目录。路径使用单个正斜杠 (`/`) 作为路径分隔符，
    // 即使执行平台本地使用不同的分隔符。路径不得包含尾部斜杠或前导斜杠，应为相对路径。
    //
    // 为了确保相同 Action 的一致哈希值，输出路径必须去重并按字典顺序按代码点进行排序（或等效于按 UTF-8 字节进行排序）。
    //
    // 由 worker 创建通向输出路径的目录，即使它们不是输入根的明确部分。
    //
    // 自 v2.1 版本起：此字段取代已弃用的 `output_files` 和 `output_directories` 字段。
    // 如果使用 `output_paths`，则 `output_files` 和 `output_directories` 将被忽略。
    OutputPaths []string `protobuf:"bytes,7,rep,name=output_paths,json=outputPaths,proto3" json:"output_paths,omitempty"`
    
    // 执行环境的平台要求。服务器可以选择在任何满足要求的 worker 上执行操作，
    // 因此客户端应确保在任何这样的 worker 上运行操作将具有相同的结果。
    // 详细的词汇表可以在附带的 platform.md 中找到。
    // 自 v2.2 版本起已弃用：平台属性现在直接在 action 中指定。
    // 请参阅 [Action][build.bazel.remote.execution.v2.Action] 中的文档注释以进行迁移。
    //
    // Deprecated: 不要使用。
    Platform *Platform `protobuf:"bytes,5,opt,name=platform,proto3" json:"platform,omitempty"`
    
    // 命令运行时的工作目录，相对于输入根。它必须是输入树中存在的目录。
    // 如果留空，则在输入根中运行操作。
    WorkingDirectory string `protobuf:"bytes,6,opt,name=working_directory,json=workingDirectory,proto3" json:"working_directory,omitempty"`
    
    // 客户端期望从输出文件和目录中检索的节点属性键列表。
    // 键可以是字符串的 [NodeProperty][build.bazel.remote.execution.v2.NodeProperty] 的名称，
    // 或 [NodeProperties][build.bazel.remote.execution.v2.NodeProperties] 中字段的名称。
    // 为了确保等效的 `Action` 始终生成相同的哈希值，节点属性必须按名称按字典顺序排序。
    // 字符串的排序按代码点进行，等效于按 UTF-8 字节进行排序。
    //
    // 字符串属性的解释取决于服务器。如果服务器未识别属性，将返回 `INVALID_ARGUMENT`。
    OutputNodeProperties []string `protobuf:"bytes,8,rep,name=output_node_properties,json=outputNodeProperties,proto3" json:"output_node_properties,omitempty"`
}
```

- `EnvironmentVariables`: 运行命令时设置的环境变量。如：`["PATH": "/usr/bin/:/usr/local/bin"]`。
- `OutputPaths`: 客户端期望从动作中检索的输出路径列表。路径相对于动作执行的工作目录。如`["User/buttering/pcc/pcc/cmd/cc/a.out"]`。
- `WorkingDirectory`: 命令运行时的工作目录，相对于输入根。
- `OutputNodeProperties`: 客户端期望从输出文件和目录中检索的节点属性键列表。

`Command`不持有自身的摘要。
### 动作
```go
// Action 捕获了所有需要的信息，以重现一次执行。
//
// `Action` 是 [Execution] 服务的核心组件。单个 `Action` 表示可以由执行服务
// 执行的可重复的动作。`Action` 可以通过其线格式编码的摘要简洁地识别，
// 一旦 `Action` 被执行，将缓存到动作缓存中。未来的请求可以使用缓存的结果，
// 而不需要重新执行。
//
// 当服务器完成一个 [Action][build.bazel.remote.execution.v2.Action] 的执行后，
// 服务器可以选择将 [result][build.bazel.remote.execution.v2.ActionResult]
// 缓存到 [ActionCache][build.bazel.remote.execution.v2.ActionCache] 中，
// 除非 `do_not_cache` 为 `true`。客户端应该期待服务器这样做。
// 默认情况下，未来调用 [Execute][build.bazel.remote.execution.v2.Execution.Execute] 相同的 `Action`
// 也将从缓存中提供其结果。客户端必须注意理解缓存行为。
// 理想情况下，所有 `Action` 都应该是可再现的，以便从缓存中提供结果始终是可取且正确的。
type Action struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields

    // [Command][build.bazel.remote.execution.v2.Command] 的摘要
    // 必须存在于 [ContentAddressableStorage][build.bazel.remote.execution.v2.ContentAddressableStorage] 中。
    CommandDigest *Digest `protobuf:"bytes,1,opt,name=command_digest,json=commandDigest,proto3" json:"command_digest,omitempty"`

    // 输入文件的根目录 [Directory][build.bazel.remote.execution.v2.Directory] 的摘要。
    // 目录树中的文件将在命令执行之前位于构建机器上的正确位置。
    // 根目录以及引用的每个子目录和内容 blob 必须存在于
    // [ContentAddressableStorage][build.bazel.remote.execution.v2.ContentAddressableStorage] 中。
    InputRootDigest *Digest `protobuf:"bytes,2,opt,name=input_root_digest,json=inputRootDigest,proto3" json:"input_root_digest,omitempty"`

    // 执行应在其后被终止的超时时间。如果超时不存在，则客户端指定执行应该继续，
    // 只要服务器允许。如果客户端未指定超时，服务器应强制执行超时，
    // 但是，如果客户端指定的超时长于服务器的最大超时，服务器必须拒绝请求。
    //
    // 超时仅用于覆盖指定动作的 "执行" 时间，而不是队列中的时间
    // 或在执行前后的任何开销时间，比如输入/输出的序列化。
    // 服务器应避免包括客户端无法控制的时间，并可以延长或减少超时
    // 以适应执行过程中发生的延迟或加速（例如，从内容可寻址存储中懒加载数据、
    // 虚拟机的实时迁移、仿真开销）。
    //
    // 超时是 [Action][build.bazel.remote.execution.v2.Action] 消息的一部分，
    // 因此两个具有不同超时的 `Action` 是不同的，即使它们在其他方面相同。
    // 这是因为，如果它们不是，那么以低于所需超时运行 `Action` 可能会导致缓存命中
    // 来自具有更长超时的执行运行，从而隐藏超时太短的事实。
    // 通过直接在 `Action` 中编码，更低的超时将导致缓存未命中，并且执行超时将立即失败，
    // 而不是在缓存项被驱逐时。
    Timeout *duration.Duration `protobuf:"bytes,6,opt,name=timeout,proto3" json:"timeout,omitempty"`

    // 如果为 true，则 `Action` 的结果不能被缓存，并且同一 `Action` 的正在进行的请求可能不会合并。
    DoNotCache bool `protobuf:"varint,7,opt,name=do_not_cache,json=doNotCache,proto3" json:"do_not_cache,omitempty"`

    // 可选的额外盐值，用于将此 `Action` 放入与具有相同字段内容的其他实例不同的缓存命名空间中。
    // 这个盐值通常来自操作配置，特定于源如库和服务配置，
    // 并允许放弃可能因有缺陷的软件或工具故障而受到污染的整个 ActionResults 集合。
    Salt []byte `protobuf:"bytes,9,opt,name=salt,proto3" json:"salt,omitempty"`

    // 可选的执行环境平台要求。服务器可以选择在任何满足要求的 worker 上执行操作，
    // 因此客户端应确保在任何这样的 worker 上运行操作将具有相同的结果。
    // 详细词汇表可以在附带的 platform.md 中找到。
    // 新增于 v2.2 版本：客户端应设置这些平台属性以及 [Command][build.bazel.remote.execution.v2.Command] 中的属性。
    // 服务器应优先考虑此处设置的属性。
    Platform *Platform `protobuf:"bytes,10,opt,name=platform,proto3" json:"platform,omitempty"`
}
```

- `CommandDigest`: 要执行的命令的摘要，该命令必须存在于内容可寻址存储（CAS）中。
- `InputRootDigest`: 输入文件的根目录的摘要，这些文件在命令执行之前会被放在构建机器上的正确位置。
- `Timeout`:  执行应在其后被终止的超时时间。如果客户端未指定超时，服务器应强制执行超时。不同的超时时间的Action，其摘要也不同。
> 为什么超时会(要)导致摘要不同？
> 在远程执行系统中，Action 的摘要不仅依赖于包含的文件和命令，还依赖于其所有元数据，包括超时时间。这样一来，即使命令和文件完全相同，但如果超时不同，摘要就会不同。
> 假设有两个相同的 Action，但是一个有较短的超时，另一个有较长的超时。如果这两个 Action共享同一个摘要，系统可能会以为它们是完全相同的操作，并使用缓存中的结果。但是，如果较短超时的 Action 实际上需要更长时间来完成，并且它因超时而失败，那么这次执行结果将被错误地缓存，并在未来的请求中被使用，从而导致不一致且不可预料的行为。

`action` 同样不持有自身的摘要。
### 平台信息
```go
// 一个 `Platform` 表示一组要求，例如硬件、操作系统或编译工具链，
// 以用于 [Action][build.bazel.remote.execution.v2.Action] 的执行环境。
// 一个 `Platform` 由一系列表示该平台要求的键值对组成。
type Platform struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 构成此平台的属性。为了确保等效的 `Platform` 总是哈希到相同的值，
    // 属性必须按名称进行字典顺序排序，然后按值排序。字符串的排序是按代码点进行的，
    // 等效于按 UTF-8 字节排序。
    Properties []*Platform_Property `protobuf:"bytes,1,rep,name=properties,proto3" json:"properties,omitempty"`
}
```
`Platform_Property` 表示一个键值对：
```go
// 环境的单个属性。服务器负责指定接收的属性 `name`。如果在对
// [Action][build.bazel.remote.execution.v2.Action] 的要求中提供了未知的 `name`，服务器应该
// 拒绝执行请求。如果服务器允许，相同的 `name` 可以出现多次。
//
// 服务器还负责指定属性 `value` 的解释。例如，一个描述所需
// RAM 的属性可以被解释为允许一个具有 16GB 的工作节点完成对
// 8GB 请求的要求，而描述操作系统环境的属性可能需要与工作节点的 OS 完全匹配。
//
// 服务器可以使用一个或多个属性的 `value` 来决定如何设置执行环境，
// 例如通过使特定的系统文件对工作节点可用。
//
// 名称和值通常是区分大小写的。请注意，平台隐含地是操作摘要的一部分，
// 因此，名称或值中的微小变化（如更改大小写）可能会导致不同的操作缓存条目。
type Platform_Property struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 属性名称。
    Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
    // 属性值。
    Value string `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}
```
### 请求
#### 请求元数据
请求元数据是 gRPC 协议的一部分，作为gRPC请求的元数据携带。
```go
// 一个可选的元数据，用于附加到任何 RPC 请求中，以告知服务器有关该请求的外部上下文。
// 服务器可以将其用于日志记录或其他用途。使用该元数据时，客户端将其使用规范的 proto 序列化附加到调用中：
//
// * 名称：`build.bazel.remote.execution.v2.requestmetadata-bin`
// * 内容：基于 base64 编码的二进制 `RequestMetadata` 消息。
// 注意：gRPC 库默认将二进制头编码为 base64（https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md#requests）。
// 因此，如果使用 gRPC 库传递/检索这些元数据，用户可以忽略 base64 编码，并假设它只是作为二进制消息序列化的。
type RequestMetadata struct {
  state         protoimpl.MessageState
  sizeCache     protoimpl.SizeCache
  unknownFields protoimpl.UnknownFields
  // 发出请求的工具的详细信息。
  ToolDetails *ToolDetails `protobuf:"bytes,1,opt,name=tool_details,json=toolDetails,proto3" json:"tool_details,omitempty"`
  // 将多个请求与同一操作关联在一起的标识符。
  // 例如，多个对 CAS、Action Cache 和 Execution API 的请求用于编译 foo.cc。
  ActionId string `protobuf:"bytes,2,opt,name=action_id,json=actionId,proto3" json:"action_id,omitempty"`
  // 将多个操作与最终结果关联在一起的标识符。
  // 例如，构建和运行 foo_test 需要多个操作。
  ToolInvocationId string `protobuf:"bytes,3,opt,name=tool_invocation_id,json=toolInvocationId,proto3" json:"tool_invocation_id,omitempty"`
  // 将多个工具调用关联在一起的标识符。 例如，在提交补丁后的运行中运行 foo_test、bar_test 和 baz_test。
  CorrelatedInvocationsId string `protobuf:"bytes,4,opt,name=correlated_invocations_id,json=correlatedInvocationsId,proto3" json:"correlated_invocations_id,omitempty"`
  // 对此类操作的简要描述，例如 CppCompile 或 GoLink。
  // 对于这个字段的值没有标准的一致性，预期在不同客户端工具之间会有所不同。
  ActionMnemonic string `protobuf:"bytes,5,opt,name=action_mnemonic,json=actionMnemonic,proto3" json:"action_mnemonic,omitempty"`
  // 生成此操作的目标的标识符。
  // 不对有多少操作与单个目标相关联做任何保证。
  TargetId string `protobuf:"bytes,6,opt,name=target_id,json=targetId,proto3" json:"target_id,omitempty"`
  // 生成目标的配置的标识符，e.g. 用于区分构建主机工具或不同的目标平台。
  // 没有期望此值具有任何特定的结构或跨调用相等性，尽管一些客户端工具可能会提供这些保证。
  ConfigurationId string `protobuf:"bytes,7,opt,name=configuration_id,json=configurationId,proto3" json:"configuration_id,omitempty"`
}
```
服务器使用`metadata.FromIncomingContext(ctx context.Context)`进行拦截；
客户端使用`metadata.NewOutgoingContext(ctx context.Context, md MD)`将元数据添加到请求中。
#### 执行请求
```go
// 一个 [Execution.Execute][build.bazel.remote.execution.v2.Execution.Execute] 的请求消息。
type ExecuteRequest struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 要操作的执行系统实例。服务器可能支持执行系统的多个实例
    // （各自拥有各自的工作节点、存储、缓存等）。服务器可以要求使用此字段
    // 在其定义的方式中进行选择，否则可以省略此字段。
    InstanceName string `protobuf:"bytes,1,opt,name=instance_name,json=instanceName,proto3" json:"instance_name,omitempty"`
    // 如果为真，则无论该动作的结果是否已经存在于
    // [ActionCache][build.bazel.remote.execution.v2.ActionCache] 中，都会执行该动作。
    // 但是，执行仍然允许与其他正在进行的相同动作的执行合并 - 从语义上讲，
    // 服务仅必须保证在对应的执行请求发送之前，不可见带有此字段设置的执行结果。
    // 注意，设置此字段的执行请求中的动作完成后仍然可以被录入操作缓存，
    // 并且服务应覆盖任何现有条目。这允许 skip_cache_lookup 请求用于替换
    // 引用不再可用的输出或以任何方式受损的操作缓存条目。
    // 如果为假，结果可以从操作缓存中提供。
    SkipCacheLookup bool `protobuf:"varint,3,opt,name=skip_cache_lookup,json=skipCacheLookup,proto3" json:"skip_cache_lookup,omitempty"`
    // 要执行的 [Action][build.bazel.remote.execution.v2.Action] 的摘要。
    ActionDigest *Digest `protobuf:"bytes,6,opt,name=action_digest,json=actionDigest,proto3" json:"action_digest,omitempty"`
    // 可选的动作执行策略。
    // 如果未提供，服务器将使用默认策略。
    ExecutionPolicy *ExecutionPolicy `protobuf:"bytes,7,opt,name=execution_policy,json=executionPolicy,proto3" json:"execution_policy,omitempty"`
    // 可选的远程缓存执行结果策略。
    // 如果未提供，服务器将使用默认策略。
    // 这可以应用于 ActionResult 和相关的 blobs。
    ResultsCachePolicy *ResultsCachePolicy `protobuf:"bytes,8,opt,name=results_cache_policy,json=resultsCachePolicy,proto3" json:"results_cache_policy,omitempty"`
    // 用于计算动作摘要的摘要函数。
    //
    // 如果使用的摘要函数是 MD5、MURMUR3、SHA1、SHA256、SHA384、SHA512 或 VSO 中之一，
    // 客户端可以不设置此字段。在这种情况下，服务器应通过动作摘要哈希长度和服务器的功能中声明的摘要函数来推断摘要函数。
    DigestFunction DigestFunction_Value `protobuf:"varint,9,opt,name=digest_function,json=digestFunction,proto3,enum=build.bazel.remote.execution.v2.DigestFunction_Value" json:"digest_function,omitempty"`
}
```
#### 批量上传
```go
// [ContentAddressableStorage.BatchUpdateBlobs][build.bazel.remote.execution.v2.ContentAddressableStorage.BatchUpdateBlobs] 
// 的请求消息。
type BatchUpdateBlobsRequest struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 要操作的执行系统实例。服务器可能支持多个执行系统实例（各自拥有各自的工作节点、存储、缓存等）。
    // 服务器可以要求使用此字段在其定义的方式中进行选择，否则可以省略此字段。
    InstanceName string `protobuf:"bytes,1,opt,name=instance_name,json=instanceName,proto3" json:"instance_name,omitempty"`
    // 各个上传请求。
    Requests []*BatchUpdateBlobsRequest_Request `protobuf:"bytes,2,rep,name=requests,proto3" json:"requests,omitempty"`
    // 用于计算上传 blob 的摘要的摘要函数。
    //
    // 如果使用的摘要函数是 MD5、MURMUR3、SHA1、SHA256、SHA384、SHA512 或 VSO 中之一，
    // 客户端可以不设置此字段。在这种情况下，服务器应通过 blob 摘要哈希的长度和服务器的能力中声明的摘要函数来推断摘要函数。
    DigestFunction DigestFunction_Value `protobuf:"varint,5,opt,name=digest_function,json=digestFunction,proto3,enum=build.bazel.remote.execution.v2.DigestFunction_Value" json:"digest_function,omitempty"`
}

// 对应于客户端要上传的单个 blob 的请求。
type BatchUpdateBlobsRequest_Request struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // blob 的摘要。这必须是 `data` 的摘要。所有摘要必须使用相同的摘要函数。
    Digest *Digest `protobuf:"bytes,1,opt,name=digest,proto3" json:"digest,omitempty"`
    // 原始二进制数据。
    Data []byte `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
    // `data` 的格式。必须是 `IDENTITY`/未指定，或 [CacheCapabilities.supported_batch_compressors][build.bazel.remote.execution.v2.CacheCapabilities.supported_batch_compressors] 字段中列出的压缩器之一。
    Compressor Compressor_Value `protobuf:"varint,3,opt,name=compressor,proto3,enum=build.bazel.remote.execution.v2.Compressor_Value" json:"compressor,omitempty"`
}
```
### 响应
#### 相关结构体
##### 操作
首先介绍Operation消息，被定义在google.longrunning.Operation API中。可以用Operation于跟踪和管理长时间运行的操作状态和结果。
```go
// 此资源表示网络 API 调用的结果，即一个长时间运行的操作。
type Operation struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 服务器分配的名称，仅在最初返回它的同一服务中唯一。
    // 如果使用默认的 HTTP 映射，`name` 应为以 `operations/{unique_id}` 结尾的资源名称。
    Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
    // 与操作相关的特定服务的元数据。它通常包含进度信息和常见元数据（如创建时间）。
    // 某些服务可能不提供这些元数据。任何返回长时间运行操作的方法应记录元数据类型（如果有）。
    Metadata *anypb.Any `protobuf:"bytes,2,opt,name=metadata,proto3" json:"metadata,omitempty"`
    // 如果值为 `false`，表示操作仍在进行中。
    // 如果值为 `true`，表示操作已完成，并且 `error` 或 `response` 可用。
    Done bool `protobuf:"varint,3,opt,name=done,proto3" json:"done,omitempty"`
    // 操作结果，可以是 `error` 或有效的 `response`。
    // 如果 `done` == `false`，则 `error` 和 `response` 都不会设置。
    // 如果 `done` == `true`，则 `error` 或 `response` 中会设置其中一个。
    //
    // 可以分配给 Result 的类型:
    //	*Operation_Error
    //	*Operation_Response
    Result isOperation_Result `protobuf_oneof:"result"`
}
```
长时操作的API请求的返回值都会被包裹在`Operation`中。
当请求完成且成功时，`Result`字段为`Operation_Response`：
```go
type Operation_Response struct {
	// 成功情况下操作的正常响应。如果原方法在成功时不返回数据，例如 `Delete`，响应为 `google.protobuf.Empty`。
	// 如果原方法是标准的 `Get`/`Create`/`Update`，响应应为资源。对于其他方法，响应的类型应为 `XxxResponse`，
	// 其中 `Xxx` 是原方法名称。例如，如果原方法名称是 `TakeSnapshot()`，推断的响应类型是 `TakeSnapshotResponse`。
	Response *anypb.Any `protobuf:"bytes,5,opt,name=response,proto3,oneof"`
}
```
**例如，Execute在生成响应时，会将**`**Operation.Result.Response**`**赋为**`**ExecuteResponse**`**，将**`**Operation.Metadata**`**赋为**`**ExecuteOperationMetadata**`**。**
##### 状态
`Status`用于记录操作是否成功等信息。定义在googleapi.rpc中。
```go
// `Status` 类型定义了适用于不同编程环境的逻辑错误模型，包括 REST APIs 和 RPC APIs。它被 [gRPC](https://github.com/grpc) 使用。
// 每个 `Status` 消息包含三部分数据：错误代码、错误消息和错误详情。
//
// 可以在 [API Design Guide](https://cloud.google.com/apis/design/errors) 中了解更多关于此错误模型以及如何使用它的信息。
type Status struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 状态码，应该是 [google.rpc.Code][google.rpc.Code] 的枚举值。
    Code int32 `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
    // 面向开发者的错误消息，应该是英文的。任何面向用户的错误消息应该在 [google.rpc.Status.details][google.rpc.Status.details] 字段中进行本地化，或者由客户端进行本地化。
    Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
    // 携带错误详情的消息列表。API 会使用一组通用的消息类型。
    Details []*anypb.Any `protobuf:"bytes,3,rep,name=details,proto3" json:"details,omitempty"`
}

```
状态码列举如下：

- OK (Code = 0)：表示操作成功。
- Canceled (Code = 1)：表示操作被取消（通常是由调用方发起）。
- Unknown (Code = 2)：表示未知错误，通常在接收到未知错误空间的 Status 值时返回。
- InvalidArgument (Code = 3)：表示客户端指定了无效的参数，例如格式错误的文件名。
- DeadlineExceeded (Code = 4)：表示操作在完成之前已过期。
- NotFound (Code = 5)：表示请求的实体（如文件或目录）未找到。
- AlreadyExists (Code = 6)：表示试图创建的实体已经存在。
- PermissionDenied (Code = 7)：表示调用方没有执行指定操作的权限。
- ResourceExhausted (Code = 8)：表示某些资源已耗尽，例如用户配额或文件系统空间不足。
- FailedPrecondition (Code = 9)：表示操作被拒绝，因为系统不处于执行操作所需的状态。
- Aborted (Code = 10)：表示操作被中止，通常由于并发问题。
- OutOfRange (Code = 11)：表示操作超出有效范围，如搜索或读取超出文件末尾。
- Unimplemented (Code = 12)：表示操作未实现或在服务中不受支持。
- Internal (Code = 13)：表示内部错误，系统预期的不变量被破坏。
- Unavailable (Code = 14)：表示服务当前不可用，可能是暂时的。
- DataLoss (Code = 15)：表示数据丢失或损坏不可恢复。
- Unauthenticated (Code = 16)：表示请求没有有效的认证凭证来执行操作。
#### 执行
```go
// [Execution.Execute][build.bazel.remote.execution.v2.Execution.Execute] 的响应消息，
// 将包含在 [Operation][google.longrunning.Operation.response] 的 [response 字段][google.longrunning.Operation.response] 中。
type ExecuteResponse struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields

    // 动作的执行结果。
    Result *ActionResult `protobuf:"bytes,1,opt,name=result,proto3" json:"result,omitempty"`
    
    // 如果结果是从缓存中提供的，则为 true；如果是执行的结果，则为 false。
    CachedResult bool `protobuf:"varint,2,opt,name=cached_result,json=cachedResult,proto3" json:"cached_result,omitempty"`
    
    // 如果状态代码不是 `OK`，则表示操作未完成执行。例如，如果操作在执行期间超时，状态将有一个 `DEADLINE_EXCEEDED` 代码。
    // 服务器必须使用此字段来报告执行中的错误，而不是 `Operation` 对象上的 `error` 字段。
    //
    // 如果状态代码不是 `OK`，则结果不能被缓存。对于错误状态，`result` 字段是可选的；
    // 如果服务器有可用的信息，它可以填充输出、stdout 和 stderr 相关的字段，例如超时操作的 stdout 和 stderr。
    Status *status.Status `protobuf:"bytes,3,opt,name=status,proto3" json:"status,omitempty"`
    
    // 服务器希望提供的附加日志输出的可选列表。服务器可以使用此字段按需返回执行相关的日志。
    // 这主要是为了让用户更容易调试可能在实际作业执行之外的问题，例如标识执行动作的工作节点或提供工作节点设置阶段的日志。
    // 键应该是人类可读的，以便客户端可以将其显示给用户。
    ServerLogs map[string]*LogFile `protobuf:"bytes,4,rep,name=server_logs,json=serverLogs,proto3" json:"server_logs,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
    
    // 包含操作执行细节的自由格式信息消息，可能在失败时或明确请求时显示给用户。
    Message string `protobuf:"bytes,5,opt,name=message,proto3" json:"message,omitempty"`
}
```
```go
// 关于正在进行的 [执行][build.bazel.remote.execution.v2.Execution.Execute] 的元数据，
// 将包含在 [Operation][google.longrunning.Operation] 的 [metadata field][google.longrunning.Operation.response] 中。
type ExecuteOperationMetadata struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 当前的执行阶段。
    Stage ExecutionStage_Value `protobuf:"varint,1,opt,name=stage,proto3,enum=build.bazel.remote.execution.v2.ExecutionStage_Value" json:"stage,omitempty"`
    // 正在执行的 [Action][build.bazel.remote.execution.v2.Action] 的摘要。
    ActionDigest *Digest `protobuf:"bytes,2,opt,name=action_digest,json=actionDigest,proto3" json:"action_digest,omitempty"`
    // 如果设置了，客户端可以使用这个资源名称和 [ByteStream.Read][google.bytestream.ByteStream.Read] 从托管流响应的端点流式传输标准输出。
    StdoutStreamName string `protobuf:"bytes,3,opt,name=stdout_stream_name,json=stdoutStreamName,proto3" json:"stdout_stream_name,omitempty"`
    // 如果设置了，客户端可以使用这个资源名称和 [ByteStream.Read][google.bytestream.ByteStream.Read] 从托管流响应的端点流式传输标准错误。
    StderrStreamName string `protobuf:"bytes,4,opt,name=stderr_stream_name,json=stderrStreamName,proto3" json:"stderr_stream_name,omitempty"`
    // 客户端可以阅读此字段以查看正在进行的执行的详细信息。
    PartialExecutionMetadata *ExecutedActionMetadata `protobuf:"bytes,5,opt,name=partial_execution_metadata,json=partialExecutionMetadata,proto3" json:"partial_execution_metadata,omitempty"`
}
```
#### 批量上传
```go
// [ContentAddressableStorage.BatchUpdateBlobs][build.bazel.remote.execution.v2.ContentAddressableStorage.BatchUpdateBlobs] 
// 的响应消息。
type BatchUpdateBlobsResponse struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 请求的响应。
    Responses []*BatchUpdateBlobsResponse_Response `protobuf:"bytes,1,rep,name=responses,proto3" json:"responses,omitempty"`
}

// 对客户端尝试上传的单个 blob 的响应。
type BatchUpdateBlobsResponse_Response struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
    // 此响应对应的 blob 摘要。
    Digest *Digest `protobuf:"bytes,1,opt,name=digest,proto3" json:"digest,omitempty"`
    // 尝试上传该 blob 的结果。
    Status *status.Status `protobuf:"bytes,2,opt,name=status,proto3" json:"status,omitempty"`
}
```
## 接口
### 执行
```go
// ExecutionServer 提供远程执行操作的接口。
type ExecutionServer interface {
    // Execute 在远程执行一个操作。
    //
    // 为了执行一个操作，客户端必须首先上传所有的输入，包括
    // 要运行的 [Command][build.bazel.remote.execution.v2.Command]，
    // 以及 [Action][build.bazel.remote.execution.v2.Action] 到
    // [ContentAddressableStorage][build.bazel.remote.execution.v2.ContentAddressableStorage]。
    // 然后调用 `Execute`，并传递一个引用它们的 `action_digest`。
    // 服务器将运行该操作，并最终返回结果。
    //
    // 输入 `Action` 的字段必须符合文档中对其类型的规范化要求，
    // 以便它与其他逻辑上等效的 `Action` 具有相同的摘要。
    // 服务器可能会强制执行这些要求，并在接收到非规范化输入时返回错误。
    // 服务器也可能由于性能原因，不全部验证这些要求。
    // 如果服务器不验证要求，那么它会将哈希值不同的 `Action` 视为不同的逻辑操作。
    //
    // 返回描述执行结果的 [google.longrunning.Operation][google.longrunning.Operation] 消息流，
    // 最终的 `response` 为 [ExecuteResponse][build.bazel.remote.execution.v2.ExecuteResponse]。
    // 操作的 `metadata` 类型为 [ExecuteOperationMetadata][build.bazel.remote.execution.v2.ExecuteOperationMetadata]。
    //
    // 如果客户端在首次响应返回后保持连接，则会像客户端调用
    // [WaitExecution][build.bazel.remote.execution.v2.Execution.WaitExecution] 一样流式传输更新，
    // 直到执行完成或请求遇到错误。操作还可以使用 [Operations API][google.longrunning.Operations.GetOperation] 查询。
    //
    // 服务器不需要实现 Operations API 的其他方法或功能。
    //
    // 在创建 `Operation` 过程中发现的错误将作为 gRPC Status 错误报告，而在运行操作时发生的错误将报告在 `ExecuteResponse` 的 `status` 字段中。
    // 服务器不得设置 `Operation` proto 的 `error` 字段。可能的错误包括：
    //
    // * `INVALID_ARGUMENT`: 一个或多个参数无效。
    // * `FAILED_PRECONDITION`: 设置操作请求时发生一个或多个错误，例如缺少输入或命令，或没有可用的工作节点。客户端可能可以修复错误并重试。
    // * `RESOURCE_EXHAUSTED`: 运行操作时某些资源的配额不足。
    // * `UNAVAILABLE`: 由于暂时性情况（如所有工作节点被占用且服务器不支持排队），操作无法启动。客户端应该重试。
    // * `INTERNAL`: 执行引擎或工作节点发生内部错误。
    // * `DEADLINE_EXCEEDED`: 执行超时。
    // * `CANCELLED`: 操作被客户端取消。此状态仅当服务器实现了 Operations API 的 CancelOperation 方法，并对当前执行调用了该方法时可能发生。
    //
    // 在缺少输入或命令的情况下，服务器还应该发送 `PreconditionFailure` 错误细节，
    // 对于 CAS 中不存在的每个请求的 blob 都有一个 `Violation`，
    // 其 `type` 为 `MISSING`，`subject` 为 `"blobs/{digest_function/}{hash}/{size}"`，指示缺失 blob 的摘要。
    // `subject` 的格式与提供给 [ByteStream.Read][google.bytestream.ByteStream.Read] 的 `resource_name` 相同，省略前面的实例名称。如果 `digest_function` 的值为 MD5、MURMUR3、SHA1、SHA256、SHA384、SHA512 或 VSO，则必须省略 `digest_function`。
    //
    // 服务器不需要保证对该方法的调用最多导致一次操作执行。服务器可能会多次执行操作，可能是并行的。
    // 即使操作已完成，这些冗余执行可能会继续运行。
    Execute(*ExecuteRequest, Execution_ExecuteServer) error

    // WaitExecution 等待执行操作完成。当客户端初次发出请求时，服务器立即响应执行的当前状态。
    // 服务器将保持请求流打开直到操作完成，然后响应完成的操作。
    // 服务器可能选择在执行过程中流式传输其他更新，例如提供执行状态的更新。
    WaitExecution(*WaitExecutionRequest, Execution_WaitExecutionServer) error
}
```
Execute向客户端返回的是一个`google.longrunning.Operation`结构。
```protobuf
  rpc Execute(ExecuteRequest) returns (stream google.longrunning.Operation) {
    option (google.api.http) = { post: "/v2/{instance_name=**}/actions:execute" body: "*" };
  }
```
### 内容寻址存储
```go
// ContentAddressableStorageServer 是 ContentAddressableStorage 服务的服务器 API。
type ContentAddressableStorageServer interface {
    // 确定 CAS 中是否存在 blobs。
    //
    // 客户端可以在上传 blobs 之前使用此 API 来确定哪些 blobs 已经存在于 CAS 中，不需要再次上传。
    //
    // 如果需要且适用，服务器应该增加引用 blobs 的生存期。
    //
    // 没有方法特定的错误。
    FindMissingBlobs(context.Context, *FindMissingBlobsRequest) (*FindMissingBlobsResponse, error)

    // 一次性上传多个 blobs。
    //
    // 服务器可能会强制执行使用此 API 上传的 blobs 的总大小限制。这个限制可以通过
    // [Capabilities][build.bazel.remote.execution.v2.Capabilities] API 获取。
    // 超过限制的请求应该被拆分为更小的块，或使用
    // [ByteStream API][google.bytestream.ByteStream] 上传。
    //
    // 这个请求相当于并行地对每个独立 blob 调用 Bytestream `Write` 请求。
    // 请求可以独立地成功或失败。
    //
    // 错误：
    //
    // * `INVALID_ARGUMENT`: 客户端尝试上传超过服务器支持的限制。
    //
    // 个别请求可能会返回以下错误：
    //
    // * `RESOURCE_EXHAUSTED`: 磁盘配额不足，无法存储 blob。
    // * `INVALID_ARGUMENT`: 提供的数据与 [Digest][build.bazel.remote.execution.v2.Digest] 不匹配。
    BatchUpdateBlobs(context.Context, *BatchUpdateBlobsRequest) (*BatchUpdateBlobsResponse, error)

    // 一次性下载多个 blobs。
    //
    // 服务器可能会强制执行使用此 API 下载的 blobs 的总大小限制。这个限制可以通过
    // [Capabilities][build.bazel.remote.execution.v2.Capabilities] API 获取。
    // 超过限制的请求应该被拆分为更小的块，或使用
    // [ByteStream API][google.bytestream.ByteStream] 下载。
    //
    // 这个请求相当于并行地对每个独立 blob 调用 Bytestream `Read` 请求。
    // 请求可以独立地成功或失败。
    //
    // 错误：
    //
    // * `INVALID_ARGUMENT`: 客户端尝试读取超过服务器支持的限制。
    //
    // 每个读操作的所有错误都会在对应的摘要状态中返回。
    BatchReadBlobs(context.Context, *BatchReadBlobsRequest) (*BatchReadBlobsResponse, error)

    // 获取以某个节点为根的整个目录树。
    //
    // 此请求必须针对存储在
    // [ContentAddressableStorage][build.bazel.remote.execution.v2.ContentAddressableStorage]
    // (CAS) 中的 [Directory][build.bazel.remote.execution.v2.Directory]。
    // 服务器将递归地列举 `Directory` 树，并返回从根节点派生的每个节点。
    //
    // GetTreeRequest.page_token 参数可以用于跳过流中的前面部分（例如在重试部分完成和中止的请求时），
    // 通过将其设置为上次成功处理的 GetTreeResponse 的 GetTreeResponse.next_page_token 的值。
    //
    // 精确的遍历顺序是未指定的，除非从较早的请求中检索后续页面，否则无法保证在多个 `GetTree` 调用中保持稳定。
    //
    // 如果树的一部分在 CAS 中缺失，服务器将返回存在的部分并省略剩余部分。
    //
    // 错误：
    //
    // * `NOT_FOUND`: 请求的树根在 CAS 中不存在。
    GetTree(*GetTreeRequest, ContentAddressableStorage_GetTreeServer) error
}
```
## gRPC心跳
在 gRPC （Google Remote Procedure Call） 中，长连接的心跳机制是由 gRPC 底层库自动处理的。具体来说，gRPC 使用 HTTP/2 协议，该协议内置了持久连接和心跳机制。HTTP/2 协议本身支持长连接模式，并通过设置 PING 帧来进行连接的健康检查和保持活跃。
在使用 gRPC 时：

1. 长连接：

gRPC 客户端与服务器之间建立的连接是长连接，默认情况下会一直保持开启，除非客户端或服务器主动关闭连接。

2. 自动心跳：

gRPC 基于 HTTP/2 的连接层心跳机制，HTTP/2 会定期发送 PING 帧来检查连接的健康状况，确保连接是活跃的，且能够检测到潜在的连接中断。

3. 超时和重试：

连接超时和故障恢复机制是 gRPC 内置的特性，当检测到连接断开或无法响应时，gRPC 客户端会尝试自动重连。
## gRPC消息大小限制
gRPC 使用 Protocol Buffers（protobuf）来定义消息结构。一个 protobuf 消息可能包含**多个字段（不只是data）**，而 gRPC 在传输时将整个消息序列化为一个字节流。
grpc.MaxRecvMsgSize 和 grpc.MaxSendMsgSize 用于配置选项限制的是客户端或服务器在单个 RPC 调用中发送或接收的最大消息大小。默认消息大小为4MB。
