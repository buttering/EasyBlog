---
title: BuildFarm Server 简要分析
date: 2024-08-15 18:48:29
toc: true
mathjax: true
tags:
- buildfarm
- remote execution api
- grpc
- 构建系统
---


## 框架
BuildaFarm Server使用Spring Boot框架，实现了构建系统的调度等功能。

- service/xxxxxService.java:  一系列的gRPC服务
- controller/WebController.java: 基于http服务提供了一些管理界面、健康检查、监控等内容服务。

利用 HTTP 服务的简单性来提供管理和监控接口，而使用 gRPC 来提供高性能和强类型的远程过程调用。

## 启动过程
gRPC初始化
```java
@PostConstruct
public void init() throws OptionsParsingException {
    try {
      start(
          // ServerBuilder用于构建gPRC服务器
          ServerBuilder.forPort(configs.getServer().getPort()),
          configs.getServer().getPublicName());
    } catch () {
      // 捕捉grpc相关异常
    }
}

// 使用 ServerBuilder 来配置和启动 gRPC 服务器
public synchronized void start(ServerBuilder<?> serverBuilder, String publicName)
  throws IOException, ConfigurationException, InterruptedException {
instance = createInstance();

ServerInterceptor headersInterceptor = new ServerHeadersInterceptor();
if (configs.getServer().getSslCertificatePath() != null
    && configs.getServer().getSslPrivateKeyPath() != null) {
  // There are different Public Key Cryptography Standards (PKCS) that users may format their
  // certificate files in.  By default, the JDK cannot parse all of them.  In particular, it
  // cannot parse PKCS #1 (RSA Cryptography Standard).  When enabling TLS for GRPC, java's
  // underlying Security module is used. To improve the robustness of this parsing and the
  // overall accepted certificate formats, we add an additional security provider. BouncyCastle
  // is a library that will parse additional formats and allow users to provide certificates in
  // an otherwise unsupported format.
  Security.addProvider(new BouncyCastleProvider());
  File ssl_certificate = new File(configs.getServer().getSslCertificatePath());
  File ssl_private_key = new File(configs.getServer().getSslPrivateKeyPath());
  serverBuilder.useTransportSecurity(ssl_certificate, ssl_private_key);
}

serverBuilder
    // 添加多个 gRPC 服务
    .addService(healthStatusManager.getHealthService())
    .addService(new ActionCacheService(instance))
    .addService(new CapabilitiesService(instance))
    .addService(new ContentAddressableStorageService(instance))
    .addService(new ByteStreamService(instance))
    .addService(new ExecutionService(instance, keepaliveScheduler))
    .addService(new OperationQueueService(instance))
    .addService(new OperationsService(instance))
    .addService(new AdminService(instance))
    .addService(new FetchService(instance))
    .addService(ProtoReflectionService.newInstance())
    .addService(new PublishBuildEventService())
    .intercept(TransmitStatusRuntimeExceptionInterceptor.instance())
    .intercept(headersInterceptor);
GrpcMetrics.handleGrpcMetricIntercepts(serverBuilder, configs.getServer().getGrpcMetrics());

// ...省略其他代码...
server = serverBuilder.build();
// 使http服务持有同样的实例
instance.start(publicName);
WebController.setInstance((ShardInstance) instance);
// 启动服务
server.start();
// ...省略其他代码...
}
```
## ExectionService.java
继承了ExecutionImplBase接口，是Remote Exection API Execution rpc的具体实现。
```java
public class ExecutionService extends ExecutionGrpc.ExecutionImplBase {}
```
主要功能是处理远程执行请求和监控操作状态，同时通过 KeepaliveWatcher 保持连接活跃，并根据配置发布度量指标。
###  成员变量
```java
private final Instance instance;  // 用于处理具体的执行操作。
private final long keepaliveAfter;  //用于定义发送 keepalive 消息的间隔时间，以确保长时间操作期间连接不会超时。
private final ScheduledExecutorService keepaliveScheduler;  // 用于调度 keepalive 任务。
private final MetricsPublisher metricsPublisher;  // 用于发布度量指标的对象，如 AWS、GCP 或日志。
private static BuildfarmConfigs configs = BuildfarmConfigs.getInstance();
```
### rpc服务实现
```java
@Override
//  客服端等待operationName指定的操作完成
public void waitExecution(
    // StreamObserver是 gRPC 客户端用于监听服务端响应的观察者
    WaitExecutionRequest request, StreamObserver<Operation> responseObserver) {
  String operationName = request.getName();

  ServerCallStreamObserver<Operation> serverCallStreamObserver =
      (ServerCallStreamObserver<Operation>) responseObserver;
  // 处理操作取消和回调
  withCancellation(
      serverCallStreamObserver,
      instance.watchOperation(
          operationName,
          // 创建一个 KeepaliveWatcher 实例，以便在操作期间发送 keepalive 消息
          createWatcher(serverCallStreamObserver, TracingMetadataUtils.fromCurrentContext())));
}

// 执行请求
@Override
public void execute(ExecuteRequest request, StreamObserver<Operation> responseObserver) {
  ServerCallStreamObserver<Operation> serverCallStreamObserver =
      (ServerCallStreamObserver<Operation>) responseObserver;
  try {
    RequestMetadata requestMetadata = TracingMetadataUtils.fromCurrentContext();
    withCancellation(
        serverCallStreamObserver,
        instance.execute(
            request.getActionDigest(),
            request.getSkipCacheLookup(),
            request.getExecutionPolicy(),
            request.getResultsCachePolicy(),
            requestMetadata,
            createWatcher(serverCallStreamObserver, requestMetadata)));
  } catch (InterruptedException e) {
    Thread.currentThread().interrupt();
  }
}
```
`instance.excute()`是实际的执行动作：

