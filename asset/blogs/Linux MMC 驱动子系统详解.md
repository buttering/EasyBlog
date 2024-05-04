---
title: Linux MMC 驱动子系统详解
date: 2023-02-07 15:39:13
toc: true
mathjax: true
tags:
- linux
- MMC
- 驱动
---

# Linux MMC 驱动子系统

SD/SDIO/MMC 驱动是一种基于 SDMMC 和 SD SPI 主机驱动的协议级驱动程序，目前已支持 SD 存储器、SDIO 卡和 eMMC 芯片。

因为linux内核mmc子系统里面已经实现了这些协议，我们以后并不需要重新实现这些，只需要对协议有个简单的了解。

mmc是比较老的存储卡了，sd是mmc的替代者，sdio是基于sd而额外开发出的一种io接口卡。

## 硬件关联

CPU、MMC controller、存储设备之间的关联如下图所示，主要包括了MMC controller、总线、存储卡等内容的连接，针对控制器与设备的总线连接，主要包括时钟、数据、命令三种类型的引脚，而这些引脚中的cd引脚主要用于卡的在位检测，当mmc controller检测到该位的变化后，则会进行mmc card的注册或注销操作。

![cpu mmc硬件关联图](https:/raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/f458025b97d662eade4badd77c4d0d83/319603986631c7006d61c0afa39a33c9.jpeg)


## 目录说明

针对mmc子系统，在代码实现上主要包括mmc core、mmc block、 mmc host这三个模块

- mmc card：衔接最上层应用，主要用于实现mmc block驱动以及mmc driver即**mmc层驱动**（实际上我研究的源代码并没有这个目录（5.15.0-52-generic），猜测是合并到了core目录下）；
- 而mmc core：实现mmc/sd/sdio协议，主要包括mmc 总线、sdio总线的实现、mmc device、mmc driver的注册接口、mmc host与mmc card的注册与注销接口等内容。
- mmc host：存放各个mmc/sd/sdio**控制器的驱动**代码，最终操作mmc/sd/sdio卡的部分；

## mmc子系统的逻辑架构

MMC子系统从上到下分为3层

- **块设备层**(MMC card)：与Linux的块设备子系统对接，实现块设备驱动以及完成请求，如sys_open调用；通过调用core接口函数(具体如host->ops->rquest),驱动MMC core抽象出来的虚拟的card设备，如mmc、sd、tf卡，实现读写数据。

- **核心层**(MMC core)：是不同协议和规范的实现，为MMC控制器层和块设备驱动层提供接口函数。
  
  核心层封装了 MMC/SD 卡的命令（CMD)，例如存储卡的识别、设置、读写、识别、设置等命令。
  
  MMC核心层由三个部分组成：MMC，SD和SDIO，分别为三类设备驱动提供接口函数；
  
  core.c 把 MMC 卡、 SD 卡的共性抽象出来，它们的差别由 sd.c 和 sd_ops.c 、 mmc.c 和 mmc_ops.c 来完成。

- **控制器层**(MMC host)：主机端MMC controller的驱动，依赖于平台，由struct mmc_host描述。

  围绕此结构设计了struct mmc_host_ops(访问方法)、struct mmc_ios(相关参数)、struct mmc_bus_ops(电源管理和在位检测方法)
  

针对不同芯片，实现不同控制器对应的驱动代码。

![mmc子系统框架结构图](https:/raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/f458025b97d662eade4badd77c4d0d83/e87c1da5cde68259ed1f33f61fe6a152.jpeg)


块设备层与Linux的块设备子系统对接，实现块设备驱动以及完成请求，具体协议经过核心层的接口，最终通过控制器层完成传输，对MMC设备进行实际的操作。

更详细的结构图如下，指明了个部分的相关实现文件：

![在这里插入图片描述](https:/raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/f458025b97d662eade4badd77c4d0d83/00a1076a36325dcc59fb0280fae2b253.png)


