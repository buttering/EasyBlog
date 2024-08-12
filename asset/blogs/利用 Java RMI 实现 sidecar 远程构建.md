---
title: 利用 Java RMI 实现 sidecar 远程构建
date: 2024-08-12 10:33:20
toc: true
mathjax: true
tags: 
- java
- k8s
- 软件构建
- RMI
---

## 任务背景
在研究如何使用 sidecar 模式实现 Buildfarm Worker 在自定义镜像中运行构建任务的过程中，注意到如下类：
![image.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/e38ecda0021bfe8bd9e1e5ce43308863/fd879079ca2df7c30b561b9e27efdfc9.png)
可以看到，`ProcessBuilder`的实例记录了一次构建任务的全部上下文，包括具体执行的指令，输出目录，环境变量。
Buildfarm Worker 在生成了这样一个对象后，运行`start`方法，生成`Process`类实例，在新的线程中执行任务，并通过`Process`对象获取任务的状态（标准输出、标准错误、是否结束、退出码等）。
因此，如果能在 sidecar 容器（这个容器从自定义镜像创建）中运行一个代理，将对`ProcessBuilder`和`Process`的操作发送到代理中，就可以实现我们的需求。
## RMI 概述
[RMI远程调用 - Java教程 - 廖雪峰的官方网站 (liaoxuefeng.com)](https://liaoxuefeng.com/books/java/network/rmi/)
Java RMI（Remote Method Invocation）是一个用于构建分布式应用程序的API。它允许开发者在不同的Java虚拟机（JVM）间调用远程对象的方法，像调用本地对象一样简单。以下是RMI的几个关键特性：

1. **远程对象**：在RMI中，远程对象是运行在服务器上的对象，客户端通过代理（stub）与其进行交互。
2. **代理和骨架**：RMI框架生成代理和骨架代码。代理在客户端，负责请求的发送；骨架在服务器，处理这些请求并调用相应的方法。
3. **序列化**：RMI使用Java的序列化机制将方法参数和返回值转换为字节流，以便通过网络传输。
4. **网络通信**：RMI通常使用TCP/IP协议在客户端与服务器之间建立可靠的通讯。
5. **简化开发**：开发者可以像调用本地方法那样调用远程方法，从而简化了分布式编程的复杂性。
6. **动态发现**：RMI注册中心（RMI Registry）允许客户端动态查找和绑定远程对象。

RMI适用于构建需要跨网络调用的分布式系统，尤其是在纯Java环境中。通过RMI，开发者可以创建灵活且易于维护的网络应用程序。
## 注意点
### 可序列化的对象
通过 Java RMI 传输的对象，必须是可序列化的（实现java.io.Serializable接口），而`ProcessBuilder`，`Process`对象都是不可序列化的。因此，不能直接将这两个对象进行传输。
### 生命周期
远程对象同样遵循JVM的垃圾回收机制。客户端对远程对象的引用是通过一个代理（stub）实现的。当客户端本地对应的对象由于某种原因失去引用时，会导致远程对象的引用状态变化。
举例两个会导致远程对象失去引用的情形：

- 局部变量超出作用域: 如果客户端在某个方法中定义了一个局部变量来持有对远程对象的引用，由于这个变量生命周期有限，当方法执行完后，变量会被垃圾回收。
- 将引用显式设置为 null: 如果客户端代码将持有远程对象的引用显式设置为 null，则该引用将失去对远程对象的引用。
## RMI 实现
### 对象代理
实现`ProcessBuilder`和`Process`的代理类
```java
package com.alicloud.basic_tech;

import java.io.File;
import java.rmi.RemoteException;
import java.rmi.server.UnicastRemoteObject;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

public class RemoteProcessBuilderImpl extends UnicastRemoteObject implements RemoteProcessBuilder {
    private String command;
    private File directory;
    private Map<String, String> environment;

    protected RemoteProcessBuilderImpl() throws RemoteException {
        super();
        environment = new HashMap<>();
    }

    @Override
    public RemoteProcessBuilder directory(File directory) throws RemoteException {
        this.directory = directory;
        System.out.println("Directory set to: " + (directory != null ? directory.getAbsolutePath() : "null"));
        return this;
    }

    @Override
    public RemoteProcessBuilder setEnvironmentVariable(String key, String value) throws RemoteException {
        this.environment.put(key, value);
        System.out.println("Environment variable set: " + key + " = " + value);
        return this;
    }

    @Override
    public Map<String, String> environment() throws RemoteException {
        return environment;
    }

    @Override
    public RemoteProcessBuilder command(List<String> command) throws RemoteException {
        this.command = String.join(" ", new ArrayList<>(command));
        System.out.println("Command set to: " + this.command);
        return this;
    }

    @Override
    public RemoteProcessBuilder command(String... command) throws RemoteException {
        this.command = String.join(" ", command);
        System.out.println("Command set to: " + this.command);
        return this;
    }

    @Override
    public RemoteProcess start() throws RemoteException {
        try {
            System.out.println("Starting process with command: " + command);
            ProcessBuilder builder = new ProcessBuilder("bash", "-c", command);
            if (directory != null) {
                builder.directory(directory);
                System.out.println("Process will run in directory: " + directory.getAbsolutePath());
            } else {
                System.out.println("Process will run in the default directory.");
            }

            // 设置环境变量
            Map<String, String> currentEnv = builder.environment();
            currentEnv.clear();  // 清空原有环境变量
            if (environment != null) {
                currentEnv.putAll(environment);
            }
            System.out.println("Environment variables set: " + currentEnv);

            Process process = builder.start();
            System.out.println("Process started successfully. PID: " + process.hashCode());

            return new RemoteProcessImpl(process);
        } catch (Exception e) {
            System.err.println("Error starting process: " + e.getMessage());
            throw new RemoteException("Error starting process", e);
        }
    }
}
```
```java
package com.alicloud.basic_tech;

import java.rmi.RemoteException;
import java.rmi.server.UnicastRemoteObject;
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.util.concurrent.TimeUnit;

public class RemoteProcessImpl extends UnicastRemoteObject implements RemoteProcess {
    private String stdOutput; // 存储标准输出
    private String stdError;  // 存储标准错误
    private Process process;
    private Thread outputThread;  // 输出线程
    private Thread errorThread;    // 错误线程

    public RemoteProcessImpl(Process process) throws RemoteException {
        this.process = process;
        System.out.println("Process started. Capturing output...");
        String command = String.join(" ", process.info().command().orElse("Unknown Command"));
        System.out.println("Command executed: " + command);

        captureOutput();
    }

    private void captureOutput() {
        // 线程处理标准输出
        outputThread = new Thread(() -> {
            StringBuilder stdOutputBuilder = new StringBuilder();
            try (BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()))) {
                String line;
                int lineCount = 0; // 计数读取的行数
                while ((line = reader.readLine()) != null) {
                    stdOutputBuilder.append(line).append("\n");
                    System.out.println("Standard Output (Line " + (++lineCount) + "): " + line);
                }
                System.out.println("Total lines read from Standard Output: " + lineCount);
            } catch (Exception e) {
                System.err.println("Error reading standard output: " + e.getMessage());
            }
            stdOutput = stdOutputBuilder.toString();  // 更新标准输出
        });

        // 线程处理标准错误
        errorThread = new Thread(() -> {
            StringBuilder stdErrorBuilder = new StringBuilder();
            try (BufferedReader reader = new BufferedReader(new InputStreamReader(process.getErrorStream()))) {
                String line;
                int lineCount = 0; // 计数读取的行数
                while ((line = reader.readLine()) != null) {
                    stdErrorBuilder.append(line).append("\n");
                    System.err.println("Standard Error (Line " + (++lineCount) + "): " + line);
                }
                System.err.println("Total lines read from Standard Error: " + lineCount);
            } catch (Exception e) {
                System.err.println("Error reading standard error: " + e.getMessage());
            }
            stdError = stdErrorBuilder.toString();  // 更新标准错误
        });

        outputThread.start();
        errorThread.start();
    }

    @Override
    public String getStdInput() throws RemoteException {
        return stdOutput;
    }

    @Override
    public String getStdError() throws RemoteException {
        return stdError;
    }

    @Override
    public int waitFor() throws RemoteException {
        try {
            process.waitFor();  // 阻塞等待进程完成
            outputThread.join();  // 等待输出线程结束
            errorThread.join();    // 等待错误线程结束
            System.out.println("Process finished successfully.");
            return process.exitValue();  // 返回进程的退出码
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt(); // 重新设置中断状态
            throw new RemoteException("Process was interrupted", e);
        }
    }

    @Override
    public boolean waitFor(long timeout, TimeUnit unit) throws RemoteException {
        boolean finished;

        try {
            finished = process.waitFor(timeout, unit);
            outputThread.join();  // 等待输出线程结束
            errorThread.join();    // 等待错误线程结束
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt(); // 重新设置中断状态
            throw new RemoteException("Process was interrupted while waiting", e);
        }

        if (finished) {
            System.out.println("Process finished within timeout.");
        } else {
            System.out.println("Process did not finish within timeout.");
        }

        return finished;
    }

    @Override
    public void destroy() throws RemoteException {
        if (process != null) {
            process.destroy(); // 优雅地终止进程
            System.out.println("Process destroyed");
        }
    }

    @Override
    public Process destroyForcibly() throws RemoteException {
        if (process != null) {
            process.destroyForcibly(); // 强制终止进程
            System.out.println("Process forcibly destroyed");
            return process;
        }
        return null;
    }

    @Override
    public int exitValue() throws RemoteException {
        try {
            return process.exitValue();
        } catch (IllegalThreadStateException e) {
            throw new RemoteException("Process has not finished yet.", e);
        }
    }
}
```
值的注意的是，`Porcess.getInputStream`获取的对象同样是不可序列化的，因此需要在代理对象中先将其输出收集存储，然后再以`String`类型传输。
### RMI 服务器
运行一个服务器，监听本地1099端口。将我们的代理对象注册到其中。
```java
package com.alicloud.basic_tech;

import java.rmi.registry.LocateRegistry;
import java.rmi.registry.Registry;

public class RMIServer {
    public static void main(String[] args) {
        try {
            RemoteProcessBuilder processBuilder = new RemoteProcessBuilderImpl();

            Registry registry = LocateRegistry.createRegistry(1099);
            registry.rebind("RemoteProcessBuilder", processBuilder);

            System.out.println("RMI Server is running and ready to manage processes.");
        }catch (Exception e) {
            e.printStackTrace();
        }
    }
}
```
### 客户端
在需要调用远程对象的地方，只需要用 RMI 客户端获取远程对象。这里写一个测试程序进行模拟。
sidecar 模式中，两个容器运行在同一个 Pod 中，共享网络空间，因此直接访问本地的 1099 端口即可。
```java
package com.alicloud.basic_tech;

import java.io.File;
import java.rmi.registry.LocateRegistry;
import java.rmi.registry.Registry;

public class RMIServerClient {
    public static void main(String[] args) {
        try {
            // 连接到 RMI 注册表
            Registry registry = LocateRegistry.getRegistry("localhost", 1099); // 修改服务器地址和端口
            // registry.list() // 查找所有可用的绑定对象
            RemoteProcessBuilder remoteProcessBuilder = (RemoteProcessBuilder) registry.lookup("RemoteProcessBuilder"); // 查找远程对象

            // 要输出的内容
            String logMessage = "Hello, World!";
            // 创建输出文件的路径
            String outputFilePath = "log";
            // 创建完整的命令
            String command = String.format("echo \"%s\" | tee %s", logMessage, outputFilePath);

            // 启动远程进程
            RemoteProcess remoteProcess = remoteProcessBuilder
                    .command(command) // 使用 bash 执行命令
                    .directory(new File("/mesh-tmp")) // 使用默认工作目录
                    .setEnvironmentVariable("PATH", "/usr/bin")
                    .start(); // 启动远程进程

            // 等待进程结束并获取退出代码
            int exitCode = remoteProcess.waitFor();
            String output = remoteProcess.getStdInput(); // 获取标准输出（如果有的话）
            String error = remoteProcess.getStdError(); // 获取标准错误

            // 打印输出结果
            System.out.println("Standard Output: \n" + output);
            System.out.println("Standard Error: \n" + error);
            System.out.println("Process exited with code: " + exitCode);
        } catch (Exception e) {
            e.printStackTrace();
        }
    }
}
```
## 测试
我们用 maven 将服务器和客户端编译为 jar 包（这里采用 fat jar 格式，引入guava库，使其可以在任何地方运行）。
在同一个 Pod 中，一个容器运行服务端：
```shell
java -jar RMIServer.jar
```
在另一个容器中，运行客户端，结果如下
![img.png](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/e38ecda0021bfe8bd9e1e5ce43308863/d10c88f869301b1238f53cfdff8e9d7c.png)