---
title: printk函数出现延迟输出的问题
date: 2022-10-15 17:51:24
toc: true
mathjax: true
categories:
- 杂项
tags:
- linux
- 内核
---

# 错误描述
关于测试内核read接口
有如下代码片段：

```c
static ssize_t hello_read(struct file *filp, char __user *buf, size_t count, loff_t *ppos){
    int ret = 0;
    printk("[read task]count=%ld", count);
    memcpy(readbuf, kerneldata, sizeof(kerneldata));

    ret = copy_to_user(buf, readbuf, count);
    printk("[read data]output string: %s", readbuf);

    return count;
}
```
编写应用程序调用该接口，使用dmesg指令发现仅有第一个printk语句输出到了内核信息缓存中：

```bash
[28560.208483] [read task]count=50
```
第二个printk没有被输出到缓存中。
第二次调用该接口，出现以下结果：

```bash
[29233.035136] [read task]count=50
[29233.035144] [read data]output string: This is the kernel data
[29238.092577] [read task]count=50
```
# 错误原因
原因在于没有在字符串末尾添加换行符`\n`，导致printk的输出未被刷新。
# 解决办法
在printk末尾添加换行符，即可正确输出。