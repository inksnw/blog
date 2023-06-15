---
title: "K8s使用外部权限认证"
date: 2023-06-15T22:58:57+08:00
draft: true
---

直接访问

```
https://192.168.50.50:6443/
```

访问被禁止

```json
{
    "kind": "Status",
    "apiVersion": "v1",
    "metadata": {},
    "status": "Failure",
    "message": "forbidden: User \"system:anonymous\" cannot get path \"/\"",
    "reason": "Forbidden",
    "details": {},
    "code": 403
}
```

### 配置文件格式

`Webhook` 模式需要一个 HTTP 配置文件，通过 `--authorization-webhook-config-file=SOME_FILENAME` 的参数声明。

```yaml
apiVersion: v1
kind: Config
clusters:
  - name: webhook-cluster
    cluster:
      insecure-skip-tls-verify: true
      server: http://192.168.50.251:3000/authorize

# users 代表 API 服务器的 webhook 配置
users:
  - name: webhook-user

current-context: webhook
contexts:
- context:
    cluster: webhook-cluster
    user: webhook-user
  name: webhook
```

### 修改apiserver配置文件 

vi /etc/kubernetes/manifests

```
- --authorization-mode=Node,Webhook
- --runtime-config=authorization.k8s.io/v1beta1=true
- --authorization-webhook-config-file=/etc/config/webhook-config
```

把config文件挂载进容器

```yaml
  - mountPath: /etc/config 
      name: webhook-config
      readOnly: true
  
  - hostPath:  # 将 webhook-config.json 文件放在 master 节点的 /etc/config 目录下
      path: /etc/config
      type: DirectoryOrCreate
    name: webhook-config
```

### 写一个鉴权服务

```go
package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
)

func main() {
	r := gin.Default()
	r.POST("/authorize", func(c *gin.Context) {
		data, _ := io.ReadAll(c.Request.Body)
		fmt.Printf("接到请求 %s", string(data))
		c.JSON(200, gin.H{
			"apiVersion": "authorization.k8s.io/v1beta1",
			"kind":       "SubjectAccessReview",
			"status": gin.H{
				"allowed": true,
			},
		})
	})
	r.Run("0.0.0.0:3000")
}
```

等待apiserver启动后, 可以看到鉴权已由我们自己服务来实现

![Snipaste_2023-06-16_00-21-07](http://inksnw.asuscomm.com:3001/blog/k8s使用外部权限认证_3e96ae7d4eb4b7fb38fc88d30acf0131.png)

