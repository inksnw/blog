---
title: "K8s权限"
date: 2022-08-10T00:27:31+08:00
---

## 证书生成

生成证书文件

```bash
openssl genrsa -out client.key 2048
openssl req -new -key client.key -out client.csr -subj "/CN=user1"
openssl x509 -req -in client.csr -CA /etc/kubernetes/pki/ca.crt -CAkey /etc/kubernetes/pki/ca.key -CAcreateserial -out client.crt -days 365
```
生成如下文件

```bash
.
├── client.crt
├── client.csr
└── client.key
```

查看k8s的api地址


```bash
kubectl get endpoints
```
请求
```bash
curl --cert ./client.crt --key ./client.key --cacert /etc/kubernetes/pki/ca.crt -s https://192.168.50.40:6443/api
# 可以使用 --insecure 代替 --cacert /etc/kubernetes/pki/ca.crt 从而忽略服务端证书验证
```

正常结果

```json
{
  "kind": "APIVersions",
  "versions": [
    "v1"
  ],
  "serverAddressByClientCIDRs": [
    {
      "clientCIDR": "0.0.0.0/0",
      "serverAddress": "192.168.50.40:6443"
    }
  ]
}
```
证书反解
```bash
#如果你忘了证书设置的CN(Common name)是啥 可以用下面的命令搞定
openssl x509 -noout -subject -in client.crt
```

## 添加config

查看当前上下文

```
kubectl config current-context
```

添加用户到config

```bash
kubectl config  set-credentials user1  --client-certificate=./client.crt --client-key=./client.key
```

```bash
#添加上下文
kubectl config set-context user1_context --cluster=cluster.local  --user=user1
#指定当前上下文：
kubectl config  use-context user1_context
```

## 创建role与RoleBinding

创建一个角色

```yaml
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: default
  name: mypod
rules:
  - apiGroups: [ "*" ]
    resources: [ "pods" ]
    verbs: [ "get", "watch", "list" ]
```

用户和角色绑定

```bash
kubectl create rolebinding mypodbinding   --role mypod --user user1
```

方法二:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  creationTimestamp: null
  name: mypodbinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: mypod
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: user1

```

创建ClusterRole和RoleBinding

创建一个ClusterRole

```yaml
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: mypod-cluster
rules:
  - apiGroups: ["*"]
    resources: ["pods"]
    verbs: ["get", "watch", "list"]
```

创建RoleBinding,限定名称空间

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: mypodrolebinding-cluster
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: mypod-cluster
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: user1

```

创建ClusterRoleBinding,不限定名称空间

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: mypodrolebinding-cluster
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: mypod-cluster
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: user1
```

## 使用token的方式请求API

生成并设置token

```bash
kubectl config set-credentials user1 --token=$(head -c 16 /dev/urandom | od -An -t x | tr -d ' ')
```

请求测试

```bash
curl -H "Authorization: Bearer d9bf49ce2447cfa617f7c6215303fd2f" https://192.168.50.40:6443/api/v1/namespaces/default/pods --insecure
```

修改api-server启动参数

```bash
vi /etc/kubernetes/manifests/kube-apiserver.yaml
#加入
--token-auth-file=/etc/kubernetes/pki/token_auth
```

重新请求

```bash
curl -H "Authorization: Bearer d9bf49ce2447cfa617f7c6215303fd2f" https://192.168.50.40:6443/api/v1/namespaces/default/pods --insecure
```

## ServiceAccount

> Service account是为了方便Pod里面的进程调用Kubernetes API或其他外部服务而设计的。它与User account不同
>
> 　　1.User account是为人设计的，而service account则是为Pod中的进程调用Kubernetes API而设计；
>
> 　　2.User account是跨namespace的，而service account则是仅局限它所在的namespace；
>
> 　　3.每个namespace都会自动创建一个default service account
>
> 　　4.Token controller检测service account的创建，并为它们创建secret(1.24后不会再自动创建)

创建sa

```bash
cat<<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: mysa
  namespace: default
EOF
```

手动创建 Secret 资源并与 ServiceAccount 关联,删除 ServiceAccount 随之 Secret 会一并自动删除

```bash
cat<<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
type: kubernetes.io/service-account-token
metadata:
  name: mysa
  annotations:
    kubernetes.io/service-account.name: "mysa"
EOF
```

可以将sa绑定到clusterrolebinding上

```bash
kubectl create clusterrolebinding mysa-crb --clusterrole=mypod-cluster --serviceaccount=default:mysa
```

设置成一个变量

```bash
mysatoken=$(kubectl get secret mysa -o json|jq -Mr '.data.token'|base64 -d)
```

请求测试

````basic
curl -H "Authorization: Bearer $mysatoken" --insecure  https://192.168.50.40:6443/api/v1/namespaces/default/pods 
````

## pod中访问api

```bash
TOKEN=`cat /var/run/secrets/kubernetes.io/serviceaccount/token`
APISERVER="https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_PORT_443_TCP_PORT"
# pod中默认的token是default,以下可以执行成功
curl --header "Authorization: Bearer $TOKEN" --insecure -s $APISERVER/api
# 无权限
curl --header "Authorization: Bearer $TOKEN" --insecure -s $APISERVER/api/v1/namespaces/default/pods 
```

pod中默认使用的账号是default,要更改使用的账号在deployment中添加

```yaml
 spec:
      + serviceAccountName: mysa
      containers:
      ...
```

使用证书访问

```bash
TOKEN=`cat /var/run/secrets/kubernetes.io/serviceaccount/token`
APISERVER="https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_PORT_443_TCP_PORT"
curl --header "Authorization: Bearer $TOKEN" --cacert  /var/run/secrets/kubernetes.io/serviceaccount/ca.crt   $APISERVER/api/v1/namespaces/default/pods 
```

