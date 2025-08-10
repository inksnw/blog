---
title: "模拟k8s的watch"
date: 2023-06-21T22:46:44+08:00
tags: ["k8s"]
---

k8s的watch功能本质就是利用http的`chunked`功能

源码位于k8s.io/apiserver/pkg/endpoints/handlers/watch.go 200行, 调用位置在k8s.io/apiserver/pkg/endpoints/handlers/get.go 270行

<img src="http://inksnw.asuscomm.com:3001/blog/模拟k8s的watch_bf3be1aca63e5fdd11aa57af5eaf1d48.png" alt="image-20241105135855601" style="zoom:50%;" />

实现一个简单示例

```go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func main() {
	ch := make(chan []byte)
	go func() {
		for {
			data := map[string]string{
				"date": time.Now().Format(time.DateTime),
			}
			now, _ := json.Marshal(data)
			ch <- now
			time.Sleep(time.Second * 3)
		}
	}()
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		flusher, _ := writer.(http.Flusher)
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Transfer-Encoding", "chunked")
		writer.WriteHeader(http.StatusOK)
		flusher.Flush()
		for {
			select {
			case data := <-ch:
				writer.Write(data)
				flusher.Flush()
			}
		}
	})
	fmt.Println("server start")
	http.ListenAndServe(":8080", nil)
}
```

访问测试

<img src="http://inksnw.asuscomm.com:3001/blog/模拟k8s的watch_f8d0496aea5cb9e794d3e6d3f09fb9d7.png" alt="Snipaste_2023-06-21_23-00-29" style="zoom:50%;" />
