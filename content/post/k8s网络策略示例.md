---
title: "K8s网络策略示例"
date: 2023-05-27T13:01:57+08:00
tags: ["k8s"]
---

## 示例

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: test-network-policy
  namespace: default
spec:
  podSelector:
    matchLabels:
      role: db
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - ipBlock:
            except:
              - 172.17.1.0/24
            cidr: 172.17.0.0/16
        - namespaceSelector:
            matchLabels:
              project: myproject
        - podSelector:
            matchLabels:
              role: frontend
      ports:
        - protocol: TCP
          port: 6379
  egress:
    - to:
        - ipBlock:
            cidr: 10.0.0.0/24
      ports:
        - protocol: TCP
          port: 5978
```

在 Kubernetes 网络策略中，`ingress` 规则中的 `from` 字段包含的 `ipBlock`、`namespaceSelector` 和 `podSelector` 之间是 "或"（OR）的关系。

- Ingress 规则: 
  - 如果进入流量来自 `ipBlock` 所指定的 IP 地址或地址范围（除了被 `except` 排除的范围）；
  - 或者进入流量来自于 `namespaceSelector` 所选择的命名空间中的任意 Pod；
  - 或者进入流量来自于 `podSelector` 所选择的 Pod；
- Egress 规则: 
  - 流量只能发送到 CIDR `10.0.0.0/24` 下的 IP，并且只能通过 TCP 协议的 5978 端口

## 拒绝所有

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-all
  namespace: test
spec:
  podSelector: { }  # 匹配本命名空间所有pod
  policyTypes:
    - Ingress
    - Egress
  # ingress和egress没有指定规则，则不允许任何流量进出pod
```

测试

```bash
#测试访问外部（拒绝访问）：
kubectl run busybox --image=busybox -n test -- sleep 12h 
kubectl exec busybox -n test -- ping baidu.com
#测试外部pod访问（拒绝访问）：
kubectl run web --image=nginx -n test
kubectl run busybox --image=busybox -- sleep 12h 
kubectl exec busybox -- ping 10.233.96.3 
#测试内部pod之间访问（拒绝访问）：
kubectl exec busybox -n test -- ping 10.233.96.3
```

## 拒绝其它命名空间

test空间下所有pod可以互相访问, 也可以访问其它空间Pod, 但是其它空间不能访问test空间pod

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-all-namespaces
  namespace: test
spec:
  podSelector: {}
  policyTypes:
    - Ingress
  ingress:
    - from:
        - podSelector: {} #配置本ns所有pod
```
## 允许其它空间pod访问指定应用

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-all-namespaces
  namespace: test
spec:
  podSelector:
    matchLabels:
      app: web
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector: {}  # 匹配所有命名空间的pod
```

