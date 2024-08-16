---
title: Java 代码编译为环境无关的可执行文件
date: 2024-08-15 18:53:29
toc: true
mathjax: true
tags:
- java
- 编译
---

## 问题背景
在分布式构建系统项目的研究过程中，为了实现任务能够在用户指定的自定义镜像中运行的目的，采用 sidecar 模式，用户自定义镜像作为 sidecar 容器，运行一个执行器程序，与 buildfarm worker 通过某种机制通信，执行发来的指令。
由于 buildfarm 是 java 程序，因此 sidecar 中的执行器采用 Java RMI 机制进行通信。
从易用性出发，用户需要对机制无感，因此，不能要求用户镜像预先安装Java环境。故而需要找到一种能将Java程序打包为独立运行文件的方法。
## 编译为 Fat JAR 包
通常的 JAR 包都仅包含项目的编译类和资源文件，而不包括项目所依赖的库文件。为了我们的目标，需要将依赖的库文件也进行打包。
可以利用 maven 插件进行
```xml
<build>
    <plugins>
        <plugin>
            <groupId>org.apache.maven.plugins</groupId>
            <artifactId>maven-shade-plugin</artifactId>
            <version>3.2.4</version>
            <executions>
                <execution>
                    <phase>package</phase>
                    <goals>
                        <goal>shade</goal>
                    </goals>
                </execution>
            </executions>
        </plugin>
    </plugins>
</build>
```
给`pom.xml`添加这个插件后，运行`mvn clean package`会在 target 目录中生成一个包含所有依赖的 JAR 文件。这里编译时JDK版本为22.0.2
这个文件可以在虚拟机中运行：`java -jar target/xxx.jar`
## jpackage 打包
编译后的 JAR 包依然依赖 JVM 环境。使用 jpackage 可以将 Java 应用程序和 JDK 一起打包为独立的可执行文件（如 .exe、.msi、.dmg、.deb 等），使得用户不需要单独安装 Java 环境也可以运行。
jpackage 在JDK-14之后被包含在其中。
运行如下指令：
```shell
jpackage \
    --input target \
    --name mesh-executor \
    --main-jar xxxl.jar \ 
    --main-class com.alicloud.basic_tech.RMIServer \
    --type app-image  # 会生成一个包含所有必要文件的目录。可以直接进入该目录并运行应用程序
```
会在当前目录生成 mesh-executor 目录。将这个目录拷贝到需要运行的环境即可，应用程序在 bin 子目录中。打包时JDK版本同样为22.0.2；
`--type`还可指定其他类型，我尝试过`rpm`，这种类型并没有收录其所有依赖，需要使用yum进行安装（yum自动解析依赖软件并下载）。


