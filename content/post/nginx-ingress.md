---
title: "nginx-ingress"
date: 2022-08-12T10:27:37+08:00
tags: ["k8s"]
---

## 安装

ingress的本质就是一个南北向的网关,内置一个Nginx,控制器(Operator)监听ingress变化, reload nginx 配置

参考文档: https://kubernetes.github.io/ingress-nginx/deploy/

```bash
➜ kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.7.1/deploy/static/provider/cloud/deploy.yaml
```

- 修改服务模式为NodePort
- 删除掉`externalTrafficPolicy: Local`, 会重置为模式`externalTrafficPolicy: Cluster`

```bash
➜ kubectl edit svc ingress-nginx-controller -n  ingress-nginx
```

- Cluster表示：流量可以转发到其他节点上的Pod。

- Local表示：流量只发给本机的Pod。

<img src="http://inksnw.asuscomm.com:3001/blog/nginx-ingress_ad0022e96a73a6a8d7fb41c6b6f0c1ff.png" alt="image-20220922222116679" style="zoom: 50%;" />

### Cluster

注：这个是默认模式，Kube-proxy不管容器实例在哪，公平转发。

Kube-proxy转发时会替换掉报文的源IP。即：容器收的报文，源IP地址，已经被替换为上一个转发节点的了。

原因是Kube-proxy在做转发的时候，会做一次SNAT (source network address translation)，所以源IP变成了节点1的IP地址。

### Local

这种情况下，只转发给本机的容器，绝不跨节点转发。

Kube-proxy转发时会保留源IP。即：容器收到的报文，看到源IP地址还是用户的。

缺点是负载均衡可能不是很好，因为一旦容器实例分布在多个节点上，它只转发给本机，不跨节点转发流量。当然，少了一次转发，性能会相对好一丢丢。

注：这种模式下的Service类型只能为外部流量，即：LoadBalancer 或者 NodePort 两种，否则会报错。

同时，由于本机不会跨节点转发报文，所以要想所有节点上的容器有负载均衡，就需要上一级的Loadbalancer来做了。

<img src="http://inksnw.asuscomm.com:3001/blog/nginx-ingress_efba8c2e767b0cf86f72a7ace17c97ea.png" alt="1464583-20200707174845450-1319538618" style="zoom:50%;" />

不过流量还是会不太均衡，如上图，Loadbalancer看到的是2个后端（把节点的IP），每个Node上面几个Pod对Loadbalancer来说是不知道的。

想要解决负载不均衡的问题：可以给Pod容器设置反亲和，让这些容器平均的分布在各个节点上（不要聚在一起）。

```yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
           - key: k8s-app
             operator: In
             values:
             - my-app
        topologyKey: kubernetes.io/hostname
```

### 如何选择

要想性能（时延）好，当然应该选 Local 模式，流量转发少一次SNAT。

不过注意，选了这个就得考虑好怎么处理好负载均衡问题（ps：通常我们使用Pod间反亲和来达成）。

如果你是从外部LB接收流量的，那么使用：Local模式 + Pod反亲和，一般是足够的

## 创建应用

```bash
kubectl create deployment demo --image=httpd --port=80
kubectl expose deployment demo --type=NodePort
```

创建一个`ingress`

```bash
kubectl create ingress demo-localhost --class=nginx   --rule="demo.localdev.me/*=demo:80"
# 查看ingress的nodePort端口
kubectl get svc ingress-nginx-controller  -n ingress-nginx -o json|jq .spec.ports[0].nodePort
```

访问

```bash
curl -H 'Host:demo.localdev.me' http://192.168.50.40:30443
# 结果
<html><body><h1>It works!</h1></body></html>
```

## 查看nginx配置

```bash
 kubectl exec -it ingress-nginx-controller-b4fcbcc8f-gl8bq  -n ingress-nginx -- /bin/bash
 cat /etc/nginx/nginx.conf
```

发现其实本质就是配置了nginx的域名转发

<img src="http://inksnw.asuscomm.com:3001/blog/nginx-ingress_bdc7364de5da8c2f4e185ea3e55ef455.png" alt="image-20220922213751633" style="zoom:50%;" />

## 配置https

签发证书 `cert.sh`

