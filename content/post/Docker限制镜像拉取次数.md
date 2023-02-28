---
title: "Docker限制镜像拉取次数"
date: 2023-02-27T15:53:36+08:00
tags: ["k8s"]
---

### 查看限制次数

在使用 docker pull 拉取镜像时，遇到如下报错：

```bash
Error response from daemon: toomanyrequests: You have reached your pull rate limit. You may increase the limit by authenticating and upgrading: https://www.docker.com/increase-rate-limit
```

2020 年 11 月 20 日，Docker Hub 基于个人 IP 地址对匿名和免费认证的用户进行了镜像拉取次数的限制。
 对于匿名用户，限制设置为每个 IP 地址每 6 小时 100 次拉取。
 对于经过身份验证的用户，每 6 小时 200 次拉取。

那么如何查看我们剩余的拉取次数呢？
 匿名用户获取令牌：

```bash
TOKEN=$(curl "https://auth.docker.io/token?service=registry.docker.io&scope=repository:ratelimitpreview/test:pull" | jq -r .token)
```

认证用户获取令牌：

```bash
TOKEN=$(curl --user 'username:password' "https://auth.docker.io/token?service=registry.docker.io&scope=repository:ratelimitpreview/test:pull" | jq -r .token)
```

查询剩余次数

可以看到我的剩余拉取次数为196次

```bash
➜ curl --head -H "Authorization: Bearer $TOKEN" https://registry-1.docker.io/v2/ratelimitpreview/test/manifests/latest
HTTP/1.1 200 OK
date: Mon, 27 Feb 2023 07:51:11 GMT
strict-transport-security: max-age=31536000
ratelimit-limit: 200;w=21600
ratelimit-remaining: 196;w=21600
```

### k8s使用登录信息

```bash
docker login
cp ~/.docker/config.json  /var/lib/kubelet/
systemctl restart kubelet
```

