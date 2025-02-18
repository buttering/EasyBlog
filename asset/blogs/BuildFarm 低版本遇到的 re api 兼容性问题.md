---
title: BuildFarm 低版本遇到的 re api 兼容性问题
date: 2024-08-12 10:38:20
toc: true
mathjax: true
categories:
- Remote Execution
tags: 
- buildfarm
- remote execution api
- 软件构建
---

## 问题描述
在尝试使用pcc连接buildfarm进行构建时，如构建参数如下图：
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/ea8e1bb9e17241565e7ed69e20704f09/d3f21dabc402e04f99319487b547d40f.png)
报错如下
```shell
/usr/bin/ld: cannot open output file apsara/jiaomian.wjw/pcc/pcc-mega/pcc/hello: No such file or directory
collect2: error: ld returned 1 exit status
```
直接原因是这里指定的生成文件带有前缀目录"apsara/.../pcc"，而构建过程中并未提奖创建该目录，因而报错。
## 临时解决方案
在构建指令前增加mkdir指令。需要客户端配合：
但是如果这样构建![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/ea8e1bb9e17241565e7ed69e20704f09/e8c7890ced0281fd99cdccc786656d13.png)会报错：
```shell
mkdir: invalid option -- 'o'
Try 'mkdir --help' for more information.
```
这是因为执行的容器把后面cc等也当成了mkdir的参数。
需要用sh -c 包裹多条指令，最终的构建指令如下：
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/ea8e1bb9e17241565e7ed69e20704f09/c3ad1185100fc089492a512b7da61399.png)
## 问题查找过程
经检查，在执行指令前，buildfarm会预先创建存放目标产物的文件夹，
通过调试，发现在InputFetch阶段，worker执行该函数后，文件夹被建立：
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/ea8e1bb9e17241565e7ed69e20704f09/f79043e18e6d617183c8008c7bd22475.png)
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/ea8e1bb9e17241565e7ed69e20704f09/d799ee05fc85ce7cf0622072554f9ce5.png)
通过探查代码，最终锁定处理输出目录的函数
核心代码如下：
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
    // ...
    Iterable<ListenableFuture<Void>> fetchedFutures =
        fetchInputs(
            outputDirectory,  // 其他参数略
);
```
而此处的`OutputDirectory.parse`函数会对**outputDir和outFiles**进行处理，而re api v2.1定义的outputPath**并未涉及**。
因此，在ExecteAction阶段，worker执行前述指令时，会出现`cannot open output file apsara/jiaomian.wjw/pcc/pcc-mega/pcc/hello: No such file or directory`这样的错误，因为本该在InputFile阶段建立的文件夹被忽略了。
此外，在ExecuteAction阶段，因为未处理outputPath，因此同样无法对目标产物进行收集。
```java
for (String outputFile : settings.operationContext.command.getOutputFilesList()) {
      Path outputPath = settings.execDir.resolve(outputFile);
      copyFileOutOfContainer(dockerClient, containerId, outputPath);
    }
    for (String outputDir : settings.operationContext.command.getOutputDirectoriesList()) {
      Path outputDirPath = settings.execDir.resolve(outputDir);
      outputDirPath.toFile().mkdirs();
    }
```
## 问题根源
对比如下两个command结构体：
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/ea8e1bb9e17241565e7ed69e20704f09/b641e3feb5ef02d0d492ca300a03f406.png)
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/ea8e1bb9e17241565e7ed69e20704f09/4c1d0b8c2d2c432922ffccfaeb13e05e.png)
前者出现了前述错误，而后者能够正常执行。
因此，这是一个buildfarm版本和re api版本不兼容，导致的错误。
旧版本的builfarm仅适配了outputFiles和outputDirectories，而这两个字段在re api v2.1被废弃，改为了outPaths。该变更在buildfarm2.7开始支持，见issue：[https://github.com/bazelbuild/bazel-buildfarm/pull/1511](https://github.com/bazelbuild/bazel-buildfarm/pull/1511)
buildfarm v2.10.2 对应变更:
```java
  @Override
  public Path createExecDir(
      String operationName, Map<Digest, Directory> directoriesIndex, Action action, Command command)
      throws IOException, InterruptedException {
    Digest inputRootDigest = action.getInputRootDigest();
    OutputDirectory outputDirectory = createOutputDirectory(command);
    // ...
  }

@VisibleForTesting
  static OutputDirectory createOutputDirectory(Command command) {
    Iterable<String> files;
    Iterable<String> dirs;
    if (command.getOutputPathsCount() != 0) {
      files = command.getOutputPathsList();
      dirs = ImmutableList.of(); // output paths require the action to create their own directory
    } else {
      files = command.getOutputFilesList();
      dirs = command.getOutputDirectoriesList();
    }
    if (!command.getWorkingDirectory().isEmpty()) {
      files = Iterables.transform(files, file -> command.getWorkingDirectory() + "/" + file);
      dirs = Iterables.transform(dirs, dir -> command.getWorkingDirectory() + "/" + dir);
    }
    return OutputDirectory.parse(files, dirs, command.getEnvironmentVariablesList());
  }
```
## 解决办法
升级buildfarm。

