 静默安装oracle时，日志文件中打印出如下语句

![img](https:/raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/3e3a4611a17fc0188ca478c778f2bc0d/1141f1734f91d0695f51b2ae96727e62.png)

提示Oracle NetConfiguration Assistant failed，原因是找不到libaio.so.1

这是缺少依赖

执行指令

```bash
yum -y install libaio* libaio-devel* 
```

删除home文件夹，再次执行

```bash
./runInstaller	 -silent	 -ignoreSysPrereqs	 -ignorePrereq	 -responseFile /ifs/oracle/database/response/db_install.rsp
```

安装成功


oracle完整依赖

```bash
yum -y install binutils* compat-libcap1* compat-libstdc++* gcc* gcc-c++* glibc* glibc-devel* ksh* libaio* libaio-devel* libgcc* libstdc++* libstdc++-devel* libXi* libXtst* make* sysstat* elfutils* unixODBC* unzip lrzsz
```