mmc core指的是mmc 子系统的核心，这里的mmc表示的是mmc总线、结构、设备相关的统称，而下方文件名的mmc单指mmc卡，区别于sd卡和sdio卡。

drivers/mmc/core/mmc.c（提供接口），
drivers/mmc/core/mmc-ops.c（提供和mmc type card协议相关的操作）

在mmc core层中的bus指的是由core抽象出来的虚拟总线，而与物理卡连接的MMC bus是物理的实际总线，是和host controller直接关联的。

## 设备-总线-驱动模型

针对MMC子系统而言，主要使用了系统中的两个模型：**设备-总线-驱动模型**、**块设备驱动**模型。

在Linux驱动模型框架下，三者对应结构体以及MMC驱动子系统对应的实现关系如下：

- 总线 (struct bus_type) —— MMC总线（ mmc_bus )
- 设备(struct device) —— 被封装在platform_device下的**主设备** **host**
- 驱动 (struct device_driver)  —— 依附于MMC总线的MMC驱动( mmc_driver )

三者之间的关联图如下，每一个具体的总线均包括设备与驱动两部分，而每一个具体总线的所有添加的设备均链接至device下，每一个总线的所有注册的驱动均链接至drivers，而bus接口所有实现的功能也可以大致分为总线的注册、设备的注册、驱动的注册这三个部分。

![设备-总线-驱动关联图](https:/raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/f458025b97d662eade4badd77c4d0d83/aab329af4b5b41195c2ef318c21f9036.jpeg)


设备和对应的驱动必须依附于同一种总线

### 一、MMC驱动抽象模型

MMC驱动模型也是基于实际的硬件连接进行抽象的

- 针对通信总线，抽象出**mmc_bus**；
- 针对mmc controller，该子系统抽象为**mmc_host**，用于描述一个进行设备通信的控制器，提供了相应的访问接口（记为mmc_host->request）；
- 针对mmc、sd、tf卡具体设备，该子系统抽象为**mmc_card**，用于描述卡信息。mmc子系统提供年rescan接口用于mmc card的注册；
- 针对mmc、sd、tf，mmc子系统完成了统一的**mmc driver**，针对mmc总线规范以及SD规范，其已经详细的定义了一个存储卡的通信方式、通信命令，因此LINUXmmc子系统定义了mmc driver，用于和mmc、sd、tf等卡的通信，而**不需要**驱动开发人员来开发卡驱动。

![bus driver host card 关联图](https:/raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/f458025b97d662eade4badd77c4d0d83/0854870f47ba4c8366add07b619817d1.jpeg)


特点:

1. mmc总线模型仅注册一个驱动类型，即mmc driver
2. 一个mmc host与一个mmc card绑定
3. mmc card属于热插拔的设备，而mmc card的创建主要由mmc host负责探测与创建，mmc host根据卡在位检测引脚，当检测到mmc card的存在后，即创建mmc card，同时注册至mmc bus上，并完成与mmc driver的绑定操作。
4. host和card可以分别理解为 MMC device的两个子设备：MMC主设备和MMC从设备，其中host为集成于MMC设备内部的MMC controller，card为MMC设备内部实际的存储设备。

### 二、SDIO驱动抽象模型

sdio总线驱动模型和mmc类似，结构体上的区别为其driver类型为sdio_driver，并增加了sdio_func结构体变量（该结构体包含了该sdio设备相关的厂商id、设备id，同时包含了mmc_card）

因sdio主要突出接口概念，其设备端可以连接wifi、gps等设备，因此其外设备驱动**需要**由驱动工程师自己实现，sdio驱动模块不提供对应的驱动。

### 三、MMC/SDIO总线

总线接口实现的功能可分为总线的注册、设备的注册、驱动的注册这三个部分。

#### 1. 总线结构体定义

结构体定义位于```core\bus.c```

