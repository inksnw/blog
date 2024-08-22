---
title: "Harbor安装与containerd"
date: 2022-09-30T17:14:14+08:00
tags: ["k8s"]
---

## harbor安装

下载离线安装包`700M`

```bash
wget https://mirrors.tuna.tsinghua.edu.cn/github-release/goharbor/harbor/v2.11.0/harbor-offline-installer-v2.11.0.tgz
tar -zxvf harbor-offline-installer-v2.11.0.tgz 
cd harbor
cp harbor.yml.tmpl harbor.yml
```

修改 `harbor.yml`

```yaml
hostname: inksnw.asuscomm.com
http:
  port: 3003

harbor_admin_password: admin
external_database:
  harbor:
    host: 192.168.50.87
    port: 5432
    db_name: harbor
    username: postgres
    password: postgres
    ssl_mode: disable
    max_idle_conns: 2
    max_open_conns: 0
data_volume: /data/harbor
trivy:
  ignore_unfixed: false
  skip_update: false
  offline_scan: false
  security_check: vuln
  insecure: false
jobservice:
  logger_sweeper_duration: 1 #days
  max_job_workers: 10
  job_loggers:
    - STD_OUTPUT
    - FILE
notification:
  webhook_job_max_retry: 3
  webhook_job_http_client_timeout: 3 #seconds
log:
  level: info
  local:
    rotate_count: 50
    rotate_size: 200M
    location: /var/log/harbor
_version: 2.10.0
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
➜ docker compose up -d
```

访问web界面

```
http://192.168.50.87:3003/
用户名: admin
默认密码: admin
```

**踩坑**

>  用户名,密码明明是对的,却登录不上,显示密码错误,折腾了好久,docker都重装了也没用,开`F12`看到返回消息 `CSRF Token Invalid`,查到 [issue](https://github.com/goharbor/harbor/issues/12676) 清空chrome Storage即可

<img src="https://inksnw.asuscomm.com:3001/blog/harbor安装与containerd_64fda2673e4bbf152a1fc84a80e9e8a6.png" alt="134382902-03254048-8a2b-4847-9504-1d2b9bfb239d" style="zoom:50%;" />

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
          endpoint = ["http://inksnw.asuscomm.com:3003"]
        [plugins."io.containerd.grpc.v1.cri".registry.configs]
          [plugins."io.containerd.grpc.v1.cri".registry.configs."inksnw.asuscomm.com:3003".tls]
          insecure_skip_verify = true
```

重启containerd

```bash
systemctl restart containerd
```

上传镜像

```bash
nerdctl --insecure-registry login http://inksnw.asuscomm.com:3003
nerdctl -n=k8s.io images
nerdctl -n=k8s.io tag 
nerdctl -n=k8s.io --insecure-registry push inksnw.asuscomm.com:3003/calico/node:v3.23.2
```



**踩坑**

>  要注意`calico`这种目录名要提前创建好,并设为公开,否则上传时会提示`401`错误,不会帮你自动创建

上传成功

<img src="https://inksnw.asuscomm.com:3001/blog/harbor安装与containerd_83b2069f2073684f82111d6ae6285f0e.png" alt="image-20220930173345991" style="zoom:50%;" />

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

## nerdctl简单使用

```bash
wget https://github.com/containerd/nerdctl/releases/download/v1.5.0/nerdctl-1.5.0-linux-amd64.tar.gz
tar -zxvf nerdctl-1.5.0-linux-amd64.tar.gz -C /usr/local/bin/
# 清理未用到的镜像 
nerdctl -n=k8s.io image prune -a
```

### 安装buildkit

```bash
wget https://github.com/moby/buildkit/releases/download/v0.12.1/buildkit-v0.12.1.linux-amd64.tar.gz
tar -zxvf buildkit-v0.12.1.linux-amd64.tar.gz -C /usr/local/
#/etc/buildkit/buildkitd.toml为buildkitd默认配置

mkdir -p /etc/buildkit/
cat > /etc/buildkit/buildkitd.toml << 'EOF'

debug = true
root = "/var/lib/buildkit"
# insecure-entitlements allows insecure entitlements, disabled by default.
insecure-entitlements = [ "network.host", "security.insecure" ]

[worker.oci]
  enabled = true
  platforms = [ "linux/amd64", "linux/arm64" ]
  snapshotter = "auto"
  rootless = false
  noProcessSandbox = false
  gc = true
  gckeepstorage = 9000
  max-parallelism = 4

  [[worker.oci.gcpolicy]]
    keepBytes = 512000000
    keepDuration = 172800
    filters = [ "type==source.local", "type==exec.cachemount", "type==source.git.checkout"]
EOF

cat > /etc/systemd/system/buildkit.service << 'EOF'
[Unit]
Description=BuildKit
Documentation=https://github.com/moby/buildkit
[Service]
ExecStart=/usr/local/bin/buildkitd --oci-worker=false --containerd-worker=true
[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable buildkit
systemctl start buildkit
systemctl status buildkit
```

### build镜像

```bash
mkdir test
cd test
cat > Dockerfile << 'EOF'
FROM alpine
EOF
nerdctl -n=k8s.io build --platform arm64,amd64 -t  test1 .
```

## 简单镜像源

```bash
docker run -d -p 5000:5000 --name registry registry:2
docker pull nginx
docker tag d453dd892d93 localhost:5000/nginx:latest
docker push localhost:5000/nginx:latest
curl http://localhost:5000/v2/nginx/manifests/latest
curl -X GET localhost:5000/v2/_catalog
#如果推送出错，则需要修改 客户端/etc/docker/daemon.json, 加上 "insecure-registries":["192.168.50.87:5000"] 
```

