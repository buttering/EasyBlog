---
title: Oracle数据库体系结构
date: 2020-12-17 15:54:11
toc: true
mathjax: true
tags:
- 数据库
- oracle
---

## Oracle系统体系结构由三个部分组成：**实例、物理结构和逻辑结构**
## 实例和物理结构（数据库）组成了Oracle服务器。
![](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/830522c4b066364191eadc5cbb0485d7/5b9810e2920220606ad15ab424f9ac38.png)
# 一、实例
也成为服务器Server，是用来访问一个数据库文件集的**一个存储结构以及后台进程的集合**。总是打开一个且仅打开一个数据库，一个数据库可以被多个实例访问。

由内存结构和进程结构组成。
### 1.1 内存结构
内存结构由系统全局区和程序全局区构成
### 1.1.1 系统全局区（SGA）
SGA(System Global Area)：由所有服务进程、后台进程**共享**。
在实例启动时分配，时一组为系统分配的共享内存结构。是占用内存最大的一个区域，也是影响数据库性能的重要因素。

由共享池、数据块高速缓冲区、重做日志缓冲区组成，另外有两个可选的内存结构：大型池、JAVA池。
##### 1.1.1.1 <font color='red'>共享池（共享储存区）</font>
用来储存最近最多执行的SQL语句和最近最多使用的数据定义

主要由库缓冲区和数据字典缓冲区组成
##### 1.1.1.2 共享池——库缓冲区
储存最近使用的SQL和PL/SQL语句信息。
它能够使普遍使用的语句能被共享。
由两种结构组成，其各自的大小由共享池内部指定：
- 共享SQL区域
- 共享PL/SQL区域
##### 1.1.1.3 共享池——数据字典缓冲区
使数据库里最经常使用的对象定义的集合。
包括数据文件名、表、索引、列、用户权限和其他数据库对象等信息。数据库需要这些信息时，将查找数据字典获取关联对象信息。
缓存数据字典信息在内存区能提高查询数据响应时间。
##### 1.1.1.4 <font color='red'>数据块高速缓冲区</font>
储存以前从数据文件中取出过的数据块的拷贝信息。
通常只缓存数据库大小的1%~2%。使用LRU（最近最少使用）算法来管理可用空间。
##### 1.1.1.5 <font color='red'>重做日志缓冲区</font>
记录数据块的所有变化。重做项会被周期性分批写到重做日志文件中，以便再数据块恢复过程中用于恢复操作。
##### 1.1.1.6 大型池
只配置在共享服务器环境中，能减轻共享池的负担。
##### 1.1.1.7 Java池
目的是为JAVA命令提供语法分析，如果安装并使用JAVA是必须的。
### 1.1.2 程序全局区（PGA）
PGA(Program Global Area)：由每个服务进程、后台进程专有；每个进程都有一个PGA。
当客户进程访问oracle服务器时，会在oracle服务器端为用户进程分配相应的**服务进程**，并且为该服务进程分配相应的内存空间来存放其数据和控制信息，每一个**后台进程**也需要为其分配专用的存储空间。也就是**PGA**
### 1.2 进程结构
分别为用户进程、服务器进程和后台进程。
![](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/830522c4b066364191eadc5cbb0485d7/3ba51db064968810f1883ab13250724a.png)

### 1.2.1 用户进程
用户进程是要求Oracle服务器交互的一种进程。
当数据库用户要求**连接**到Oracle服务器时启动，不直接和Oracle服务器连接。
### 1.2.2 服务器进程
连接ORacle实例，当用户建立一个**会话**时开始启动。

> **连接和会话**是两个不同的概念。  连接是用户进程到实例之间的一条物理路径；会话是实例中存在的一个逻辑实体。一条连接上可以建立0个，1个或多个会话，而且各个会话单独且独立的。
> 使用connect和disconnect创建或结束会话

服务器进程就是代表客户会话完成工作的进程。几乎所有的工作都是由它们来做的，因此占用系统cpu的时间最多。

**任务：**
- 对sql进行解析和执行
- 如果所需的数据不在sga中，则server process会去磁盘上将其读到sga的database_buffer_cache中。
- 把结果返回给应用程序

可用专用服务器模式或共享服务器模式创建会话。
### 1.2.3 后台进程
若干个常驻内存的操作系统进程，在进程启动时分配。
数据库的**物理结构与内存结构之间的交互**通过这些进程完成。
##### 1.2.3.1 <font color='red'>数据库复写器（DBWn）
负责管理缓冲储存区。主要任务是将缓冲区的脏数据写入磁盘。

![](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/830522c4b066364191eadc5cbb0485d7/043fe5eabf6f67b620004f455e9ff65e.png)

##### 1.2.3.2 <font color='red'>日志复写器（LGWR）
负责管理日志缓冲区，将上次写入磁盘以来的全部日志缓冲区写入磁盘上的日志文件。

![](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/830522c4b066364191eadc5cbb0485d7/db4e77f2ab2774894f5af64263b94ecd.png)

##### 1.2.3.3 <font color='red'>系统监控进程（SMON）
该实例启动时，执行**实例恢复**，还负责清理不再使用的临时段。

![](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/830522c4b066364191eadc5cbb0485d7/cb693c08fcce0d3f3b238c33444c1fa9.png)

##### 1.2.3.4 <font color='red'>进程监控器（PMON）
该进程在用户进程出现故障时执行进程恢复，负责清理内存储区和释放该进程所使用的资源。

![](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/830522c4b066364191eadc5cbb0485d7/9f8ff208cad3045f3aca2e816fbfba0e.png)

##### 1.2.3.5 <font color='red'>检查点（CKPT）

![](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/830522c4b066364191eadc5cbb0485d7/a7b7b17ad4fdd80c9bc07df0f71e439c.png)

##### 1.2.3.6 <font color='red'>归档进程</font>（ARCn）（可选）
当ArchiveLog模式被设置时，自动归档联机重做日志文件，保存所有数据库变化。

![](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/830522c4b066364191eadc5cbb0485d7/72c2721a0a15f574fd097d30ef0ab070.png)

# 二、物理结构
包括了数据文件、日志文件和控制文件。
### 2.1 <font color='red'>数据文件</font>
每个Oracle数据库有一个或多个物理的数据文件。一个数据库的数据文件包括全部数据库数据。逻辑结构中的一个表空间有一个或多个数据文件组成。
### 2.2 <font color='red'>重做日志文件</font>
每个数据库有两个或多个日志文件的组。每个**日志文件组**用于收集数据库日志。
日志的主要功能是记录对数据所做的修改，用于保护数据库以防止故障。
### 2.3 <font color='red'>控制文件</font>
每一Oracle数据库有一个控制文件。用于记录数据库的物理结构
# 三、逻辑结构
描述了数据库的物理空间怎样运用，是一种层次结构。包括**表空间、段、片区、快**。