```c
static struct bus_type mmc_bus_type = {
  // 总线名称
    .name        = "mmc",
    .dev_groups    = mmc_dev_groups,
    // match接口用于实现mmc card与mmc driver的匹配检测，返回值均为1；
    .match        = mmc_bus_match,  
    // 应用层通知接口,用于添加该mmc bus的uevent参数（在调用device_add时，会调用kobject_uevent向应用层发送设备添加相关的事件,而kobject_uevent会调用该device所属bus和class的uevent接口，添加需要发送到应用的event参数
    .uevent        = mmc_bus_uevent,
    // probe接口主要用于mmc card与mmc driver匹配成功后，则会调用该mmc bus的probe接口实现探测操作；
    .probe        = mmc_bus_probe,
    // remove接口主要用于mmc card与mmc driver解绑时，调用该接口，进行remove操作（对于mmc drivemmc_ops
    .shutdown    = mmc_bus_shutdown,
    // pm是电源管理相关的接口。
    .pm        = &mmc_bus_pm_ops,
};
```

**总线匹配接口 .mmc_bus_match**
当向linux系统总线添加设备或驱动时，总是会调用各总线对应的match匹配函数来判断驱动和设备是否匹配。

此处的mmc_bus_match并没有进行匹配检测，直接返回1，表示mmc子系统实现的mmc driver可匹配所有注册至mmc bus上的mmc card

**\*sdio总线结构体**
位于```sdio_bus.c```

```c
static struct bus_type sdio_bus_type = {
    .name        = "sdio",
    .dev_groups    = sdio_dev_groups,mmc_ops
    .match        = sdio_bus_match,  // 根据id_table来匹配
    .uevent        = sdio_bus_uevent,
    .probe        = sdio_bus_probe,
    .remove        = sdio_bus_remove,
    .pm        = &sdio_bus_pm_ops,
};
```

#### 2. 总线注册

调用入口位于```core/core.c```，通过```mmc_init()```实现。

*core/core.c*

```c
subsys_initcall(mmc_init);

static int __init mmc_init(void)
{
    int ret;
    // 将mmc总线注册到linux的总线系统中,管理块设备
    ret = mmc_register_bus();

    // 注册mmc_host_class
    ret = mmc_register_host_class();

    // 注册sido总线到linux的总线系统中,管理sdio接口类型的设备
    ret = sdio_register_bus();

    return 0;
}
```

主要工作是：

a. ```mmc_register_bus```注册mmc总线，这个总线主要是为card目录里实现的mmc设备驱动层和mmc控制器实例化一个mmc(包括sd/sdio)设备对象建立的。

b. ```sdio_register_bus```这是sdio的部分，它比较特殊，需要额外的一条总线

具体包括两个方面：

- 利用 bus_register() 注册 mmc_bus，包括mmc总线和sdio总线。对应sysfs下的 /sys/bus/mmc/ 目录。
- 利用 class_register() 注册 mmc_host_class 。对应sysfs下的 /sys/class/mmc_host 目录。

*core/bus.c*

```c
int mmc_register_bus(void)
{
    // 实际调用内核接口,注册总线
    return bus_register(&mmc_bus_type);
}
```

*core/sdio_bus.c*

```c
int sdio_register_bus(void)
{
    return bus_register(&sdio_bus_type);
}
```

#### 3. 驱动注册

mmc_dirver的注册、注销接口是对内核函数的封装。实现将mmc_driver注册到mmc_bus总线中。

调用入口位于```core/block.c```，通过```mmc_blk_init()```实现，先给出mmc设备结构体的定义：。

```c
static struct mmc_driver mmc_driver = {
    .drv        = {device_register
        .name    = "mmcblk",
        .pm    = &mmc_blk_pm_ops,
    },
    .probe        = mmc_blk_probe,  // probe回调函数
    .remove        = mmc_blk_remove,
    .shutdown    = mmc_blk_shutdown,
};
```

入口函数：

*core/block.c*

