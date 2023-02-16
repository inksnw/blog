---
title: "使用grpc操作containerd"
date: 2023-02-16T19:43:26+08:00
tags: ["k8s"]
---

下载containerd二进制包

```bash
wget https://github.com/containerd/containerd/releases/download/v1.6.18/containerd-1.6.18-linux-amd64.tar.gz
# 解压包中的二进制到 /usr/local/bin/
```

生成配置文件

```bash
mkdir -p /etc/containerd
containerd config default > /etc/containerd/config.toml
```

修改 /etc/containerd/config.toml文件,允许远程访问

```toml
[grpc]
tcp_address = "0.0.0.0:8989" 
[plugins]
	[plugins."io.containerd.grpc.v1.cri"]
		disable_tcp_service = false
```

配置系统服务到`/lib/systemd/system/containerd.service  `

```bash
[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target local-fs.target

[Service]
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/local/bin/containerd

Type=notify
Delegate=yes
KillMode=process
Restart=always
RestartSec=5
LimitNPROC=infinity
LimitCORE=infinity
LimitNOFILE=infinity
TasksMax=infinity
OOMScoreAdjust=-999

[Install]
WantedBy=multi-user.target
```

启动

```bash
systemctl restart containerd
systemctl status containerd
```

安装crictl
```
VERSION="v1.26.0" # check latest version in /releases page
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-$VERSION-linux-amd64.tar.gz
sudo tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
rm -f crictl-$VERSION-linux-amd64.tar.gz
```

crictl配置文件`/etc/crictl.yaml`

```
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 2
```

查看api版本号, 可以看到containerd使用的是v1版本的api

```
root@base:~# crictl version
Version:  0.1.0
RuntimeName:  containerd
RuntimeVersion:  v1.6.18
RuntimeApiVersion:  v1
```

写一段简单代码操作containerd, 里面的api版本取决于上面查到的

```go
package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/cri-api/pkg/apis/runtime/v1"
	"log"
	"time"
)

func main() {
	gopts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	addr := "192.168.50.233:8989"
	//addr := "unix:///run/containerd/containerd.sock"
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	conn, err := grpc.DialContext(ctx, addr, gopts...)
	if err != nil {
		log.Fatalln(err)
	}
	req := &v1.VersionRequest{}
	rsp := &v1.VersionResponse{}
	err = conn.Invoke(ctx, "/runtime.v1.RuntimeService/Version", req, rsp)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(rsp)

}
```