1. 验证动作摘要：确保请求的动作摘要有效。
2. 创建操作：生成并记录新的操作，加入**操作缓存**。
3. 动作执行观察：启动对操作的观察。
4. 缓存查找或直接执行：根据是否跳过缓存查找到决定执行路径。
5. 处理结果：为结果添加回调，更新操作状态。
### 工具函数
`withCancellation`函数处理操作的取消和回调。
```java
private void withCancellation(
    ServerCallStreamObserver<Operation> serverCallStreamObserver, ListenableFuture<Void> future) {
    // 添加回调
    addCallback(
      future,
      new FutureCallback<Void>() {
        // 检测取消状态
        boolean isCancelled() {
          return serverCallStreamObserver.isCancelled() || Context.current().isCancelled();
        }

        @Override

        // 处理完成操作
        public void onSuccess(Void result) {
          // 如果操作被取消，不再执行后续回调。
          if (!isCancelled()) {
            try {
              serverCallStreamObserver.onCompleted();
            } catch (Exception e) {
              onFailure(e);
            }
          }
        }

        @SuppressWarnings("NullableProblems")
        @Override
        // 处理失败操作
        public void onFailure(Throwable t) {
          if (!isCancelled() && !(t instanceof CancellationException)) {
            log.log(Level.WARNING, "error occurred during execution", t);
            serverCallStreamObserver.onError(Status.fromThrowable(t).asException());
          }
        }
      },
      Context.current().fixedContextExecutor(directExecutor()));
  // 设置取消处理器
  serverCallStreamObserver.setOnCancelHandler(() -> future.cancel(false));
}
```
`createWatcher`函数创建一个 KeepaliveWatcher 实例，用于保持连接的活跃状态。
Keepalive 机制是为了确保长时间运行的 gRPC 操作在客户端和服务器之间保持活跃状态，并避免在网络连接空闲时由于超时而断开连接。
```java
KeepaliveWatcher createWatcher(
    ServerCallStreamObserver<Operation> serverCallStreamObserver,
    RequestMetadata requestMetadata) {
  return new KeepaliveWatcher(serverCallStreamObserver) {
    @Override
    //  实现deliver抽象方法，通过定期发送 Operation 对象来保持连接活跃
    void deliver(Operation operation) {
      if (operation != null) {
        // 发布度量数据
        metricsPublisher.publishRequestMetadata(operation, requestMetadata);
      }
      // 将 Operation 对象发送给客户端
      serverCallStreamObserver.onNext(operation);
    }
  };
}
```
## FetchService.java
继承了`FetchImplBase` 接口，实现了`fetchBlob`和`fetchDirectory`两个方法，用于处理从**远程**来源获取二进制大对象（Blob）的任务。