```c
module_init(mmc_blk_init);

static int __init mmc_blk_init(void)
{
    int res;

    // 注册mmc_rpmb_bus总线
    res  = bus_register(&mmc_rpmb_bus_type);

    res = alloc_chrdev_region(&mmc_rpmb_devt, 0, MAX_DEVICES, "rpmb");

    // 注册块设备，申请块设备号
    res = register_blkdev(MMC_BLOCK_MAJOR, "mmc");

    // 将mmc_driver注册到mmc_bus总线系统中
    res = mmc_register_driver(&mmc_driver);

    return 0;
}
```

*core/bus.c*

```c
int mmc_register_driver(struct mmc_driver *drv)
{
    drv->drv.bus = &mmc_bus_type;
    // 实际调用内核接口,注册设备到总线系统
    return driver_register(&drv->drv);
}

// 使用EXPORT_SYMBOL将函数以符号的方式导出给其他模块使用。
EXPORT_SYMBOL(mmc_register_driver);
```

主要步骤包括：

a. 通过 register_blkdev() 向内核注册块设备。（仅注册，初始化的其他操作在mmc_driver结构体的**prob接口**中完成）

借助该块设备驱动模型，将mmc card与vfs（虚拟文件系统）完成了关联，即可通过系统调用借助VFS模型实现对块设备的读写访问操作。

b. 调用 mmc_register_driver() 将 mmc_driver 注册到 mmc_bus 总线系统。简单封装，和大部分驱动注册方式一致。

**\*sdio驱动注册**
这两个接口的实现与mmc_driver的实现类似，均是简单的对driver_register/driver_unregister的封装（还有设置driver需要绑定的bus_type）

*sdio_uart.c*

```c
module_init(sdio_uart_init);

static int __init sdio_uart_init(void)
{
    // ……
    ret = tty_register_driver(tty_drv);

    ret = sdio_register_driver(&sdio_uart_driver);
    // ……
}
```

*sdio_bus.c*

```c
int sdio_register_driver(struct sdio_driver *drv)
{
    drv->drv.name = drv->name;
    drv->drv.bus = &sdio_bus_type;
    return driver_register(&drv->drv);
}
EXPORT_SYMBOL_GPL(sdio_register_driver);
```

#### 4. 设备注册

主要包括mmc card内存的申请、mmc card的注册、mmc card的注销等接口。

调用入口位于实际host设备的驱动文件中，通过```xxx_driver```实现。下面以mvsdio驱动为例分析。

*host/mvsdio.c*

```c
module_platform_driver(mvsd_driver);

static struct platform_driver mvsd_driver = device_register{
    .probe        = mvsd_probe,
    .remove        = mvsd_remove,
    .driver        = {
        .name    = DRIVER_NAME,
        .probe_type = PROBE_PREFER_ASYNCHRONOUS,
        .of_match_table = mvsdio_dt_ids,
    },
};

// 在probe回调中调用
static int mvsd_probe(struct platform_device *pdev)
{
    // ……
    // 实例化一个控制器对象
    mmc = mmc_alloc_host(sizeof(struct mvsd_host), &pdev->dev);
    // ……
    mmc->ops = &mvsd_ops;  // 控制器操作集
    // ……（一系列对控制器对象的初始化工作）
    ret = mmc_add_host(mmc);
    // ……
}

// 控制器操作集，编写控制器驱动的一个主要任务就是实现这个操作集
static const struct mmc_host_ops mvsd_ops = {
    .request        = mvsd_request,  // 最终执行硬件操作的函数，参数由核心层提供，由核心层更上一层的card设备驱动层向下调用
    .get_ro            = mmc_gpio_get_ro,  // 判断是否写保护
    .set_ios        = mvsd_set_ios,  // 配置控制器的函数
    .enable_sdio_irq    = mvsd_enable_sdio_irq,  // 与sdio相关
};
```

*host.c*

mmc host子系统提供了延迟队列机制，在执行mmc_alloc_host、mmc_add_host后，则完成了mmc card rescan延迟工作队列及其处理接口的创建```INIT_DELAYED_WORK```

若要触发mmc card rescan（即调度工作队列），则调用**mmc_detect_change**接口，即可触发mmc card rescan(即完成mmc_host->detect队列的调度)；

