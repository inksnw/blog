---
title: "Helm库的tls问题"
date: 2024-06-24T15:13:34+08:00
tags: ["k8s"]
---

最近使用helm sdk实现配置repo源,默认使用跳过tls验证参数,  发现`v3.11.1` 版本可以正常工作, `v3.14.1` 版本部分源不能正常工作, 报错 tls: handshake failure

## 复现

```go
package main

import (
	"bytes"
	"fmt"
	"net/url"
	"time"

	"helm.sh/helm/v3/pkg/getter"
)

type RepoCredential struct {
	Username              string `json:"username,omitempty"`
	Password              string `json:"password,omitempty"`
	CertFile              string `json:"certFile,omitempty"`
	KeyFile               string `json:"keyFile,omitempty"`
	CAFile                string `json:"caFile,omitempty"`
	InsecureSkipTLSVerify *bool  `json:"insecureSkipTLSVerify,omitempty"`
}

func main() {
	//u := "https://charts.bitnami.com/bitnami/index.yaml"
	u := "https://helm.nginx.com/stable/index.yaml"
	parsedURL, err := url.Parse(u)
	if err != nil {
		panic(err)
	}
	var resp *bytes.Buffer
	cred := RepoCredential{}

	indexURL := parsedURL.String()
	g, _ := getter.NewHTTPGetter()
	resp, err = g.Get(indexURL,
		getter.WithTimeout(5*time.Minute),
		getter.WithURL(u),
		getter.WithInsecureSkipVerifyTLS(true),
		getter.WithTLSClientConfig(cred.CertFile, cred.KeyFile, cred.CAFile),
		getter.WithBasicAuth(cred.Username, cred.Password),
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(resp.String())
}
```

使用 `WithInsecureSkipVerifyTLS` 为 true 的情况下, 发现情况

- nginx 源能正常访问
- bitnami 源报 tls: handshake failure
- 设置WithInsecureSkipVerifyTLS 为 false 就都可以访问
- 手动访问 bitnami 发现有重定向的情况

查看github看到有一个提交更改了 https://github.com/helm/helm/commit/c94306f75d73a84a4e81b93ecfbe70ef4ca79998 一个逻辑的进入条件

![image-20240624153140680](http://inksnw.asuscomm.com:3001/blog/helm库的tls问题_2db07e7baaf2681d1582b4a2b26ede9b.png)

单步调试代码, 发现新旧版本区别就是是否设置了 tls 的serverName 值, 怀疑可能是serverName只在第一次访问设置了, 后续重定向后域名就变了, 于是tls验证就失败了

## 验证

```go
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

func main() {
	//u := "https://helm.nginx.com/stable/index.yaml"
	//ServerName := "helm.nginx.com"
	u := "https://charts.bitnami.com/bitnami/index.yaml"
	ServerName := "charts.bitnami.com"
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		panic(err)
	}
	tlsConfig := &tls.Config{
		RootCAs:    rootCAs,
		ServerName: ServerName,
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			fmt.Println("Redirect!!!!")
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			targetURL, err := url.Parse(req.URL.String())
			if err != nil {
				return err
			}
			hostname := targetURL.Hostname()
			tlsConfig.ServerName = hostname
			return nil
		},
	}
	resp, err := client.Get(u)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(body))
}
```

结果证明当配置了重定向后主动更新 ServerName 逻辑, 就能正常访问了

所以, 选一个方案

- 回退sdk版本
- 把WithInsecureSkipVerifyTLS 设置为 false 
- 自定义 getter.WithTransport

官方有给这个issue提pr, 但是至今还未合并 https://github.com/helm/helm/pull/9318