1. 管理外部资源的下载：提供从外部 URI（如 HTTP URLs）下载 Blob 的功能，并将下载的 Blob 存储在内容地址存储（CAS，Content Addressable Storage）中。
2. 确保资源完整性：验证下载的 Blob 内容与预期的哈希值是否匹配，以确保资源的完整性和一致性。
3. 优化构建过程：通过预先下载构建所需的外部资源，减少构建过程中对外部依赖的实时请求，从而优化构建性能和可靠性。
### rpc服务实现
```java
@Override
// 获取blob
public void fetchBlob(
    FetchBlobRequest request, StreamObserver<FetchBlobResponse> responseObserver) {
  try {
    fetchBlob(instance, request, responseObserver);
  } catch (InterruptedException e) {
    Thread.currentThread().interrupt();
  }
}

@Override
// 获取目录，未实现
public void fetchDirectory(
    FetchDirectoryRequest request, StreamObserver<FetchDirectoryResponse> responseObserver) {
  log.log(
      Level.SEVERE,
      "fetchDirectory: "
          + request.toString()
          + ",\n metadata: "
          + TracingMetadataUtils.fromCurrentContext());
  // 告诉客户端未实现
  responseObserver.onError(Status.UNIMPLEMENTED.asException());
}
}
```
fetchBlob函数处理实际的Blob请求逻辑。根据请求是否带校验码：