```c
struct mmc_host *mmc_alloc_host(int extra, struct device *dev)
{
    // ……
    // 将mmc_rescan指定为延时工作队列的工作函数
    INIT_DELAYED_WORK(&host->detect, mmc_rescan);
    // ……
}

EXPORT_SYMBOL(mmc_alloc_host);
```

```mmc_rescan```函数的大致调用流程如下，由mmc子系统通过mmc card的rescan机制，实现mmc card的自动检测及注册机制，依次完成了对sdio、sd和mmc设备的添加与移除操作。

换句话说，是使用事件的触发监控机制完成了卡(mmc,sd,sdio)的热插拔处理。

```c
mmc_rescan[core.c]-->
    mmc_rescan_try_freq[core.c]-->
        mmc_attach_sdio[sdio.c]-->
            mmc_attach_bus[core.c]
            mmc_sdio_init_card[sdio.c]-->
                mmc_alloc_card[bus.c]
            sdio_init_func[sdio.c]-->
                sdio_alloc_func[sdio_bus.c]
            mmc_add_card[bus.c]
            sdio_add_func[sdio_bus.c]
        mmc_attach_sd[sd.c]-->
            mmc_attach_bus[core.c]
            mmc_sd_init_card[sd.c]-->
                mmc_alloc_card[bus.c]
            mmc_add_card[bus.c]
        mmc_attach_mmc[mmc.c]-->
            mmc_attach_bus[core.c]
            mmc_init_card[mmc.c]-->
                mmc_alloc_card[bus.c]
            mmc_add_card[bus.c]
```

从mmc_rescan调用关系中可以看出，mmc设备注册的过程依次完成了sdio设备、sd卡和mmc卡设备的初始化。

**A. mmc_attach_sdio()**
SDIO卡初始化的入口

a. 向卡发送CMD5命令，该命令有两个作用：

第一，通过判断卡是否有反馈信息来判断是否为SDIO设备```mmc_send_io_op_cond()```：

1. 如果有响应，并且响应中的MP位为0，说明对应卡槽中的卡为SDIO卡，进而开始SDIO卡的初始化流程
2. 如果命令没有响应，则说明对应卡槽的卡为SD或MMC卡，进而开始SD/MMC卡的初始化流程(sdio卡时有魄力sdio协议，sd卡使用sd协议)
3. 如果有响应，且响应中的MP位为说明这个卡mmc_alloc_card不但是SDIO卡，同时也时SD卡，也就是所谓的combo卡，则进行combo卡的初始化流程mmc_sdio_ops)

第二，如果是SDIO设备，就会给host反馈电压信息，就是说告诉host，本卡所能支持的电压是多少多少。

b. 设置sdio卡的总线操作集```mmc_attach_bus()```，传入struct mmc_bus_ops类型的实现mmc_sdio_ops。

```c
void mmc_attach_bus(struct mmc_host *host, const struct mmc_bus_ops *ops)
{
    host->bus_ops = ops;
}
```

c. host根据SDIO卡反馈回来的电压要求，给其提供合适的电压```mmc_select_voltage()```

d. 对sdio卡进行探测和初始化```mmc_sdio_init_card()```

e. 注册SDIO的各个功能模块```sdio_init_func()```

f. 注册SDIO卡```mmc_add_card()```

g. 将所有SDIO功能添加到device架构中```sdio_add_func()```

**mmc_alloc_card():**
调用device模型对应的接口完成device类型变量的初始化，并完成mmc_card与mmc_host的绑定。

**mmc_add_card():**

1. 调用device_add，完成将该mmc_card注册至mmc bus上；
2. 设置mmc_card的状态为在位状态。

**sdio func**
 sdio_func的注册与注销接口对应于mmc_card的注册与注销接口。主要函数有sdio_alloc_func、sdio_add_func、sdio_remove_func、sdio_release_func（相比mmc card，多了针对acpi的配置调用）

**B. mmc_attach_sd()**
SD卡初始化的入口

