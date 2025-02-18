---
title: Golang 不常被注意的特性
date: 2024-11-14 15:35:29
toc: true
mathjax: true
categories:
- Golang
tags:
- Golang
- 编程语言
---

# Golang 不常被注意的特性

阅读项目https://github.com/astaxie/build-web-application-with-golang进行的查漏补缺。

## 1. rune和byte类型

`rune`和`byte`是 go 内置的两种类型别名。其中`rune`是`int32`的别称，`byte`是`uint8`的别称。

在处理中文时，使用`rune`可以正确计算字符串长度（截取字符串也是这样）：

```go
fmt.Println(len("Go语言编程"))  // 输出：14  
// 转换成 rune 数组后统计字符串长度
fmt.Println(len([]rune("Go语言编程")))  // 输出：6
```

这是因为UTF-8使用1~4个字节编码

[参见]: https://www.cnblogs.com/cheyunhua/p/16007219.html	"详解 Go 中的 rune 类型"



## 2. slice的容量

每个slice 都对应一个底层数组（一个数组可以对应多个 slice），slice 可以视为一个结构体，包含了三个元素

- 一个指针，指向数组中`slice`指定的开始位置
- 长度，即`slice`的长度
- 最大长度，也就是`slice`开始位置到数组的最后位置的长度

举一个例子:

```go

// 声明一个数组
var array = [10]byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j'}
// 声明两个slice
var aSlice, bSlice []byte

aSlice = array[3:7]  // aSlice包含元素: d,e,f,g，len=4，cap=7(d~j)
// 从slice中获取slice
bSlice = aSlice[0:6] // 对slice的slice可以在cap范围内扩展，此时bSlice包含：d,e,f,g,h,i
```

最后一个操作合法是因为`aSlice`的 cap 默认设置为**切片**起始位置到**数组**末尾的长度。

如果在声明切片时显示指定cap，这样这个产生的新的slice就没办法访问最后的三个元素。：

```go
aslice = array[3:7:8]  // aSlice包含元素: d,e,f,g，len=4，cap=5(d~h)
bSlice = array[0:6]  // panic: runtime error: slice bounds out of range [:6] with capacity 5
```

初始容量和增长率规则：

**初始 `cap`**：取决于创建切片的方式（可以指定，也可以通过字面量决定）。

**增长率**：

- 当容量小于 1024 时，`cap` 按 2 倍增长。
- 当容量大于等于 1024 时，`cap` 按 25% 增长。

## 3. switch 优化

如果 `switch` 语句的条件是 **连续整数值**，Go 编译器可能会优化生成一个 **查找表**（jump table）来加速分支跳转，实现 O(1) 的跳转速度。

如果 `switch` 中的条件值是非连续但可以排序的，编译器可能会将 `switch` 语句转换为一种二分查找的形式，从而减少分支的比较次数。这种优化适用于较大的 `switch` 语句，尤其当分支数较多时。

如果 `switch` 语句中的条件是字符串或布尔值（或其他复杂类型），通常不会有查找表优化或二分查找优化，而是逐一比较每个分支，这与 `if-else` 的行为基本一致。

## 4. 区分 new、make 和 字面量初始化

在 Go 语言中，`new`、`make` 和字面量初始化（也称为字面量表达式）是用于创建和初始化变量的不同方式。它们之间有一些重要的区别，适用于不同的场景。下面将详细比较这三者：

### 4.1. `new` 函数

- **功能**：`new(T)` 分配了类型 `T` 的内存，并返回一个指向该类型零值的指针（即 `*T` 类型）。
- **适用类型**：`new` 可以用于任何类型（包括基本类型、结构体、数组、切片、map、channel 等）。**`new` 的使用相对较少**，主要用于在没有字面量初始化的情况下，快速获取一个类型的零值指针。在一些复杂的反射操作中，`new` 可以用来动态创建类型实例，特别是当你需要通过反射动态创建某个类型的实例时。在实际开发中，**大部分情况下开发者更倾向于使用**结构体、数组或切片字面量语法来进行初始化，而不是依赖 `new`。
- **返回值**：返回类型 `T` 的零值指针。
- **初始化行为**：`new` 不会初始化为非零值，只会分配零值（例如：`int` 为 `0`，`string` 为 `""`，`struct` 所有字段为零值）。

示例：

```go
x := new(int)   // 返回一个指向 int 类型零值的指针，x 的值是 *int 类型，初始为 0
y := new([]int) // 返回一个指向空切片（零值切片）的指针
m := new(map[string]int) // 返回一个 *map[string]int 类型的指针，值是 nil
```

### 4.2. `make` 函数

