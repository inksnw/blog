---
title: "访问pod的方式"
date: 2023-02-09T10:32:57+08:00
tags: ["k8s"]
---

## 访问方式

### 直接访问pod ip

在pod中直接访问pod ip, 如

```
cur 10.233.96.61 
```

### 访问ip+域名

```
curl 10-233-96-61.default.pod.cluster.local
```

### 使用完全限定名

创建一个pod,指定`hostname`和`subdomain`

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: mynginx
  labels:
    app: mynginx
spec:
  hostname: myweb
  subdomain: ink
  containers:
    - name: mynginx
      image: nginx
      imagePullPolicy: IfNotPresent
```

访问

```bash
curl myweb.ink.default.svc.cluster.local
```

### 通过svc配置externalIP访问

创建一个svc

```yaml
apiVersion: v1
kind: Service
metadata:
  name: testsvc
spec:
  ports:
    - port: 80
      targetPort: 80
  type: ClusterIP
  externalIPs:
    - 192.168.50.48
  selector:
    app: mynginx
```

查看ipvs信息

```bash
➜ ipvsadm
IP Virtual Server version 1.2.1 (size=4096)
Prot LocalAddress:Port Scheduler Flags
  -> RemoteAddress:Port           Forward Weight ActiveConn InActConn
TCP  node1:8989 rr
  -> 10.233.92.0:http             Masq    1      0          1 
```

访问测试, 查看arp表信息可以看到`externalIPs`的mac地址与`node1`相同

```bash
➜ curl 192.168.50.48:8989
➜ arp -n 
Address                  HWtype  HWaddress           Flags Mask            Iface
192.168.50.48            ether   52:54:00:34:d1:8a   C                     br0
192.168.50.50            ether   52:54:00:34:d1:8a   C                     br0
```

## 使用metalb实现内网负载均衡

如果正在使用`ipvs`模式,需要先进行如下修改

```bash
kubectl edit configmap -n kube-system kube-proxy
```

设置`strictARP: true`

```yaml
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
mode: "ipvs"
ipvs:
  strictARP: true
```

安装,参考文档https://metallb.universe.tf/installation/

```bash
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.7/config/manifests/metallb-native.yaml
```

> **Layer 2模式** 
>
> 每个svc会有集群中的一个node来负责,当服务客户端 发起arp解析的时候,对应的node会响应该arp请求,之后,该svc的所有流量都会指向该node,如果该node故障,MetalLB会迁移ip到另一个node

> **BGP模式** 
>
> BGP需要支持BGP路由协议的路由器设备,node与路由器建立BGP连接,并且告知路由如何转发svc流量

查看安装的pod

```bash
➜ kubectl get pod -n metallb-system
NAME                          READY   STATUS    RESTARTS   AGE
controller-84d6d4db45-trgfr   1/1     Running   0          5m55s
speaker-7t7rj                 1/1     Running   0          5m55s
speaker-mtt8d                 1/1     Running   0          5m55s
speaker-q8ws6                 1/1     Running   0          5m55s
```

controller负责监听svc变化,依据对应的Ip池进行ip的分配

speaker负责监听svc变化,并依据具体的协议发起相应的广播或应答,节点leader的选举

创建配置文件

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: first-pool
  namespace: metallb-system
spec:
  addresses:
  - 192.168.50.50-192.168.50.52
```

创建一个资源使用metallb, 设置`type: LoadBalancer`

```yaml
apiVersion: v1
kind: Service
metadata:
  name: testsvc
spec:
  ports:
    - port: 8989
      targetPort: 80
  type: LoadBalancer
  selector:
    app: mynginx
---
apiVersion: v1
kind: Pod
metadata:
  name: mynginx
  labels:
    app: mynginx
spec:
  hostname: myweb
  subdomain: ink
  containers:
    - name: mynginx
      image: nginx
      imagePullPolicy: IfNotPresent
```

查看svc,发现被分配了ip`192.168.50.50`访问测试

```bash
➜ kubectl get svc
NAME         TYPE           CLUSTER-IP      EXTERNAL-IP     PORT(S)          AGE
kubernetes   ClusterIP      10.233.0.1      <none>          443/TCP          18h
testsvc      LoadBalancer   10.233.49.203   192.168.50.50   8989:30910/TCP   3m59
➜ curl 192.168.50.50:8989
```

> bug?
>
> 设置EXTERNAL-IP 有机率导致节点的arp信息错乱? master节点向worker节点的arp信息没了
>
```bash
➜ arp -n|grep 192.168.50.5
192.168.50.51                    (incomplete)                              enp1s0
192.168.50.52                    (incomplete)                              enp1s0
```

### 使用ExternalName代理访问

```yaml
apiVersion: v1
kind: Service
metadata:
  name: testsvc
spec:
  externalName: www.baidu.com
  type: ExternalName
---
apiVersion: v1
kind: Pod
metadata:
  name: mynginx
spec:
  containers:
    - name: mynginx
      image: nginx
      imagePullPolicy: IfNotPresent
```

访问测试

```bash
➜ kubectl get svc
NAME         TYPE           CLUSTER-IP   EXTERNAL-IP     PORT(S)   AGE
testsvc      ExternalName   <none>       www.baidu.com   <none>    5s
kubectl exec -it mynginx -- curl -H "host: www.baidu.com" https://testsvc --insecure
```

> 在v1.23.8上测试通过, 在v1.25.3不行

