---
title: "使用grpc操作containerd"
date: 2023-02-16T19:43:26+08:00
tags: ["k8s"]
---

## 环境配置

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

拉取镜像

```bash
➜ crictl pull docker.io/alpine:3.12
➜ crictl images
IMAGE                      TAG                 IMAGE ID            SIZE
docker.io/library/alpine   3.12                24c8ece58a1aa       2.81MB
```

## grpc操作

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

## crictl创建pod

### cri配置文件

创建容器配置`nginx.yaml`

```yaml
metadata:
  name: myngx
image: docker.io/nginx:1.18-alpine
log_path: ngx.log
```

创建沙箱配置`sandbox.yaml`

```yaml
metadata:
  name: mysandbox
  namespace: default
log_directory: "/root/temp"
port_mappings:
  - protocol: 0
    container_port: 80
```

### cni配置

下载基本cni文件

```
wget https://github.com/containernetworking/plugins/releases/download/v1.2.0/cni-plugins-linux-amd64-v1.2.0.tgz
```

创建目录,将二进制放到这里, 目录取决于containerd配置文件中的`bin_dir = "/opt/cni/bin`和` conf_dir = "/etc/cni/net.d"`

```bash
mkdir -p /opt/cni/bin
mkdir -p /opt/cni/net.d
tar -zxvf cni-plugins-linux-amd64-v1.2.0.tgz -C /opt/cni/bin/
```

创建cni配置文件到`/etc/cni/net.d/10-mynet.conf`, 参考自https://github.com/containernetworking/cni

 ```json
 {
 	"cniVersion": "0.2.0",
 	"name": "mynet",
 	"type": "bridge",
 	"bridge": "cni0",
 	"isGateway": true,
 	"ipMasq": true,
 	"ipam": {
 		"type": "host-local",
 		"subnet": "10.22.0.0/16",
 		"routes": [
 			{ "dst": "0.0.0.0/0" }
 		]
 	}
 }
 ```

重启containerd

```
systemctl restart containerd
```

查看cni装载情况

```bash
# 看到 "lastCNILoadStatus": "OK", "lastCNILoadStatus.default": "OK" 为正常
crictl info
```

### 创建pod

```
crictl run nginx.yaml sandbox.yaml
```

> 报错 registry.k8s.io/pause:3.6 拉不下来

修改containerd的配置文件将`registry.k8s.io/pause:3.6` 改为 `registry.cn-beijing.aliyuncs.com/kubesphereio/pause:3.8`

> 报错:  exec: "runc": executable file not found in $PATH: unknown

安装`runc`

