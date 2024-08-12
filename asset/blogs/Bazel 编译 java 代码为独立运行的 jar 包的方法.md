---
title: Bazel 编译 java 代码为独立运行的 jar 包的方法
date: 2024-08-12 10:40:20
toc: true
mathjax: true
tags: 
- bazel
- java
- 编译
---

以buildfarm v2.10.2 [https://github.com/bazelbuild/bazel-buildfarm](https://github.com/bazelbuild/bazel-buildfarm/releases) 为例。
根目录下存在`BUILD`文件，定义了构建规则。
```python
load("//:jvm_flags.bzl", "add_opens_sun_nio_fs", "ensure_accurate_metadata")

package(
    default_visibility = ["//src:__subpackages__"],
)

filegroup(
    name = "configs",
    srcs = ["logging.properties"],
    visibility = ["//visibility:public"],
)

java_binary(
    name = "buildfarm-server",
    classpath_resources = [
        ":configs",
    ],
    jvm_flags = ensure_accurate_metadata() + add_opens_sun_nio_fs(),
    main_class = "build.buildfarm.server.BuildFarmServer",
    visibility = ["//visibility:public"],
    runtime_deps = [
        "//src/main/java/build/buildfarm/server",
        "@maven//:org_slf4j_slf4j_simple",
    ],
)

java_binary(
    name = "buildfarm-shard-worker",
    classpath_resources = [
        ":configs",
    ],
    jvm_flags = ensure_accurate_metadata() + add_opens_sun_nio_fs(),
    main_class = "build.buildfarm.worker.shard.Worker",
    visibility = ["//visibility:public"],
    runtime_deps = [
        "//src/main/java/build/buildfarm/worker/shard",
        "@maven//:org_slf4j_slf4j_simple",
    ],
)
```
`java_binary`规则分别指定了`buildfarm-server`和`buildfarm-shard-worker`两个可运行程序。其中的`main_class`指定了main函数所在的位置。
但是，如果直接运行构建指令，如
`bazel build //src/main/java/build/buildfarm:buildfarm-shard-worker`
然后运行，会报错：
```shell
#java -jar bazel-bin/src/main/java/build/buildfarm/buildfarm-shard-worker.jar --jvm_flag=-Djava.util.logging.config.file=$(pwd)/examples/logging.properties $(pwd)/examples/config.minimal.yml
no main manifest attribute, in bazel-bin/src/main/java/build/buildfarm/buildfarm-shard-worker.jar
```
提示找不到主类，查看编译后的jar包，发现没有指定主类：
```shell
[root@j63e03474.sqa.eu95 /home/jiaomian.wjw/bazel-buildfarm/bazel-buildfarm-2.10.2/bazel-bin/src/main/java/build/buildfarm]
#unzip -p buildfarm-shard-worker.jar META-INF/MANIFEST.MF
Manifest-Version: 1.0
Created-By: singlejar
```
要想运行这样构建后的产物，需要通过bazel run指令。
而想要构建出能独立运行的软件包，**需要在构建指令的二进制包后加上后缀**`**_deploy.jar**`，这是一个约定。
```shell
#bazel build //src/main/java/build/buildfarm:buildfarm-shard-worker_deploy.jarINFO: Analyzed target //src/main/java/build/buildfarm:buildfarm-shard-worker_deploy.jar (0 packages loaded, 2 targets configured).
INFO: Found 1 target...
Target //src/main/java/build/buildfarm:buildfarm-shard-worker_deploy.jar up-to-date:
  bazel-bin/src/main/java/build/buildfarm/buildfarm-shard-worker_deploy.jar
INFO: Elapsed time: 0.763s, Critical Path: 0.51s
INFO: 3 processes: 2 internal, 1 linux-sandbox.
INFO: Build completed successfully, 3 total actions

[root@j63e03474.sqa.eu95 /home/jiaomian.wjw/bazel-buildfarm/bazel-buildfarm-2.10.2/bazel-bin/src/main/java/build/buildfarm]
#unzip -p buildfarm-shard-worker_deploy.jar META-INF/MANIFEST.MF
Manifest-Version: 1.0
Created-By: singlejar
Main-Class: build.buildfarm.worker.shard.Worker
Multi-Release: true
```
参考：[https://www.cnblogs.com/rongfengliang/p/12249593.html](https://www.cnblogs.com/rongfengliang/p/12249593.html)

附：手动编译jar包
```shell
bash-4.2$ javac -d bin src/com/alicloud/basic_tech/*.java
bash-4.2$ jar cvf mesh-executor.jar -C bin/ .
added manifest
adding: com/(in = 0) (out= 0)(stored 0%)
adding: com/alicloud/(in = 0) (out= 0)(stored 0%)
adding: com/alicloud/basic_tech/(in = 0) (out= 0)(stored 0%)
adding: com/alicloud/basic_tech/RMIServer.class(in = 923) (out= 570)(deflated 38%)
adding: com/alicloud/basic_tech/RemoteProcessBuilderImpl.class(in = 3947) (out= 1742)(deflated 55%)
adding: com/alicloud/basic_tech/RemoteProcessImpl.class(in = 4951) (out= 2292)(deflated 53%)
adding: com/alicloud/basic_tech/RemoteProcessBuilder.class(in = 914) (out= 351)(deflated 61%)
adding: com/alicloud/basic_tech/RemoteProcess.class(in = 412) (out= 252)(deflated 38%)
```