```bash
cat > ca-config.json <<EOF
{
    "signing": {
        "default": {
            "expiry": "87600h"
        },
        "profiles": {
            "kubernetes": {
                "expiry": "87600h",
                "usages": [
                    "signing",
                    "key encipherment",
                    "server auth",
                    "client auth"
                ]
            }
        }
    }
}
EOF
cat > ca-csr.json <<EOF
{
    "CN": "kubernetes",
    "key": {
        "algo": "rsa",
        "size": 4096
    },
    "names": [
        {
            "C": "CN",
            "L": "BJ",
            "ST": "BJ"
        }
    ]
}
EOF

cfssl gencert -initca ca-csr.json | cfssljson -bare ca - 


cat  > web-csr.json <<EOF
{
    "CN": "demo.localdev.me",
    "hosts": [],
    "key": {
        "algo": "rsa",
        "size": 2048
    },
    "names": [
        {
            "C": "CN",
            "L": "BeiJing",
            "ST": "BeiJing"
        }
    ]
}
EOF

cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json -profile=kubernetes web-csr.json | cfssljson -bare demo.localdev.me 
```

查看生成的文件

```bash
root@node1:~/https# tree
.
├── ca-config.json
├── ca.csr
├── ca-csr.json
├── ca-key.pem
├── ca.pem
├── cert.sh
├── demo.localdev.me.csr
├── demo.localdev.me-key.pem
├── demo.localdev.me.pem
└── web-csr.json
```

将证书保存到k8s中

```bash
kubectl create secret tls mytls --cert=demo.localdev.me.pem --key=demo.localdev.me-key.pem
```

配置ingress

```bash
# 在spec.tls添加字段 secretName: mytls
kubectl edit ingress demo-localhost
```

```yaml
spec:
  ingressClassName: nginx
  rules:
  - host: demo.localdev.me
    http:
      paths:
      - backend:
          service:
            name: demo
            port:
              number: 80
        path: /
        pathType: Prefix
  tls:
  - hosts:
    - demo.localdev.me
    secretName: mytls
```

查看访问端口, 可以看到https端口为31062

```bash
kubectl get svc ingress-nginx-controller  -n ingress-nginx
root@node1:~/https# kubectl get svc ingress-nginx-controller  -n ingress-nginx 
NAME                       TYPE       CLUSTER-IP      EXTERNAL-IP   PORT(S)                      AGE
ingress-nginx-controller   NodePort   10.233.17.206   <none>        80:32021/TCP,443:31062/TCP   53m
```

访问测试

<img src="http://inksnw.asuscomm.com:3001/blog/nginx-ingress_08c782a6666c57fcfb6e01c47b1d8fe3.png" alt="image-20230527123409161" style="zoom:50%;" />



## BaseAuth认证

文档 https://kubernetes.github.io/ingress-nginx/examples/auth/basic/

生成secret文件

```bash
apt install apache2-utils
htpasswd -c auth inksnw
htpasswd auth inksnw2  #增加用户
```

创建密文

```bash
kubectl -n default create secret generic basic-auth --from-file=auth
```

向相应的ingress添加注解

```yaml
annotations:
    # type of authentication
    nginx.ingress.kubernetes.io/auth-type: basic
    # name of the secret that contains the user/password definitions
    nginx.ingress.kubernetes.io/auth-secret: basic-auth
    # message to display with an appropriate context why the authe
    nginx.ingress.kubernetes.io/auth-realm: 'Authentication Required - foo'
```

再次访问

```
http://demo.localdev.me:30443/
```

<img src="http://inksnw.asuscomm.com:3001/blog/nginx-ingress_8202422854fee1e25162af095b4e22ef.jpg" alt="Snipaste_2022-09-23_10-26-22" style="zoom:50%;" />

## 限流设置

文档: https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/#rate-limiting

nginx.ingress.kubernetes.io/limit-rps：每秒从指定 IP 接受的请求数。突发限制设置为此限制乘以突发乘数，默认乘数为5。当客户端超过此限制时，返回limit-req-status-code default: 503。

编辑相应的ingress

```yaml
annotations:
    nginx.ingress.kubernetes.io/limit-rps: "1"
    # 突发请求倍数
    nginx.ingress.kubernetes.io/limit-burst-multiplier: "5"
```

访问测试

```bash
while true;do curl -s -w "%{http_code}\n" -o /dev/null -H 'Host:demo.localdev.me' http://192.168.50.40:30443;done
```

## Nginx原生配置

文档: https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/#server-snippet

