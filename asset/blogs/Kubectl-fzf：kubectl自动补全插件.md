---
title: Kubectl-fzf：kubectl自动补全插件
date: 2025-02-18 10:27:42
toc: true
mathjax: true
tags: 
- k8s
- kubectl
- fzf
- 工具安装
---

## **第一步 激活`kubectl`自带指令补全插件**

kubectl 通过命令 `kubectl completion zsh` 生成 Zsh 自动补全脚本。 在 Shell 中导入（Sourcing）该自动补全脚本，将启动 kubectl 自动补全功能。

在`zsh` 中，只需将下面内容添加到启动文件`~/.zshrc`中

```bash
source <(kubectl completion zsh)
```

激活后，在输入kubectl指令时，按`tab`键即可提供简单的自动补全功能。

![image-20241210113528907](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/77e95e18b901c5372df99f5dff8d9110/359fb02daf31a1c775ca15a1bf6569de.png)

如果使用其他终端，可以查看文档：[在 Linux 系统中安装并设置 kubectl | Kubernetes](https://kubernetes.io/zh-cn/docs/tasks/tools/install-kubectl-linux/#enable-shell-autocompletion)

## **第二步 安装模糊搜索 `fzf`**

```bash
apt install fzf
```

检查`~./zshrc`是否被添加了以下语句：

```bash
[ -f ~/.fzf.zsh ] && source ~/.fzf.zsh
```

更新之后，即可使用`ctrl+T` 或输入`**`后按`tab`唤起模糊搜索。

![image-20241210113842347](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/77e95e18b901c5372df99f5dff8d9110/03ed9f3c11e20cdfc28a92d65cdf8432.png)

对于特定指令，`fzf`有针对性支持，比如`ssh`，会自动查找本机路由表:

![image-20241210113936802](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/77e95e18b901c5372df99f5dff8d9110/f2f2f51ab665c74d6e44c9d6c6fe7203.png)

操作手册：[fzf: Shell Integration | junegunn.choi.](https://junegunn.github.io/fzf/shell-integration/#alt-c)

## **第三步 安装Kubectl-fzf**

使用Go安装：

```bash
# 安装 autocompletion 组件
go install github.com/bonnefoa/kubectl-fzf/v3/cmd/kubectl-fzf-completion@main

# 如果需要本地运行 server，安装 server 组件
go install github.com/bonnefoa/kubectl-fzf/v3/cmd/kubectl-fzf-server@main

# 下载Zsh补全脚本
wget https://raw.githubusercontent.com/bonnefoa/kubectl-fzf/main/shell/kubectl_fzf.plugin.zsh -O ~/.kubectl_fzf.plugin.zsh
```

确保 `$GOPATH/bin` 包含在你的 `$PATH` 环境变量中：

```bash
export PATH=$PATH:$GOPATH/bin
```

编辑`~/.zshrc`，添加如下语句并更新终端：

```bash
# 启用 kubectl-fzf 的自动补全
source ~/.kubectl_fzf.plugin.zsh
```

此时尝试使用自动补全，不会有任何结果，需要启动`kubectl-fzf-server`来监控k8s信息。

kubectl-fzf-server分为本地版本和pod版本。本地版本自己运行在自己的机器上，pod版本运行在集群中。本地版本使用如下指令运行：

```bash
kubectl-fzf-server
```

该服务会将集群信息缓存在本地 `/tmp/kubectl_fzf_cache` 目录中。

添加如下参数可以修改kubectl-fzf-server所监听的端口：

```bash
kubectl-fzf-server --listen-address=localhost:8081
```

使用如下环境变量使kubectl-fzf监听对应端口：

```bash
KUBECTL_FZF_PORT_FORWARD_LOCAL_PORT=8081
```

新开一个终端，键入以下内容后，结果如图

```bash
kubectl get pods <tab>
```

![image-20241210115129324](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/77e95e18b901c5372df99f5dff8d9110/da6765030d6405de55dd6e67132af558.png)

选择目标pod后，会自动将pod和namespace信息添加到终端的输入：

![image-20241210115218586](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/77e95e18b901c5372df99f5dff8d9110/cca144abb2e9262f25854ae03316719d.png)

也可以将`kubectl-fzf-server`设为守护进程或k8s pod，具体见：[bonnefoa/kubectl-fzf: A fast kubectl autocompletion with fzf](https://github.com/bonnefoa/kubectl-fzf?tab=readme-ov-file#kubectl-fzf-server)
