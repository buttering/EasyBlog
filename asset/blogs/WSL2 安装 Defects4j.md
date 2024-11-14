---
title: WSL2 安装 Defects4j
date: 2024-11-14 15:35:29
toc: true
mathjax: true
tags:
- WSL
- WSL2
- Defects4j
- 数据集
---

## 环境

虚拟环境：Win11 23H2 + WSL2

操作系统：Ubutnu(Linux DESKTOP-OC45MQL 5.10.16.3-microsoft-standard-WSL2 #1 SMP Fri Apr 2 22:23:49 UTC 2021 x86_64 x86_64 x86_64 GNU/Linux)

## 外部依赖

```bash
apt install subversion -y  # svn
apt install git -y  # git 
apt install build-essential -y # gcc库
apt install perl -y  # perl
```

cpan是perl自带的包管理工具，安装后，运行`cpan` 进入配置界面，然后输入`o conf init` 来初始化。

然后安装cpanm

```bash
cpan App::cpanminus
```

## 安装

[rjust/defects4j: A Database of Real Faults and an Experimental Infrastructure to Enable Controlled Experiments in Software Engineering Research](https://github.com/rjust/defects4j)

```bash
mkdir ~/defects4j && cd ~/defects4j
git clone https://github.com/rjust/defects4j
cpanm --installdeps .
./init.sh
```

## 错误解决

init.sh脚本运行时可能出现类似下面的错误

```bash
Checking system configuration ...
Setting up project repositories ...

Setting up Major ...
Downloading https://mutation-testing.org/downloads/major-3.0.1_jre11.zip
curl: (23) Failed writing received data to disk/application
retrying curl https://mutation-testing.org/downloads/major-3.0.1_jre11.zip
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100 7119k  100 7119k    0     0  1798k      0  0:00:03  0:00:03 --:--:-- 1798k
Downloaded https://mutation-testing.org/downloads/major-3.0.1_jre11.zip

Setting up EvoSuite ...
Downloading https://github.com/EvoSuite/evosuite/releases/download/v1.1.0/evosuite-1.1.0.jar
curl: (23) Failed writing received data to disk/application
retrying curl https://github.com/EvoSuite/evosuite/releases/download/v1.1.0/evosuite-1.1.0.jar
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
  0     0    0     0    0     0      0      0 --:--:--  0:00:56 --:--:--     0
```

显示curl下载失败。可能与存储空间、文件系统权限或文件路径有关。但是检查无果，后采取将curl改为wget的方法解决：

```bash
vim init.sh
# 修改其中download_url函数，改为以下内容
download_url() {
    if [ "$#" -ne 1 ]; then
        echo "Illegal number of arguments"
    fi
    URL=$1
    echo "Downloading ${URL}"

    # 使用 wget 进行下载
    BASENAME="$(basename "$URL")"
    
    # 如果文件已经存在，则添加 -N 参数仅下载更新版本
    if [ -f "$BASENAME" ]; then
        wget -nv -N "$URL" || print_error_and_exit "Could not download $URL"
    else
        wget -nv "$URL" || print_error_and_exit "Could not download $URL"
    fi
    
    echo "Downloaded $URL"
}
```