支持通过注解`nginx.ingress.kubernetes.io/xxx`的方式配置不同的原生配置

server-snippet作用于nginx `server`段

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/server-snippet: |
        set $agentflag 0;

        if ($http_user_agent ~* "(Mobile)" ){
          set $agentflag 1;
        }

        if ( $agentflag = 1 ) {
          return 301 https://m.example.com;
        }
```

configuration-snippet作用于nginx `location`段

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/configuration-snippet: |
  		more_set_headers "Request-Id: $req_id";
```

## 灰度发布

通过配置 Ingress Annotations 来实现不同场景下的灰度发布和测试。 [Nginx Annotations](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/#canary) 支持以下 4 种 Canary 规则：

- `nginx.ingress.kubernetes.io/canary-by-header`：基于 Request Header 的流量切分，适用于灰度发布以及 A/B 测试。当 Request Header 设置为 `always`时，请求将会被一直发送到 Canary 版本；当 Request Header 设置为 `never`时，请求不会被发送到 Canary 入口；对于任何其他 Header 值，将忽略 Header，并通过优先级将请求与其他金丝雀规则进行优先级的比较。
- `nginx.ingress.kubernetes.io/canary-by-header-value`：要匹配的 Request Header 的值，用于通知 Ingress 将请求路由到 Canary Ingress 中指定的服务。当 Request Header 设置为此值时，它将被路由到 Canary 入口。该规则允许用户自定义 Request Header 的值，必须与上一个 annotation (即：canary-by-header）一起使用。
- `nginx.ingress.kubernetes.io/canary-weight`：基于服务权重的流量切分，适用于蓝绿部署，权重范围 0 - 100 按百分比将请求路由到 Canary Ingress 中指定的服务。权重为 0 意味着该金丝雀规则不会向 Canary 入口的服务发送任何请求。权重为 100 意味着所有请求都将被发送到 Canary 入口。
- `nginx.ingress.kubernetes.io/canary-by-cookie`：基于 Cookie 的流量切分，适用于灰度发布与 A/B 测试。用于通知 Ingress 将请求路由到 Canary Ingress 中指定的服务的cookie。当 cookie 值设置为 `always`时，它将被路由到 Canary 入口；当 cookie 值设置为 `never`时，请求不会被发送到 Canary 入口；对于任何其他值，将忽略 cookie 并将请求与其他金丝雀规则进行优先级的比较。

注意：金丝雀规则按优先顺序进行如下排序：

> ```
> canary-by-header - > canary-by-cookie - > canary-weight
> ```

测试

```bash
kubectl create deployment canary1 --image=nginx --port=80
kubectl expose deployment canary1
kubectl create deployment canary2 --image=nginx --port=80
kubectl expose deployment canary2
#手动进入容器修改一下网页
echo "this is 1">/usr/share/nginx/html/index.html
echo "this is 2">/usr/share/nginx/html/index.html
```

正常的

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my1
spec:
  ingressClassName: nginx
  rules:
  - host: canary.test.com
    http:
      paths:
      - backend:
          service:
            name: canary1
            port:
              number: 80
        path: /
        pathType: Prefix
```

灰度的

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my2
  annotations:
    nginx.ingress.kubernetes.io/canary: "true"
    nginx.ingress.kubernetes.io/canary-by-header: "canary"
  nginx.ingress.kubernetes.io/canary-by-header-value: "true"
spec:
  ingressClassName: nginx
  rules:
  - host: canary.test.com
    http:
      paths:
      - backend:
          service:
            name: canary2
            port:
              number: 80
        path: /
        pathType: Prefix
```

查看

```bash
kubectl get ingress
NAME             CLASS   HOSTS              ADDRESS         PORTS   AGE
my1              nginx   canary.test.com    10.233.51.220   80      2m13s
my2              nginx   canary.test.com    10.233.51.220   80      104s
```

执行测试，不添加header，访问的默认是正式版本：

```bash
# curl -H 'Host:canary.test.com'  http://192.168.50.40:30443
this is 1
```

添加header，可以看到，访问的已经是灰度版本了

```bash
# curl -H "canary: true" -H 'Host:canary.test.com'  http://192.168.50.40:30443
this is 2
```

## 流量复制

文档: https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/#mirror

```yaml
annotations:
    nginx.ingress.kubernetes.io/mirror-target: https://test.env.com/$request_uri
```

