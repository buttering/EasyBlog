---
title: cpp 编译工具一览
date: 2024-07-26 14:25:20
toc: true
mathjax: true
tags: 
- gnu
- c++
- 编译
---

## GCC（GNU Compiler Collection）
GCC是一个强大的编译器套件，支持多种编程语言。GCC 由自由软件基金会（FSF）开发和维护，是 GNU 项目的一部分。GCC包含多个编译器前端和一个通用的后端。这个通用后端负责将**中间表示**（由前端生成的，通常是**汇编代码**）转换为目标机器代码。
## gcc（GNU C Compiler）
gcc 是 GCC 的一个组成部分，它是 GCC 编译器集合中用于编译 C（以及其他语言）的命令行工具和驱动程序。用户通过 gcc 命令行工具来进行编译，而 gcc 内部会调用 GCC 编译器集合的其他组件来实际执行编译任务。
总结一下，“GCC” 是整个工具集的名字，“gcc” 是其中一个主要组件的名字，特指 C/C++ 的编译器。不过在日常交流中，“gcc” 经常被泛指整个 GCC 工具集。在现代环境中，“gcc” 往往被视为一个通用编译器，能够处理多种编程语言。
**参数**

- `-L`： 用于指定链接器搜索库文件的目录（如果这些库文件不在系统预定义的路径中）。包括静态库(.a)和共享库(.so)。
- `-l`：用于指定链接时需要的库。如果库文件名为 libmylib.a 或 libmylib.so，则使用 -lmylib。
- `-o`：指定输出文件的名称。目标文件（.o）、可执行文件、预处理输出文件（.i）、汇编代码文件（.s）等。
- `-E`：仅执行预编译步骤。预处理器会处理所有的宏定义、文件包含和条件编译指令。
- `-S`：将源代码编译成汇编代码，而不进行汇编和链接。
- `-c`：从源文件编译生成目标文件（.o 文件），而不进行链接。
- `-I`： 指定预处理器搜索头文件的目录。当头文件不在默认搜索路径中时，可以使用此选项添加自定义路径。
- `-iquote`: 指定以引号指定的头文件的目录。如（#include "foo.h")，不影响尖括号的寻找路径。
- `-include`: 指定在每个编译单元的开头包含指定的头文件。它主要用于确保某个头文件在编译**任何**源文件之前被包含。
- `-isystem`: 用于指定系统头文件的搜索路径，这些路径中的头文件通常被认为是可信赖的，不会触发警告信息。
- `-g`: 用于生成包含调试信息的可执行文件。供调试器（如 gdb）进行调试。
- `-D`: 用于定义预处理器宏，如`-DDEBUG`，定义了一个名为DEBUG的宏。

