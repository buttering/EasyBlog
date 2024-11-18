---
title: 使用 GDB 或 Delve 对已经运行起来的 Go 程序进行远程调试
date: 2024-11-18 13:36:29
toc: true
mathjax: true
tags:
- Golang
- GDB
- Delve
- 调试
- 远程调试
- Goland
---

# 使用 GDB 或 Delve 对已经运行起来的 Go 程序进行远程调试

## 背景

Java 程序可以很方便地通过 jdwp 参数指定一个对外端口进行远程调试，如

```bash
java \
-agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=50055 \
-jar remote-debug-0.0.1-SNAPSHOT.jar &
```

这样，我们在本地机器就可以很方便地调试远程已经运行起来的 Java 程序了。

那么，Golang 是否有类似的方法能实现呢？

## GDB

通过以下指令安装

```bash
apt install gdbserver  #  在远程机器上
apt install gdb  # 在本地机器上
```

### 1. 启动 Go 程序

假设我们有一个http 服务端程序：

```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

func sayHelloName(w http.ResponseWriter, r *http.Request) {
	fmt.Println()
	_ = r.ParseForm() //解析参数，默认是不会解析的
	fmt.Println(r.Form)
	fmt.Println("path", r.URL.Path)
	fmt.Println("scheme", r.URL.Scheme)
	fmt.Println(r.Form["url_long"])
	for k, v := range r.Form {
		fmt.Println("key", k)
		fmt.Println("val", strings.Join(v, ""))
	}
	fmt.Fprintf(w, "Hello astaxie!")
}

func main() {
	http.HandleFunc("/", sayHelloName)
	// 使用默认路由器开始监听
	err := http.ListenAndServe(":9090", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
```

将其编译并运行：

```bash
go build -o main
./main
```

这样，此时，这个简单的go程序就开始运行了。

### 2. 使用 gdbserver 连接程序

另起一个终端，使用 gdbserver 连接这个进程

```bash
## 获取进程pid
# ps aut | grep main   
root      3664  0.0  0.0 1600584 6012 pts/1    Sl+  12:38   0:00 ./main
## 连接到这个进程，<2345>是自行指定的对外端口
# gdbserver localhost:2345 --attach 3664
Attached; pid = 3664
Listening on port 2345
```

这样，我们的开发机就准备好远程调试我们的go程序了。

*顺便，第1、2步可以直接使用gdbserver指令代替：*

```bash
gdbserver localhost:2345 ./main
```

### 3. 使用 gdb 连接到 gdbserver

在本地的机器上使用 gdb 连接启动的 gdbserver 服务。这里在同一台机器(localhost)上模拟

```bash
# gdb  # 运行gdb    
GNU gdb (Ubuntu 12.1-0ubuntu1~22.04.3) 12.1
Copyright (C) 2022 Free Software Foundation, Inc.
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.
Type "show copying" and "show warranty" for details.
This GDB was configured as "x86_64-linux-gnu".
Type "show configuration" for configuration details.
For bug reporting instructions, please see:
<https://www.gnu.org/software/gdb/bugs/>.
Find the GDB manual and other documentation resources online at:
    <http://www.gnu.org/software/gdb/documentation/>.

For help, type "help".
Type "apropos word" to search for commands related to "word".
(gdb) target remote localhost:2345  
Remote debugging using localhost:2345
Reading /root/project/go-web/main from remote target...
warning: File transfers from remote targets can be slow. Use "set sysroot" to access files locally instead.
Reading /root/project/go-web/main from remote target...
Reading symbols from target:/root/project/go-web/main...
Reading /lib/x86_64-linux-gnu/libc.so.6 from remote target...
Reading /lib64/ld-linux-x86-64.so.2 from remote target...
Reading symbols from target:/lib/x86_64-linux-gnu/libc.so.6...
Reading symbols from /usr/lib/debug/.build-id/49/0fef8403240c91833978d494d39e537409b92e.debug...
Reading symbols from target:/lib64/ld-linux-x86-64.so.2...
Reading symbols from /usr/lib/debug/.build-id/41/86944c50f8a32b47d74931e3f512b811813b64.debug...
Reading /lib64/ld-linux-x86-64.so.2 from remote target...
Reading /usr/lib/debug/.build-id/04/f611a778dee3b5f92c1df94f899d200c106375.debug from remote target...
internal/runtime/syscall.Syscall6 () at /usr/local/go/src/internal/runtime/syscall/asm_linux_amd64.s:36
36              CMPQ    AX, $0xfffffffffffff001
(gdb) list
31              MOVQ    DI, DX  // a3
32              MOVQ    CX, SI  // a2
33              MOVQ    BX, DI  // a1
34              // num already in AX.
35              SYSCALL
36              CMPQ    AX, $0xfffffffffffff001
37              JLS     ok
38              NEGQ    AX
39              MOVQ    AX, CX  // errno
40              MOVQ    $-1, AX // r1
(gdb) 
```

使用 gdb 指令`target remote`指定远程gdbserver的地址即可。

## Delve

gdb只能看到对应的汇编代码（想查看源代码可能需要更多设置），而且常用的 IDE Goland 并不支持 GDB 调试 Go 程序。

更好的解决办法是使用 Delve 构建。

在远程机器上安装:

```bash
apt install delve
```

如果在之后的第二步运行时报错：

