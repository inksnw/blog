---
title: "Rust学习:所有权"
date: 2022-09-09T00:12:03+08:00
tags: ["rust"]
---

## 所有权

### 栈与堆

>
> 栈和堆都是代码在运行时可供使用的内存，但是它们的结构不同。栈以放入值的顺序存储值并以相反顺序取出值。这也被称作 **后进先出**（*last in, first out*）。增加数据叫做 **进栈**（*pushing onto the stack*），而移出数据叫做 **出栈**（*popping off the stack*）。栈中的所有数据都必须占用已知且固定的大小。在编译时大小未知或大小可能变化的数据，要改为存储在堆上。 堆是缺乏组织的：当向堆放入数据时，你要请求一定大小的空间。内存分配器（memory allocator）在堆的某处找到一块足够大的空位，把它标记为已使用，并返回一个表示该位置地址的 **指针**（*pointer*）。这个过程称作 **在堆上分配内存**（*allocating on the heap*），有时简称为 “分配”（allocating）。（将数据推入栈中并不被认为是分配）。因为指向放入堆中数据的指针是已知的并且大小是固定的，你可以将该指针存储在栈上，不过当需要实际数据时，必须访问指针。
>
> 当你的代码调用一个函数时，传递给函数的值（包括可能指向堆上数据的指针）和函数的局部变量被压入栈中。当函数结束时，这些值被移出栈。
>
> 跟踪哪部分代码正在使用堆上的哪些数据，最大限度的减少堆上的重复数据的数量，以及清理堆上不再使用的数据确保不会耗尽空间，这些问题正是所有权系统要处理的。一旦理解了所有权，你就不需要经常考虑栈和堆了，不过明白了所有权的主要目的就是为了管理堆数据，能够帮助解释为什么所有权要以这种方式工作。

### 所有权规则

> 1. Rust 中的每一个值都有一个 **所有者**（*owner*）。
> 2. 值在任一时刻有且只有一个所有者。
> 3. 当所有者（变量）离开作用域，这个值将被丢弃。

#### 变量与数据交互的方式

```rust
  let x = 5;
  let y = x;
```

因为整数是有已知固定大小的简单值，所以有两个 `5` 被放入了栈中。

现在看看这个 `String` 版本：

```rust
let s1 = String::from("hello");
let s2 = s1;
println!("{}, world!", s1); // 编译错误: use of moved value: `s1`
```

为了避免出现内存二次释放的问题，上面的代码当`s1`被赋给`s2`后，`s1`不再有效，当s2, s1离开作用域时只有s2会清理变量的堆内存。 这里相当于`s1`被移动到了`s2`中。