a. 发送CMD41指令，（sd卡支持该指令，但mmc卡不支持，所以可以以此区分）```mmc_send_app_op_cond()```

b. 设置sdio卡的总线操作集```mmc_attach_bus()```，传入struct mmc_bus_ops类型的实现mmc_sd_ops。

c. 设置合适的电压```mmc_select_voltage()```

d. 调用```mmc_sd_init_card()```（探测和初始化），获取mmc card的csd、cid，并创建mmc_card，并对mmc card进行初始化（如是否只读等信息）

e.调用```mmc_add_card()```，将该mmc_card注册至mmc_bus中，该接口会调用device_register将mmc_card注册至mmc_bus上，而这即触发mmc_driver与mmc_card的绑定流程，从而调用mmc_driver->probe接口，即执行mmc block device的注册操作（待解决，没有找到device_register相关代码）。

**c. mmc_attach_mmc()**
mmc卡初始化入口

a. 发送CMD1指令```mmc_send_op_cond()```

b. 设置mmc卡的总线操作集```mmc_attach_bus()```，传入struct mmc_bus_ops类型的实现mmc_ops。

c. 选择一个card和host都支持的最低工作电压```mmc_select_voltage()```

d. 初始化card使其进入工作状态```mmc_init_card()```

e. 为card构造对应的mmc_card并且注册到mmc_bus中```mmc_add_card()```，之后mmc_card就挂在了mmc_bus上，会和mmc_bus上的block（mmc_driver）匹配起来。相应block（mmc_driver）就会进行probe，驱动card，实现card的实际功能（也就是存储设备的功能）。会对接到块设备子系统中。

上面多次提到了**mmc_bus_ops**结构体，这是一个定义在core/core.h中的，用于表示总线操作的结构体。

```c
struct mmc_bus_ops {
    void (*remove)(struct mmc_host *);
    void (*detect)(struct mmc_host *);
    int (*pre_suspend)(struct mmc_host *);
    int (*suspend)(struct mmc_host *);
    int (*resume)(struct mmc_host *);
    int (*runtime_suspend)(struct mmc_host *);
    int (*runtime_resume)(struct mmc_host *);
    int (*alive)(struct mmc_host *);
    int (*shutdown)(struct mmc_host *);
    int (*hw_reset)(struct mmc_host *);
    int (*sw_reset)(struct mmc_host *);
    bool (*cache_enabled)(struct mmc_host *);
    int (*flush_cache)(struct mmc_host *);
};
```

### 四、MMC设备控制器（mmc host）

#### 1. 控制器结构体定义

该模块最重要的数据结构为```mmc_host```，位于```core/host.h```文件，用于描述一个mmc controller

而围绕着mmc controller又定义了相应的数据结构，用于描述mmc controller的各种行为（包括针对该mmc controller的访问方法抽象而来的数据结构```mmc_host_ops```、该mmc controller相关的参数抽象而来的数据结构体```mmc_ios```、针对mmc card相关的电源管理及在位检测方法抽象而来的数据结构```mmc_bus_ops```）

部分成员如下：

- `struct device class_dev`:
  host对应的device，用于借助设备模型与系统中注册的设备、class完成关联。
- `const struct mc_host_ops *ops`:
  定义本mmc_host的操作接口，包括与mmccard通信的接口request、卡检测相关的接口等。
- 块设备相关参数，包括段大小、块大小等。
- `u32 caps` `u32 caps2`:
  指示host支持的功能特性，包括4bit数据模式、mmc high speed mode、sd high speed mode、spi mode、8bit data、noneremovable mode（emmc mode）、cd与wp引脚的active high/low等等。
  

## MMC驱动注册

调用入口位于mmc/core/block.c,将mmc_driver注册到mmc_bus总线中.

主要步骤包括：

- 通过 register_blkdev() 向内核注册块设备。
  
  借助该块设备驱动模型，将mmc card与vfs完成了关联，即可通过系统调用借助VFS模型实现对块设备的读写访问操作。

