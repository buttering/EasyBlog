---
title: Bazel 报错：/tmp/external/gcc_toolchain_x86_64_files/bin/x86_64-linux-gcc： No such file or directory
date: 2024-06-20 15:27:42
toc: true
mathjax: true
tags: 
- bazel
- cpp
- 编译
- 软件构建
---


在测试bazel编译cpp项目时，遇到一个问题：
```shell
$bazel build //main:hello-world
Starting local Bazel server and connecting to it...
WARNING: --enable_bzlmod is set, but no MODULE.bazel file was found at the workspace root. Bazel will create an empty MODULE.bazel file. Please consider migrating your external dependencies from WORKSPACE to MODULE.bazel. For more details, please refer to https://github.com/bazelbuild/bazel/issues/18958.
INFO: Analyzed target //main:hello-world (69 packages loaded, 6450 targets configured).
ERROR: /apsara/xumiaoyong.xmy/bazel-example/cpp-tutorial/stage2/main/BUILD:3:11: Compiling main/hello-greet.cc failed: (Exit 1): gcc failed: error executing CppCompile command (from target //main:hello-greet) external/gcc_toolchain_x86_64/bin/gcc -fstack-protector -Wall -Wunused-but-set-parameter -Wno-free-nonheap-object -fno-omit-frame-pointer '-std=c++0x' -MD -MF ... (remaining 28 arguments skipped)

Use --sandbox_debug to see verbose messages from the sandbox and retain the sandbox build root for debugging
external/gcc_toolchain_x86_64/bin/gcc: line 45: /tmp/external/gcc_toolchain_x86_64_files/bin/x86_64-linux-gcc: No such file or directory
ERROR: /apsara/xumiaoyong.xmy/bazel-example/cpp-tutorial/stage2/main/BUILD:9:10: Compiling main/hello-world.cc failed: (Exit 1): gcc failed: error executing CppCompile command (from target //main:hello-world) external/gcc_toolchain_x86_64/bin/gcc -fstack-protector -Wall -Wunused-but-set-parameter -Wno-free-nonheap-object -fno-omit-frame-pointer '-std=c++0x' -MD -MF ... (remaining 32 arguments skipped)

Use --sandbox_debug to see verbose messages from the sandbox and retain the sandbox build root for debugging
external/gcc_toolchain_x86_64/bin/gcc: line 45: /tmp/external/gcc_toolchain_x86_64_files/bin/x86_64-linux-gcc: No such file or directory
Target //main:hello-world failed to build
Use --verbose_failures to see the command lines of failed build steps.
INFO: Elapsed time: 78.477s, Critical Path: 0.22s
INFO: 6 processes: 6 internal.
ERROR: Build did NOT complete successfully
```
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/fc966805db49ccdcada60c563834b7ff/06afef71b9888296c0367bcd9eafc946.png)<br />很神奇的是，曾经对该项目的编译是能正常进行的，而在未修改任何代码的情况下，这个错误出现在了某次编译（和之后）。

在github上找到一个issue：[https://github.com/bazelbuild/bazel/issues/20533](https://github.com/bazelbuild/bazel/issues/20533)<br />按照讨论，在指令后添加`--sandbox_add_mount_pair=/tmp`指令能修复这个问题。
```shell
$bazel build //main:hello-world  --sandbox_add_mount_pair=/tmp
INFO: Analyzed target //main:hello-world (0 packages loaded, 0 targets configured).
INFO: Found 1 target...
Target //main:hello-world up-to-date:
  bazel-bin/main/hello-world
INFO: Elapsed time: 1.001s, Critical Path: 0.81s
INFO: 3 processes: 1 internal, 2 linux-sandbox.
INFO: Build completed successfully, 3 total actions
```
