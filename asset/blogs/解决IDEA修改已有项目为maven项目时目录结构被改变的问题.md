---
title: 解决IDEA修改已有项目为maven项目时目录结构被改变的问题
date: 2020-09-18 17:50:07
toc: true
mathjax: true
tags: 
- 杂项
- idea
- maven
---

Idea可以在项目根目录上右键选择“添加框架支持”，选择maven，为项目添加Maven支持。
![](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/446167091db518e4736eeacdebd957da/dcb96eca0348bd69ee5a0e6ed0f3b35b.png)
但这样会导致原有项目的目录结构被破坏。

更好的方法是在根目录添加pom.xml文件
在\<build>标签内添加 \<sourceDirectory>标签，并填入源码根目录的路径名

接着右键点击pom.xml文件,选择Add as Maven project,即可在不修改目录结构的情况下完成转换.
![在这里插入图片描述](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/446167091db518e4736eeacdebd957da/71ccae19e510f3b114d58a648796704c.png)
pom.xml 代码示例

```xml
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>


    <groupId>groupId</groupId>
    <artifactId>frontend</artifactId>
    <version>1.0-SNAPSHOT</version>
    <build>
        <sourceDirectory>src</sourceDirectory>
        <plugins>
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-compiler-plugin</artifactId>
                <configuration>
                    <source>1.8</source>
                    <target>1.8</target>
                    <encoding>GBK</encoding>
                </configuration>
            </plugin>
        </plugins>
    </build>

    <dependencies>

    </dependencies>



</project>
```