- 存在校验码：从校验码生成摘要，判断摘要对应文件是否存在，如果存在则返回摘要。
- 无校验码：
```java
private void fetchBlob(
    Instance instance,
    FetchBlobRequest request,
    StreamObserver<FetchBlobResponse> responseObserver)
    throws InterruptedException {
  Digest expectedDigest = null;
  RequestMetadata requestMetadata = TracingMetadataUtils.fromCurrentContext();
  if (request.getQualifiersCount() == 0) {
    throw Status.INVALID_ARGUMENT.withDescription("Empty qualifier list").asRuntimeException();
  }
  //  处理请求带来的限定符
  for (Qualifier qualifier : request.getQualifiersList()) {
    String name = qualifier.getName();
    if (name.equals("checksum.sri")) {
      // 资源校验，利用校验码生成摘要
      expectedDigest = parseChecksumSRI(qualifier.getValue());
      Digest.Builder result = Digest.newBuilder();
      if (instance.containsBlob(expectedDigest, result, requestMetadata)) {
        responseObserver.onNext(
            // 如果匹配，将摘要返回给客户端
            FetchBlobResponse.newBuilder().setBlobDigest(result.build()).build());
        responseObserver.onCompleted();
        return;
      }
    } else {
      responseObserver.onError(
          Status.INVALID_ARGUMENT
              .withDescription(format("Invalid qualifier '%s'", name))
              .asException());
      return;
    }
  }
  if (expectedDigest == null) {
    responseObserver.onError(
        Status.INVALID_ARGUMENT
            .withDescription(format("Missing qualifier 'checksum.sri'"))
            .asException());
  } else if (request.getUrisCount() != 0) {
    // 处理URI列表并获取Blob文件
    addCallback(
        // 下载文件
        instance.fetchBlob(request.getUrisList(), expectedDigest, requestMetadata),
        new FutureCallback<Digest>() {
          @Override
          public void onSuccess(Digest actualDigest) {
            log.log(
                Level.INFO,
                format(
                    "fetch blob succeeded: %s inserted into CAS",
                    DigestUtil.toString(actualDigest)));
            responseObserver.onNext(
                FetchBlobResponse.newBuilder().setBlobDigest(actualDigest).build());
            responseObserver.onCompleted();
          }

          @SuppressWarnings("NullableProblems")
          @Override
          public void onFailure(Throwable t) {
            // handle NoSuchFileException
            log.log(Level.SEVERE, "fetch blob failed", t);
            responseObserver.onError(t);
          }
        },
        directExecutor());
  } else {
    responseObserver.onError(
        Status.INVALID_ARGUMENT.withDescription("Empty uris list").asRuntimeException());
  }
}
```
checksum.sri 是一种用来验证文件完整性的方法，特别适用于从网络上下载的文件。SRI 校验和通常采用包含哈希函数名和 Base64 编码的哈希值的格式：`<hash function>-<base64 encoded hash>`。例如`sha256-abcdef1234567890...`
### 工具函数
`blob.fetchBlob()`用于下载指定的文件，并生成对应的摘要。
```java
@Override
public ListenableFuture<Digest> fetchBlob(
    Iterable<String> uris, Digest expectedDigest, RequestMetadata requestMetadata) {
  ImmutableList.Builder<URL> urls = ImmutableList.builder();
  for (String uri : uris) {
    try {
      // 将uri转化为url
      urls.add(new URL(new java.net.URL(uri)));
    } catch (Exception e) {
      return immediateFailedFuture(e);
    }
  }
  return fetchBlobUrls(urls.build(), expectedDigest, requestMetadata);
}

@VisibleForTesting
ListenableFuture<Digest> fetchBlobUrls(
    Iterable<URL> urls, Digest expectedDigest, RequestMetadata requestMetadata) {
  for (URL url : urls) {
    Digest.Builder actualDigestBuilder = expectedDigest.toBuilder();
    try {
      // some minor abuse here, we want the download to set our built digest size as side effect
      // 下载url  
      downloadUrl(
          url,
          // Function<Long, OutputStream> 类型的匿名函数，在下载后被调用
          contentLength -> {
            // 验证文件大小
            Digest actualDigest = actualDigestBuilder.setSizeBytes(contentLength).build();
            if (expectedDigest.getSizeBytes() >= 0
                && expectedDigest.getSizeBytes() != contentLength) {
              throw new DigestMismatchException(actualDigest, expectedDigest);
            }
            return getBlobWrite(
                    Compressor.Value.IDENTITY, actualDigest, UUID.randomUUID(), requestMetadata)
                // 返回一个OutputStream对象，用于写入文件
                .getOutput(1, DAYS, () -> {});
          });
      // 返回下载的文件摘要
      return immediateFuture(actualDigestBuilder.build());
    } catch (Write.WriteCompleteException e) {
      return immediateFuture(actualDigestBuilder.build());
    } catch (Exception e) {
      log.log(Level.WARNING, "download attempt failed", e);
      // ignore?
    }
  }
  return immediateFailedFuture(new NoSuchFileException(expectedDigest.getHash()));
}
```
`downloadUrl`用于下载文件并写入输入流
```java
private static void downloadUrl(URL url, ContentOutputStreamFactory getContentOutputStream)
    throws IOException {
  // 建立http连接
  HttpURLConnection connection = (HttpURLConnection) url.openConnection();
  // connect timeout?
  // proxy?
  // authenticator?
  connection.setInstanceFollowRedirects(true);
  // request timeout?
  long contentLength = connection.getContentLengthLong();
  int status = connection.getResponseCode();

  if (status != HttpURLConnection.HTTP_OK) {
    String message = connection.getResponseMessage();
    // per docs, returns null if no valid string can be discerned
    // from the responses, i.e. invalid HTTP
    if (message == null) {
      message = "Invalid HTTP Response";
    }
    message = "Download Failed: " + message + " from " + url;
    throw new IOException(message);
  }

  // 建立输入流并写入输出流
  try (InputStream in = connection.getInputStream();
      OutputStream out = getContentOutputStream.create(contentLength)) {
    ByteStreams.copy(in, out);
  }
}
```
## ContentAddressableStorageService.java
继承了`ContentAddressableStorageGrpc.ContentAddressableStorageImplBase`，主要处理与存储和检索构建工件有关的操作，如查找、读取和更新 Blob，获取构建树等。
### rpc服务实现
查找缺失的Blobs、批量读取、获取目录树：
```java
  @Override
  public void findMissingBlobs(
      FindMissingBlobsRequest request, StreamObserver<FindMissingBlobsResponse> responseObserver) {
    instanceFindMissingBlobs(instance, request, responseObserver);
  }

  @Override
  public void batchReadBlobs(
      BatchReadBlobsRequest batchRequest, StreamObserver<BatchReadBlobsResponse> responseObserver) {
    batchReadBlobs(instance, batchRequest, responseObserver);
  }

  @Override
  public void getTree(GetTreeRequest request, StreamObserver<GetTreeResponse> responseObserver) {
    int pageSize = request.getPageSize();
    if (pageSize < 0) {
      responseObserver.onError(Status.INVALID_ARGUMENT.asException());
      return;
    }

    getInstanceTree(
        instance, request.getRootDigest(), request.getPageToken(), pageSize, responseObserver);
  }
```
重点看批量更新Blogs，实现了高效的批量异步 Blob 上传并处理其结果。
```java
  @Override
  public void batchUpdateBlobs(
      BatchUpdateBlobsRequest batchRequest,
      StreamObserver<BatchUpdateBlobsResponse> responseObserver) {
    BatchUpdateBlobsResponse.Builder response = BatchUpdateBlobsResponse.newBuilder();
    ListenableFuture<BatchUpdateBlobsResponse> responseFuture =
        // 将一个 ListenableFuture 转换为另一个 ListenableFuture
        transform(
            // 将多个 ListenableFuture 合并成一个 ListenableFuture
            allAsList(
                // 将一个 Spliterator 转换为顺序或并行的 Stream
                StreamSupport.stream(
                        // 生成多个上传 Blob 的 ListenableFuture
                        putAllBlobs(
                                instance,
                                batchRequest.getRequestsList(),
                                writeDeadlineAfter,
                                TimeUnit.SECONDS)
                            .spliterator(),
                        false)
                    // 将stream的值转化并收集到List中，transform用于添加每个bolb响应到response构建器中
                    .map((future) -> transform(future, response::addResponses, directExecutor()))
                    .collect(Collectors.toList())),
            (result) -> response.build(),
            directExecutor());

    addCallback(
        responseFuture,
        new FutureCallback<BatchUpdateBlobsResponse>() {
          @Override
          public void onSuccess(BatchUpdateBlobsResponse response) {
            responseObserver.onNext(response);
            responseObserver.onCompleted();
          }

          @SuppressWarnings("NullableProblems")
          @Override
          public void onFailure(Throwable t) {
            responseObserver.onError(t);
          }
        },
        directExecutor());
  }
```
`putAllBlobs`接受一组Blob上传请求，返回上传操作的`ListenableFuture`列表，列表每个元素表示一个Blob上传结果。
```java
private static Iterable<ListenableFuture<Response>> putAllBlobs(
    Instance instance,
    Iterable<Request> requests,
    long writeDeadlineAfter,
    TimeUnit writeDeadlineAfterUnits) {
    // 用于构建结果
    ImmutableList.Builder<ListenableFuture<Response>> responses = new ImmutableList.Builder<>();
    // 对每个请求获取其摘要，调用putBlobFuture
    for (Request request : requests) {
        Digest digest = request.getDigest();
        ListenableFuture<Digest> future =
        putBlobFuture(
            instance,
            Compressor.Value.IDENTITY,
            digest,
            request.getData(),
            writeDeadlineAfter,
            writeDeadlineAfterUnits,
            TracingMetadataUtils.fromCurrentContext());
        // 
        responses.add(
            // 将Blob上传任务转化为一个Response对象
            toResponseFuture(
                // 捕获可能的异常
                catching(
                    // 将future的结果映射为OK
                    transform(future, (d) -> Code.OK, directExecutor()),
                    Throwable.class,
                    (e) -> Status.fromThrowable(e).getCode(),
                    directExecutor()),
                digest));
    }
    return responses.build();
}
```
 `putBlobFutrue`将传入的data和digest上传，并返回异步的上传结果。