```bash
# dlv attach 5600 --headless --listen=:2345 --api-version=2 --log
API server listening at: [::]:2345
2024-11-18T12:57:43+08:00 warning layer=rpc Listening for remote connections (connections are not authenticated nor encrypted)
2024-11-18T12:57:43+08:00 info layer=debugger attaching to pid 5600
2024-11-18T12:57:43+08:00 error layer=debugger can't find build-id note on binary
2024-11-18T12:57:44+08:00 error layer=debugger could not patch runtime.mallogc: no type entry found, use 'types' for a list of valid types
panic: <nil> not an Int

goroutine 1 [running]:
go/constant.Int64Val({0x0, 0x0})
        go/constant/value.go:502 +0x119
github.com/go-delve/delve/pkg/proc.(*Variable).parseG.func2({0xa580c0, 0xc000a41e48})
        github.com/go-delve/delve/pkg/proc/variables.go:879 +0x49
github.com/go-delve/delve/pkg/proc.(*Variable).parseG(0xb3a588)
        github.com/go-delve/delve/pkg/proc/variables.go:901 +0x5b5
github.com/go-delve/delve/pkg/proc.GetG({0xb3a588, 0xc00035c460})
        github.com/go-delve/delve/pkg/proc/variables.go:258 +0xae
github.com/go-delve/delve/pkg/proc.NewTarget({0xb3e4e0, 0xc0001b4480}, 0x15e0, {0xb3a588, 0xc00035c460}, {{0xc000288050, 0xe}, {0xc0001d5110, 0x1, 0x1}, ...})
        github.com/go-delve/delve/pkg/proc/target.go:210 +0x2cf
github.com/go-delve/delve/pkg/proc/native.(*nativeProcess).initialize(0xc0001b4480, {0xc000288050, 0xe}, {0xc0001d5110, 0x1, 0x1})
        github.com/go-delve/delve/pkg/proc/native/proc.go:268 +0x145
github.com/go-delve/delve/pkg/proc/native.Attach(0x5a3725, {0xc0001d5110, 0x1, 0x1})
        github.com/go-delve/delve/pkg/proc/native/proc_linux.go:157 +0x18f
github.com/go-delve/delve/service/debugger.(*Debugger).Attach(0xc00035c230, 0xc000000004, {0x0, 0x5})
        github.com/go-delve/delve/service/debugger/debugger.go:349 +0xd6
github.com/go-delve/delve/service/debugger.New(0xc0001ce1c0, {0xc00014c970, 0x0, 0x4})
        github.com/go-delve/delve/service/debugger/debugger.go:160 +0x5c5
github.com/go-delve/delve/service/rpccommon.(*ServerImpl).Run(0xc000125260)
        github.com/go-delve/delve/service/rpccommon/server.go:122 +0xd6
github.com/go-delve/delve/cmd/dlv/cmds.execute(0x15e0, {0xc00014c970, 0x0, 0x4}, 0xc0001d8ea0, {0x0, 0x0}, 0x3, {0xc00014c960, 0x1, ...}, ...)
        github.com/go-delve/delve/cmd/dlv/cmds/commands.go:971 +0xc7f
github.com/go-delve/delve/cmd/dlv/cmds.attachCmd(0xc0001eaf00, {0xc00014c960, 0x1, 0x5})
        github.com/go-delve/delve/cmd/dlv/cmds/commands.go:771 +0x145
github.com/spf13/cobra.(*Command).execute(0xc0001eaf00, {0xc00014c910, 0x5, 0x5})
        github.com/spf13/cobra/command.go:860 +0x5f8
github.com/spf13/cobra.(*Command).ExecuteC(0xc0001eac80)
        github.com/spf13/cobra/command.go:974 +0x3bc
github.com/spf13/cobra.(*Command).Execute(...)
        github.com/spf13/cobra/command.go:902
main.main()
        github.com/go-delve/delve/cmd/dlv/main.go:24 +0x18a
```

这可能是Delve版本不匹配，可以改用以下方法安装： 

```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

### 1. 编译并运行 Go 程序

同样地，我们使用之前的 http 服务作为例子。

`go build` 的参数`all=-N -l`表示关闭优化和内联，这会让调试更加准确。

```bash
go build -gcflags="all=-N -l" -o main
./main
```

### 2. 使用 delve 连接运行中的进程

和 gdbserver 操作类似，需要指定进程 pid 和对外端口。

```bash
# ps aux | grep main 
root      7799  0.0  0.0 1602348 10320 pts/1   tl+  13:03   0:00 ./main
# dlv attach 7799 --headless --listen=:2345 --api-version=2 --log 
```

### 3. 在本地机器连接到远程的 Delve 服务器

这里直接使用 Goland 内置的 Delve 工具

创建一个 Go Remote 的配置

![image-20241118131927872](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/8fd7e0c88e79ac2f30a8e8853a833c73/e2550a0625da6a3d87a59c6bc0853ccb.png)

指定远程 delve 服务的 ip 和端口

![image-20241118132046333](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/8fd7e0c88e79ac2f30a8e8853a833c73/7e083fec3c569697ee51b6025d96eb77.png)

配置完成，启动调试，就会在我们预设的断点处停止了。

![image-20241118132201030](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/8fd7e0c88e79ac2f30a8e8853a833c73/60c9bca9498ed458096dbacd23f35313.png)

如同在本地一般，我们可以查看变量并对其进行调试：

![image-20241118132246804](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/8fd7e0c88e79ac2f30a8e8853a833c73/b31e511e42d3a410260456f61f028ec4.png)