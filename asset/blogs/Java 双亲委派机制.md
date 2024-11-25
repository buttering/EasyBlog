---
title: Java 双亲委派机制
date: 2024-11-25 17:24:29
toc: true
mathjax: true
tags:
- Java
- JVM
- 双亲委派机制
- 类加载器
---

# Java 双亲委派机制

一句话总结：自己不加载，交给父级加载器加载；打破双亲委派，就是自己去加载，**继承 ClassLoder 并重写 loadClass 方法。**

## Java 类声明周期

![image-20241125131246682](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/8e8733471e0331beb1c691c2212411dc/35e1606a361014016fe91b60e8a0e143.png)

Java 源文件到最终运行需要经过**编译**和**加载**两个阶段。

- 编译：Java  编译器将源代码编译为 .class（Java 字节码）文件
- 加载：将字节码文件加载到 JVM 内存中，得到一个 `Class` 对象（反射常用，可用于创建对应实例）。

## 类加载器

 JVM 提供了三类加载器

- Bootstrap ClassLoader（引导类加载器）：最高级的类加载器，加载 Java 的核心类库；会去寻找`$JAVA_HOME/lib`下的资源，如 java.lang, java.util, rt.jar, resource.jar。
- Extension ClassLoader（扩展类加载器）：加载对核心类库的扩展，查找`$JAVA_HOME/lib/ext`。
- Application ClassLoader（应用类加载器）：当前应用（class path）的所有 jar 包和类文件，即程序员编写的代码和第三方类库。

除了Bootstrap ClassLoader 是C/C++实现，其他类加载器都继承了`java.lang.ClassLoader`

此外可以通过继承`java.lang.ClassLoader`来实现自定义的类加载器。

## 父委托模型

类加载器之间具有层级关系，如下图。这个层级并不是通过继承实现的，而是子加载器使用一个字段`parent`保存了父加载器的变量（组合）。

![image-20241125133318339](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/8e8733471e0331beb1c691c2212411dc/57baf3b168069c28ea1b6e87fbda2068.png)

具体的加载过程即是按照类加载器的关系由下至上逐层进行委派。父加载器无法加载时，子加载器才会尝试用自己的规则加载。

![image-20241125133643297](https://raw.githubusercontent.com/buttering/EasyBlogs/master/asset/pictures/8e8733471e0331beb1c691c2212411dc/23c1b49f803990e1f7c6eb6dd309801a.png)

## 优点

- 安全：为类加载过程提供了优先级，可以避免核心类库的类被覆盖。
- 可以避免重复加载：一个类（根据限定名区分，如`com.keats.test.A`）只会被加载一次（首次被 new 或调用静态方法时）。
  - 每个类加载器都有一个内部缓存，存储已经加载的类。
  - 类加载器在 `findLoadedClass()` 中会检查缓存，如果该类已经加载过，直接返回缓存中的类对象，而不会重复加载。


## 破坏双亲委派

通过重写自定义类中的`loadClass` 方法，可以破坏双亲委派原则。破坏双亲委派机制的常见场景如 Java 应用服务器中的插件机制、OSGi 框架等。

```java
public class CustomClassLoader extends ClassLoader {
    @Override
    public Class<?> loadClass(String name) throws ClassNotFoundException {
        // 不再委派给父加载器，直接使用自定义加载方式
        return findClass(name); 
    }

    @Override
    protected Class<?> findClass(String name) throws ClassNotFoundException {
        // 自定义的类加载逻辑，省略细节
        return super.findClass(name);
    }
}
```

改进是使用`findClass`方法。`findClass()` 是 `loadClass()` 的最后一步，用来**自行实现类的查找和加载**。当父类加载器无法加载某个类时，本加载器通过 `findClass()` 尝试加载该类。