- **功能**：`make(T, args)` 用于初始化内建类型（`slice`、`map` 和 `channel`）。`make` 会为这些类型分配内存，并且初始化数据结构的内部字段（例如：`slice` 的底层数组，`map` 的哈希表等）。
- **适用类型**：只能用于 `slice`、`map` 和 `channel` 类型。
- **返回值**：返回类型 `T`，不是指针。返回的是已经初始化的类型，而不是类型的零值。
- **初始化行为**：`make` 会将这些类型的内部结构初始化为有效状态，使它们能够被使用。对于 `slice`，它会返回一个具有指定长度和容量的切片；对于 `map` 和 `channel`，它会分配内存并返回一个空的、有效的 `map` 或 `channel`。

示例：

```go
s := make([]int, 10)       // 创建一个长度为 10 的切片，元素为零值（0）
m := make(map[string]int)  // 创建一个空的 map
ch := make(chan int, 2)    // 创建一个缓冲区大小为 2 的 channel
```

### 4.3. 字面量初始化

- **功能**：字面量初始化是通过直接使用字面量来创建并初始化一个变量。你可以为变量的各个字段或元素指定值。
- **适用类型**：可以用于所有类型，包括基本类型、数组、结构体、切片、map 和 channel。
- **返回值**：返回已初始化的值，通常是一个变量，而不是指针（除非显式使用 `&` 来获取指针）。
- **初始化行为**：字面量初始化会直接赋值并初始化变量为指定值，可以是零值，也可以是非零值。

示例：

```go
x := 42                // int 类型变量，直接赋值为 42
s := []int{1, 2, 3}    // 创建并初始化一个切片
m := map[string]int{"a": 1, "b": 2} // 创建并初始化一个 map
b := Box{width: 10, height: 5}   // 创建并初始化一个结构体
```

### 4.4. 比较总结

| 特性             | `new`                                    | `make`                                    | 字面量初始化                               |
| ---------------- | ---------------------------------------- | ----------------------------------------- | ------------------------------------------ |
| **适用类型**     | 所有类型（包括基本类型、结构体、数组等） | 仅限内建类型（`slice`、`map`、`channel`） | 所有类型                                   |
| **返回值**       | 返回类型的零值指针（`*T` 类型）          | 返回初始化后的值（`T` 类型）              | 返回初始化后的值（`T` 类型）               |
| **初始化行为**   | 初始化为零值                             | 初始化为有效的非零值                      | 根据字面量的定义初始化，可能是零值或非零值 |
| **是否需要指针** | 返回一个指针                             | 返回一个值，非指针                        | 返回一个值，非指针（除非使用 `&`）         |
| **用途**         | 主要用于获取指向零值的指针               | 用于初始化切片、映射和通道                | 用于直接创建和初始化变量，最常用           |

## 4. 无类型常量和类型自动匹配

Golang 不支持变量的隐式转换。

> 在 Go 中，常量可以是**无类型常量**（untyped constant）。这意味着常量在声明时并不直接指定其类型，而是由使用该常量的上下文决定其类型。无类型常量的特性使得它们在赋值或作为表达式的部分时，可以自动地根据目标类型进行转换或匹配。

但是在常量和特定情况下，Go 会在编译时进行类型自动匹配（例如，将 `int` 类型的常量转为 `byte` 或 `float64` 类型）。

看下面的例子：

```go
package main

import "fmt"

const(
	WHITE = iota  // 无类型常量
	BLACK
	BLUE
	RED
	YELLOW
)

type Color byte

type Box struct {
	width, height, depth float64
	color Color
}

func (b *Box) SetColor(c Color) { 
	b.color = c 
}

func (bl BoxList) BiggestColor() Color {
	v := 0.00
	k := Color(WHITE)  // 进行显示转换
	for _, b := range bl {
		if bv := b.Volume(); bv > v {
			v = bv
			k = b.color
		}
	}
	return k
}

func (bl BoxList) PaintItBlack() {
	for i := range bl {
		bl[i].SetColor(BLACK)  // 没有进行显式转换
	}
}
```

注意到`BiggestColor`和`PaintItBlack`方法中有对于常量`WHITE`的赋值有两个不同的写法。

- `k := Color(WHITE)`强制进行了类型转换，这是因为**变量`k`**会自动推断为等号右侧的类型，如果不进行类型转换，`k`会被视为`iota`的默认类型`int`，使得`k = b.color`这一行出现编译器报错：**无法将 'b. color' (类型 Color) 用作类型 int**。
- `bl[i].SetColor(BLACK) `没有参数进行类型转换，`BLACK`传入`SetColor` 函数后自动变为了`Color`类型（可以通过DEBUG证实）。这是因为`Color`是`byte`类型的别名，而`int`类型能够安全转化为`byte`类型，因此编译器自动进行了转换。

