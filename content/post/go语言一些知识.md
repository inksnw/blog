---
title: "Go语言一些知识"
date: 2022-04-02T21:24:41+08:00
tags: ["golang"]
---
<!--more-->
## 常规

### 可变参数

```go
package main

import "fmt"

type User struct {
	Id   int
	Name string
}

type UserAttrFunc func(user *User)
type UserAttrFuncs []UserAttrFunc

func (t UserAttrFuncs) apply(u *User) {
	for _, fun := range t {
		fun(u)
	}
}

func withId(id int) UserAttrFunc {
	return func(user *User) {
		user.Id = id
	}
}

func withName(name string) UserAttrFunc {
	return func(user *User) {
		user.Name = name
	}
}

func NewUser(fs ...UserAttrFunc) *User {
	u := new(User)
	UserAttrFuncs(fs).apply(u)
	return u
}

func main() {
	ins := NewUser(withId(2), withName("xiaoming"))
	fmt.Printf("%+v", ins)
}

```

### Nil!=nil

```go
package main

import (
	"fmt"
	"reflect"
)

func main() {
	var f func()
	type ss struct {
		Name string
	}
	var a *ss
	fmt.Println(f, a)

	list := []interface{}{f, a}
	for _, item := range list {
		if item == nil {
			fmt.Println("nil")
		}
	}
	for _, item := range list {
		if reflect.ValueOf(item).IsNil() {
			fmt.Println("nilllll")
		}
	}

}
```

### 读写关闭的chan

```go
package main

import "fmt"

func main() {
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2
	ch <- 3
	close(ch)
	//读正常
	for {
		item, ok := <-ch
		if ok {
			fmt.Print(item)
		} else {
			fmt.Printf("已经读完了 %d,%t \n", item, ok)
			break
		}
	}
	ch <- 5 //写panic
}

```

## defer

### defer参数问题

```go
package main

import "fmt"

func main() {
	a := 1
	//defer func() {
	//	fmt.Println(a)
	//}() 输出2

	defer func(input int) {
		fmt.Println(input)
	}(a) //输出1
	a++
}

```

### defer非闭包链式调用

```go
package main

import "fmt"

type test struct {
}

func (t *test) do(i int) *test {
	fmt.Println(i)
	return t
}

func main() {
	a := test{}
	//defer a.do(1).do(2)
	//a.do(3)
	//输出 1 3 2,defer如果不在闭包函数中,链式调用只有最后一个函数会延时执行

	defer func() {
		defer a.do(1).do(2)
	}()
	a.do(3)
	//输出3,1,2
}

```

### defer 执行顺序

```go
package main

import "fmt"

func main() {
	for i := 0; i < 3; i++ {
		//defer func() {
		//	fmt.Println(i)
		//}() 输出333

		//tmp := i //添加临时变量
		//defer func() {
		//	fmt.Println(tmp)
		//}()
		//输出2,1,0 栈结构,先入后出

		//传参方式
		defer func(input int) {
			fmt.Println(input)
		}(i) //输出2,1,0
	}
}

```

defer和panic执行顺序

```go
package main

import "fmt"

func main() {
	defer func() {
		defer func() { fmt.Println("打印前") }()
		defer func() { fmt.Println("打印中") }()
		defer func() { fmt.Println("打印后") }()
		panic("触发异常1")

	}()
	panic("触发异常2")
}

//输出
//打印后
//打印中
//打印前
//panic: 触发异常2
//panic: 触发异常1

```

## 协程

### 协程N++

```go
package main

import (
	"fmt"
	"sync"
)

func main() {
	var n int32 = 0
	wg := sync.WaitGroup{}
	for i := 0; i < 1000000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			//atomic.AddInt32(&n, 1)//输出1000000
			n++ //输出未知
		}()
	}
	wg.Wait()
	fmt.Println(n)
}

```

### 限制协程数量

```go
package main

import (
	"fmt"
	"sync"
	"time"
)

func job(index int) {
	time.Sleep(time.Millisecond * 500)
	fmt.Printf("执行完毕,序号:%d\n", index)
}

var pool chan struct{}

func main() {
	maxNum := 10
	pool = make(chan struct{}, maxNum)

	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		pool <- struct{}{} //到达最大长度阻塞
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			defer func() {
				<-pool
			}()
			job(index)
		}(i)
	}
	wg.Wait()
}
```

### 超时控制

```go
package main

import (
	"fmt"
	"time"
)

//1 .业务过程放到协程
// 2、把业务结果塞入channel
func job() chan string {
	ret := make(chan string)
	go func() {
		time.Sleep(time.Second * 2)
		ret <- "success"
	}()
	return ret
}
func run() (interface{}, error) {
	c := job()
	select {
	case r := <-c:
		return r, nil
	case <-time.After(time.Second * 3):
		return nil, fmt.Errorf("time out")
	}

}
func main() {
	fmt.Println(run())
}

```

### 单核下,协程顺序

```go
package main

import (
	"fmt"
	"runtime"
	"sync"
)

func main() {
	runtime.GOMAXPROCS(1)
	wg := sync.WaitGroup{}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(sub int) {
			defer wg.Done()
			fmt.Println(sub)
		}(i)
	}
	wg.Wait()
}
//输出 40123 单核下,p默认执行最后一个协程
```

## 数据结构

### 自带链表

```go
package main

import (
	"container/list"
	"fmt"
)

func main() {
	data := list.New()

	e8 := data.PushBack(8)
	data.PushBack(9)
	data.PushBack(10)
	data.PushFront(7)

	e85 := data.InsertAfter(8.5, e8)
	data.MoveAfter(e8, e85)
	for e := data.Front(); e != nil; e = e.Next() {
		fmt.Printf("%v ", e.Value)
	}
}
```