使用-v选项可以查看详细的编译过程
假设我们有以下项目结构：
main.c
foo.c
foo.h
```shell
% gcc foo.c main.c -o main -v
Apple clang version 15.0.0 (clang-1500.3.9.4)
Target: arm64-apple-darwin23.4.0
Thread model: posix
InstalledDir: /Library/Developer/CommandLineTools/usr/bin
 "/Library/Developer/CommandLineTools/usr/bin/clang" -cc1 -triple arm64-apple-macosx14.0.0 -Wundef-prefix=TARGET_OS_ -Wdeprecated-objc-isa-usage -Werror=deprecated-objc-isa-usage -Werror=implicit-function-declaration -emit-obj -mrelax-all --mrelax-relocations -disable-free -clear-ast-before-backend -disable-llvm-verifier -discard-value-names -main-file-name foo.c -mrelocation-model pic -pic-level 2 -mframe-pointer=non-leaf -fno-strict-return -ffp-contract=on -fno-rounding-math -funwind-tables=1 -fobjc-msgsend-selector-stubs -target-sdk-version=14.4 -fvisibility-inlines-hidden-static-local-var -target-cpu apple-m1 -target-feature +v8.5a -target-feature +crc -target-feature +lse -target-feature +rdm -target-feature +crypto -target-feature +dotprod -target-feature +fp-armv8 -target-feature +neon -target-feature +fp16fml -target-feature +ras -target-feature +rcpc -target-feature +zcm -target-feature +zcz -target-feature +fullfp16 -target-feature +sm4 -target-feature +sha3 -target-feature +sha2 -target-feature +aes -target-abi darwinpcs -debugger-tuning=lldb -target-linker-version 1053.12 -v -fcoverage-compilation-dir=/Users/buttering/pcc/pcc/functest/link_lib -resource-dir /Library/Developer/CommandLineTools/usr/lib/clang/15.0.0 -isysroot /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk -I/usr/local/include -internal-isystem /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/usr/local/include -internal-isystem /Library/Developer/CommandLineTools/usr/lib/clang/15.0.0/include -internal-externc-isystem /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/usr/include -internal-externc-isystem /Library/Developer/CommandLineTools/usr/include -Wno-reorder-init-list -Wno-implicit-int-float-conversion -Wno-c99-designator -Wno-final-dtor-non-final-class -Wno-extra-semi-stmt -Wno-misleading-indentation -Wno-quoted-include-in-framework-header -Wno-implicit-fallthrough -Wno-enum-enum-conversion -Wno-enum-float-conversion -Wno-elaborated-enum-base -Wno-reserved-identifier -Wno-gnu-folding-constant -fdebug-compilation-dir=/Users/buttering/pcc/pcc/functest/link_lib -ferror-limit 19 -stack-protector 1 -fstack-check -mdarwin-stkchk-strong-link -fblocks -fencode-extended-block-signature -fregister-global-dtors-with-atexit -fgnuc-version=4.2.1 -fmax-type-align=16 -fcommon -fcolor-diagnostics -clang-vendor-feature=+disableNonDependentMemberExprInCurrentInstantiation -fno-odr-hash-protocols -clang-vendor-feature=+enableAggressiveVLAFolding -clang-vendor-feature=+revert09abecef7bbf -clang-vendor-feature=+thisNoAlignAttr -clang-vendor-feature=+thisNoNullAttr -mllvm -disable-aligned-alloc-awareness=1 -D__GCC_HAVE_DWARF2_CFI_ASM=1 -o /var/folders/mr/rm9crpln3lj4d111hkl01g_m0000gp/T/foo-4172f5.o -x c foo.c
clang -cc1 version 15.0.0 (clang-1500.3.9.4) default target arm64-apple-darwin23.4.0
ignoring nonexistent directory "/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/usr/local/include"
ignoring nonexistent directory "/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/Library/Frameworks"
#include "..." search starts here:
#include <...> search starts here:
 /usr/local/include
 /Library/Developer/CommandLineTools/usr/lib/clang/15.0.0/include
 /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/usr/include
 /Library/Developer/CommandLineTools/usr/include
 /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/System/Library/Frameworks (framework directory)
End of search list.
 "/Library/Developer/CommandLineTools/usr/bin/clang" -cc1 -triple arm64-apple-macosx14.0.0 -Wundef-prefix=TARGET_OS_ -Wdeprecated-objc-isa-usage -Werror=deprecated-objc-isa-usage -Werror=implicit-function-declaration -emit-obj -mrelax-all --mrelax-relocations -disable-free -clear-ast-before-backend -disable-llvm-verifier -discard-value-names -main-file-name main.c -mrelocation-model pic -pic-level 2 -mframe-pointer=non-leaf -fno-strict-return -ffp-contract=on -fno-rounding-math -funwind-tables=1 -fobjc-msgsend-selector-stubs -target-sdk-version=14.4 -fvisibility-inlines-hidden-static-local-var -target-cpu apple-m1 -target-feature +v8.5a -target-feature +crc -target-feature +lse -target-feature +rdm -target-feature +crypto -target-feature +dotprod -target-feature +fp-armv8 -target-feature +neon -target-feature +fp16fml -target-feature +ras -target-feature +rcpc -target-feature +zcm -target-feature +zcz -target-feature +fullfp16 -target-feature +sm4 -target-feature +sha3 -target-feature +sha2 -target-feature +aes -target-abi darwinpcs -debugger-tuning=lldb -target-linker-version 1053.12 -v -fcoverage-compilation-dir=/Users/buttering/pcc/pcc/functest/link_lib -resource-dir /Library/Developer/CommandLineTools/usr/lib/clang/15.0.0 -isysroot /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk -I/usr/local/include -internal-isystem /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/usr/local/include -internal-isystem /Library/Developer/CommandLineTools/usr/lib/clang/15.0.0/include -internal-externc-isystem /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/usr/include -internal-externc-isystem /Library/Developer/CommandLineTools/usr/include -Wno-reorder-init-list -Wno-implicit-int-float-conversion -Wno-c99-designator -Wno-final-dtor-non-final-class -Wno-extra-semi-stmt -Wno-misleading-indentation -Wno-quoted-include-in-framework-header -Wno-implicit-fallthrough -Wno-enum-enum-conversion -Wno-enum-float-conversion -Wno-elaborated-enum-base -Wno-reserved-identifier -Wno-gnu-folding-constant -fdebug-compilation-dir=/Users/buttering/pcc/pcc/functest/link_lib -ferror-limit 19 -stack-protector 1 -fstack-check -mdarwin-stkchk-strong-link -fblocks -fencode-extended-block-signature -fregister-global-dtors-with-atexit -fgnuc-version=4.2.1 -fmax-type-align=16 -fcommon -fcolor-diagnostics -clang-vendor-feature=+disableNonDependentMemberExprInCurrentInstantiation -fno-odr-hash-protocols -clang-vendor-feature=+enableAggressiveVLAFolding -clang-vendor-feature=+revert09abecef7bbf -clang-vendor-feature=+thisNoAlignAttr -clang-vendor-feature=+thisNoNullAttr -mllvm -disable-aligned-alloc-awareness=1 -D__GCC_HAVE_DWARF2_CFI_ASM=1 -o /var/folders/mr/rm9crpln3lj4d111hkl01g_m0000gp/T/main-423eb1.o -x c main.c
clang -cc1 version 15.0.0 (clang-1500.3.9.4) default target arm64-apple-darwin23.4.0
ignoring nonexistent directory "/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/usr/local/include"
ignoring nonexistent directory "/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/Library/Frameworks"
#include "..." search starts here:
#include <...> search starts here:
 /usr/local/include
 /Library/Developer/CommandLineTools/usr/lib/clang/15.0.0/include
 /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/usr/include
 /Library/Developer/CommandLineTools/usr/include
 /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk/System/Library/Frameworks (framework directory)
End of search list.
 "/Library/Developer/CommandLineTools/usr/bin/ld" -demangle -lto_library /Library/Developer/CommandLineTools/usr/lib/libLTO.dylib -no_deduplicate -dynamic -arch arm64 -platform_version macos 14.0.0 14.4 -syslibroot /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk -o main -L/usr/local/lib /var/folders/mr/rm9crpln3lj4d111hkl01g_m0000gp/T/foo-4172f5.o /var/folders/mr/rm9crpln3lj4d111hkl01g_m0000gp/T/main-423eb1.o -lSystem /Library/Developer/CommandLineTools/usr/lib/clang/15.0.0/lib/darwin/libclang_rt.osx.a
```

