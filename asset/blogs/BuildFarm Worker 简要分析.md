---
title: BuildFarm Worker 简要分析
date: 2024-08-15 18:49:29
toc: true
mathjax: true
categories:
- Remote Execution
tags:
- buildfarm
- 构建系统
---


> 仅关注任务队列和具体执行

worker 官方文档：[https://bazelbuild.github.io/bazel-buildfarm/docs/architecture/workers/](https://bazelbuild.github.io/bazel-buildfarm/docs/architecture/workers/)
## 启动
启动流程
```json
main() -> SpringApplication.run(Worker.class, args)
   -> @PostConstruct init() -> start()
      -> 检查和加载配置
      -> 初始化 DigestUtil、Backplane、CAS、ExecFileSystem
      -> 创建 gRPC Server 并启动
      -> 开始 Pipeline 和 Failsafe Registration

```
## backplane
backplane.start设置任务执行服务和 Redis 客户端，并启动订阅线程和 FAILSAFE 线程。
```java
    if (SHARD.equals(configs.getBackplane().getType())) {
      backplane =
          new RedisShardBackplane(identifier, this::stripOperation, this::stripQueuedOperation);
      backplane.start(configs.getWorker().getPublicName());
    } else {
      throw new IllegalArgumentException("Shard Backplane not set in config");
    }
```
```java
  @Override
  public void start(String clientPublicName) throws IOException {
    // Construct a single redis client to be used throughout the entire backplane.
    // We wish to avoid various synchronous and error handling issues that could occur when using
    // multiple clients.
    client = new RedisClient(jedisClusterFactory.get());
    // Create containers that make up the backplane
    state = DistributedStateCreator.create(client);

    if (configs.getBackplane().isSubscribeToBackplane()) {
      startSubscriptionThread();
    }
    if (configs.getBackplane().isRunFailsafeOperation()) {
      startFailsafeOperationThread();
    }

    // Record client start time
    client.call(
        jedis -> jedis.set("startTime/" + clientPublicName, Long.toString(new Date().getTime())));
  }
```
下面的函数启动对通道的监听线程，工作通道名为`"WorkerChannel"`，线程为`RedisShardSubscription`。
```java
  private void startSubscriptionThread() {
    ListMultimap<String, TimedWatchFuture> watchers =
        Multimaps.synchronizedListMultimap(
            MultimapBuilder.linkedHashKeys().arrayListValues().build());
    subscriberService = BuildfarmExecutors.getSubscriberPool();
    subscriber =
        new RedisShardSubscriber(
            watchers,
            storageWorkerSet,
            configs.getBackplane().getWorkerChannel(),
            subscriberService);

    operationSubscription =
        new RedisShardSubscription(
            subscriber,
            /* onUnsubscribe=*/ () -> {
              subscriptionThread = null;
              if (onUnsubscribe != null) {
                onUnsubscribe.runInterruptibly();
              }
            },
            /* onReset=*/ this::updateWatchedIfDone,
            /* subscriptions=*/ subscriber::subscribedChannels,
            client);

    // use Executors...
    subscriptionThread = new Thread(operationSubscription, "Operation Subscription");

    subscriptionThread.start();
  }
```
RedisShardSubscriber：负责订阅消息和处理通道消息。
RedisShardSubscription：处理订阅的生命周期，包括启动、停止和重置等。
在订阅的通道收到任务后，`RedisShardSubscription`负责调用相应的处理逻辑，其继承了`JedisPubSub`类，onMessage 方法处理从 Redis 频道接收到的消息。

- onWorkerMessage 和 onWorkerChange 方法处理工作节点变更消息，包括新增和移除节点。
- onOperationMessage 和 onOperationChange 方法处理操作变更消息，包括重置和过期操作。
```java
  @Override
  public void onMessage(String channel, String message) {
    if (channel.equals(workerChannel)) {
      onWorkerMessage(message);
    } else {
      onOperationMessage(channel, message);
    }
  }

  void onWorkerMessage(String message) {
    try {
      onWorkerChange(parseWorkerChange(message));
    } catch (InvalidProtocolBufferException e) {
      log.log(Level.INFO, format("invalid worker change message: %s", message), e);
    }
  }

  void onWorkerChange(WorkerChange workerChange) {
    switch (workerChange.getTypeCase()) {
      case TYPE_NOT_SET:
        log.log(
            Level.SEVERE,
            format(
                "WorkerChange oneof type is not set from %s at %s",
                workerChange.getName(), workerChange.getEffectiveAt()));
        break;
      case ADD:
        addWorker(workerChange.getName());
        break;
      case REMOVE:
        removeWorker(workerChange.getName());
        break;
    }
  }

   void onOperationMessage(String channel, String message) {
       try {
           onOperationChange(channel, parseOperationChange(message));
       } catch (InvalidProtocolBufferException e) {
           log.log(
               Level.INFO, format("invalid operation change message for %s: %s", channel, message), e);
       }
   }

   void onOperationChange(String channel, OperationChange operationChange) {
       switch (operationChange.getTypeCase()) {
           case TYPE_NOT_SET:
               log.log(
                   Level.SEVERE,
                   format(
                       "OperationChange oneof type is not set from %s at %s",
                       operationChange.getSource(), operationChange.getEffectiveAt()));
               break;
           case RESET:
               resetOperation(channel, operationChange.getReset());
               break;
           case EXPIRE:
               terminateExpiredWatchers(
                   channel,
                   toInstant(operationChange.getEffectiveAt()),
                   operationChange.getExpire().getForce());
               break;
       }
   }
```
看到这里，可以看出此处redis并没有用于实际action任务的分配，而是参与了worke调度和任务生命周期的轮转。这不是我们关心的。实际action任务涉及的代码还要进一步分析。
## pipeline
createServer方法中，定义了数个pipeline
```java
  private Server createServer(
      ServerBuilder<?> serverBuilder,
      ContentAddressableStorage storage,
      Instance instance,
      Pipeline pipeline,
      ShardWorkerContext context) {
    serverBuilder.addService(healthStatusManager.getHealthService());
    serverBuilder.addService(new ContentAddressableStorageService(instance));
    serverBuilder.addService(new ByteStreamService(instance));
    serverBuilder.addService(new ShutDownWorkerGracefully(this));

    // We will build a worker's server based on it's capabilities.
    // A worker that is capable of execution will construct an execution pipeline.
    // It will use various execution phases for it's profile service.
    // On the other hand, a worker that is only capable of CAS storage does not need a pipeline.
    if (configs.getWorker().getCapabilities().isExecution()) {
      PipelineStage completeStage =
          new PutOperationStage((operation) -> context.deactivate(operation.getName()));
      PipelineStage errorStage = completeStage; /* new ErrorStage(); */
      PipelineStage reportResultStage = new ReportResultStage(context, completeStage, errorStage);
      PipelineStage executeActionStage =
          new ExecuteActionStage(context, reportResultStage, errorStage);
      PipelineStage inputFetchStage =
          new InputFetchStage(context, executeActionStage, new PutOperationStage(context::requeue));
      PipelineStage matchStage = new MatchStage(context, inputFetchStage, errorStage);

      pipeline.add(matchStage, 4);
      pipeline.add(inputFetchStage, 3);
      pipeline.add(executeActionStage, 2);
      pipeline.add(reportResultStage, 1);

      serverBuilder.addService(
          new WorkerProfileService(
              storage, inputFetchStage, executeActionStage, context, completeStage, backplane));
    }
    GrpcMetrics.handleGrpcMetricIntercepts(serverBuilder, configs.getWorker().getGrpcMetrics());
    serverBuilder.intercept(new ServerHeadersInterceptor());

    return serverBuilder.build();
  }
```
最关键的任务执行阶段是 ExecuteActionStage，它负责实际的任务执行:
iterate是一个重写方法，负责从任务队列中取出任务，并分配给相应的执行器。
```java
  @Override
  protected void iterate() throws InterruptedException {
    OperationContext operationContext = take();
    ResourceLimits limits = workerContext.commandExecutionSettings(operationContext.command);
    Executor executor = new Executor(workerContext, operationContext, this);
    Thread executorThread = new Thread(() -> executor.run(limits), "ExecuteActionStage.executor");

    synchronized (this) {
      executors.add(executorThread);
      int slotUsage = executorClaims.addAndGet(limits.cpu.claimed);
      executionSlotUsage.set(slotUsage);
      logStart(operationContext.operation.getName(), getUsage(slotUsage));
      executorThread.start();
    }
  }
```
`take()`函数会返回一个操作上下文，其中包含了要执行的具体action对象以及command对象等，均为proto所定义的结构：
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/c039d34bd96972f017837a82e6337831/c5faa1301b1faacf93021bef02d890aa.png)
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/c039d34bd96972f017837a82e6337831/451b8837bcb99b7d6975bc9e390204fe.png)
然后在同步块中进行任务的实际执行。
梳理到这里，需要分别往上查看如何获取操作上下文和往下查看如何执行。
### 获取操作上下文
```java
  @Override
  public OperationContext take() throws InterruptedException {
    return takeOrDrain(queue);
  }
```
```java
  protected OperationContext takeOrDrain(BlockingQueue<OperationContext> queue)
      throws InterruptedException {
    boolean interrupted = false;
    InterruptedException exception;
    try {
      while (!isClosed() && !output.isClosed()) {
        OperationContext context = queue.poll(10, TimeUnit.MILLISECONDS);
        if (context != null) {
          return context;
        }
      }
      exception = new InterruptedException();
    } catch (InterruptedException e) {
      // only possible way to be terminated
      exception = e;
      // clear interrupted flag
      interrupted = Thread.interrupted();
    }
    waitForReleaseOrCatastrophe(queue);
    if (interrupted) {
      Thread.currentThread().interrupt();
    }
    throw exception;
  }
```
queue是ExecueteAction类的一个私有成员，是一个阻塞队列。
```java
  private final BlockingQueue<OperationContext> queue = new ArrayBlockingQueue<>(1);
```
该队列只有一处入队点：matchStage阶段返回的operationContext。
matchStage会不断从消息队列从尝试获取operationContext。
```java
  @Override
  protected void iterate() throws InterruptedException {
    // stop matching and picking up any works if the worker is in graceful shutdown.
    if (inGracefulShutdown) {
      return;
    }
    Stopwatch stopwatch = Stopwatch.createStarted();
    OperationContext operationContext = OperationContext.newBuilder().build();
    if (!output.claim(operationContext)) {
      return;
    }
    MatchOperationListener listener = new MatchOperationListener(operationContext, stoxpwatch);
    try {
      logStart();
      workerContext.match(listener);
    } finally {
      if (!listener.wasMatched()) {
        output.release();
      }
    }
  }
```
`workerContext`在此处注入的是`ShardWorkerContext`的实例，实际调用的match方法如下。
```java
  @Override
  public void match(MatchListener listener) throws InterruptedException {
    RetryingMatchListener dedupMatchListener =
        new RetryingMatchListener() {
          boolean matched = false;

          @Override
          public boolean getMatched() {
            return !matched;
          }

          @Override
          public void onWaitStart() {
            listener.onWaitStart();
          }

          @Override
          public void onWaitEnd() {
            listener.onWaitEnd();
          }

          @Override
          public boolean onEntry(QueueEntry queueEntry) throws InterruptedException {
            if (queueEntry == null) {
              matched = true;
              return listener.onEntry(null);
            }
            String operationName = queueEntry.getExecuteEntry().getOperationName();
            if (activeOperations.putIfAbsent(operationName, queueEntry) != null) {
              log.log(Level.WARNING, "matched duplicate operation " + operationName);
              return false;
            }
            matched = true;
            boolean success = listener.onEntry(queueEntry);
            if (!success) {
              requeue(operationName);
            }
            return success;
          }

          @Override
          public void onError(Throwable t) {
            Throwables.throwIfUnchecked(t);
            throw new RuntimeException(t);
          }

          @Override
          public void setOnCancelHandler(Runnable onCancelHandler) {
            listener.setOnCancelHandler(onCancelHandler);
          }
        };
    while (dedupMatchListener.getMatched()) {
      try {
        matchInterruptible(dedupMatchListener);
      } catch (IOException e) {
        throw Status.fromThrowable(e).asRuntimeException();
      }
    }
  }
```
该方法通过调用`matchInterruptible`方法从队列中获取任务，并通过 `dedupMatchListener` 进行处理。
`dedupMatchListener`的是一个`RetryingMatchListener`类，重写了onEntry函数，会在`MathListener`接收到一个新的实体时被调用。用于确保一个任务不会被匹配多次。
`matchInterruptible`方法会从Backplane中获取任务（QueueEntry实体）。
```java
  @SuppressWarnings("ConstantConditions")
  private void matchInterruptible(MatchListener listener) throws IOException, InterruptedException {
    listener.onWaitStart();
    QueueEntry queueEntry = null;
    try {
      queueEntry =
          backplane.dispatchOperation(
              configs.getWorker().getDequeueMatchSettings().getPlatform().getPropertiesList());
    } catch (IOException e) {
      // ....
      // transient backplane errors will propagate a null queueEntry
    }
    listener.onWaitEnd();

    if (queueEntry == null
        || DequeueMatchEvaluator.shouldKeepOperation(matchProvisions, queueEntry)) {
      listener.onEntry(queueEntry);
    } else {
      backplane.rejectOperation(queueEntry);
  // ...
  }
```
listener.onWaitStart()通知 MatchListener 任务匹配开始，接着调用backplane.dispatchOperation 方法尝试从任务队列中获取 QueueEntry。
```java
  @SuppressWarnings("ConstantConditions")
  @Override
  public QueueEntry dispatchOperation(List<Platform.Property> provisions)
      throws IOException, InterruptedException {
    return client.blockingCall(jedis -> dispatchOperation(jedis, provisions));
  }

  private @Nullable QueueEntry dispatchOperation(
      JedisCluster jedis, List<Platform.Property> provisions) throws InterruptedException {
    String queueEntryJson = state.operationQueue.dequeue(jedis, provisions);
    if (queueEntryJson == null) {
      return null;
    }

    QueueEntry.Builder queueEntryBuilder = QueueEntry.newBuilder();
    try {
      JsonFormat.parser().merge(queueEntryJson, queueEntryBuilder);
    } catch (InvalidProtocolBufferException e) {
      log.log(Level.SEVERE, "error parsing queue entry", e);
      return null;
    }
    QueueEntry queueEntry = queueEntryBuilder.build();

    String operationName = queueEntry.getExecuteEntry().getOperationName();
    Operation operation = keepaliveOperation(operationName);
    publishReset(jedis, operation);

    long requeueAt =
        System.currentTimeMillis() + configs.getBackplane().getDispatchingTimeoutMillis();
    DispatchedOperation o =
        DispatchedOperation.newBuilder().setQueueEntry(queueEntry).setRequeueAt(requeueAt).build();
    boolean success = false;
    try {
      String dispatchedOperationJson = JsonFormat.printer().print(o);

      /* if the operation is already in the dispatch list, fail the dispatch */
      success =
          state.dispatchedOperations.insertIfMissing(jedis, operationName, dispatchedOperationJson);
    } catch (InvalidProtocolBufferException e) {
      log.log(Level.SEVERE, "error printing dispatched operation", e);
      // very unlikely, printer would have to fail
    }

    if (success) {
      if (!state.operationQueue.removeFromDequeue(jedis, queueEntryJson)) {
        log.log(
            Level.WARNING,
            format(
                "operation %s was missing in %s, may be orphaned",
                operationName, state.operationQueue.getDequeueName()));
      }
      state.dispatchingOperations.remove(jedis, operationName);

      // Return an entry so that if it needs re-queued, it will have the correct "requeue attempts".
      return queueEntryBuilder.setRequeueAttempts(queueEntry.getRequeueAttempts() + 1).build();
    }
    return null;
  }

```
此处client为redisClient实例。`disPatchOperation`方法是从任务队列中获取任务的核心方法（`String queueEntryJson = state.operationQueue.dequeue(jedis, provisions);`。任务储存在Redis集群中。
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/c039d34bd96972f017837a82e6337831/42f2ebb860678071bba7b7dd2397c821.png)
在此可以确定，builfarm的默认实现确实是采用了redis作为action队列。之后只需确定使用的队列名称即可进行监控。
### 实际执行
回顾`ExecuteActionStage`的`iterate`方法, 此方法结合了线程管理、资源限制和任务执行，是整个类中最关键的部分:
```java
  @Override
  protected void iterate() throws InterruptedException {
    OperationContext operationContext = take();
    ResourceLimits limits = workerContext.commandExecutionSettings(operationContext.command);
    Executor executor = new Executor(workerContext, operationContext, this);
    Thread executorThread = new Thread(() -> executor.run(limits), "ExecuteActionStage.executor");

    synchronized (this) {
      executors.add(executorThread);
      int slotUsage = executorClaims.addAndGet(limits.cpu.claimed);
      executionSlotUsage.set(slotUsage);
      logStart(operationContext.operation.getName(), getUsage(slotUsage));
      executorThread.start();
    }
  }
```
其中的同步代码块用于线程管理和资源计数。`executors`是一个存放`Thread`对象的哈希集合。
`executor`对象负责执行实际的任务操作。`operationCopntext`在构造函数中被赋给一个私有final成员。其`run`方法被运行在一个新的线程中，是`Execute`类的核心。
```java
public void run(ResourceLimits limits) {
  long stallUSecs = 0;
  Stopwatch stopwatch = Stopwatch.createStarted();
  String operationName = operationContext.operation.getName();
  try {
    stallUSecs = runInterruptible(stopwatch, limits);
  } catch (InterruptedException e) {
  // ...
  } catch (Exception e) {
  // ...
  } finally {
    boolean wasInterrupted = Thread.interrupted();
    try {
      owner.releaseExecutor(operationName, limits.cpu.claimed, stopwatch.elapsed(MICROSECONDS), stallUSecs, exitCode);
    } finally {
      if (wasInterrupted) {
        Thread.currentThread().interrupt();
      }
    }
  }
}
```
`runInterruptible`方法管理整个任务的执行过程，包括状态更新、资源设置、计时和超时控制。
```java
  private long runInterruptible(Stopwatch stopwatch, ResourceLimits limits)
      throws InterruptedException {
    // ...
    Operation operation =
        operationContext
            .operation
            .toBuilder()
            .setMetadata(
                Any.pack(
                    ExecutingOperationMetadata.newBuilder()
                        .setStartedAt(startedAt)
                        .setExecutingOn(workerContext.getName())
                        .setExecuteOperationMetadata(executingMetadata)
                        .setRequestMetadata(
                            operationContext.queueEntry.getExecuteEntry().getRequestMetadata())
                        .build()))
            .build();

    boolean operationUpdateSuccess = false;
    try {
      operationUpdateSuccess = workerContext.putOperation(operation);
    } catch (IOException e) {}
    // ...
    try {
      return executePolled(operation, limits, policies, timeout, stopwatch);
    } finally {
      operationContext.poller.pause();
    }
  }
```
`executePolled`负责执行具体的任务，包括命令行的执行和结果处理。
该方法执行时会调用`executeCommand`方法，下面详细解释该方法：
#### executeCommand方法
**方法签名**
```java
@SuppressWarnings("ConstantConditions")
private Code executeCommand(  // 返回 gRPC 的 Code，表示执行状态码
    String operationName,  // 操作名称
    Path execDir,  //  执行目录，任务将在此目录中执行
    List<String> arguments,  // 命令行参数
    List<EnvironmentVariable> environmentVariables,  // 环境变量列表
    ResourceLimits limits,  // 资源限制，包括 CPU 和内存限制
    Duration timeout,  // 执行超时
    ActionResult.Builder resultBuilder)  // 构建任务执行结果
    throws IOException, InterruptedException {
```
`arguments`即为希望执行的具体指令：
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/c039d34bd96972f017837a82e6337831/ce71395143d921edf947651cc1607f48.png)
**初始化 ProcessBuilder 和环境变量**
```java
    ProcessBuilder processBuilder =
        new ProcessBuilder(arguments).directory(execDir.toAbsolutePath().toFile());

    Map<String, String> environment = processBuilder.environment();
    environment.clear();
    for (EnvironmentVariable environmentVariable : environmentVariables) {
      environment.put(environmentVariable.getName(), environmentVariable.getValue());
    }
    for (Map.Entry<String, String> environmentVariable :
        limits.extraEnvironmentVariables.entrySet()) {
      environment.put(environmentVariable.getKey(), environmentVariable.getValue());
    }
```
ProcessBuilder是 Java 中提供的一个用于创建和管理操作系统进程的类，能够执行外部命令和可执行文件，并与它们进行交互。首先清空了默认的环境变量。然后添加 environmentVariables 和 limits 中的额外环境变量。
最终环境变量：
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/c039d34bd96972f017837a82e6337831/1d14d244f52433d7ad51e8932d090c47.png)
**判断是否启用docker**
```java
if (limits.debugBeforeExecution) {  // 如果资源限制中启用了调试前执行，则执行相应调试操作。
  return ExecutionDebugger.performBeforeExecutionDebug(processBuilder, limits, resultBuilder);
}

if (limits.containerSettings.enabled) {
    // ...
}
```
如果资源限制中启用了 Docker 容器，则通过 Docker 执行命令。
> buildfarm 2.10.2更新以下代码

