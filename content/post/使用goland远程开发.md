---
title: "使用goland远程开发"
date: 2022-11-13T23:36:36+08:00
tags: ["工程化"]
---

## 起因

发现了一个`goland`的bug, 当引入 `_ "k8s.io/cli-runtime/pkg/resource"` 这个库时,单步调试就百分百进入 proc.go

在windows和Linux下都是正常的,只有mac 不行,网上有相关的Issue

https://youtrack.jetbrains.com/issue/GO-13786/Cannot-step-into-functions-from-certain-local-packages-with-CFNetwork-framework

`vscode`也有相关的问题

https://github.com/golang/vscode-go/issues/1617

```go
package main

import (
	"fmt"
	_ "k8s.io/cli-runtime/pkg/resource"
)

func main() {
	another()
}
func another() {
	fmt.Println("单步调试进不到 Println内部 ")
}
```

折腾了半天解决不了后,决定试一下远程开发

## 配置

- 在右上角的编辑配置打开以下配置菜单
- 把原本的`本地机器` 改过通过`管理目标`添加的远程主机,可以指定源码和二进制的目录
<img src="http://inksnw.asuscomm.com:3001/blog/使用goland远程开发_2b21e985d3f722bd34ae811ca0368f1c.png" alt="image-20221113235729865" style="zoom:50%;" />

- 注意勾选上 **在远程目标上构建**, 否则还是会在本地编译

<img src="http://inksnw.asuscomm.com:3001/blog/使用goland远程开发_69d0a34c9a4223cda14f14fee4582c7e.png" alt="image-20221113235449156" style="zoom:50%;" />

## 验证

此时点击`运行`或者`单步调试`都会在远程主机上运行, 远程主机目录中 `bin` 为我指定的二进制目录, `source` 为源码, `DPQVC5mPha` 为golang调试用的 `dlv`程序

```
root@server:~/dmp# tree -L 1
.
├── bin
├── DPQVC5mPha
└── source

3 directories, 0 files
```

一些补充信息

- 代码默认会自动同步,和网上说的要手动配置 `部署FTP` 勾选上自动同步不同
- 程序运行于远端服务器,如果想从本机发送api请求可用 `ssh -CfNg -L 8080:192.168.50.20:8080 root@192.168.50.20` 类似命令转发一下
- 编译时,会从服务器到本机传送约`30M`的数据,我的代码编译后`70M`,代码几百k,理论上编译和运行都是在远端,不理解这个传输的数据是什么, 因为Linux服务器在内网所以传输速度很快, 如果使用公网服务器,可能会导致这个过程很慢

总之, 这样就能愉快的远程开发(搬砖)了 -_-

