---
title: WSL 运行make提示/lib/modules/xxx/build: No such file or directory. Stop.错误解决办法
date: 2022-11-24 18:26:16
toc: true
mathjax: true
tags:
- WSL
- 驱动
- Ubuntu
- linux
---

# 错误描述
在WSL下试图编译驱动文件，使用make命令编译c文件出现以下报错：
```bash
wang@DESKTOP-55P8P0H:/mnt/d/GithubDesktop/linux-driver-learning-experiment/1.hello_driver$ make
make -C /lib/modules/5.10.16.3-microsoft-standard-WSL2/build M=/mnt/d/GithubDesktop/linux-driver-learning-experiment/1.hello_driver modules
make[1]: *** /lib/modules/5.10.16.3-microsoft-standard-WSL2/build: No such file or directory.  Stop.
make: *** [Makefile:5: all] Error 2
```
# 错误原因
WSL2的内核是修改过的，无法使用 ubuntu上游的内核头文件和modules文件，因此，我们需要手动编译并安装一个版本。


# 解决步骤
参考：[WSL2编译和安装内核以支持驱动编译](https://www.cnblogs.com/hartmon/p/15771731.html)
## 1. 下载对应版本的内核代码
查看内核版本，我的是5.10.16.3版本
```bash
wang@DESKTOP-55P8P0H:~/kernel$ uname -r
5.10.16.3-microsoft-standard-WSL2
```
到[WSL git仓库](https://github.com/microsoft/WSL2-Linux-Kerne)，找到对应的release
```bash
git clone --depth=1 https://github.com/microsoft/WSL2-Linux-Kernel
```
![对应版本release](https:/raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/aed2424a629ae84d5396aaf9d9093a34/13763bc1a0ce0270aeb3990ba6bea4c8.jpeg)
复制下载链接，下载tar包，解压
```bash
wget https://github.om/microsoft/WSL2-Linux-Kernel/archive/refs/tags/linux-msft-wsl-5.10.16.3.tar.gz
tar -cvzf linux-msft-wsl-5.10.16.3.tar.gz
```
## 2. 编译和安装
```bash
cd WSL2-Linux-Kernel-linux-msft-wsl-5.10.16.3
LOCALVERSION= make KCONFIG_CONFIG=Microsoft/config-wsl -j8
sudo LOCALVERSION= make KCONFIG_CONFIG=Microsoft/config-wsl modules_install -j8
```
运行第二步出现问题：
```bash
  ERROR: Kernel configuration is invalid.
         include/generated/autoconf.h or include/config/auto.conf are missing.
         Run 'make oldconfig && make prepare' on kernel src to fix it.
```
尝试运行```make oldconfig && make prepare```
再次报错：
```bash
*
* Unable to find the ncurses package.
* Install ncurses (ncurses-devel or libncurses-dev
* depending on your distribution).
*
* You may also need to install pkg-config to find the
* ncurses installed in a non-default location.
*
make[1]: *** [scripts/kconfig/Makefile:211: scripts/kconfig/mconf-cfg] Error 1
make: *** [Makefile:619: menuconfig] Error 2
```
需要安装ncurses、flex和bison
```bash
sudo apt install ncurses-dev
sudo apt install flex
sudo apt install bison
```
然后运行```make menuconfig```选择```<Exit>```直接退出，自动生成```.config```文件

再次运行wang```make oldconfig && make prepare```
报错：
```bash
scripts/extract-cert.c:21:10: fatal error: openssl/bio.h: No such file or directory
   21 | #include <openssl/bio.h>
      |          ^~~~~~~~~~~~~~~
compilation terminated.
make[1]: *** [scripts/Makefile.host:95: scripts/extract-cert] Error 1
make: *** [Makefile:1234: scripts] Error 2
```
安装libssl-dev
```bash
sudo apt install libssl-dev
```
**再次尝试第二步，成功编译内核。**
## 3. 安装headers
```bash
sudo make headers_install ARCH=x86_64 INSTALL_HDR_PATH=/usr
```
# 结果
回到工作目录，再次尝试编译，成功。