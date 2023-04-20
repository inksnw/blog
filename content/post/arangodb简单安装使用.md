---
title: "Arangodb简单安装使用"
date: 2023-04-20T10:23:49+08:00
---

## 安装

```
curl -OL https://download.arangodb.com/arangodb310/DEBIAN/Release.key
sudo apt-key add - < Release.key
```

```bash
echo 'deb https://download.arangodb.com/arangodb310/DEBIAN/ /' | sudo tee /etc/apt/sources.list.d/arangodb.list
sudo apt-get install apt-transport-https
sudo apt-get update
sudo apt-get install arangodb3=3.10.5-1
```

查看安装状态

```bash
systemctl status arangodb3
```

配置

```bash
vi /etc/arangodb3/arangod.conf
# 设置
endpoint = tcp://0.0.0.0:8529
authentication = false
```

重启

```
systemctl restart  arangodb3
```

## 使用代码访问

```go
package main

import (
	"context"
	"fmt"
	"github.com/arangodb/go-driver"
	"github.com/arangodb/go-driver/http"
)

func main() {
	conn, err := http.NewConnection(http.ConnectionConfig{
		Endpoints: []string{"http://192.168.50.54:8529"},
	})
	if err != nil {
		panic(err)
	}
	// Create a client
	c, err := driver.NewClient(driver.ClientConfig{
		Connection: conn,
	})
	if err != nil {
		panic(err)
	}
	v, err := c.Version(context.TODO())
	if err != nil {
		panic(err)
	}
	fmt.Printf("版本: %s", v.Version)
}
```

