---
title: "nginx-ingress"
date: 2022-08-12T10:27:37+08:00
---

# nginx-ingress

安装nginx-ingress

参考文档: https://kubernetes.github.io/ingress-nginx/deploy/

```bash
➜ kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.3.0/deploy/static/provider/cloud/deploy.yaml
```

修改服务模式为NodePort

```
➜ kubectl edit svc ingress-nginx-controller -n  ingress-nginx
```
