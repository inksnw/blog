---
title: "grpc示例"
date: 2022-08-15T21:24:41+08:00
tags: ["golang"]
---
## 准备工作

报错

```bash
➜ protoc --go_out=. --go-grpc_out=. hello.proto
protoc-gen-go: unable to determine Go import path for "hello.proto"
```

解决办法

```go
syntax = "proto3";
package services;

//添加option信息
//option go_package = "path;name";
//path 表示生成的go文件的存放地址，会自动生成目录的。
//name 表示生成的go文件所属的包名

option go_package = "./;hello";
message ProdRequest {
  int32 prod_id = 1;
}

message ProdResponse{
  int32 prod_stock = 1;
}
```

生成文件

```bash
protoc --go_out=. --go-grpc_out=. hello.proto
```

目录结构

```bash
.
├── cmd
│   ├── client
│   │   └── main.go
│   └── server
│       └── main.go
├── go.mod
├── go.sum
├── pbfiles
│   ├── prod_model.pb.go
│   ├── prod_service.pb.go
│   └── prod_service_grpc.pb.go
├── protos
│   ├── prod_model.proto
│   └── prod_service.proto
├── readme.md
└── service
    └── ProdService.go

```

代码参考

http://inksnw.asuscomm.com:3000/inksnw/grpc

## grpc-gateway

<img src="http://inksnw.asuscomm.com:3001/blog/grpc示例_5874e0789443312fbfaba72906001c9d.png" alt="grpc-gateway" style="zoom: 50%;" />

当 HTTP 请求到达 gRPC-Gateway 时，它将 JSON 数据解析为 Protobuf 消息。然后，它使用解析的 Protobuf 消息发出正常的 Go gRPC 客户端请求。Go gRPC 客户端将 Protobuf 结构编码为 Protobuf 二进制格式，然后将其发送到 gRPC 服务器。gRPC 服务器处理请求并以 Protobuf 二进制格式返回响应。Go gRPC 客户端将其解析为 Protobuf 消息，并将其返回到 gRPC-Gateway，后者将 Protobuf 消息编码为 JSON 并将其返回给原始客户端。

### 安装gateway插件

```bash
go get github.com/grpc-ecosystem/grpc-gateway/v2
```

### 修改proto文件

进行类似的修改

```go
import "google/api/annotations.proto";
```

**该文件需要手动从 `https://github.com/googleapis/googleapis/tree/master/google/api` 目录复制到自己的项目中**

目录结构

```bash
proto
├── google
│   └── api
│       ├── annotations.proto
│       └── http.proto
└── yourpath
    └── yourfile.proto
```

添加option信息

```diff
 syntax = "proto3";
 package your.service.v1;
 option go_package = "github.com/yourorg/yourprotos/gen/go/your/service/v1";
+
+import "google/api/annotations.proto";
+
 message StringMessage {
   string value = 1;
 }

 service YourService {
-  rpc Echo(StringMessage) returns (StringMessage) {}
+  rpc Echo(StringMessage) returns (StringMessage) {
+    option (google.api.http) = {
+      post: "/v1/example/echo"
+      body: "*"
+    };
+  }
 }
```

### 重新生成

```bash
protoc --go_out=. --go-grpc_out=. *.proto  --grpc-gateway_out .
```

同时启动grpc和http

```go
package main

import (
	"context"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"grpc/pbfiles"
	"grpc/service"
	"log"
	"net"
	"net/http"
)

func main() {
	svc := grpc.NewServer()
	pbfiles.RegisterProdServiceServer(svc, service.NewProdService())
	lis, _ := net.Listen("tcp", ":8080")
	go func() {
		log.Fatalln(svc.Serve(lis))
	}()

	// Create a client connection to the gRPC server we just started
	// This is where the gRPC-Gateway proxies the requests
	conn, err := grpc.DialContext(
		context.Background(),
		"0.0.0.0:8080",
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalln("Failed to dial server:", err)
	}

	gwmux := runtime.NewServeMux()
	// Register Greeter
	err = pbfiles.RegisterProdServiceHandler(context.Background(), gwmux, conn)
	if err != nil {
		log.Fatalln("Failed to register gateway:", err)
	}

	gwServer := &http.Server{
		Addr:    ":8090",
		Handler: gwmux,
	}

	log.Println("Serving gRPC-Gateway on http://0.0.0.0:8090")
	log.Fatalln(gwServer.ListenAndServe())
}

```

### 访问测试

![Snipaste_2022-08-17_18-33-35](https://inksnw.asuscomm.com:3001/blog/grpc示例_5024445232ebfe2ee23351bd6ed2f956.jpg)
