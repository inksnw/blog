---
title: "Go Containerregistry库的一个小坑"
date: 2024-05-15T16:34:18+08:00
tags: ["k8s"]
---

先说结论: Containerregistry 库会先尝试访问443, 再回落到80, 如果网络不通就会卡住

## 起因

客户反应私有源查询镜像是正常的, 查询镜像版本就会卡住

### 排查

使用shell访问私有源, 这里以dockerhub为例

```bash
TOKEN=$(curl -s "https://auth.docker.io/token?service=registry.docker.io&scope=repository:zichenkkkk/migrate:pull" | jq -r .token)
curl -H "Authorization: Bearer $TOKEN"   "https://registry-1.docker.io/v2/zichenkkkk/migrate/tags/list"
```

可以正常看到结果

```json
{"name":"zichenkkkk/migrate","tags":["2.1.0","2.1.1"]}
```

说明私有源是正常工作的, 但为什么代码访问不了呢

## 验证

配置一个私有源

```bash
docker run -d -p 5000:5000 --restart=always --name registry registry
docker pull busybox
docker tag busybox:latest 127.0.0.1:5000/busybox:latest
docker  push 127.0.0.1:5000/busybox:latest
```

```bash
# 查看源的信息
curl 127.0.0.1:5000/v2/_catalog
{"repositories":["busybox"]}
```
查看镜像的版本列表, 可以看到使用shell可以正常查询
```bash
curl  "http://127.0.0.1:5000/v2/busybox/tags/list"
{"name":"busybox","tags":["latest"]}
```

写一段代码查询

```go
package main

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"k8s.io/klog/v2"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Println("Usage: <username> <password> <repo> <src>")
		return
	}

	userName := os.Args[1]
	Password := os.Args[2]
	repo := os.Args[3]
	src := os.Args[4]
	b64Auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", userName, Password)))
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: &loggingTransport{rt: tr}}
	registryURL, err := url.Parse(repo)
	if err != nil {
		klog.Errorf("failed to parse registry URL: %s", repo)
		return
	}
	klog.Infof("registryURL: %s", registryURL)

	repoName, err := name.NewRepository(src, name.Insecure)
	if err != nil {
		klog.Errorf("failed to create repository: %s", src)
		return
	}
	klog.Infof("repoName: %s", repoName)
	authConfig := authn.AuthConfig{
		Username: userName,
		Password: Password,
		Auth:     b64Auth,
	}
	options := []remote.Option{
		remote.WithAuth(authn.FromConfig(authConfig)),
		remote.WithTransport(client.Transport),
	}

	tags, err := remote.List(repoName, options...)
	if err != nil {
		klog.Errorf("failed to list tags: %v", err)
		return
	}

	for _, tag := range tags {
		klog.Infof(tag)
	}
}

type loggingTransport struct {
	rt http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	trace := &httptrace.ClientTrace{
		ConnectStart: func(network, addr string) {
			klog.Infof("ConnectStart: %s %s", network, addr)
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	return t.rt.RoundTrip(req)
}
```

运行

```bash
go run main.go xxx xxx http://127.0.0.1:5000 127.0.0.1:5000/busybox
```

```bash
I0515 16:26:33.410112   20635 main.go:37] registryURL: http://127.0.0.1:5000
I0515 16:26:33.410196   20635 main.go:44] repoName: 127.0.0.1:5000/busybox
I0515 16:26:33.410322   20635 main.go:73] ConnectStart: tcp 127.0.0.1:5000
I0515 16:26:33.411537   20635 main.go:73] ConnectStart: tcp 127.0.0.1:5000
I0515 16:26:33.414504   20635 main.go:62] latest
```

代码运行没有发现异常, 但如果运行在一个域名或者80上

```bash
docker run -d -p 80:5000 --restart=always --name registry registry
docker tag busybox:latest 127.0.0.1/busybox:latest
docker push 127.0.0.1/busybox:latest
```

再次运行程序

```bash
go run main.go xxx xxx http://127.0.0.1 127.0.0.1/busybox
```

```bash
I0515 16:30:01.044632   20731 main.go:37] registryURL: http://127.0.0.1
I0515 16:30:01.044711   20731 main.go:44] repoName: 127.0.0.1/busybox
I0515 16:30:01.044824   20731 main.go:73] ConnectStart: tcp 127.0.0.1:443
I0515 16:30:01.044979   20731 main.go:73] ConnectStart: tcp 127.0.0.1:80
I0515 16:30:01.047822   20731 main.go:62] latest
```

会发现他会先去请求一下 443 端口, 由于客户的网络环境只开放了80, 于是就卡住了, 追源码可以看到

https://github.com/google/go-containerregistry/blob/ff385a972813c79bbd5fc89357ff2cefe3e5b43c/pkg/v1/remote/transport/ping.go#L47

```go
// Ping does a GET /v2/ against the registry and returns the response.
func Ping(ctx context.Context, reg name.Registry, t http.RoundTripper) (*Challenge, error) {
	// This first attempts to use "https" for every request, falling back to http
	// if the registry matches our localhost heuristic or if it is intentionally
	// set to insecure via name.NewInsecureRegistry.
	schemes := []string{"https"}
	if reg.Scheme() == "http" {
		schemes = append(schemes, "http")
	}
	if len(schemes) == 1 {
		return pingSingle(ctx, reg, t, schemes[0])
	}
	return pingParallel(ctx, reg, t, schemes)
}
```

这里写明了, 会发起一个 ping 的动作，http 会作为 fallback，要求 https 端口能快速响应

## 解决方案

开启相应地址的 443 端口, 或者自己改一下这个库吧