- 调用 driver_register() 将 mmc_driver 注册到 mmc_bus 总线系统。和其他驱动注册方式一致。

关键函数为：```platform_driver_register()  -->  driver_register()  -->  bus_add_driver()```

## MMC设备注册

每个host均有调用入口,使用moduSD/SDIO/MMC 驱动是一种基于 SDMMC 和 SD SPI 主机驱动的协议级驱动程序，目前已支持 SD 存储器、SDIO 卡和 eMMC 芯片。

le_platform_driver()宏实现。

驱动入口函数中将注册 platform_driver 和 platform_device ， name 均定义为 xxx_mmc 。根据驱动模型，最终会回调 xxx_mmc_driver 中的 probe() 函数： xxx_mmc_probe() 。

关键函数为: ```platform_device_add()  -->  device_add()  -->  bus_add_device()  /  bus_probe_device()```

## 注册过程(瑞芯微MMC驱动源码)

设备启动时，首先向linux系统注册mmc_bus和sdio_bus两条总线,用来管理块设备和sdio接口类型的设备。同时注册mmc_host_class类

```c
# core.c
subsys_initcall(mmc_init);
```

接着调用module_init向系统注册一条mmc_rpmb_bus总线、一个mmc块设备和mmc driver。

```c
# block.c
module_init(mmc_blk_init);
```

最后调用module_platform_driver，把mmc controler注册到platform总线，同时扫描一次挂载到mmc控制器上的设备。

```c
# meson-mx-sdio.c
module_platform_driver(mmc_pwrseq_emmc_driver);
```

## mmc卡(mmc type card)协议相关操作

mmc_ops提供了部分和mmc卡协议相关的操作。

**mmc_go_idle**
发送CMD0指令，GO_IDLE_STATE
使mmc card进入idle state。
虽然进入到了Idle State，但是上电复位过程并不一定完成了，这主要靠读取OCR的busy位来判断，而流程归结为下一步。

**mmc_send_op_cond**
发送CMD1指令，SEND_OP_COND
这里会设置card的工作电压寄存器OCR，并且通过busy位（bit31）来判断card的上电复位过程是否完成，如果没有完成的话需要重复发送。
完成之后，mmc card进入ready state。

**mmc_all_send_cid**
这里会发送CMD2指令，ALL_SEND_CID
广播指令，使card回复对应的CID寄存器的值。在这里就相应获得了CID寄存器的值了，存储在cid中。
完成之后，MMC card会进入Identification State。

**mmc_set_relative_addr**
发送CMD3指令，SET_RELATIVE_ADDR
设置该mmc card的关联地址为card->rca，也就是0x0001
完成之后，该MMC card进入standby模式。

**mmc_send_csd**
发送CMD9指令，MMC_SEND_CSD
要求mmc card发送csd寄存器，存储到card->raw_csd中，也就是原始的csd寄存器的值。
此时mmc card还是处于standby state

**mmc_select_card & mmc_deselect_cards**
发送CMD7指令，SELECT/DESELECT CARD
选择或者断开指定的car成员或者sending-data state依次来进行数据的传输

**mmc_get_ext_csd**
发送CMD8指令，SEND_EXT_CSD
这里要求处于transfer state的card发送ext_csd寄存器，这里获取之后存放在ext_csd寄存器中
这里会使card进入sending-data state，完成之后又退出到transfer state。

**mmc_switch**
发送CMD6命令，MMC_SWITCH
用于设置ext_csd寄存器的某些bit

**mmc_send_status**
发送CMD13命令，MMC_SEND_STATUS
要求card发送自己当前的状态寄存器

**mmc_send_cid**
发送CMD10命令，MMC_SEND_CID
要求mmc card回复cid寄存器

**mmc_card_sleepawake**
发送CMD5命令，MMC_SLEEP_AWAKE
使card进入或者退出sleep state，由参数决定。关于sleep state是指card的一种状态，具体参考emmc 5.1协议。

## DTS配置

