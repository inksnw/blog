---
title: "Rust学习:包与泛型"
date: 2022-09-09T15:39:58+08:00
tags: ["rust"]
---

## 包、Crate和模块

rust的模块系统包括:

- 包: cargo的一个功能，它允许你构建、测试、分享crate。一个包可以包含多个二进制crate和一个可选的库crate
- crate: 一个模块的树形结构，它形成了库或二进制项目
- 模块(mod)和`use`: 模块通过use来使用，允许你控制作用域和路径的私有性
- 路径(path): 一个命名例如结构体、函数或模块等项的方式

### 包和crate

crate是一个二进制项或库。 `crate root`是一个源文件，crate 是一个二进制项或者库。Rust 编译器以`crate root`为起始点，构成你的 crate 的根模块。 crate将一个作用域相关的功能分组到一起，使得这些功能可以方便的在多个项目间共享。

包提供一系列功能的一个或多个crate。一个包会包含有一个 Cargo.toml 文件，阐述如何去构建这些 crate。一个包中至多 只能包含一个库crate(library crate)；包中可以包含任意多个二进制 crate(binary crate)；包中至少包含一个crate，无论是库的还是二进制的。

### 定义模块控制作用域和私有性

模块让我们可以将一个crate中的代码进行分组，以提高可读性与重用性。模块还可以控制项的私有性，crate中所有项(模块、函数、结构体、方法、枚举、常量)都是私有的，需要使用`pub`修饰才能暴露给外部。

创建一个lib可以通过命令`cargo new --lib libname`来进行。我们在`c10_package`下使用`cargo new --lib restaurant`创建一个新的名称为`restaurant`的库:

```bash
c10_package
├── Cargo.toml
├── restaurant
│   ├── Cargo.toml
│   └── src
│       └── lib.rs
└── src
    └── main.rs
```

在lib.rs中创建模块:

```rust
pub mod front_of_house {
    pub mod hosting {
        pub fn add_to_waitlist() {}
        pub fn seat_at_table() {}
    }
    pub mod serving {
        pub fn take_order() {}
        pub fn server_order() {}
        pub fn take_payment() {}
    }
}
pub fn eat_at_restaurant() {
    // 绝对逻辑使用模块中的项
    crate::front_of_house::hosting::add_to_waitlist();
    // 相对路径使用模块中的项
    front_of_house::hosting::add_to_waitlist();
}
```

### 使用use关键字将名称引入作用域

```rust
use crate::front_of_house::hosting;
use crate::front_of_house::hosting::add_to_waitlist as add_to;

pub fn eat_at_restaurant() {
    hosting::add_to_waitlist();
    add_to();
}
```

## 常见集合

vector允许在一个单独的数据结构中储存多于一个的值，所有值在内存中彼此相邻排列。vector中只能储存相同类型的值。

创建vector的方式:

```rust
fn main() {
    let mut x: Vec<i32> = Vec::new();
    x.push(2);
    println!("{}", x[0]);
    let mut y = vec![1, 2, 3];
    y.push(4);
    println!("{}", y[0]);
}
```

丢弃vector时也会丢弃其所有元素:

```rust
fn main() {
    let s = String::from("hello");
    {
        let mut v :Vec<String> = Vec::new();
        v.push(s)
    } // v离开作用域，其中的str也被一起被丢弃
    println!("{}", s); // 编译错误 borrow of moved value: `str`
}
```

读取vector中的元素:

```rust
fn main() {
    let nums = vec![1, 2];
    let one: &i32 = &nums[0];
    println!("nums[0]={}", one);
// nums[3]; // 索引越界会panic
    match nums.get(1) {
        Some(value) => println!("{}", value),
        None => println!("none"),
    }
    match nums.get(3) {
        Some(value) => println!("{}", value),
        None => println!("none"),
    }
}
```

遍历vector中元素:

```rust
fn main() {
    let mut nums = vec![1, 2];
    for num in &mut nums {
        print!("{}, ", num);
        *num += 1;
    }
    println!();
// 遍历vector(不可变方式)
    for num in &nums {
        print!("{}, ", num);
    }
}
```

使用枚举来储存多种类型:

```rust
enum SpreadsheetCell {
    Int(i32),
    Float(f64),
    Text(String),
}

let row = vec![
    SpreadsheetCell::Int(3),
    SpreadsheetCell::Text(String::from("blue")),
    SpreadsheetCell::Float(10.12),
];
```

### 字符串(String)

新建字符串:

```rust
fn main() {
    let mut s = String::new();
    s.push_str("hello");
    println!("{}", s);
    let s = String::from("world");
    println!("{}, {}", s, s.len()); // 5
    let s = String::from("世界");
    println!("{}, {}", s, s.len()); // 6
    let s="hello world".to_string();
    println!("{}", s);
}
```

更新字符串:

