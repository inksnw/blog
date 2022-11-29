---
title: "Harbor安装与containerd"
date: 2022-09-30T17:14:14+08:00
tags: ["k8s"]
---

## harbor安装

下载离线安装包`700M`

```bash
wget https://mirrors.tuna.tsinghua.edu.cn/github-release/goharbor/harbor/v2.6.0/harbor-offline-installer-v2.6.0.tgz
```

解压从`harbor.yml.tmpl`复制一份`harbor.yml`

```yaml
hostname: harbor.inksnw.com
http:
  port: 3002
harbor_admin_password: Harbor12345
# 数据库要提前建好
external_database:
  harbor:
    host: 192.168.50.xx
    port: 5432
    db_name: harbor
    username: xxx
    password: xxx
    ssl_mode: disable
    max_idle_conns: 2
    max_open_conns: 0
data_volume: /data/harbor
trivy:
  ignore_unfixed: false
  skip_update: false
  offline_scan: false
  insecure: false
jobservice:
  max_job_workers: 10
notification:
  webhook_job_max_retry: 10
chart:
  absolute_url: disabled
log:
  level: info
  local:
    rotate_count: 50
    rotate_size: 200M
    location: /var/log/harbor
_version: 2.6.0
proxy:
  http_proxy:
  https_proxy:
  no_proxy:
  components:
    - core
    - jobservice
    - trivy
upload_purging:
  enabled: true
  age: 168h
  interval: 24h
  dryrun: false
cache:
  enabled: false
  expire_hours: 24
```

生成配置文件安装

```bash
➜ ./prepare
➜ docker-compose up -d
```

访问web界面

```
http://192.168.50.209:3002/
用户名: admin
默认密码: Harbor12345
```

**踩坑**

>  用户名,密码明明是对的,却登录不上,显示密码错误,折腾了好久,docker都重装了也没用,开`F12`看到返回消息 `CSRF Token Invalid`,查到 [issue](https://github.com/goharbor/harbor/issues/12676) 清空chrome Storage即可

<img src="http://inksnw.asuscomm.com:3001/blog/harbor安装与containerd_64fda2673e4bbf152a1fc84a80e9e8a6.png" alt="134382902-03254048-8a2b-4847-9504-1d2b9bfb239d" style="zoom:50%;" />

## 配置containerd

配置 `/etc/containerd/config.toml`如下

```toml
[plugins]
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
    runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
      SystemdCgroup = true
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "registry.cn-beijing.aliyuncs.com/kubesphereio/pause:3.7"
    [plugins."io.containerd.grpc.v1.cri".cni]
      bin_dir = "/opt/cni/bin"
      conf_dir = "/etc/cni/net.d"
      max_conf_num = 1
      conf_template = ""
    [plugins."io.containerd.grpc.v1.cri".registry]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
          endpoint = ["https://registry-1.docker.io"]
        #以下是新增的
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."harbor.inksnw.com:3002"]
          endpoint = ["http://harbor.inksnw.com:3002"]
        [plugins."io.containerd.grpc.v1.cri".registry.configs]
          [plugins."io.containerd.grpc.v1.cri".registry.configs."harbor.inksnw.com:3002".tls]
          insecure_skip_verify = true
```

重启containerd

```bash
systemctl restart containerd
```

上传镜像

```bash
# 登录
nerdctl --insecure-registry login http://harbor.inksnw.com:3002
# 要指定`-n=k8s.io` 否则会找不到镜像
nerdctl -n=k8s.io tag calico/node:v3.23.2 harbor.inksnw.com:3002/calico/node:v3.23.2
nerdctl -n=k8s.io images
nerdctl -n=k8s.io --insecure-registry push harbor.inksnw.com:3002/calico/node:v3.23.2
```

**踩坑**

>  要注意`calico`这种目录名要提前创建好,并设为公开,否则上传时会提示`401`错误,不会帮你自动创建

上传成功

<img src="http://inksnw.asuscomm.com:3001/blog/harbor安装与containerd_83b2069f2073684f82111d6ae6285f0e.png" alt="image-20220930173345991" style="zoom:50%;" />

### 批量上传

```bash
# 批量打标签
➜ nerdctl -n=k8s.io images|grep -v inksnw|tail -n +2|awk '{print "nerdctl -n=k8s.io tag "  $1":"$2   " harbor.inksnw.com:3002/" $1":"$2  }'|sh
# 批量上传
➜ nerdctl -n=k8s.io images|grep inksnw|awk '{print "nerdctl -n=k8s.io --insecure-registry push " $1":"$2}'|sh
# 批量删除为上传生成的image
➜ nerdctl -n=k8s.io images|grep inksnw|awk '{print "nerdctl -n=k8s.io rmi " $1":"$2}'|sh
# 查看删除后镜像
nerdctl -n=k8s.io images
```

