---
title: find 查找 Bazel 构建覆盖率文件的一个☝️坑
date: 2024-06-20 15:29:59
toc: true
mathjax: true
tags: 
- bazel
- linux
- 覆盖率
---

> **Bazel** 是由 Google 开发的一个高效、可扩展的开源构建和测试工具，主要用于管理大型代码库。它支持多语言（如 C++, Java, Python）和多平台构建，通过强大的构建缓存和并行化机制显著提高构建速度。

## 背景
利用Bazel构建项目时，想要通过它生成覆盖率报告，其中有一个关键文件，是在构建项目时一并生成的gcno类型的文件。但是，利用find查找却无法找到。
```shell
# 设定参数生成覆盖率文件
$bazel build --copt=--coverage --linkopt=-lgcov --spawn_strategy=standalone --genrule_strategy=standalone --collect_code_coverage   //main:hello_test
DEBUG: Rule 'gtest' indicated that a canonical reproducible form can be obtained by modifying arguments commit = "58d77fa8070e8cec2dc1ed015d66b454c8d78850" and dropping ["branch"]
DEBUG: Repository gtest instantiated at:
  /apsara/jiaomian.wjw/bazel-example/examples-main/cpp-tutorial/stage-test/WORKSPACE:2:15: in <toplevel>
Repository rule git_repository defined at:
  /apsara/jiaomian.wjw/.cache/bazel/_bazel_jiaomian.wjw/85047200f6c215149976ede38a6439b5/external/bazel_tools/tools/build_defs/repo/git.bzl:189:33: in <toplevel>
INFO: Analyzed target //main:hello_test (77 packages loaded, 1220 targets configured).
INFO: Found 1 target...
Target //main:hello_test up-to-date:
  bazel-bin/main/hello_test
INFO: Elapsed time: 1815.686s, Critical Path: 5.57s
INFO: 29 processes: 10 internal, 19 local.
INFO: Build completed successfully, 29 total actions

$find . -name "*.gcno"
# 寻找不到任何东西
```
尝试了各种办法，看了各种文档教程，无论是折腾参数还是修改`.bazelrc`文件，都无法生成gcno文件。
当然，gcda也查不到。
## 排查
在保存构建产物的过程中，进行了将产物重定向到build目录的操作，此时在build下查找gcda，竟然找到了。
```shell
$find -name '*.gcda'
./external/gtest/_objs/gtest/gmock-internal-utils.pic.gcda
./external/gtest/_objs/gtest/gmock-cardinalities.pic.gcda
./external/gtest/_objs/gtest/gtest-typed-test.pic.gcda
./external/gtest/_objs/gtest/gtest-matchers.pic.gcda
./external/gtest/_objs/gtest/gtest-filepath.pic.gcda
./external/gtest/_objs/gtest/gtest-assertion-result.pic.gcda
./external/gtest/_objs/gtest/gtest-printers.pic.gcda
./external/gtest/_objs/gtest/gmock-matchers.pic.gcda
./external/gtest/_objs/gtest/gtest-port.pic.gcda
./external/gtest/_objs/gtest/gtest-death-test.pic.gcda
./external/gtest/_objs/gtest/gmock-spec-builders.pic.gcda
./external/gtest/_objs/gtest/gmock.pic.gcda
./external/gtest/_objs/gtest/gtest-test-part.pic.gcda
./external/gtest/_objs/gtest/gtest.pic.gcda
./external/gtest/_objs/gtest_main/gmock_main.pic.gcda
./main/_objs/hello_test/hello_test.pic.gcda
```
结合bazel文档提到，它的产物是以符号连接的形式生成在工作区（WORDKSPACE）中的。
到项目根目录查看内容：
```shell
[jiaomian.wjw@j63e03474.sqa.eu95 /apsara/jiaomian.wjw/bazel-example/examples-main/cpp-tutorial/stage-test]
$ls -l
total 172
lrwxrwxrwx 1 jiaomian.wjw users    128 Jun  5 20:12 bazel-bin -> /apsara/jiaomian.wjw/.cache/bazel/_bazel_jiaomian.wjw/85047200f6c215149976ede38a6439b5/execroot/_main/bazel-out/k8-fastbuild/bin
lrwxrwxrwx 1 jiaomian.wjw users    111 Jun  5 20:12 bazel-out -> /apsara/jiaomian.wjw/.cache/bazel/_bazel_jiaomian.wjw/85047200f6c215149976ede38a6439b5/execroot/_main/bazel-out
lrwxrwxrwx 1 jiaomian.wjw users    101 Jun  5 20:12 bazel-stage-test -> /apsara/jiaomian.wjw/.cache/bazel/_bazel_jiaomian.wjw/85047200f6c215149976ede38a6439b5/execroot/_main
lrwxrwxrwx 1 jiaomian.wjw users    133 Jun  5 20:12 bazel-testlogs -> /apsara/jiaomian.wjw/.cache/bazel/_bazel_jiaomian.wjw/85047200f6c215149976ede38a6439b5/execroot/_main/bazel-out/k8-fastbuild/testlogs
drwxr-xr-x 2 jiaomian.wjw users   4096 Jun  5 09:50 main
-rw-r--r-- 1 jiaomian.wjw users    399 Jun  5 19:40 MODULE.bazel
-rw-r--r-- 1 jiaomian.wjw users 145955 Jun  5 20:12 MODULE.bazel.lock
-rw-r--r-- 1 jiaomian.wjw users    188 Jun  5 09:51 WORKSPACE
```
果然，这几个bazel的产物都是符号连接。
而find指令默认不会进入软连接进行查找。
## 解决办法
给find增加-L参数即可
```shell
$find -L -name '*.gcno'
./bazel-bin/external/gtest/_objs/gtest/gmock.pic.gcno
./bazel-bin/external/gtest/_objs/gtest/gtest-printers.pic.gcno
./bazel-bin/external/gtest/_objs/gtest/gtest-filepath.pic.gcno
./bazel-bin/external/gtest/_objs/gtest/gtest-typed-test.pic.gcno
./bazel-bin/external/gtest/_objs/gtest/gmock-spec-builders.pic.gcno
./bazel-bin/external/gtest/_objs/gtest/gmock-internal-utils.pic.gcno
./bazel-bin/external/gtest/_objs/gtest/gtest.pic.gcno
./bazel-bin/external/gtest/_objs/gtest/gtest-assertion-result.pic.gcno
./bazel-bin/external/gtest/_objs/gtest/gmock-matchers.pic.gcno
./bazel-bin/external/gtest/_objs/gtest/gtest-test-part.pic.gcno
./bazel-bin/external/gtest/_objs/gtest/gtest-matchers.pic.gcno
./bazel-bin/external/gtest/_objs/gtest/gtest-death-test.pic.gcno
./bazel-bin/external/gtest/_objs/gtest/gmock-cardinalities.pic.gcno
./bazel-bin/external/gtest/_objs/gtest/gtest-port.pic.gcno
./bazel-bin/external/gtest/_objs/gtest_main/gmock_main.pic.gcno
./bazel-bin/main/_objs/hello_test/hello_test.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gmock.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-printers.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-filepath.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-typed-test.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gmock-spec-builders.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gmock-internal-utils.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-assertion-result.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gmock-matchers.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-test-part.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-matchers.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-death-test.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gmock-cardinalities.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-port.pic.gcno
./bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest_main/gmock_main.pic.gcno
./bazel-out/k8-fastbuild/bin/main/_objs/hello_test/hello_test.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gmock.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-printers.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-filepath.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-typed-test.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gmock-spec-builders.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gmock-internal-utils.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-assertion-result.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gmock-matchers.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-test-part.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-matchers.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-death-test.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gmock-cardinalities.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest/gtest-port.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/external/gtest/_objs/gtest_main/gmock_main.pic.gcno
./bazel-stage-test/bazel-out/k8-fastbuild/bin/main/_objs/hello_test/hello_test.pic.gcno
```
看，这不是很多吗：）


