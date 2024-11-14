---
title: WSL2 使用 Clash for Windows 代理连接
date: 2024-11-14 15:35:29
toc: true
mathjax: true
tags:
- WSL
- WSL2
- Clash
- 代理
- 网络
---

# WSL2 使用 Clash for Windows 代理连接

## 环境

Window版本：23H2 （可使用`winver`查看）

## 方法

### 1. Clash 客户端设置

打开Allow LAN开关，并记下端口号，默认为`7890` 

### 2. WSL配置

编辑`C:\Users\<UserName>\.wslconfig`（如果没有则创建一个），在其中添加以下内容关闭DNS通道。

```.wslconfig
[wsl2]
dnsTunneling=false
```

> [这个文件为使用 WSL 2 运行的所有 Linux 发行版全局配置设置](https://learn.microsoft.com/zh-cn/windows/wsl/wsl-config#wslconfig)。因此无论你的WSL安装在哪儿，配置文件都是这个位置。
>
> 你可以在Powershell中输入` echo $env:UserProfile`指令在确认位置。

### 3. 配置代理

进入WSL编辑`~/.bashrc` （或其他如`~/.zashrc`），在文件末尾添加以下三行，将端口号改为第一步记下的数字：

```bash
host_ip=$(cat /etc/resolv.conf |grep "nameserver" |cut -f 2 -d " ")
export http_proxy="http://$host_ip:[端口]"
export https_proxy="http://$host_ip:[端口]"
```

修改后执行`source ~/.bashrc`

### 4. 验证网络

```bash
# curl -I https://www.google.com
HTTP/1.1 200 Connection established

HTTP/2 200
content-type: text/html; charset=ISO-8859-1
content-security-policy-report-only: object-src 'none';base-uri 'self';script-src 'nonce-g0gER0a10nWTL0LJCF1g5w' 'strict-dynamic' 'report-sample' 'unsafe-eval' 'unsafe-inline' https: http:;report-uri https://csp.withgoogle.com/csp/gws/other-hp
accept-ch: Sec-CH-Prefers-Color-Scheme
p3p: CP="This is not a P3P policy! See g.co/p3phelp for more info."
date: Mon, 21 Oct 2024 06:21:18 GMT
server: gws
x-xss-protection: 0
x-frame-options: SAMEORIGIN
expires: Mon, 21 Oct 2024 06:21:18 GMT
cache-control: private
set-cookie: AEC=AVYB7cqVGkWFi38dxlgSWfDsPlSFsyvaGIB-MDiB6NeVPpueYGLH2XBIRg; expires=Sat, 19-Apr-2025 06:21:18 GMT; path=/; domain=.google.com; Secure; HttpOnly; SameSite=lax
set-cookie: NID=518=FMl1UheDMzzQPJbWiIdrVLCPPw1qm6nKMvCWzCLJAcqxw-P639nFXYwDojE0YUdvc_nbpSy2p2FnAIIibgehMcA2pfBBf3DZZ7CnaTUGyILVyz_B434pGhd2I1Tejk690pgKLpeJ2D-eecq8yWOE1_TEGmuv_Efn3dzVxAjlazjccBC0JtSrMuzmDN1YoQA; expires=Tue, 22-Apr-2025 06:21:18 GMT; path=/; domain=.google.com; HttpOnly
alt-svc: h3=":443"; ma=2592000,h3-29=":443"; ma=2592000
```



参考：[在 WSL2 中使用 Clash for Windows 代理连接 - East Monster 个人博客](https://eastmonster.github.io/2022/10/05/clash-config-in-wsl/)