总结一下`batchUpdateBlobs`，其拥有批量上传多个来自请求的Blob，通过异步和回调处理多个上传请求，并将结果返回给客户端。
可以通过batchRequest的proto进行对照：
```protobuf
// 用于 [ContentAddressableStorage.BatchUpdateBlobs][build.bazel.remote.execution.v2.ContentAddressableStorage.BatchUpdateBlobs] 的请求消息。
message BatchUpdateBlobsRequest {
  
  // 客户端希望上传的单个 blob 的请求。
  message Request {
  
    // blob 的摘要。这必须是 `data` 的摘要。所有摘要必须使用相同的摘要函数。
    Digest digest = 1;
    
    // 原始二进制数据。
    bytes data = 2;
    
    // `data` 的格式。必须为 `IDENTITY`/未指定，或 [CacheCapabilities.supported_batch_compressors][build.bazel.remote.execution.v2.CacheCapabilities.supported_batch_compressors] 字段中列出的压缩器之一。
    Compressor.Value compressor = 3;
  }
  
  // 要操作的执行系统实例。服务器可能支持多个执行系统实例（每个都具有自己的工作节点、存储、缓存等）。
  // 服务器可能需要使用该字段以实现特定的选择方式，否则可以省略。
  string instance_name = 1;
  
  // 个体的上传请求。
  repeated Request requests = 2;
  
  // 用于计算上传的 blob 摘要的摘要函数。
  //
  // 如果使用的摘要函数是 MD5、MURMUR3、SHA1、SHA256、SHA384、SHA512 或 VSO 中的一个，客户端可以不设置此字段。
  // 在这种情况下，服务器应使用 blob 摘要哈希的长度和服务器功能中宣布的摘要函数来推断摘要函数。
  DigestFunction.Value digest_function = 5;
}
```
## 排队机制
Server收到exection请求后，会将对应的action生成一个longrunning.Operation。放入操作队列（Operation Queue）中，该队列会按顺序保存操作。Worker会主动拉取。
为支持操作执行的平台需求，可以设置多个队列，每个队列对应一组平台属性。如果操作未指定，则置于默认队列。
匹配算法在Server推送或Worker拉取操作的过程，用于找到合适的队列执行操作。
匹配算法的工作原理如下：按照配置顺序检查每个供应队列。选择并使用第一个符合条件的供应队列。在决定一个操作是否符合供应队列的条件时，会单独检查每个平台属性。默认情况下，每个键/值必须完全匹配。可以使用通配符（"*"）来避免完全匹配。