这里如果将`WHITE = iota`语句改为`WHITE Color = iota`，即可省去`BiggestColor`方法中的显示转换。

[更详细可见博客]: https://www.cnblogs.com/apocelipes/p/17235955.html	"小心golang中的无类型常量"

## 5. 指针作为receiver

Go 中 method 可以作用于任何自定义类型中（不只是 struct，可以是`type` 声明的任何类型），此时这个类型称为方法的 receiver。

用Rob Pike的话来说就是：

> "A method is a function with an implicit first argument, called a receiver."

也就是说，可以把 receiver 当作方法的第一个参数（如 Python 的 self），因此，想要区分在普通类型和指针上方法的特性，就可以使用函数的值传递和引用传递的视角来解读：如果一个方法想要修改结构体内的成员，那么这个方法需要定义在指针上；否则，方法的接收者实际上只是结构体的一个 copy。

此外，Go能智能地识别调用的是**指针的方法**还是**非指针的方法**：

- 如果一个method的receiver是*T,你可以在一个T类型的实例变量V上面调用这个method，而不需要&V去调用这个method；
- 如果一个method的receiver是T，你可以在一个*T类型的变量P上面调用这个method，而不需要 *P去调用这个method；

回顾 4 的例子：

```go
package main

import "fmt"

const(
	WHITE = iota  // 无类型常量
	BLACK
	BLUE
	RED
	YELLOW
)

type Color byte

type Box struct {
	width, height, depth float64
	color Color
}

// 因为要修改结构体成员，所以要使用指针作为方法的接收者。
func (b *Box) SetColor(c Color) { 
	b.color = c  // 不需要写作`*b.color = c`或`b->color = c`，Go会自动识别这是个指针。
}

func (bl BoxList) PaintItBlack() {
	for i := range bl {
        bl[i].SetColor(BLACK)  // 调用指针上的方法。这里不需要写作`&(bl[i]).SetColor(BLACK)`，同样，Go会自动识别。
	}
}
```

## 6. 结构体匿名字段成员重载和method重写

我们知道，Go 结构体允许存在匿名字段：

```go
type Human struct {
	name string
	age int
	weight int
}

type Student struct {
	Human  // 匿名字段，那么默认Student就包含了Human的所有字段
    int    // 任意的内置类型或自定义类型都可以作为匿名字段
}

func main() {
	// 初始化学生Jane
    jane := Student{Human:Human{"Jane", 35, 100}, int: 100}
	// 可以直接通过 Human 的实例访问匿名字段的方法
	fmt.Println("Her name is ", jane.name)  // "Jane"
    fmt.Println("Her preferred number is", jane.int)  // 100
```

可以很方便地在 Student **重载** Human 的字段：

```go
type Human struct {
	name string
	age int
	weight int 
}

type Student struct {
	Human
    int
    weight int  // 重载Human的相同字段
}

func main() {
	// 初始化学生Jane
    jane := Student{Human:Human{"Jane", 35, 100}, int: 100, weight: 70}
	// 最外层的优先访问
	fmt.Println("Her weight is ", jane.weight)  // 70
    // 如果我们要访问Human的weight字段
    fmt.Println("Human's weight is", jane.Human.weight)  // 100
}
```

如果匿名字段拥有方法，那么包含这个匿名字段的结构体也能调用这个方法：

```go
type Human struct {
	name string
	age int
	phone string
}

type Student struct {
	Human //匿名字段
	school string
}

//在human上面定义了一个method
func (h *Human) SayHi() {
	fmt.Printf("Hi, I am %s you can call me on %s\n", h.name, h.phone)
}

func main() {
	mark := Student{Human{"Mark", 25, "222-222-YYYY"}, "MIT"}
	mark.SayHi()  // 直接调用匿名字段的方法
}
```

同样，可以对这个方法进行重写：

```go
type Human struct {
	name  string
	age   int
	phone string
}

type Student struct {
	Human  //匿名字段
	school string
}

// Human定义method
func (h *Human) SayHi() {
	fmt.Printf("Hi, I am %s you can call me on %s\n", h.name, h.phone)
}

// Student的method重写Human的method
func (s *Student) SayHi() {
	fmt.Printf("Hi, I am %s, I study in %s. Call me on %s\n", s.name,
		s.school, s.phone)
}

func main() {
	mark := Student{Human{"Mark", 25, "222-222-YYYY"}, "MIT"}
	mark.SayHi()  // 调用重载后的方法
	mark.Human.SayHi()  // 重载前的方法
}
```