- 第6行调用了clang的前端 cc1 将foo.c 编译成目标文件 foo-4172f5.o
- 第18行将main.c编译成main-423eb1.o
- 第30行链接了 foo-4172f5.o 和 main-423eb1.o 目标文件，并生成可执行文件 main。

也可手动执行：

1. 预处理
```shell
gcc -E foo.c -o foo.i   
gcc -E main.c -o main.i
```

2. 编译为汇编文件
```shell
gcc -S foo.i -o foo.s
gcc -S main.i -o main.s
```

3. 汇编为目标文件
```shell
gcc -c foo.s -o foo.o
gcc -c main.s -o main.o
```
以上三步等价于：
```shell
gcc -c foo.c -o foo.o
gcc -c main.c -o foo.c
```

4. 链接为可执行文件
```shell
gcc foo.o main.o -o main
```
也可以选择不生成main的目标文件，直接编译：
```shell
gcc -c foo.c -o foo.o
gcc foo.o main.c -o main
```
## g++
g++ 是 GNU 编译器集合的一部分，专用于编译 C++ 语言代码。g++ 负责协调预处理、编译、汇编和链接各个阶段的工作，可以将 C++ 源代码编译成可执行文件、目标文件或者库文件。
## cc1、cc1plus
cc1：C 语言编译前端
cc1plus：C++ 语言编译前端
```shell
gcc -S hello.c -o hello.s
```
等价于（省略一些必要的编译选项和路径）：
```shell
cc1 hello.c -o hello.s
```
## as、ld
as：也称为汇编器（Assembler），它是 GNU 汇编器（GNU Assembler）的命令行工具。汇编器的主要任务是将汇编代码（汇编语言）转换成机器语言（目标代码或目标文件）。通常会生成以 .o 或 .obj 作为扩展名的目标文件，这些文件包含了可被链接的机器代码。
ld：也称为链接器（Linker），它是 GNU 链接器的命令行工具。链接器的主要任务是将一个或多个目标文件和库文件链接成一个可执行文件或共享库。
## ar
（使用目标文件）创建、修改和提取存档文件（静态库，通常扩展名为 .a）。

