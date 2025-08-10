---
title: "使用pyroscope追查资源占用"
date: 2023-07-03T17:54:47+08:00
tags: ["工程化"]
---

Pyroscope是一个开源的持续分析系统，使用Go语言实现。服务端使用web页面查看，提供丰富的分析的功能，客户端提供Go、Java、Python、Ruby、PHP、.NET等多种语言的支持

## 安装

```bash
wget https://github.com/grafana/pyroscope/releases/download/v1.2.1/pyroscope_1.2.1_linux_amd64.rpm
rpm -ivh pyroscope_1.2.1_linux_amd64.rpm
rpm -ql pyroscope
systemctl start pyroscope
```

访问 `http://192.168.50.209:4040/` 即可

## 写一个程序

```go
package main

import (
	"fmt"
	"github.com/grafana/pyroscope-go"
	"math"
	"os"
	"runtime"
	"time"
)

func main() {
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	pyroscope.Start(pyroscope.Config{
		ApplicationName: "simple.golang.app",
		ServerAddress:   "http://192.168.50.209:4040",
		Logger:          pyroscope.StandardLogger,
		Tags:            map[string]string{"hostname": os.Getenv("HOSTNAME")},
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})

	numCores := runtime.NumCPU()
	// 设置使用的CPU核心数为全部可用核心
	runtime.GOMAXPROCS(numCores)
	go highCpu()
	go highMem()
	fmt.Printf("当前系统可用的CPU核心数：%d, 开始资源占用测试\n", numCores)
	select {}
}

// 计算平方根以模拟CPU密集型任务
func highCpu() {
	for {
		math.Sqrt(123456789)
	}
}

// 高内存消耗函数
func highMem() {
	var mem [][]byte
	maxMemory := 2 * 1024 * 1024 * 1024 // 最大内存占用 2GB

	for {
		// 分配1MB内存以模拟内存密集型任务
		newBlock := make([]byte, 1024*1024)
		mem = append(mem, newBlock)
		// 计算当前内存占用总大小
		totalSize := len(mem) * 1024 * 1024
		// 如果内存占用超过最大限制，释放最旧的内存块
		if totalSize > maxMemory {
			mem = mem[1:]
			mem[0] = nil // 显式置为nil，帮助垃圾回收
		}
		fmt.Printf("当前大小 %d M\n ", totalSize/1024/1024)
		time.Sleep(10 * time.Millisecond)
	}
}
```

在界面上切换要监听的程序

<img src="http://inksnw.asuscomm.com:3001/blog/使用pyroscope追查资源占用_9395294a94cc66a18cef093a6a139588.png" alt="image-20230703180710548" style="zoom:50%;" />

### 选项解释

- `accloc_objects`：表示在代码执行期间分配的对象总数。它反映了代码的内存使用情况，用于衡量代码中对象创建的频率。

- `accloc_space`：表示代码执行期间已分配的对象的总内存占用量。它反映了代码的内存使用情况，用于衡量代码在执行过程中所占用的内存空间。

- `inuse_objects`：表示当前在内存中使用的对象数量。它反映了代码在某一时刻实际占用的内存对象数量。

- `inuse_space`：表示当前在内存中使用的对象的总内存占用量。它反映了代码在某一时刻实际占用的内存空间。

## cpu占用

可以看到`main.highCpu` 函数最占cpu

<img src="http://inksnw.asuscomm.com:3001/blog/使用pyroscope追查资源占用_dd9b2a28a44745ee0ffe77238e5d2800.png" alt="image-20230703181135182" style="zoom: 67%;" />

## 内存占用

`main.highMem` 最占用内存

<img src="http://inksnw.asuscomm.com:3001/blog/使用pyroscope追查资源占用_4e1c8c1d85e5130a7958e50193fa9b4a.png" alt="image-20230703181252627" style="zoom: 67%;" />
