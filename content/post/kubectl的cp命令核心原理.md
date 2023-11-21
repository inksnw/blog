---
title: "Kubectl的cp命令核心原理"
date: 2023-10-16T11:45:56+08:00
tags: ["k8s"]
---

 `kubectl cp` 命令核心原理是利用`exec`命令调用`tar`命令结合linux管道实现, 因此kubectl源码中也写到, 需要保证容器有tar命令

源码位置  `staging/src/k8s.io/kubectl/pkg/cmd/cp/cp.go` 44行

>  !!!Important Note!!!  
> Requires that the 'tar' binary is present in your container  
> image.  If 'tar' is not present, 'kubectl cp' will fail.  

## 示例

实现 `kubectl cp` 命令的细节包括以下步骤：

1. 本地系统中，`kubectl cp` 命令使用 `tar` 命令将文件和目录打包成一个压缩包。
2. 压缩包的内容通过管道传输到目标 Pod。
3. 在目标 Pod 中，`tar` 命令解压缩压缩包，并将文件和目录还原到指定的位置。
4. Pod 复制文件到本地:  `kubectl cp` 命令从 Pod 中读取并创建一个 `tar` 压缩包，并通过管道将其传输到本地，再解压缩到指定目的地。

示例：

从本地系统复制文件到 Pod

```bash
kubectl cp test.txt default/nginx:/tmp/
# 等价于
tar cf - test.txt | kubectl exec -i -n default nginx -- tar xf - -C /tmp/
```

从 Pod 复制文件到本地系统

```bash
kubectl cp default/nginx:/tmp/foo foo
# 等价于
kubectl exec -n default nginx -- tar cf - /tmp/foo | tar xf - -C ./
```

## 代码实现

```go
package main

import (
	"archive/tar"
	"fmt"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"os"
)

var (
	namespace = "default"
	podName   = "nginx"
)

func main() {
	config, _ := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	client, _ := kubernetes.NewForConfig(config)
	copyToPod(client, config)
	copyFromPod(client, config)
}

func copyFromPod(client *kubernetes.Clientset, config *restclient.Config) {
	command := []string{"tar", "cf", "-", "/tmp/foo"}
	req := getReq(client, command)
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		panic(err)
	}

	pipReader, pipWriter := io.Pipe()
	defer pipReader.Close()

	go stream(exec, os.Stdin, pipWriter)

	fmt.Println("从容器读取文件完成")

	reader := tar.NewReader(pipReader)
	
	for {
		header, err := reader.Next()
		if err != nil {
			break
		}
		fmt.Println("读到的文件名是: ", header.FileInfo().Name())
		f, err := os.Create(header.FileInfo().Name())
		if err != nil {
			panic(err)
		}
		_, err = io.Copy(f, reader)
		if err != nil {
			panic(err)
		}
		fmt.Println("本地写入文件完成")
	}

}

func copyToPod(client *kubernetes.Clientset, config *restclient.Config) {
	command := []string{"tar", "xf", "-", "-C", "/tmp/"}

	req := getReq(client, command)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		panic(err)
	}

	localFilePath := "test.txt"
	localFile, err := os.Open(localFilePath)
	if err != nil {
		panic(err)
	}
	defer localFile.Close()
	pipReader, pipWriter := io.Pipe()
	defer pipReader.Close()

	go stream(exec, pipReader, os.Stdout)

	tarWriter := tar.NewWriter(pipWriter)
	defer tarWriter.Close()
	fileInfo, _ := localFile.Stat()
	header := &tar.Header{
		Name: "test.txt",
		Mode: int64(fileInfo.Mode()),
		Size: fileInfo.Size(),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		panic(err)
	}
	if _, err := io.Copy(tarWriter, localFile); err != nil {
		panic(err)
	}
	fmt.Println("本地读取文件完成")
	fmt.Println("写入容器完成")
}

func getReq(client *kubernetes.Clientset, command []string) *restclient.Request {
	req := client.CoreV1().RESTClient().Post().Resource("pods").
		Name(podName).Namespace(namespace).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: command,
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, scheme.ParameterCodec)
	return req
}

func stream(exec remotecommand.Executor, pipReader io.Reader, write io.Writer) {
	err := exec.Stream(remotecommand.StreamOptions{
		Stdin:  pipReader,
		Stdout: write,
		Stderr: os.Stderr,
		Tty:    false,
	})
	if err != nil {
		panic(err)
	}
}
```

