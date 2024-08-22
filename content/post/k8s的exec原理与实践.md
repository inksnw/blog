---
title: "K8s的exec原理与实践"
date: 2023-06-15T21:31:40+08:00
tags: ["k8s"]
---

![kubectl-exec](https://inksnw.asuscomm.com:3001/blog/k8s的exec原理与实践_8f78e3983f575fbd9860b99d33b642b3.png)

## exec原理

### 开启containerd远程连接

修改 /etc/containerd/config.toml文件,允许远程访问

```toml
[grpc]
tcp_address = "0.0.0.0:8989" 
[plugins]
	[plugins."io.containerd.grpc.v1.cri"]
		disable_tcp_service = false
		stream_server_address = "192.168.50.50"
```



### 启动一个apiserver

```go
package main

import (
	"k8s.io/apimachinery/pkg/util/proxy"
	api "k8s.io/kubernetes/pkg/apis/core"
	"log"
	"net/http"
	"net/url"
	"time"
)

const defaultFlushInterval = 200 * time.Millisecond

func normalizeLocation(location *url.URL) *url.URL {
	normalized, _ := url.Parse(location.String())
	if len(normalized.Scheme) == 0 {
		normalized.Scheme = "http"
	}
	return normalized
}
func main() {
	//kubernetes-1.22.15/pkg/registry/core/pod/rest/subresources.go

	middleware := func() http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			params := url.Values{}
			params.Add(api.ExecCommandParam, "pwd")
			urlStr := "http://localhost:9090/exec/default/nginx-pod/nginx" // 反代到 Kubelet
			urlObj, err := url.Parse(urlStr)
			if err != nil {
				panic(err)
			}
			urlObj.RawQuery = params.Encode()

			// Call the original handler
			handler := &proxy.UpgradeAwareHandler{
				Location:        normalizeLocation(urlObj),
				Transport:       http.DefaultTransport,
				WrapTransport:   false,
				UpgradeRequired: true,
				FlushInterval:   defaultFlushInterval,
				Responder:       proxy.NewErrorResponder(nil),
			}
			handler.ServeHTTP(w, r)
		})
	}

	wrappedHandler := middleware()
	log.Println("启动假的 apiserver")
	http.ListenAndServe(":6443", wrappedHandler)
}

```

### 启动一个kubelet

```go
package main

import (
	"context"
	"github.com/emicklei/go-restful"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/apimachinery/pkg/util/proxy"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"
	api "k8s.io/kubernetes/pkg/apis/core"
	"log"
	"net/http"
	"net/url"
	"time"
)

// 启动 kubelet 模拟 exec 服务端
func main() {
	//原入口位于 kubernetes-1.24.12/pkg/kubelet/server/server.go 256行
	container := restful.NewContainer()
	ws := new(restful.WebService)
	ws.Path("/exec")
	ws.Route(ws.GET("/{podNamespace}/{podID}/{containerName}").To(GetExec))
	ws.Route(ws.POST("/{podNamespace}/{podID}/{containerName}").To(GetExec))
	ws.Route(ws.GET("/{podNamespace}/{podID}/{uid}/{containerName}").To(GetExec))
	ws.Route(ws.POST("/{podNamespace}/{podID}/{uid}/{containerName}").To(GetExec))
	container.Add(ws)
	klog.Info("启动 kubelet exec 服务，监听9090端口")
	http.ListenAndServe(":9090", container)
}

const RemoteRuntimeAddress = "192.168.50.50:8989" // 修改远程的 runtime 地址

func initRuntimeClient() runtimeapi.RuntimeServiceClient {
	gopts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	conn, err := grpc.DialContext(ctx, RemoteRuntimeAddress, gopts...)
	if err != nil {
		log.Fatalln(err)
	}
	return runtimeapi.NewRuntimeServiceClient(conn)

}

func GetExec(request *restful.Request, response *restful.Response) {
	klog.Info("请求方法：", request.Request.Method)

	// Get command from request parameters
	cmd := request.Request.URL.Query()[api.ExecCommandParam]
	klog.Info("运行cmd：", cmd)

	req := &runtimeapi.ExecRequest{
		ContainerId: "7da6c6638bdf6", // 修改要连接的容器 ID
		Cmd:         cmd,
		Tty:         true,
		Stdin:       true,
		Stdout:      true,
		Stderr:      false,
	}

	runtimeClient := initRuntimeClient()
	resp, err := runtimeClient.Exec(context.TODO(), req)

	if err != nil {
		panic(err)
	}
	klog.Info("得到的URL是：", resp.Url)
	urlF, _ := url.Parse(resp.Url)
	handler := proxy.NewUpgradeAwareHandler(urlF, nil, false, true, nil)
	handler.ServeHTTP(response.ResponseWriter, request.Request)
}
```

### 启动一个kubectl

```go
package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/client-go/tools/remotecommand"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	apiServerUrl := "http://localhost:6443"
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	rt := spdy.NewRoundTripper(tlsConfig)

	reader := bufio.NewReader(os.Stdin)
	req, _ := http.NewRequest(http.MethodPost, apiServerUrl, nil)
	req.Header.Set("Upgrade", "SPDY/3.1")
	req.Header.Set("Connection", "Upgrade")
	executor, err := remotecommand.NewSPDYExecutorForTransports(rt, rt, http.MethodPost, req.URL)
	if err != nil {
		log.Fatal(err)
	}
	for {
		fmt.Print("->: ")
		cmdString, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading command: %v", err)
			continue
		}
		cmdString = strings.TrimSuffix(cmdString, "\n")
		if cmdString == "exit" {
			return
		}

		err = executor.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
			Stdin:  strings.NewReader(cmdString),
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			Tty:    true,
		})
		if err != nil {
			log.Fatalln(err)
		}
	}
}

```

当执行 `kubectl exec` 命令时，实际上发生的事情如下：

1. kubectl 工具会向 Kubernetes API server 发送一个 API 请求，描述了 Pod 和容器名。

2. Kubernetes API server 接收到这个请求后，解析出pod所在节点，反代相应节点的 kubelet 。

3. kubelet 接收到 API server 的指示后，它通过grpc连接相应的cri接口执行操作。


以上代码实现了这个过程,但仅用于原理验证, 只实现了单次执行, 实际`kubectl exec -it` 时, apiserver会通过`spdy`协议解析到流中的输入命令,持续执行, 这个代码还未能调通

## 直连kubelet

```go
package main

import (
	"bytes"
	"fmt"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"net/url"
)

func main() {
	// 此处我们直接使用了/root/.kube/config文件，因此我们有足够的权限
	// 如果使用service account token的话，还需要额外创建role和rolebinding
	// 当直接访问kubelet接口的时候，kubelet支持使用证书和token进行认证，
	// 使用rbac对请求进行鉴权操作
	config, err := clientcmd.BuildConfigFromFlags("", "/Users/inksnw/.kube/config")
	if err != nil {
		panic(err)
	}

	params := url.Values{}
	params.Add("tty", "1")
	params.Add("input", "0")
	params.Add("output", "1")
	params.Add("error", "0")
	params.Add("command", "ls")
	params.Add("command", "/")
	path := &url.URL{
		Scheme:   "https",
		Host:     "192.168.50.51:10250",
		Path:     "/exec/default/nginx/nginx",
		RawQuery: params.Encode(),
	}
	// 此处配置证书相关配置，由于是测试，这块直接忽略了，在实际生产使用时可从文件读取
	// 此处配置的CAFile在更新后，client-go能够定时自动刷新
	config.TLSClientConfig.CAFile = ""
	config.TLSClientConfig.CAData = nil
	config.TLSClientConfig.Insecure = true
	executor, err := remotecommand.NewSPDYExecutor(config, "POST", path)
	if err != nil {
		panic(err)
	}

	done := make(chan error)
	var buf bytes.Buffer
	wrap := func() {
		err := executor.Stream(remotecommand.StreamOptions{
			Stdout: &buf,
			Tty:    true,
		})
		done <- err
	}
	go wrap()
	fmt.Println("wait for out")
	select {
	case err := <-done:
		if err != nil {
			fmt.Println("Command exit with error", err)
		}
		fmt.Println(string(buf.Bytes()))
	}
}
```

