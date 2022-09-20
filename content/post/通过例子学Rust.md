---
title: "通过例子学Rust:1.Hello World"
date: 2022-09-20T22:01:13+08:00
tags: ["rust"]
subs: ["通过例子学Rust"]
---

手敲一遍 [通过例子学Rust](https://rustwiki.org/zh-CN/rust-by-example/index.html)

# 1.2.格式化输出

## 调试(Debug)

所有的类型,若想使用 `std::fmt` 的格式化打印,都要求实现至少一个可打印的`traits`.仅有一些类型提供了自动实现,比如`std`库中的类型.其它类型都必须手动实现.

`fmt::Debug`这个`trait`使这项工作变得相当简单.所有类型都能推导(`derive`,即自动创建)`fmt::Debug`的实现.但是`fmt::Display`需要手动实现.

```rust
//这个结构体不能使用`fmt::Display`或`fmt::Debug`来进行打印.
struct Unprintable(i32);
//`derive`属性会自动创建所需的实现,使这个`struct`能使用`fmt::Debug`打印
struct DebugPrintable(i32);
```

所有`std`库类型都天生可以使用`{:?}`来打印:

```rust
//`Structure`是一个包含单个`i32`的结构体
#[derive(Debug)]
struct Structure(i32);

//将`Structure`放到结构体`Deep`中.然后使`Deep`也能够打印.
#[derive(Debug)]
struct Deep(Structure);

fn main() {
    //使用`{:?}打印和使用`{}`类似
    println!("{:?} months in a year.", 12);
    println!("{1:?} {0:?} is the {actor:?} name.",
             "Slater",
             "Christian",
             actor = "actor's"
    );
    //输出 "Christian" "Slater" is the "actor's" name.
    //Structure也可以打印
    println!("Now {:?} will print", Structure(3));
    //使用`derive`的一个问题是不能控制输出的形式,如只想展示一个`7`
    println!("Now {:?} will print", Structure(7));
}
```

所以`fmt::Debug`确实使这些内容可以打印,但是牺牲了一些美感.Rust也通过`{:#?}`提供了"美化打印"的功能

```rust
#[derive(Debug)]
struct Persion<'a> {
    name: &'a str,
    age: u8,
}

fn main() {
    let name = "Peter";
    let age = 27;
    let peter = Persion { name, age };
    //美化打印
    println!("{:#?}", peter)
}
```

你可以通过手动实现 `fmt::Display` 来控制显示效果。

## 显示(Display)

`fmt::Debug`通常看起来不太简结,可能过手动实现`fmt::Display`来做到.`fmt::Display`采用`{}`标记.

```rust
//导入`fmt`模块使用`fmt::Display`可用
use std::fmt;
use std::fmt::Formatter;

struct Structure(i32);

//为了使用`{}`标记,必须手动为类型实现`fmt::Display`trait
impl fmt::Display for Structure {
    //这个trait要求`fmt`使用与下面函数完全一致的函数签名
    fn fmt(&self, f: &mut Formatter<'_>) -> fmt::Result {
        //仅将self的第一个元素写入到给定的输出流
        write!(f, "{}", self.0)
    }
}

fn main() {
    let ins = Structure(3);
    println!("{}", ins)
}
```

`fmt::Display`的效果可能比`fmt::Debug`简洁,但对于`std`库来说,这就有一个问题.模棱两可的类型该如何显示呢?举个例子,假设标准库对所有的`Vec<T>`都实现了同一种输出样式,那么他应该是哪种样式?下面两种中的一种吗?

- Vec<path>: /:/etc:/home/username:/bin (使用`:`)分隔
- Vec<number>: 1,2,3 (使用`,`分隔)

我们没有这样做,因为没有一种合适的样式适用于所有的类型,对于`Vec<T>`或者其它泛型容器(generic container), `fmt::Display`都没有实现.因此在这些泛型情况下要使用`fmt::Debug`.

这并不是一个问题,因为对于任何**非**泛型的**容器**类型,`fmt::Display`都能够实现.

```rust
use std::fmt;
use std::fmt::Formatter;

#[derive(Debug)]
struct MinMax(i64, i64);

impl fmt::Display for MinMax {
    fn fmt(&self, f: &mut Formatter<'_>) -> fmt::Result {
        write!(f, "{}, {}", self.0, self.1)
    }
}

#[derive(Debug)]
struct Point2D {
    x: f64,
    y: f64,
}

impl fmt::Display for Point2D {
    fn fmt(&self, f: &mut Formatter<'_>) -> fmt::Result {
        write!(f, "x: {}, y: {}", self.x, self.y)
    }
}

fn main() {
    let minmax = MinMax(0, 14);
    println!("Compare structures: ");
    println!("Display: {}", minmax);
    println!("Debug: {:?}", minmax);

    let big_range = MinMax(-300, 300);
    let small_range = MinMax(-3, 3);
    println!("The big range is {big} and the small is {small}", small = small_range, big = big_range);

    let point = Point2D { x: 3.3, y: 7.2 };
    println!("Compare points: ");
    println!("Display: {}", point);
    println!("Debug: {:?}", point);
    // 报错。`Debug` 和 `Display` 都被实现了，但 `{:b}` 需要 `fmt::Binary`
    // 得到实现。这语句不能运行。
    // println!("What does Point2D look like in binary: {:b}?", point);
}
```

`fmt::Display`被实现了,而`fmt::Binary`没有,因此`fmt::Binary`不能使用.`std::fmt`有很多这样的`trait`,它们都有各自的实现.这些内容将在后面的`std::fmt`章节中详细介绍.

## 实例测试: List

对一个结构体实现`fmt::Display`,其中的元素需要一个接一个的处理,这可能会很麻烦.问题在于每个`write!`都要生成一个`fmt::Resut`.正确的实现需要处理**所有**的Result. Rust专门为解决这个问题提供了`?`操作符.

在`write!`上使用`?`会像是这样:

```rust
//对`write`进行尝试,观察是否出错.若发生错误,返回相应的错误.
//否则继续执行后面的语句.
write!(f,"{}",value)?;
```

有了`?`,对一个`Vec`实现`fmt::Display`就很简单了:

```rust
use std::fmt;
use std::fmt::{Formatter, write};

struct List(Vec<i32>);

impl fmt::Display for List {
    fn fmt(&self, f: &mut Formatter<'_>) -> fmt::Result {
        let vec = &self.0;
        write!(f, "[")?;
        //使用`v`对`vec`进行迭代,并用`count`记录迭代次数
        for (count, v) in vec.iter().enumerate() {
            //对每一个元素,除第一个,加上逗号
            //使用`?`或`try!`来返回错误
            if count != 0 { write!(f, ", ")?; }
            write!(f, "{}", v)?;
        }
        write!(f, "]")
    }
}

fn main() {
    let v = List(vec![1, 2, 3]);
    println!("{}", v);
}
```

## 格式化

我们已经看到,格式化的方式是通过**格式字符串**来指定的:

- ```rust
  format!("{}",foo)->"3735928559"
  ```
- ```rust
  format!("0x{:X}",foo)->"0xDEADBEEF"
  ```
- ```rust
  format!("0o{:o}",foo)->"0o33653337357"
  ```

根据使用的**参数类型**是`X`,`O`还是**未指定**,同样的变量`foo`能够格式化成不同的格式.

这个格式化的功能是通过 trait 实现的,每种参数类型都对应一种 trait .最常见的格式化 trait 就是`Display`,它可以处理参数未指定的情况,比如 `{}`

```rust
use std::fmt::{self, Display, Formatter};

struct City {
    name: &'static str,
    lat: f32,
    lon: f32,
}

impl Display for City {
    // `f`是一个缓冲区 (buffer) ,些方法将格式化后的字符串写入其中
    fn fmt(&self, f: &mut Formatter<'_>) -> fmt::Result {
        let lat_c = if self.lat >= 0.0 { 'N' } else { 'S' };
        let lon_c = if self.lon > 0.0 { 'E' } else { 'W' };
        //`write!`和`format!`类似,但它会将格式化后的字符串写入
        write!(f, "{}: {:.3}°{} {:.3}°{}", self.name, self.lat.abs(), lat_c, self.lon.abs(), lon_c)
    }
}
```



在 [`fmt::fmt`](https://rustwiki.org/zh-CN/std/fmt/) 文档中可以查看[格式化 traits 一览表](https://rustwiki.org/zh-CN/std/fmt/#formatting-traits)和它们的参 数类型