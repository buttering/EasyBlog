---
title: 使用docker安装joern
date: 2025-02-18 15:27:42
toc: true
mathjax: true
tags: 
- docker
- joern
- 工具安装
---

# 使用docker安装joern

下载项目并构建镜像：

```shell
git pull https://github.com/joernio/joern.git
cd joern
docker build -t joern .
docker run 
```

如果docker 仓库下载失败

```shell
❯ docker build  -t joern .
[+] Building 5.6s (8/9)                                                                                                                                                                                                               docker:default
 => [internal] load build definition from Dockerfile                                                                                                                                                                                            0.0s
 => => transferring dockerfile: 542B                                                                                                                                                                                                            0.0s
 => WARN: LegacyKeyValueFormat: "ENV key=value" should be used instead of legacy "ENV key value" format (line 8)                                                                                                                                0.0s
 => WARN: LegacyKeyValueFormat: "ENV key=value" should be used instead of legacy "ENV key value" format (line 9)                                                                                                                                0.0s
 => WARN: LegacyKeyValueFormat: "ENV key=value" should be used instead of legacy "ENV key value" format (line 10)                                                                                                                               0.0s
 => [internal] load metadata for docker.io/library/alpine:latest                                                                                                                                                                                0.0s
 => [internal] load .dockerignore                                                                                                                                                                                                               0.0s
 => => transferring context: 2B                                                                                                                                                                                                                 0.0s
 => [1/6] FROM docker.io/library/alpine:latest                                                                                                                                                                                                  0.0s
 => CACHED [2/6] RUN apk update && apk upgrade && apk add --no-cache openjdk17-jdk python3 git curl gnupg bash nss ncurses php                                                                                                                  0.0s
 => CACHED [3/6] RUN ln -sf python3 /usr/bin/python                                                                                                                                                                                             0.0s
 => CACHED [4/6] RUN curl -sL "https://github.com/sbt/sbt/releases/download/v1.10.3/sbt-1.10.3.tgz" | gunzip | tar -x -C /usr/local                                                                                                             0.0s
 => ERROR [5/6] RUN git clone https://github.com/joernio/joern && cd joern && sbt stage                                                                                                                                                         5.5s
------
 > [5/6] RUN git clone https://github.com/joernio/joern && cd joern && sbt stage:
0.381 Cloning into 'joern'...
5.426 fatal: unable to access 'https://github.com/joernio/joern/': TLS connect error: error:00000000:lib(0)::reason(0)
------

 3 warnings found (use docker --debug to expand):
 - LegacyKeyValueFormat: "ENV key=value" should be used instead of legacy "ENV key value" format (line 8)
 - LegacyKeyValueFormat: "ENV key=value" should be used instead of legacy "ENV key value" format (line 9)
 - LegacyKeyValueFormat: "ENV key=value" should be used instead of legacy "ENV key value" format (line 10)
Dockerfile:14
--------------------
  12 |
  13 |     # building joern
  14 | >>> RUN git clone https://github.com/joernio/joern && cd joern && sbt stage
  15 |     WORKDIR /joern
  16 |
--------------------
ERROR: failed to solve: process "/bin/sh -c git clone https://github.com/joernio/joern && cd joern && sbt stage" did not complete successfully: exit code: 128
```

这是docker中git pull失败，我们修改`DOCKERFILE`文件，改为使用本地已经下载好的项目：

```dockerfile
FROM alpine:latest

# dependencies
RUN apk update && apk upgrade && apk add --no-cache openjdk17-jdk python3 git curl gnupg bash nss ncurses php
RUN ln -sf python3 /usr/bin/python

# sbt
ENV SBT_VERSION 1.10.3
ENV SBT_HOME /usr/local/sbt
ENV PATH ${PATH}:${SBT_HOME}/bin
RUN curl -sL "https://github.com/sbt/sbt/releases/download/v$SBT_VERSION/sbt-$SBT_VERSION.tgz" | gunzip | tar -x -C /usr/local

# building joern
COPY . /joern  # 使用本地项目
RUN cd joern && sbt stage
WORKDIR /joern
```

成功打包镜像后，就可以运行了：

```shell
docker run --rm -it -v /tmp:/tmp -v path_to_java_project:/app:rw -t joern ./joern
```

把`path_to_java_projecth`换成你需要转换的项目路径

比如我从defect4j中导出一个项目：

```shell
defects4j checkout -p Lang -v 1 -w /tmp/lang_1_buggy
```

那么可以这样启动镜像：

```shell
 docker run --rm -it -p 8081:8081 -v /tmp:/tmp -v /tmp/lang_1_buggy:/app:rw -t joern ./joern
```

进入后，使用`importCode` 指令载入特定项目。

![image-20241231151403504](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/87868a97442dd2075413f3b9cbf9194c/6bb289f83b36e8b4c4724a5942b8a1b1.png)