![Snipaste_2022-09-09_00-36-00](http://inksnw.asuscomm.com:3001/blog/rust学习:所有权_1a6d8334026b6a1fe0deb0cfebd77490.png)

rust永远不会自动创建数据的"深拷贝"，如果确实需要深度复制String中堆上的数据，而不仅仅是栈上的数据，可以使用一个叫做`clone`的通用函数。

```rust
let s1 = String::from("hello");
let s2 = s1.clone();
println!("s1 = {}, s2 = {}", s1, s2);
```

只有在栈上的数据变量间赋值时才是拷贝(copy)，而对在堆上的数据变量间赋值时是移动(move)。 

### 所有权与函数

```rust
fn main() {
    let s = String::from("hello");  // s 进入作用域
    takes_ownership(s);             // s 的值移动到函数里 ...
                                    // ... 所以到这里不再有效
    let x = 5;                      // x 进入作用域
    makes_copy(x);                  // x 应该移动函数里，
                                    // 但 i32 是 Copy 的，所以在后面可继续使用 x

} // 这里, x 先移出了作用域，然后是 s。但因为 s 的值已被移走，
  // 所以不会有特殊操作

fn takes_ownership(some_string: String) { // some_string 进入作用域
    println!("{}", some_string);
} // 这里，some_string 移出作用域并调用 `drop` 方法。占用的内存被释放

fn makes_copy(some_integer: i32) { // some_integer 进入作用域
    println!("{}", some_integer);
}
```

### 返回值与作用域

```rust
fn main() {
    let s1 = gives_ownership();         // gives_ownership 将返回值
                                        // 移给 s1
    let s2 = String::from("hello");     // s2 进入作用域
    let s3 = takes_and_gives_back(s2);  // s2 被移动到
                                        // takes_and_gives_back 中,
                                        // 它也将返回值移给 s3
} // 这里, s3 移出作用域并被丢弃。s2 也移出作用域，但已被移走，
  // 所以什么也不会发生。s1 移出作用域并被丢弃

fn gives_ownership() -> String {             // gives_ownership 将返回值移动给
                                             // 调用它的函数
    let some_string = String::from("hello"); // some_string 进入作用域.
    some_string                              // 返回 some_string 并移出给调用的函数
}

// takes_and_gives_back 将传入字符串并返回该值
fn takes_and_gives_back(a_string: String) -> String { // a_string 进入作用域
    a_string  // 返回 a_string 并移出给调用的函数
}

```

### 引用与借用

引用（reference），它允许多处代码访问同一处数据，而无需在内存中多次拷贝。

```rust
fn main() {
    let s1 = String::from("hello");

    let len = calculate_length(&s1);

    println!("The length of '{}' is {}.", s1, len);
}

fn calculate_length(s: &String) -> usize {
    s.len()
}
```

`&`符号就是引用，它们允许你使用值但不获取其所有权。 `&s1`语法让我们创建一个指向值`s1`的引用，但是并不拥有它。因为并不拥有这个值，当引用离开作用域时其指向的值也不会被丢弃。

![Snipaste_2022-09-09_00-53-21](http://inksnw.asuscomm.com:3001/blog/rust学习:所有权_3c67e316b1c8c4e74cf12ccd3c326c06.png)





将获取引用作为函数参数称为借用（borrowing）。

对于引用的值默认是不允许修改的：

```rust
fn main() {
    let s = String::from("hello");
    change(&s);
}

fn change(some_string: &String) {
    some_string.push_str(", world");
}// 编译错误: cannot borrow immutable borrowed content `*some_string` as mutable
```

这个时候需要借助可变引用:

```rust
fn main() {
    let mut s = String::from("hello");
    change(&mut s);
}

fn change(some_string: &mut String) {
    some_string.push_str(", world");
}
```

- 在任意给定时间，**要么** 只能有一个可变引用，**要么** 只能有多个不可变引用。
- 引用必须总是有效的。

一个slice的例子

```rust
fn main() {
    let my_string = String::from("hello world");

    // `first_word` 适用于 `String`（的 slice），整体或全部
    let word = first_word(&my_string[0..6]);
    println!("{}", word);
    let word = first_word(&my_string[..]);
    println!("{}", word);
    // `first_word` 也适用于 `String` 的引用，
    // 这等价于整个 `String` 的 slice
    let word = first_word(&my_string);
    println!("{}", word);
    let my_string_literal = "hello world";

    // `first_word` 适用于字符串字面值，整体或全部
    let word = first_word(&my_string_literal[0..6]);
    println!("{}", word);
    let word = first_word(&my_string_literal[..]);
    println!("{}", word);
    // 因为字符串字面值已经 **是** 字符串 slice 了，
    // 这也是适用的，无需 slice 语法！
    let word = first_word(my_string_literal);
    println!("{}", word);
}

fn first_word(s: &str) -> &str {
    let bytes = s.as_bytes();

    for (i, &item) in bytes.iter().enumerate() {
        if item == b' ' {
            return &s[0..i];
        }
    }

    &s[..]
}

```

## 结构体

### 定义结构体并创建结构体实例

```rust
struct User {
    name: String, 
    score: u32,
}
let user1 = User {
    name: String::from("jane"),
    score: 100,
};
println!("{}-{}-{}", user1.name, user1.score);
```

### 从其它结构体实例创建新实例

```rust
//user1需要也是User的实例,同字段会覆盖,user1优先级更高
let user2 = User{
    name: String::from("joyce"),
    ..user1
};
println!("{}-{}-{}", user2.name, user2.score, user2.enabled);
```

### 元组结构体

```rust
struct Point(i32, i32);
let p1 = Point(100, 50);
let p2 = Point(100, 0);
println!("{}, {}", p1.0, p1.1);
println!("{}, {}", p2.0, p2.1);
```

### 打印结构体调试信息

```rust
#[derive(Debug)]
struct User {
    name: String, 
    score: u32,
    enabled: bool,
}
let user1 = User {
    name: String::from("jane"),
    score: 100,
    enabled: true, 
};
println!("{:?}", user1);
println!("{:#?}", user1);
```

###  结构体方法

```rust
#[derive(Debug)]
struct Bird {
    name: String,
    weight: i32,
}

impl Bird {
    fn get_name(&self) -> &str {
        &(self.name[..])
    }

    fn get_weight(&self) -> i32 {
        self.weight
    }
    fn change(&mut self) {
        self.weight = self.weight + 1
    }
    //静态方法
    fn new(name: &str, weight: i32) -> Bird {
        Bird {
            name: String::from(name),
            weight,
        }
    }
}

fn main() {
    let mut bird = Bird::new("haha", 3);
    println!("{:#?}\n, {}, {}", bird, bird.get_name(), bird.get_weight());
    bird.change();
    println!("{:#?}\n, {}, {}", bird, bird.get_name(), bird.get_weight());
}
```

如果想要在方法中改变调用方法的实例，需要将第函数一个参数改为 `&mut self` ,要修改一个struct实例,需要将基声明为mut的

### 枚举

```rust

enum Message {
    Write(String), // 单独一个String
    ChangeColor(i32, i32, i32), // 元组
}

impl Message {
    fn call(&self) {
        // 在这里定义方法体
    }
}

fn main() {

    let m = Message::Write(String::from("hello"));
    m.call();
}
```