**判断是否持久化工作进程**
```java
boolean usePersistentWorker =
    !limits.persistentWorkerKey.isEmpty() && !limits.persistentWorkerCommand.isEmpty();

if (usePersistentWorker) {
  log.fine(
      "usePersistentWorker; got persistentWorkerCommand of : "
          + limits.persistentWorkerCommand);

  Tree execTree = operationContext.tree;

  WorkFilesContext filesContext =
      WorkFilesContext.fromContext(execDir, execTree, operationContext.command);

  return PersistentExecutor.runOnPersistentWorker(
      limits.persistentWorkerCommand,
      filesContext,
      operationName,
      ImmutableList.copyOf(arguments),
      ImmutableMap.copyOf(environment),
      limits,
      timeout,
      PersistentExecutor.defaultWorkRootsDir,
      resultBuilder);
}
```
UserPersistentWorker 是一种长时间运行的工作进程，用来执行重复性高、启动开销大的任务。其主要目的是通过保持工作进程的持续运行，减少任务的初始化和拆卸时间，从而提高构建系统的效率。
##### 不使用 docker image
未指定镜像，跳过，并继续执行函数。
最终，本次执行的上下文形式如图
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/c039d34bd96972f017837a82e6337831/fd879079ca2df7c30b561b9e27efdfc9.png)
**启动进程**
使用 ProcessUtils.threadSafeStart 启动进程，并捕获可能的 IOException 以进行错误处理。
如果启动失败，设置错误消息，并返回 INVALID_ARGUMENT 状态码。
```java
    long startNanoTime = System.nanoTime();
    Process process;
    try {
      process = ProcessUtils.threadSafeStart(processBuilder);
      process.getOutputStream().close();
    } catch (IOException e) {
      log.log(Level.SEVERE, format("error starting process for %s", operationName), e);
      resultBuilder.setExitCode(INCOMPLETE_EXIT_CODE);
      Throwable t = e.getCause();
      String message;
      if (t != null) {
        message = "Cannot run program \"" + processBuilder.command().get(0) + "\": " + t.getMessage();
      } else {
        message = e.getMessage();
      }
      resultBuilder.setStderrRaw(ByteString.copyFromUtf8(message));
      return Code.INVALID_ARGUMENT;
    }
```
**创建并启动读取线程来异步读取进程的输出流**
分别为标准输出（stdout）和错误输出（stderr）创建 ByteStringWriteReader 读取器。
```java
    final Write stdoutWrite = new NullWrite();
    final Write stderrWrite = new NullWrite();
    ByteStringWriteReader stdoutReader = new ByteStringWriteReader(process.getInputStream(), stdoutWrite, (int) workerContext.getStandardOutputLimit());
    ByteStringWriteReader stderrReader = new ByteStringWriteReader(process.getErrorStream(), stderrWrite, (int) workerContext.getStandardErrorLimit());
    Thread stdoutReaderThread = new Thread(stdoutReader, "Executor.stdoutReader");
    Thread stderrReaderThread = new Thread(stderrReader, "Executor.stderrReader");
    stdoutReaderThread.start();
    stderrReaderThread.start();
```
**等待进程完成或超时**
```java
    Code statusCode = Code.OK;
    boolean processCompleted = false;
    try {
      if (timeout == null) {
        exitCode = process.waitFor();
        processCompleted = true;
      } else {
        long timeoutNanos = timeout.getSeconds() * 1000000000L + timeout.getNanos();
        long remainingNanoTime = timeoutNanos - (System.nanoTime() - startNanoTime);
        if (process.waitFor(remainingNanoTime, TimeUnit.NANOSECONDS)) {
          exitCode = process.exitValue();
          processCompleted = true;
        } else {
          log.log(Level.INFO, format("process timed out for %s after %ds", operationName, timeout.getSeconds()));
          statusCode = Code.DEADLINE_EXCEEDED;
        }
      }
    } finally {
      if (!processCompleted) {
        process.destroy();
        int waitMillis = 1000;
        while (!process.waitFor(waitMillis, TimeUnit.MILLISECONDS)) {
          log.log(Level.INFO, format("process did not respond to termination for %s, killing it", operationName));
          process.destroyForcibly();
          waitMillis = 100;
        }
      }
    }
```
**读取进程的标准输出和错误输出**
```java
    ByteString stdout = ByteString.EMPTY;
    ByteString stderr = ByteString.EMPTY;
    try {
      stdoutReaderThread.join();
      stderrReaderThread.join();
      stdout = stdoutReader.getData();
      stderr = stderrReader.getData();
    } catch (Exception e) {
      log.log(Level.SEVERE, "error extracting stdout/stderr: ", e.getMessage());
    }
    resultBuilder.setExitCode(exitCode).setStdoutRaw(stdout).setStderrRaw(stderr);
```
##### 使用 docker image
指定了镜像，则进入`DockerExecutor.runActionWithDocker`执行。
```java
if (limits.containerSettings.enabled) {
  DockerClient dockerClient = DockerClientBuilder.getInstance().build();
  DockerExecutorSettings settings = new DockerExecutorSettings();
  settings.fetchTimeout = Durations.fromMinutes(1);
  settings.operationContext = operationContext;
  settings.execDir = execDir;
  settings.limits = limits;
  settings.envVars = environment;
  settings.timeout = timeout;
  settings.arguments = arguments;
  return DockerExecutor.runActionWithDocker(dockerClient, settings, resultBuilder);
}
```
如何指定docker见[https://aliyuque.antfin.com/g/cloudstorage/devops/gz4qlwgrw2uywiff/collaborator/join?token=xqbNIsKdMdR15gxn&source=doc_collaborator#](https://aliyuque.antfin.com/g/cloudstorage/devops/gz4qlwgrw2uywiff/collaborator/join?token=xqbNIsKdMdR15gxn&source=doc_collaborator#) 《BuildFarm Worker 配置自定义镜像》
`runActionWithDocker`是一个静态方法，使用传入的 Docker 客户端来运行一个构建任务。该方法会拉取必要的 Docker 镜像、启动容器、在容器内执行构建任务、提取执行结果并清理 Docker 资源。
```java
public class DockerExecutor {
  /**
   * @brief Run the action using the docker client and populate the results.
   * @details This will fetch any images as needed, spawn a container for execution, and clean up
   *     docker resources if requested.
   * @param dockerClient Client used to interact with docker.
   * @param settings Settings used to perform action execition.
   * @param resultBuilder The action results to populate.
   * @return Grpc code as to whether buildfarm was able to run the action.
   * @note Suggested return identifier: code.
   */
  public static Code runActionWithDocker(
      DockerClient dockerClient,
      DockerExecutorSettings settings,
      ActionResult.Builder resultBuilder)
      throws InterruptedException, IOException {
    String containerId = prepareRequestedContainer(dockerClient, settings);
    String execId = runActionInsideContainer(dockerClient, settings, containerId, resultBuilder);
    extractInformationFromContainer(dockerClient, settings, containerId, execId, resultBuilder);
    cleanUpContainer(dockerClient, containerId);
    return Code.OK;
  }
```
下面依次介绍被调用的四个方法：
`prepareRequestedContainer`方法负责设置 Docker 容器以便在容器内运行构建任务。这个方法确保 Docker 镜像已经拉取到本地，启动容器，并将必要的文件复制到容器内。
```java
  /**
   * @brief Setup the container for the action.
   * @details This ensures the image is fetched, the container is started, and that the container
   *     has proper visibility to the action's execution root. After this call it should be safe to
   *     spawn an action inside the container.
   * @param dockerClient Client used to interact with docker.
   * @param settings Settings used to perform action execition.
   * @return The ID of the started container.
   * @note Suggested return identifier: containerId.
   */
  private static String prepareRequestedContainer(
      DockerClient dockerClient, DockerExecutorSettings settings) throws InterruptedException {
    // this requires network access.  Once complete, "docker image ls" will show the downloaded
    // image
    fetchImageIfMissing(
        dockerClient, settings.limits.containerSettings.containerImage, settings.fetchTimeout);

    // build and start the container.  Once complete, "docker container ls" will show the started
    // container
    String containerId = createContainer(dockerClient, settings);
    dockerClient.startContainerCmd(containerId).exec();

    // copy files into it
    populateContainer(dockerClient, containerId, settings.execDir);

    // container is ready for running actions
    return containerId;
  }
```
`runActionInsideContainer`方法使用 execCreateCmd 和 execStartCmd 在容器内执行实际的构建任务，并返回 exec ID。
```java
  /**
   * @brief Assuming the container is already created and properly populated/mounted with data, this
   *     can be used to spawn an action inside of it.
   * @details The stdout / stderr of the action execution are populated to the results.
   * @param dockerClient Client used to interact with docker.
   * @param settings Settings used to perform action execition.
   * @param containerId The ID of the container.
   * @param resultBuilder The results to populate.
   * @return The ID of the container execution.
   * @note Suggested return identifier: execId.
   */
  private static String runActionInsideContainer(
      DockerClient dockerClient,
      DockerExecutorSettings settings,
      String containerId,
      ActionResult.Builder resultBuilder)
      throws InterruptedException {
    // decide command to run
    ExecCreateCmd execCmd = dockerClient.execCreateCmd(containerId);
    execCmd.withWorkingDir(settings.execDir.toAbsolutePath().toString());
    execCmd.withAttachStderr(true);
    execCmd.withAttachStdout(true);
    execCmd.withCmd(settings.arguments.toArray(new String[0]));
    String execId = execCmd.exec().getId();
    // execute command (capture stdout / stderr)
    ExecStartCmd execStartCmd = dockerClient.execStartCmd(execId);
    ByteArrayOutputStream out = new ByteArrayOutputStream();
    ByteArrayOutputStream err = new ByteArrayOutputStream();
    execStartCmd.exec(new ExecStartResultCallback(out, err)).awaitCompletion();
    // store results
    resultBuilder.setStdoutRaw(ByteString.copyFromUtf8(out.toString()));
    resultBuilder.setStderrRaw(ByteString.copyFromUtf8(err.toString()));

    return execId;
  }
```
`extractInformationFromContainer`方法使用 inspectExecCmd 提取执行任务后的结果，包括 stdout 和 stderr 信息。
```java
  /**
   * @brief Extract information from the container after the action ran.
   * @details This can include exit code, output artifacts, and various docker information.
   * @param dockerClient Client used to interact with docker.
   * @param settings Settings used to perform action execition.
   * @param containerId The ID of the container.
   * @param execId The ID of the execution.
   * @param resultBuilder The results to populate.
   */
  private static void extractInformationFromContainer(
      DockerClient dockerClient,
      DockerExecutorSettings settings,
      String containerId,
      String execId,
      ActionResult.Builder resultBuilder)
      throws IOException {
    extractExitCode(dockerClient, execId, resultBuilder);
    copyOutputsOutOfContainer(dockerClient, settings, containerId);
  }
```
`cleanUpContainer`方法使用 stopContainerCmd 和 removeContainerCmd 停止并删除容器，释放资源
```java
  /**
   * @brief Delete the container.
   * @details Forces container deletion.
   * @param dockerClient Client used to interact with docker.
   * @param containerId The ID of the container.
   */
  private static void cleanUpContainer(DockerClient dockerClient, String containerId) {
    try {
      dockerClient.removeContainerCmd(containerId).withRemoveVolumes(true).withForce(true).exec();
    } catch (Exception e) {
      log.log(Level.SEVERE, "couldn't shutdown container: ", e);
    }
  }
```
## 输入文件下载和输出文件目录预构建
在FetchInput阶段，随着函数调用`InputFetcher.run()->runInterruptibly()->fetchPolled()->ShardWorkerContext.createExecDir()->CFCExecFileSystem.createExecDir()`，最终会调用如下函数：
```java
  public Path createExecDir(
      String operationName, Map<Digest, Directory> directoriesIndex, Action action, Command command)
      throws IOException, InterruptedException {
    Digest inputRootDigest = action.getInputRootDigest();
    OutputDirectory outputDirectory =
        OutputDirectory.parse(
            command.getOutputFilesList(),
            concat(
                command.getOutputDirectoriesList(),
                realDirectories(directoriesIndex, inputRootDigest)),
            command.getEnvironmentVariablesList());

    Path execDir = root.resolve(operationName);
    if (Files.exists(execDir)) {
      Directories.remove(execDir, fileStore);
    }
    Files.createDirectories(execDir);

    ImmutableList.Builder<String> inputFiles = new ImmutableList.Builder<>();
    ImmutableList.Builder<Digest> inputDirectories = new ImmutableList.Builder<>();

    log.log(
        Level.FINER, "ExecFileSystem::createExecDir(" + operationName + ") calling fetchInputs");
    Iterable<ListenableFuture<Void>> fetchedFutures =
        fetchInputs(
            execDir,
            execDir,
            inputRootDigest,
            directoriesIndex,
            outputDirectory,
            key -> {
              synchronized (inputFiles) {
                inputFiles.add(key);
              }
            },
            inputDirectories);
// ...
  }
```
`execDir`记录了本次构建的根目录：
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/c039d34bd96972f017837a82e6337831/cab4647543ccbf36332d1358abe17286.png)
随后调用`fetchInputs`，在`execDir`内下载输入文件及创建输出目录。
```java
  private Iterable<ListenableFuture<Void>> fetchInputs(
      Path root,
      Path path,
      Digest directoryDigest,
      Map<Digest, Directory> directoriesIndex,
      OutputDirectory outputDirectory,
      Consumer<String> onKey,
      ImmutableList.Builder<Digest> inputDirectories)
      throws IOException {
    Directory directory = directoriesIndex.get(directoryDigest);
    if (directory == null) {
      // not quite IO...
      throw new IOException(
          "Directory " + DigestUtil.toString(directoryDigest) + " is not in directories index");
    }

    Iterable<ListenableFuture<Void>> downloads =
        directory.getFilesList().stream()
            .map(
                fileNode ->
                    catchingPut(
                        fileNode.getDigest(),
                        root,
                        path.resolve(fileNode.getName()),
                        fileNode.getIsExecutable(),
                        onKey))
            .collect(ImmutableList.toImmutableList());

    downloads =
        concat(
            downloads,
            directory.getSymlinksList().stream()
                .map(symlinkNode -> putSymlink(path, symlinkNode))
                .collect(ImmutableList.toImmutableList()));

    for (DirectoryNode directoryNode : directory.getDirectoriesList()) {
      Digest digest = directoryNode.getDigest();
      String name = directoryNode.getName();
      OutputDirectory childOutputDirectory =
          outputDirectory != null ? outputDirectory.getChild(name) : null;
      Path dirPath = path.resolve(name);
      if (childOutputDirectory != null || !linkInputDirectories) {
        Files.createDirectories(dirPath);
        downloads =
            concat(
                downloads,
                fetchInputs(
                    root,
                    dirPath,
                    digest,
                    directoriesIndex,
                    childOutputDirectory,
                    onKey,
                    inputDirectories));
      } else {
        downloads =
            concat(
                downloads,
                ImmutableList.of(
                    transform(
                        linkDirectory(dirPath, digest, directoriesIndex),
                        (result) -> {
                          // we saw null entries in the built immutable list without synchronization
                          synchronized (inputDirectories) {
                            inputDirectories.add(digest);
                          }
                          return null;
                        },
                        fetchService)));
      }
      if (Thread.currentThread().isInterrupted()) {
        break;
      }
    }
    return downloads;
  }
```
这个函数负责处理命令的输入输出。该方法遍历提供的 directoryDigest 对应的 Directory，处理其中的文件、符号链接和子目录。通过递归调用自己来处理子目录当处理子目录时，会检查每个目录是否需要存在，并确保其已创建。
**注意：buildfarmv2.6.1并未支持command.outputPath，需要在v2.7之后才能正确解析传入的command.outputPath。**
**见：**[**https://aliyuque.antfin.com/g/cloudstorage/devops/zzv5tra5rlk8xak6/collaborator/join?token=TgrjTlIzqde7uqHh&source=doc_collaborator#**](https://aliyuque.antfin.com/g/cloudstorage/devops/zzv5tra5rlk8xak6/collaborator/join?token=TgrjTlIzqde7uqHh&source=doc_collaborator#)** 《BuildFarm 低版本遇到的问题》**
## 优雅关闭
`config/Worker.java`类中有`gracefulShutdownSeconds`成员，通过配置文件可以给其赋值，默认为0。
其对应的代码如下
```java
public void prepareWorkerForGracefulShutdown() {
    if (configs.getWorker().getGracefulShutdownSeconds() == 0) {
        log.info(
            "Graceful Shutdown is not enabled. Worker is shutting down without finishing executions"
            + " in progress.");
    } else {
        inGracefulShutdown = true;
        log.info(
            "Graceful Shutdown - The current worker will not be registered again and should be"
            + " shutdown gracefully!");
        pipeline.stopMatchingOperations();
        int scanRate = 30; // check every 30 seconds
        int timeWaited = 0;
        int timeOut = configs.getWorker().getGracefulShutdownSeconds();
        try {
            if (pipeline.isEmpty()) {
                log.info("Graceful Shutdown - no work in the pipeline.");
            } else {
                log.info("Graceful Shutdown - waiting for executions to finish.");
            }
            while (!pipeline.isEmpty() && timeWaited < timeOut) {
                SECONDS.sleep(scanRate);
                timeWaited += scanRate;
                log.info(
                    String.format(
                        "Graceful Shutdown - Pipeline is still not empty after %d seconds.", timeWaited));
            }
        } catch (InterruptedException e) {
            log.info(
                "Graceful Shutdown - The worker gracefully shutdown is interrupted: " + e.getMessage());
        } finally {
            log.info(
                String.format(
                    "Graceful Shutdown - It took the worker %d seconds to %s",
                    timeWaited,
                    pipeline.isEmpty()
                    ? "finish all actions"
                    : "gracefully shutdown but still cannot finish all actions"));
        }
    }
}
```
 main 函数保证了上面的函数在程序被中断时一定会被调用
```java
  public static void main(String[] args) throws Exception {
    // 。。。
    try {
      worker.start();
      worker.awaitTermination();
    } catch (IOException e) {
      log.severe(formatIOError(e));
    } catch (InterruptedException e) {
      log.log(Level.WARNING, "interrupted", e);
    } finally {
      worker.stop();  // 最终调用prepareWorkerForGracefulShutdown()
    }
  }
```
