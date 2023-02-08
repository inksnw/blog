---
title: "K8s聚合api使用"
date: 2023-02-08T14:21:20+08:00
tags: ["k8s"]
---

## 前置条件

需要`/etc/kubernetes/manifests/kube-apiserver.yaml`开启以下参数

```bash
    - --proxy-client-cert-file=/etc/kubernetes/pki/front-proxy-client.crt
    - --proxy-client-key-file=/etc/kubernetes/pki/front-proxy-client.key
    - --requestheader-allowed-names=front-proxy-client
    - --requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.crt
```

### 生成证书

```bash
openssl genrsa -out aaserver.key 2048
openssl req -new -key aaserver.key -out aaserver.csr -subj "/CN=front-proxty-client"
openssl x509 -req -days 3650 -in aaserver.csr -CA front-proxy-ca.crt -CAkey front-proxy-ca.key -CAcreateserial -out aaserver.crt
```

或者也可使用代码生成

```go
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	rd "math/rand"
	"os"
	"time"
)

const CAFile = "cmd/front-proxy-ca.crt"
const CAKey = "cmd/front-proxy-ca.key"

func check(err error) {
	if err != nil {
		panic(err)
	}

}

func main() {
	//解析k8s ca 和key 文件
	caFile, err := os.ReadFile(CAFile)
	check(err)
	caBlock, _ := pem.Decode(caFile)
	caCert, err := x509.ParseCertificate(caBlock.Bytes) //ca 证书对象
	check(err)
	//解析私钥
	keyFile, err := os.ReadFile(CAKey)
	keyBlock, _ := pem.Decode(keyFile)
	caPriKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes) //私钥对象
	check(err)
	//构建证书模板
	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(rd.Int63()), //证书序列号
		Subject: pkix.Name{
			Country: []string{"CN"},
			//Organization:       []string{"填的话这里可以用作用户组"},
			//OrganizationalUnit: []string{"可填可不填"},
			Province:   []string{"beijing"},
			CommonName: "front-proxty-client", //CN
			Locality:   []string{"beijing"},
		},
		NotBefore:             time.Now(),                                                                 //证书有效期开始时间
		NotAfter:              time.Now().AddDate(1, 0, 0),                                                //证书有效期
		BasicConstraintsValid: true,                                                                       //基本的有效性约束
		IsCA:                  false,                                                                      //是否是根证书
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}, //证书用途(客户端认证，数据加密)
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment,
		EmailAddresses:        []string{"test@test.com"},
	}

	//生成公私钥--秘钥对
	priKey, err := rsa.GenerateKey(rand.Reader, 2048)
	check(err)
	//创建证书 对象
	clientCert, err := x509.CreateCertificate(rand.Reader, certTemplate, caCert, &priKey.PublicKey, caPriKey)
	check(err)
	//编码证书文件和私钥文件
	clientCertPem := &pem.Block{Type: "CERTIFICATE", Bytes: clientCert}
	clientCertFile, err := os.OpenFile("cmd/certs/aa.crt", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	err = pem.Encode(clientCertFile, clientCertPem)
	check(err)
	buf := x509.MarshalPKCS1PrivateKey(priKey)
	keyPem := &pem.Block{Type: "PRIVATE KEY", Bytes: buf}
	clientKeyFile, err := os.OpenFile("cmd/certs/aa.key", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	check(err)
	err = pem.Encode(clientKeyFile, keyPem)
	check(err)
}
```

### 写一个最简单的程序

```go
package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

func ApiResourceList() metav1.APIResourceList {
	apiList := metav1.APIResourceList{
		GroupVersion: "apis.abc.com/v1",
		APIResources: []metav1.APIResource{
			{
				Name:       "mypod",
				Kind:       "MyPod",
				ShortNames: []string{"mp"},
				Namespaced: true,
				Verbs:      []string{"get", "list"},
			},
		},
	}
	apiList.APIVersion = "v1"
	apiList.Kind = "APIResourceList"
	return apiList
}

func List() any {
	rv := `
{
    "apiVersion": "apis.abc.com/v1",
    "items": [
        {
            "metadata": {
                "namespace": "default",
				"name":"mypod1"

            }
        }
    ],
    "kind": "MyPodList"
}
`
	r := map[string]any{}
	json.Unmarshal([]byte(rv), &r)
	return r
}

func main() {
	group := "apis.abc.com"
	version := "v1"
	root := fmt.Sprintf("/apis/%s/%s", group, version)

	r := gin.Default()
	r.GET(root, func(c *gin.Context) {
		c.JSON(200, ApiResourceList())
	})

	listUrl := fmt.Sprintf("/apis/apis.abc.com/v1/namespaces/default/mypod")
	r.GET(listUrl, func(c *gin.Context) {
		c.JSON(200, List())
	})
	err := r.RunTLS(":8443", "./cmd/certs/aa.crt", "./cmd/certs/aa.key")
	if err != nil {
		panic(err)
	}
}
```

### 部署到主机

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapi
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: myapi
  template:
    metadata:
      labels:
        app: myapi
    spec:
      nodeName: node1
      containers:
        - name: myapi
          image: alpine:3.12
          imagePullPolicy: IfNotPresent
          command: [ "./myapi" ]
          workingDir: "/app"
          ports:
            - containerPort: 8443
          volumeMounts:
            - name: app
              mountPath: /app
      volumes:
        - name: app
          hostPath:
            path: /root/app
---
apiVersion: v1
kind: Service
metadata:
  name: myapi
spec:
  type: ClusterIP
  ports:
    - port: 8443
      targetPort: 8443
  selector:
    app: myapi
```

查看运行状态

```
➜ kubectl get pod
NAME                                   READY   STATUS    RESTARTS         AGE
myapi-796d7f74c-kb8sg                  1/1     Running   0                11s
➜ get svc
NAME                     TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
myapi                    ClusterIP   10.233.1.154    <none>        8443/TCP         14s
```

注册aa api

```yaml
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1.apis.abc.com
spec:
  service:
    name: myapi
    namespace: default
    port: 8443
  group: apis.abc.com
  version: v1
  insecureSkipTLSVerify: true
  groupPriorityMinimum: 100
  versionPriority: 100
```

查看注册状态

```bash
➜ kubectl get apiservice v1.apis.abc.com
NAME              SERVICE         AVAILABLE   AGE
v1.apis.abc.com   default/myapi   True        5m28s
# 如果注册失败,会显示,可describe查看原因
NAME              SERVICE         AVAILABLE                      AGE
v1.apis.abc.com   default/myapi   False (FailedDiscoveryCheck)   98s
```

### 验证

增加自己的资源

执行`kubectl get deploy -v=6` 可以发现请求的实际地址是`https://lb.kubesphere.local:6443/apis/apps/v1/namespaces/default/deployments?limit=1`

因此,我们只要实现`/apis/apis.abc.com/v1/namespaces/default/mypod`的路由,即可实现使用kubectl 来查询我们的自定义资源

```
➜ kubectl get mypod
NAME     AGE
mypod1   <unknown>
```

