---
title: 正则表达式和 shell 通配符展开
date: 2024-09-14 14:55:29
toc: true
mathjax: true
tags:
- 正则表达式
- shell
- 通配符
---

## 问题背景
遇到这样一个语句：

![](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/0537e6df1d13d23bf545377d8fb69d12/c87179b502c80bee0bc1647a80cb2ea2.png)

`-E`参数表示为`grep`启用扩展的正则表达式`ere`，但`*`是一个限定符，表示重复前面一个字符任意次，而语句中`*`前面没有任何字符，因此这是一个错误的表达式。

问题在于：这样的表达式能够正确被`grep`解析：

![](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/0537e6df1d13d23bf545377d8fb69d12/558b2ab43db70d4a645ad9c543b96c88.png)

## 背景知识
### shell 通配符展开
[借通配符展开问题描述 shell 的整个执行流程_shell 通配　展开-CSDN博客](https://blog.csdn.net/Longyu_wlz/article/details/108026979)

shell 中使用通配符，如`*`，`?`，它们会被 shell 扩展为当前目录中匹配的文件名或路径。

如，当前目录结构如下：

```shell
$tree
.
├── 1.c
├── 1.h
├── 2.c
├── 2.h
└── dir
    ├── 3.c
    └── 3.h
```

通配符展开结果：

```shell
$echo *.h
1.h 2.h
$echo */*.h
dir/3.h
```

也就是说，命令中的`*`在执行前会被替换。

想要取消替换也很简单，为字符串添加引号包围即可。

```shell
$echo '*.h'
*.h
$echo "*.h"
*.h
```

### 正则表达式
和shell 通配符不同，正则表达式中`*`不能独立出现，必须在前方有合法字符。

如下面的 python 交互结果：

```powershell
>>> import re
>>> pattern = re.compile(r'*.h')
Traceback (most recent call last):
  File "<stdin>", line 1, in <module>
  File "/usr/lib64/python3.6/re.py", line 233, in compile
    return _compile(pattern, flags)
  File "/usr/lib64/python3.6/re.py", line 301, in _compile
    p = sre_compile.compile(pattern, flags)
  File "/usr/lib64/python3.6/sre_compile.py", line 562, in compile
    p = sre_parse.parse(p, flags)
  File "/usr/lib64/python3.6/sre_parse.py", line 855, in parse
    p = _parse_sub(source, pattern, flags & SRE_FLAG_VERBOSE, 0)
  File "/usr/lib64/python3.6/sre_parse.py", line 416, in _parse_sub
    not nested and not items))
  File "/usr/lib64/python3.6/sre_parse.py", line 616, in _parse
    source.tell() - here + len(this))
sre_constants.error: nothing to repeat at position 0
>>> pattern = re.compile(r'.*.h')
```

## 实验
回顾这个语句

```powershell
grep -E '*.h$|*.c$|*.hpp$|*.cpp$|*.cc$'
```

显然这不是一个合法的正则表达式

```powershell
>>> pattern = re.compile(r'*.h$|*.c$|*.hpp$|*.cpp$|*.cc$')
Traceback (most recent call last):
  File "<stdin>", line 1, in <module>
  File "/usr/lib64/python3.6/re.py", line 233, in compile
    return _compile(pattern, flags)
  File "/usr/lib64/python3.6/re.py", line 301, in _compile
    p = sre_compile.compile(pattern, flags)
  File "/usr/lib64/python3.6/sre_compile.py", line 562, in compile
    p = sre_parse.parse(p, flags)
  File "/usr/lib64/python3.6/sre_parse.py", line 855, in parse
    p = _parse_sub(source, pattern, flags & SRE_FLAG_VERBOSE, 0)
  File "/usr/lib64/python3.6/sre_parse.py", line 416, in _parse_sub
    not nested and not items))
  File "/usr/lib64/python3.6/sre_parse.py", line 616, in _parse
    source.tell() - here + len(this))
sre_constants.error: nothing to repeat at position 0
```

但类似的表达式在 grep 中能够正常执行：

```shell
$ls | grep -E "*.h"
1.h
2.h
```

首先怀疑是否是通配符展开的结果（虽然表达式被引号去掉了，但也许存在某种机制依然使其被展开？）

直接去掉引号试试：

```shell
$ls | grep -E *.h
# 无结果
```

反而无法匹配了。

实际上，这个语句等价于

```shell
$ls | grep -E 1.h 2.h
# 无结果
```

grep 会将`1.h`作为匹配模式，将`2.h`作为要查找的文件。

可以确定通配符展开没有在此处生效。

## 原因
注意到如下结果（注意颜色）：

![](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/0537e6df1d13d23bf545377d8fb69d12/26e4756e945d694d654d5f54aab8787f.png)

`*.h`和`.h`的表现一致，标红的字符表示匹配到了".h"后缀。

`*`和空字符串的表现也一致，将当前目录的所有内容打印出来了，没有字符被标红。在正则表达式中，空字符串可以匹配任何位置，包括每一行的开始和结束。

故而得出结论：grep 对于`*`有特殊处理，对于前方无合法字符的`*`，会将其视为一个空字符串（也不是直接无视，见下图第三个例子），而不是报错。

![](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/0537e6df1d13d23bf545377d8fb69d12/23a4f14a55ffffe9dd74322c8f9c5a4a.png)