```rust
let mut s = String::from("hello");
s.push_str(" world");
println!("{}", s);

// 合并两个String
let s1 = String::from("hello");
let s2 = String::from("world");
let s3 = s1 + " " + &s2;
// s1的所有权移交给s3, 此时不能再使用s1
// println!("{}", s1) // value borrowed here after move
println!("{}", s3);

let s1 = String::from("hello");
let s2 = String::from("world");
let s3 = format!("{} {}", s1, s2);
println!("{}", s3);
```

### hash map

```rust
use std::collections::HashMap;

fn main() {
    let mut h: HashMap<String, i32> = HashMap::new();
    h.insert("k1".to_string(), 1);
    h.insert("k2".to_string(), 2);
    // 访问HashMap:
    match h.get("k2") {
        Some(v) => println!("v = {}", v),
        _ => println!("none"),
    }
    if let Some(v) = h.get("k1") {
        println!("v = {}", v);
    }
    // 遍历:
    for (k, v) in &h {
        println!("{}:{}", k, v)
    }
    //常用操作:
    h.entry("k1".to_string()).or_insert(100); // k1键不存在时插入
    let words_line = "go rust python java groovy scala python java groovy ";
    let mut word_count_map: HashMap<String, i32> = HashMap::new();
    for word in words_line.split_whitespace() {
        let count = word_count_map.entry(String::from(word)).or_insert(0);
        *count += 1;
    }
    for (k, v) in &word_count_map {
        println!("{}: {}", k, v);
    }
}
```

### `Result`与可恢复的错误

`Result<T, E>`枚举的定义大致如下:

```rust
enum Result<T, E> {
    Ok(T),
    Err(E),
}
```

`Ok(T)`的`T`代表成功时返回的结果，`Err`的`E`代表失败是返回的错误。

```rust
#[test]
#[should_panic]
fn error_handling() {
    use std::fs::File;
    let _file = match File::open("notexits.txt") {
        Ok(file) => file,
        Err(error) => {
            panic!("fail to open file: {:?}", error);
        },
    };
}
```

如果只是需要在失败场景下直接panic的话， `Result<T, E>`提供了`unwrap`和`expect`来简写上面的Match匹配:

```rust
#[test]
#[should_panic]
fn error_handling() {
    use std::fs::File;
    let _file = File::open("notexits.txt").unwrap();
}
```

```rust
#[test]
#[should_panic]
fn error_handling4() {
    use std::fs::File;
    let _file = File::open("notexits.txt").expect("fail to open file");
}
```

###  传播错误

```rust
use std::fs::File;
use std::io;
use std::io::Read;

fn main() {
    read_username_from_file().unwrap();
}


fn read_username_from_file() -> Result<String, io::Error> {
    let f = File::open("/Users/inksnw/Desktop/rust2/src/hello.txt");
    let mut f = match f {
        Ok(file) => file,
        Err(e) => return Err(e),
    };
    let mut s = String::new();
    match f.read_to_string(&mut s) {
        Ok(_) => Ok(s),
        Err(e) => Err(e),
    }
}
```

使用`?`运算符简写错误传播:

```rust
use std::fs::File;
use std::io;
use std::io::Read;

fn main() {
    read_username_from_file().unwrap();
}

fn read_username_from_file() -> Result<String, io::Error> {
    let mut f = File::open("/Users/inksnw/Desktop/rust2/src/hello.txt")?;
    let mut s = String::new();
    f.read_to_string(&mut s)?;
    Ok(s)
}
```

## 泛型、trait

泛型是具体类型或者其它属性的抽象替代，用于减少代码重复。 使用泛型并不会造成程序上性能的损失。Rust通过在编译时进行泛型代码的单态化(monomorphization)来保证效率。 单态化是通过填充编译时使用的具体类型，将通用代码转换为特定代码的过程。

### 在函数定义中使用泛型

```rust
// 定义泛型函数
fn largest<T: PartialOrd + Copy>(list: &[T]) -> T {
    let mut largest = list[0];
    for &item in list.iter() {
        if item > largest {
            largest = item;
        }
    }
    largest
}

fn main() {
    let nums = vec![3, 15, 9, 18, 7, 11];
    let largest_num = largest(&nums);
    println!("largest_num={}", largest_num);
    let nums = vec![3.2, 15.0, 9.0, 18.1, 7.0, 11.0];
    let largest_num = largest(&nums);
    println!("largest_num={}", largest_num);
}
```

### 在结构体和方法定义中使用泛型

```rust
struct Point<T> {
    x: T,
}
impl<T> Point<T> {
    fn get_x(&self) -> &T {
        &self.x
    }
}
fn main() {
    let p1 = Point {
        x: 12,
    };
    println!("{}", p1.get_x());
}
```

### trait

trait用于定义与其它类型共享的功能，类似于其它语言中的接口

- 可以通过trait以抽象的方式定义共享的行为
- 可以使用trait bounds指定泛型是任何拥有特定行为的类型
