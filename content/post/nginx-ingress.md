---
title: "nginx-ingress"
date: 2022-08-12T10:27:37+08:00
tags: ["k8s"]
---

# 安装nginx-ingress

参考文档: https://kubernetes.github.io/ingress-nginx/deploy/

```bash
➜ kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.3.0/deploy/static/provider/cloud/deploy.yaml
```

修改服务模式为NodePort

```bash
➜ kubectl edit svc ingress-nginx-controller -n  ingress-nginx
```

# 快速创建一个`deploy`与`svc`

```bash
kubectl create deployment demo --image=httpd --port=80
kubectl expose deployment demo
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

# 查看nginx配置

```bash
 kubectl exec -it ingress-nginx-controller-b4fcbcc8f-gl8bq  -n ingress-nginx -- /bin/bash
 cat /etc/nginx/nginx.conf
```

发现其实本质就是配置了nginx的域名转发

<img src="http://inksnw.asuscomm.com:3001/blog/nginx-ingress_bdc7364de5da8c2f4e185ea3e55ef455.png" alt="image-20220922213751633" style="zoom:50%;" />
