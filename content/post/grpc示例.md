---
title: "grpc示例"
date: 2022-08-15T21:24:41+08:00
tags: ["golang"]
---
报错
```bash
$ protoc --go_out=. --go-grpc_out=. hello.proto
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

代码详见

http://inksnw.asuscomm.com:3000/inksnw/grpc