参考瑞芯微RK3568，配置文件位于

```bash
ARM：arch/arm/boot/dts/  // 主要是芯片级的配置，无需修改
ARM64：arch/arm64/boot/dts/rockchip  // 主要是板级相关的配置，可以根据实际需求修改
```

对应文件为

```bash
arch\arm64\boot\dts\rockchip\rk3568.dtsi
arch\arm68\boot\dts\rockchip\rk3568-pinctrl.dsti
arch\arm64\boot\dts\rockchip\rk3568-evb1-v10.dts
```

rk3568.dtsi

```dtsi
    sdmmc0: mmc@fe2b0000 {
        compatible = "rockchip,rk3568-dw-mshc", "rockchip,rk3288-dw-mshc";
        reg = <0x0 0xfe2b0000 0x0 0x4000>;
        interrupts = <GIC_SPI 98 IRQ_TYPE_LEVEL_HIGH>;
        clocks = <&cru HCLK_SDMMC0>, <&cru CLK_SDMMC0>,
             <&cru SCLK_SDMMC0_DRV>, <&cru SCLK_SDMMC0_SAMPLE>;
        clock-names = "biu", "ciu", "ciu-drive", "ciu-sample";
        fifo-depth = <0x100>;
        max-frequency = <150000000>;
        resets = <&cru SRST_SDMMC0>;
        reset-names = "reset";
        status = "disabled";
    };
```

## 参考资料

1. [Linux MMC 驱动子系统](https://www.cnblogs.com/hueyxu/p/13751636.html)
2. [Linux设备驱动模型和sysfs文件系统](https://www.cnblogs.com/hueyxu/p/13659262.html)
3. [LINUX MMC子系统分析之一 概述](https://jerry-cheng.blog.csdn.net/article/details/104717742)
4. [LINUX设备驱动模型分析之一 总体概念说明](https://jerry-cheng.blog.csdn.net/article/details/102655073)
5. [LINUX MMC子系统分析之二 MMC子系统驱动模型分析（包括总线、设备、驱动）](https://jerry-cheng.blog.csdn.net/article/details/104717819?spm=1001.21device_add01.3001.6650.1&utm_medium=distribute.pc_relevant.none-task-blog-2%7Edefault%7ECTRLIST%7ERate-1-104717819-blog-104717742.pc_relevant_landingrelevant&depth_1-utm_source=distribute.pc_relevant.none-task-blog-2%7Edefault%7ECTRLIST%7ERate-1-104717819-blog-104717742.pc_relevant_landingrelevant&utm_relevant_index=2)
6. [LINUX设备驱动模型分析之二 总线（BUS）接口分析](https://jerry-cheng.blog.csdn.net/article/details/102709219)
7. [LINUX MMC子系统分析之三 MMC/SDIO总线接口分析](https://jerry-cheng.blog.csdn.net/article/details/104717889)
8. [LINUX设备驱动模型分析之三 驱动（DRIVER）接口分析](https://jerry-cheng.blog.csdn.net/article/details/102768085)
9. [LINUX MMC 子系统分析之六 MMC card添加流程分析](https://blog.csdn.net/lickylin/article/details/104718117)
10. [Linux内核4.14版本——mmc core(4)——card相关模块（mmc type card）](https://blog.csdn.net/yangguoyu8023/article/details/122554472)
11. [Rockchip RK3588 kernel dts解析之MMC_loitawu的博客-CSDN博客](https://blog.csdn.net/weixin_43245753/article/details/124461034?spm=1001.2101.3001.6650.7&utm_medium=distribute.pc_relevant.none-task-blog-2~default~BlogCommendFromBaidu~Rate-7-124461034-blog-100496004.pc_relevant_3mothn_strategy_and_data_recovery&depth_1-utm_source=distribute.pc_relevant.none-task-blog-2~default~BlogCommendFromBaidu~Rate-7-124461034-blog-100496004.pc_relevant_3mothn_strategy_and_data_recovery&utm_relevant_index=8)