- `r`: 插入文件到归档文件。
- `c`: 创建新的归档文件
- `s`: 写入索引（添加符号表信息）

例：创建或更新静态库
```shell
ar rcs libmylib.a file1.o file2.o file3.o
```
## cc、cxx、c++
```shell
#ls /usr/bin -l | grep cc
lrwxrwxrwx   1 root  root           3 Feb 17  2022 cc -> gcc
```
cc 通常是系统上 C 编译器（gcc或clang等）的符号链接或别名。
c++ 或者 cxx 是系统上 C++ 编译器的符号链接或别名，通常指向具体实现如 g++ 或 clang++。
**编译 C 源文件为目标文件**
```shell
cc -c -o hello.o hello.c
```
-c: 仅编译，不进行链接操作。这会生成一个目标文件（.o 文件）。
-o: hello.o 将输出保存为hello可执行文件
hello.c：C 源文件。
**生成一个名为 hello 的可执行文件**
```shell
c++ -o hello hello.cpp
```
hello.cpp: C++源文件
**链接 foo.o 和 bar.o 两个目标文件，生成一个名为 hello 的可执行文件**
```shell
c++ -o hello foo.o bar.o
```
foo.o、bar.o: 已经编译好的目标文件
## clang、clang++
clang 和 clang++ 是 LLVM 项目中的 C 和 C++ 编译器驱动程序，它们分别用于编译 C 和 C++ 语言的代码。他们也是一个前端，解析代码并生成抽象语法树（AST）和LLVM中间表示（IR）。
## make
make 根据 Makefile 中定义的规则和依赖关系，调用编译器（如 gcc, g++, clang, clang++）来编译和链接代码。Makefile 明确列出源文件、目标文件和每个目标的生成规则。
假设我们有以下项目结构：
hello.c
main.cpp
foo.c
bar.c
```makefile
# 定义编译器
CC = gcc
CXX = g++

# 定义目标文件和可执行文件
OBJS_C = hello.o foo.o bar.o
OBJS_CPP = main.o
EXEC = myprogram

# 规则：目标文件依赖于源文件
%.o: %.c
    $(CC) -c $< -o $@

%.o: %.cpp
    $(CXX) -c $< -o $@

# 规则：可执行文件依赖于目标文件
$(EXEC): $(OBJS_C) $(OBJS_CPP)
    $(CXX) $(OBJS_C) $(OBJS_CPP) -o $(EXEC)

# 清理目标
.PHONY: clean
clean:
    rm -f $(OBJS_C) $(OBJS_CPP) $(EXEC)

```
等价于自动化以下过程：

1. 编译c文件
```shell
gcc -c hello.c -o hello.o
```

2. 编译c++文件
```shell
g++ -c main.cpp -o main.o
```

3. 链接目标文件
```shell
g++ hello.o foo.o bar.o main.o -o myprogram
```
## cmake
cmake 是一个高级构建系统生成器，通过解析 CMakeLists.txt 文件，生成适用于特定平台和编译器（如 Makefile、Ninja、Visual Studio 项目文件等）的构建文件。