## AC
ActionCache 是一种服务，用于查询已定义的操作（Action）是否已执行，并在已执行的情况下下载其结果。该服务在远程执行 API 中定义，并需要 ContentAddressableStorage（CAS）服务来存储文件数据。
一个 Action 封装了执行所需的所有信息，包括：

- 操作命令
- 输入文件/目录树
- 环境变量
- 平台信息

这些信息用于计算 Action 的摘要（哈希值），确保多次执行相同的 Action 会产生相同的输出。Action 的哈希值用于缓存 ActionResult（操作结果），在操作完成后存储其结果和输出。
使用 GetActionResult 方法，通过 Action 的哈希值从缓存中查询并获取操作结果。
使用 UpdateActionResult 方法，可以将操作结果上传到缓存，无需执行服务即可实现。

Buildfarm 中，通过 Execution 服务完成操作后会自动填充ActionResult 。或由本地 Bazel 客户端通过 UpdateActionCache 方法上传。接收到 UpdateActionResultRequest 时，ActionCache 服务根据 instance_name 找到相应实例，并更新缓存中的操作结果。
Buildfarm 接收到 GetActionResultRequest 时，ActionCache 服务根据请求中的 instance_name 找到相应实例，并异步获取操作结果。
## CFC
Content Addressable Storage File Cache (CFC) 是所有 Buildfarm 工作节点（Workers）的核心组件。CFC 的主要功能是存储内容文件，并根据用于执行操作（Actions）的输入内容的摘要（Digests）进行索引。
支持操作：Read(digest, offset, limit), Write(id), FindMissingBlobs(digests), Batch Read/Write.
## CAS
内容可寻址存储（CAS）是一组服务端点，提供对不可变的二进制大型对象（blob）的读取和创建访问。核心服务在远程执行应用程序接口（Remote Execution API）中进行了声明，同时还要求提供字节流应用程序接口（ByteStream API），并对资源名称和行为进行了特殊化。
CAS 中的条目是一个字节序列，通过散列函数计算出的摘要构成了它的地址。地址可以通过[Digest]消息或字节流请求中的资源名称来指定。
### 读取
无论是通过 BatchReadBlobs 还是 ByteStream Read 方法访问，从 CAS 读取内容都是一个相对简单的过程。在 ByteStream Read 中读取的 resource_name 必须是"{instance_name}/blobs/{hash}/{size}"。
### 写入
向 CAS 中写入内容需要事先计算地址，并对所有内容选择摘要方法。可以使用 BatchUpdateBlobs 或 ByteStream Write 方法启动写入。ByteStream Write 资源名必须以 {instance_name}/uploads/{uuid}/blobs/{hash}/{size} 开头，并可在大小之后添加任何尾部文件名，以"/"分隔。尾部内容将被忽略。uuid 是客户端为给定写入生成的标识符，可在多个摘要中共享，但应严格限于客户端本地。
Buildfarm 以多种方式实现 CAS，包括为参考实现提供内存存储、作为外部 CAS 的代理、基于 bazel 中远程缓存实现的 HTTP/1 代理，以及作为 Worker 的持久化磁盘存储，为操作补充执行文件系统，并参与稀疏分片的分布式存储。

