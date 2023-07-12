---
title: "在k8s外部验证bearertoken"
date: 2023-07-12T23:42:00+08:00
draft: true
---

## 生成token

创建一个sa账号

```bash
cat<<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: mysa
  namespace: default
EOF
```

> Token controller检测service account的创建，并为它们创建secret(1.24后不会再自动创建)

```bash
root@node1:~# kubectl get serviceaccount/mysa
NAME   SECRETS   AGE
mysa   1         6s
root@node1:~# kubectl get Secret mysa-token-wskgr 
NAME               TYPE                                  DATA   AGE
mysa-token-wskgr   kubernetes.io/service-account-token   3      50s
```

这里使用代码手动生成token

```go
package main

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/kubernetes/pkg/serviceaccount"
	"os"
)

func main() {

	config, _ := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	client, _ := kubernetes.NewForConfig(config)
	// k8s1.24前自动生成的secret中,token字段中使用的issuers 就是这个
	issuers := "kubernetes/serviceaccount"
	//来自于 /etc/kubernetes/pki/sa.key
	pk, err := keyutil.PrivateKeyFromFile("./certs/sa.key")
	if err != nil {
		panic(err)
	}
	gen, err := serviceaccount.JWTTokenGenerator(issuers, pk)
	if err != nil {
		panic(err)
	}
	secret, err := client.CoreV1().Secrets("default").Get(context.TODO(), "mysa-token-wskgr", metav1.GetOptions{})
	if err != nil {
		panic(err)
	}

	sa, err := client.CoreV1().ServiceAccounts("default").Get(context.TODO(), "mysa", metav1.GetOptions{})
	if err != nil {
		panic(err)
	}
	c, pobj := serviceaccount.LegacyClaims(*sa, *secret)
	token, err := gen.GenerateToken(c, pobj)
	if err != nil {
		panic(err)
	}
	f, err := os.OpenFile("./token.txt", os.O_TRUNC|os.O_WRONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	f.WriteString(token)
	fmt.Println("生成token成功")

}
```

JWT 通常由三个部分组成：header、payload 和 signature，它们由点 (.) 分隔。每个部分都是 base64Url 编码的 JSON 字符串。Header 包含 token 类型和加密算法，payload 包含一系列声明，signature 是对前两部分数据签名，用于验证数据的完整性。

可以使用https://jwt.io/ 查看token信息

<img src="http://inksnw.asuscomm.com:3001/blog/在k8s外部验证bearertoken_1181bba4e34083cb460bdb2ca83f3edf.png" alt="image-20230713003203964" style="zoom:50%;" />

## 使用token

再启动一个服务使用这个token验证

```go
package main

import (
	"fmt"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/keyutil"
	sa "k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/serviceaccount"
	"net/http"
)

func main() {

	config, _ := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	client, _ := kubernetes.NewForConfig(config)
	fact := informers.NewSharedInformerFactory(client, 0)

	// k8s1.24前自动生成的secret中,token字段中使用的issuers 就是这个
	issuers := []string{"kubernetes/serviceaccount"}
	//来自于 /etc/kubernetes/pki/sa.pub
	publicKeys, err := keyutil.PublicKeysFromFile("./certs/sa.pub")
	if err != nil {
		panic(err)
	}
	// 可填空
	audi := []string{"abc"}
	saGetter := sa.NewGetterFromClient(
		client,
		fact.Core().V1().Secrets().Lister(),
		fact.Core().V1().ServiceAccounts().Lister(),
		fact.Core().V1().Pods().Lister(),
	)
	validator, err := serviceaccount.NewLegacyValidator(false, saGetter, client.CoreV1())
	if err != nil {
		panic(err)
	}

	tokenAuthenticator := serviceaccount.JWTTokenAuthenticator(issuers, publicKeys, audi, validator)

	bt := bearertoken.New(tokenAuthenticator)

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		rsp, ok, err := bt.AuthenticateRequest(request)
		if err != nil {
			writer.WriteHeader(401)
			writer.Write([]byte("出错了:" + err.Error()))
			return
		}
		if !ok {
			writer.WriteHeader(401)
			writer.Write([]byte("验证参数不合法")) //验证没通过
			return
		}
		writer.WriteHeader(200)
		rv := fmt.Sprintf("token使用的账号是: %s", rsp.User.GetName())
		writer.Write([]byte(rv))
	})
	fmt.Println("验证服务启动")
	http.ListenAndServe(":9090", nil)

}
```



## 访问测试

<img src="http://inksnw.asuscomm.com:3001/blog/在k8s外部验证bearertoken_8f74306191085e6a05e64be98e760823.png" alt="image-20230713000337476" style="zoom: 50%;" />
