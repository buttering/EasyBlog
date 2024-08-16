---
title: BuildFarm Worker 配置自定义镜像
date: 2024-08-15 18:50:29
toc: true
mathjax: true
tags:
- buildfarm
- 构建系统
- docker
---

官方文档： [https://bazelbuild.github.io/bazel-buildfarm/docs/configuration/configuration/#all-configurations](https://bazelbuild.github.io/bazel-buildfarm/docs/configuration/configuration/#all-configurations) （不全）
## 步骤总结

1. 配置文件中开启flag
```yaml
worker:
  publicName: "localhost:8981"
  allowBringYourOwnContainer: true
```

2. 在action/command中指定镜像
```go
	// 上传 cmdProto
	cmdProto := re.Command{
		Arguments: r.args,
		EnvironmentVariables: []*re.Command_EnvironmentVariable{
			{Name: "PATH", Value: "/usr/bin:/usr/local/bin"}, // TODO(chao.shi): 可配置
		},
		Platform: &re.Platform{
			Properties: []*re.Platform_Property{
				{Name: "container-image", Value: "reg.docker.alibaba-inc.com/apsara-citest/janus-union:go1.20"},
			},
		},
	}
	for f := range r.outputs {
		cmdProto.OutputFiles = append(cmdProto.OutputFiles, f) // nolint:staticcheck
	}
	cmdDigest := uploadProto(r.conn, &cmdProto)
```
3.为镜像开启dind模式
## 参数解析过程
buldfarm 在ExecuteAction阶段，会判断是否指定了镜像。
```java
    if (allowBringYourOwnContainer && !limits.containerSettings.containerImage.isEmpty()) {
      // enable container execution
      limits.containerSettings.enabled = true;

      // Adjust additional flags for when a container is being used.
      adjustContainerFlags(limits);
    }
```
**首先在启动worker时指定的配置文件中开启flag**
```yaml
worker:
  publicName: "localhost:8981"
  allowBringYourOwnContainer: true
```
buildfarm2.6.1中，containerImage仍是由command proto指定的（自 v2.2 版本起已弃用，改为action指定）：
```java
  public static ResourceLimits Parse(Command command) {
    // Build parser for all exec properties
    Map<String, BiConsumer<ResourceLimits, Property>> parser = new HashMap<>();
    // ...
    parser.put(ExecutionProperties.CONTAINER_IMAGE, ExecutionPropertiesParser::storeContainerImage);
    // ...
    ResourceLimits limits = new ResourceLimits();
    command
        .getPlatform()
        .getPropertiesList()
        .forEach((property) -> evaluateProperty(parser, limits, property));
    return limits;
  }
```
**因此，需要在构造action时指定需要的image：**
```go
	// 上传 cmdProto
	cmdProto := re.Command{
		Arguments: r.args,
		EnvironmentVariables: []*re.Command_EnvironmentVariable{
			{Name: "PATH", Value: "/usr/bin:/usr/local/bin"}, // TODO(chao.shi): 可配置
		},
		Platform: &re.Platform{
			Properties: []*re.Platform_Property{
				{Name: "container-image", Value: "reg.docker.alibaba-inc.com/apsara-citest/janus-union:go1.20"},
			},
		},
	}
	for f := range r.outputs {
		cmdProto.OutputFiles = append(cmdProto.OutputFiles, f) // nolint:staticcheck
	}
	cmdDigest := uploadProto(r.conn, &cmdProto)
```
> 值的注意的是，remote execution api的对paltform的定义中，限定了`Platform.Properties.Name`只能为`OSFamily`和`ISA`（见其仓库的platform.md）。这里buildfarm作为官方实现，自行拓展了接口定义。因此，如果我们自己实现一个构建平台，不妨也进行拓展。

指定的镜像经过解析被赋给`ResourceLimits`对象中：
![image.png](https://intranetproxy.alipay.com/skylark/lark/0/2024/png/140156364/1720940168358-aaa23605-7ce7-4f07-80ad-4395b794c429.png#clientId=u4ebd36fd-0fd2-4&from=paste&height=306&id=uc080d1e6&originHeight=612&originWidth=1338&originalType=binary&ratio=2&rotation=0&showTitle=false&size=350287&status=done&style=none&taskId=u0ea32b3c-a646-4626-9d0a-1503e9ba352&title=&width=669)
解析到指定了镜像时，还会对构建行为作出调整：
```java
  private static void adjustLimits(
      ResourceLimits limits,
      Command command,
      String workerName,
      int defaultMaxCores,
      boolean onlyMulticoreTests,
      boolean limitGlobalExecution,
      int executeStageWidth,
      boolean allowBringYourOwnContainer,
      SandboxSettings sandbox) {
    // store worker name
    limits.workerName = workerName;

    // ...

    // Decide whether the action will run in a container
    if (allowBringYourOwnContainer && !limits.containerSettings.containerImage.isEmpty()) {
      // enable container execution
      limits.containerSettings.enabled = true;

      // Adjust additional flags for when a container is being used.
      adjustContainerFlags(limits);
    }

    // we choose to resolve variables after the other variable values have been decided
    resolveEnvironmentVariables(limits);
  }

private static void adjustContainerFlags(ResourceLimits limits) {
    // bazelbuild/bazel-toolchains provides container images that start with "docker://".
    // However, docker is unable to fetch images that have this as a prefix in the URI.
    // Our solution is to remove the prefix when we see it.
    // https://github.com/bazelbuild/bazel-buildfarm/issues/1060
    limits.containerSettings.containerImage =
        StringUtils.removeStart(limits.containerSettings.containerImage, "docker://");

    // Avoid using the existing execution policies when running actions under docker.
    // The programs used in the execution policies likely won't exist in the container images.
    limits.useExecutionPolicies = false;
    limits.description.add("configured execution policies skipped because of choosing docker");

    // avoid limiting resources as cgroups may not be available in the container.
    // in fact, we will use docker's cgroup settings explicitly.
    // TODO(thickey): use docker's cgroup settings given existing resource limitations.
    limits.cpu.limit = false;
    limits.mem.limit = false;
    limits.description.add("resource limiting disabled because of choosing docker");
  }
```
buildfarm如何使用docker image执行构建任务，见[https://aliyuque.antfin.com/g/cloudstorage/devops/ivayavkvokllqp9q/collaborator/join?token=Cs86jzRaXkabVs29&source=doc_collaborator#](https://aliyuque.antfin.com/g/cloudstorage/devops/ivayavkvokllqp9q/collaborator/join?token=Cs86jzRaXkabVs29&source=doc_collaborator#) 《BuildFarm Worker 简要分析》
## 镜像拉取
buildfarm每次执行任务都会运行下面的函数（如果指定了docker image）：
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

  /**
   * @brief Fetch the user requested image for running the action.
   * @details The image will not be fetched if it already exists.
   * @param dockerClient Client used to interact with docker.
   * @param imageName The name of the image to fetch.
   * @param fetchTimeout When to timeout on fetching the container image.
   */
  private static void fetchImageIfMissing(
      DockerClient dockerClient, String imageName, Duration fetchTimeout)
      throws InterruptedException {
    if (!isLocalImagePresent(dockerClient, imageName)) {
      dockerClient
          .pullImageCmd(imageName)
          .exec(new PullImageResultCallback())
          .awaitCompletion(fetchTimeout.getSeconds(), TimeUnit.SECONDS);
    }
  }

  /**
   * @brief Check to see if the image was already available.
   * @details Checking to see if the image is already available can avoid having to re-fetch it.
   * @param dockerClient Client used to interact with docker.
   * @param imageName The name of the image to check for.
   * @return Whether or not the container image is already present.
   * @note Suggested return identifier: isPresent.
   */
  private static boolean isLocalImagePresent(DockerClient dockerClient, String imageName) {
    // Check if image is already downloaded. Is this the most efficient way? It would be better to
    // not use exceptions for control flow.
    try {
      dockerClient.inspectImageCmd(imageName).exec();
    } catch (NotFoundException e) {
      return false;
    }
    return true;
  }
```
`fetchImageIfMissing`使用指定的`containerImage`拉取镜像。
如果使用完整连接，如`"reg.docker.alibaba-inc.com/rpmbuild7u2"`，且镜像不存在，dockerclient会到对应仓库下下载，这会花一段时间。
## dind
在实践中， worker常常被部署在容器中。为了支持自定义镜像，可以使用dind（dockr in docker）模式。
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bfworker
spec:
  replicas: 1
  selector:
    matchLabels:
      app: bfworker
  template:
    metadata:
      labels:
        app: bfworker
    spec:
      containers:
        - name: bfworker
          image: reg.docker.alibaba-inc.com/apsara-citest/buildmesh-worker:v0.2
          ports:
            - containerPort: 5005
            - containerPort: 2375
          securityContext:
            privileged: true
            runAsUser: 0  # 以 root 用户运行
          volumeMounts:  # 挂载docker
            - mountPath: /var/run/docker.sock
              name: docker-sock
            - mountPath: /var/lib/docker
              name: docker-lib
      volumes:
        - name: docker-sock
          hostPath:
            path: /var/run/docker.sock
        - name: docker-lib
          hostPath:
            path: /var/lib/docker
```
我使用的dockerfile如下，关键步骤是安装docker-ce和docker-ce-cli，同时启用2375端口：
```dockerfile
FROM reg.docker.alibaba-inc.com/apsara-citest/buildfarm-worker:v2.6.1 AS buildfarm-worker

# 使用 mag gcc9 系统镜像。这样方便测试编译 kuafu 和 ebs。
FROM reg.docker.alibaba-inc.com/mag/alios7u-gcc921-oldabi-x86_64:20230807

# mag 镜像中设置了 cc 默认是 mag-cc，而非 gcc。我们这里改为 gcc。同理 c++。
RUN ln -sf gcc /usr/bin/cc && ln -sf g++ /usr/bin/c++
ENV CC=gcc CXX=g++

# 从 buildfarm-worker 镜像中复制 /app 目录
COPY --from=buildfarm-worker /app /app

COPY ./run.sh /
COPY ./public/logging.properties ./public/config.yml /
COPY ./public/wait-for-it.sh /usr/local/bin/wait-for-it.sh

# 安装需要的工具，包括 OpenJDK 8、netcat 和一些 Docker 的依赖
RUN yum install -y java-1.8.0-openjdk-headless nmap-ncat && \
    chmod +x /run.sh /usr/local/bin/wait-for-it.sh && \
    yum install -y docker-ce docker-ce-cli -b test

# 创建 Docker 运行时目录
RUN mkdir -p /var/lib/docker

EXPOSE 8981 5005 2375

# 启动 Docker Daemon 并执行自定义脚本
CMD ["/run.sh"]
```
run.sh脚本中还需要启动dockerd
```shell
# 启动 Docker Daemon
dockerd --host=unix:///var/run/docker.sock --host=tcp://0.0.0.0:2375 &
```
## v2.10.2 疑似bug
如上所述决定worker是否使用镜像的关键值`limits.containerSettings.containerImage`，是根据传入的command决定的。
```java
  public static ResourceLimits Parse(Command command) {
    // Build parser for all exec properties
    Map<String, BiConsumer<ResourceLimits, Property>> parser = new HashMap<>();
    // ...
    parser.put(ExecutionProperties.CONTAINER_IMAGE, ExecutionPropertiesParser::storeContainerImage);
    // ...
    ResourceLimits limits = new ResourceLimits();
    command
        .getPlatform()
        .getPropertiesList()
        .forEach((property) -> evaluateProperty(parser, limits, property));
    return limits;
  }
```
在v2.10.2中，追溯调用链，可以看到如下代码：
```
OperationContext fetchedOperationContext =
    operationContext.toBuilder()
        .setExecDir(execDir)
        .setAction(action)
        .setCommand(command)
        .setTree(tree)
        .build();
boolean claimed = owner.output().claim(fetchedOperationContext);
```
根据re api定义，api 2.2版本后应使用action指定platform。按照该定义，此时command的platform为null。但是fetchedOperationContext在构建时，并未将action对象的platform复制到command对象中。这就导致了action指定的platfarm完全不能生效。
又尝试重新为command添加platform字段，结果是运行直接卡死。

## 附

buildfarm对自定义镜像对对实现是每当有新的构建任务都起一个新的容器，这样效率很低。可以修改相关代码，将自定义镜像改为sidecar容器来执行任务。

