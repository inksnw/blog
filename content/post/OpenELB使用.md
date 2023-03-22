---
title: "OpenELB使用"
date: 2022-06-22T11:01:26+08:00
tags: ["k8s"]
---

二层网络即流量都在一个二层网络中。此模式借助其他组件例如Kube-proxy，来实现负载均衡。OpenELB会把svc的ip配置在K8S集群节点的local网卡上（比如kube-ipvs0），当Client访问时，由于在同一个二层，会广播ARP请求，某个节点会进行ARP Reply，从而Client请求流量会发送到此节点，然后由节点上的Kube-proxy进行负载，转到真正的Pod地址。

参考: https://www.qikqiak.com/post/openelb/

## 安装

```bash
kubectl apply -f https://raw.githubusercontent.com/openelb/openelb/master/deploy/openelb.yaml
```
查看 Pod 的状态是否正常：

```bash
➜ kubectl get pods -n openelb-system              
NAME                               READY   STATUS      RESTARTS   AGE
openelb-admission-create-47c6n     0/1     Completed   0          51m
openelb-admission-patch-jpvn5      0/1     Completed   1          51m
openelb-keepalive-vip-247vh        1/1     Running     0          50m
openelb-keepalive-vip-jgdzj        1/1     Running     0          50m
openelb-keepalive-vip-ptwtp        1/1     Running     0          50m
openelb-manager-6f8cd5bd56-fhqdr   1/1     Running     0          51m
➜ kubectl get validatingwebhookconfiguration       
NAME                WEBHOOKS   AGE
openelb-admission   1          51m
➜ kubectl get mutatingwebhookconfigurations        
NAME                WEBHOOKS   AGE
openelb-admission   1          51m
```
查看安装的 CRD：
```bash
➜ kubectl get crd |grep kubesphere
bgpconfs.network.kubesphere.io                        2022-06-22T02:48:34Z
bgppeers.network.kubesphere.io                        2022-06-22T02:48:34Z
eips.network.kubesphere.io                            2022-06-22T02:48:34Z
```

查看所有安装的资源

```bash
kubectl api-resources -o name --verbs=list --namespaced | xargs -n 1 kubectl get --show-kind --ignore-not-found -n openelb-system
```

## 配置

为了使其他节点不对local网卡上的Service External IP进行ARP Reply,由 OpenELB 处理 ARP 请求,需要为 kube-proxy 启用 `strictARP` 

```basic
➜ kubectl edit configmap kube-proxy -n kube-system
......
ipvs:
  strictARP: true
......
```

执行以下命令重启 kube-proxy ：

```bash
➜ kubectl rollout restart daemonset kube-proxy -n kube-system
```

创建一个 Eip 对象来充当 OpenELB 的 IP 地址池

```yaml
apiVersion: network.kubesphere.io/v1alpha2
kind: Eip
metadata:
  name: eip-pool
spec:
  address: 192.168.50.50-192.168.50.60
  protocol: layer2
  disable: false
  interface: enp1s0
```

这里我们通过 `address` 属性指定了 IP 地址池，可以填写一个或多个 IP 地址（要注意不同 Eip 对象中的 IP 段不能重叠），将被 OpenELB 使用。值格式可以是：

- IP地址，例如 192.168.0.100
- IP地址/子网掩码，例如 192.168.0.0/24
- IP地址1-IP地址2，例如192.168.0.91-192.168.0.100

`protocol` 可以配置为 layer2 或 bgp，默认为 bgp 模式，我们这里配置 layer2 模式,

`interface` 是用来指定 OpenELB 监听 ARP 或 NDP 请求的网卡，该字段仅在协议设置为 layer2 时有效

```bash
➜ kubectl get eip          
NAME       CIDR                          USAGE   TOTAL
eip-pool   192.168.50.50-192.168.50.60   0       9
➜ kubectl get eip eip-pool -o yaml
apiVersion: network.kubesphere.io/v1alpha2
kind: Eip
metadata:
  finalizers:
  - finalizer.ipam.kubesphere.io/v1alpha1
  name: eip-pool
spec:
  address: 192.168.50.50-192.168.50.60
  interface: ens33
  protocol: layer2
status:
  firstIP: 192.168.50.50
  lastIP: 192.168.50.60
  poolSize: 9
  ready: true
  v4: true
```

到这里 LB 的地址池就准备好了，接下来创建一个简单的服务，通过 LB 来进行暴露：

```yaml
# openelb-nginx.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  selector:  
    matchLabels:
      app: nginx
  template:  
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx
        ports:
        - containerPort: 80
```

然后创建一个 `LoadBalancer` 类型的 Service 来暴露 nginx 服务：

```bash
# openelb-nginx-svc.yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    lb.kubesphere.io/v1alpha1: openelb
    protocol.openelb.kubesphere.io/v1alpha1: layer2
    eip.openelb.kubesphere.io/v1alpha2: eip-pool
spec:
  selector:
    app: nginx
  type: LoadBalancer
  ports:
    - name: http
      port: 80
      targetPort: 80
```

注意这里我们为 Service 添加了几个 annotations 注解：

- `lb.kubesphere.io/v1alpha1: openelb` 用来指定该 Service 使用 OpenELB
- `protocol.openelb.kubesphere.io/v1alpha1: layer2` 表示指定 OpenELB 用于 Layer2 模式
- `eip.openelb.kubesphere.io/v1alpha2: eip-pool` 指定了 OpenELB 使用的 Eip 对象，未配置OpenELB 会自动使用与协议匹配的第一个可用 Eip 对象，此外也可以删除此注解并添加 `spec:loadBalancerIP` 字段（例如 spec:loadBalancerIP: 192.168.50.51）以将特定 IP 地址分配给 Service。

同样直接创建上面的 Service：

```bash
➜ kubectl apply -f openelb-nginx-svc.yaml                
service/nginx created
➜ kubectl get svc nginx                   
NAME    TYPE           CLUSTER-IP   EXTERNAL-IP     PORT(S)        AGE
nginx   LoadBalancer   10.233.8.8   192.168.50.50   80:31438/TCP   3h19m
```

看到 Service 服务被分配了一个 `EXTERNAL-IP`，可以通过该地址来访问上面的 nginx 服务了：

```bash
http://192.168.50.50/
```

## 查看

查看网卡信息可以看到`EXTERNAL-IP`的地址被分配到了kube-ipvs0网卡上

```bash
➜ ip addr show kube-ipvs0
4: kube-ipvs0: <BROADCAST,NOARP> mtu 1500 qdisc noop state DOWN group default 
    link/ether a2:e8:1f:f7:e9:af brd ff:ff:ff:ff:ff:ff
    inet 10.233.0.3/32 scope global kube-ipvs0
       valid_lft forever preferred_lft forever
    inet 10.233.0.1/32 scope global kube-ipvs0
       valid_lft forever preferred_lft forever
    inet 10.233.11.71/32 scope global kube-ipvs0
       valid_lft forever preferred_lft forever
    inet 10.233.8.8/32 scope global kube-ipvs0
       valid_lft forever preferred_lft forever
    inet 192.168.50.50/32 scope global kube-ipvs0
       valid_lft forever preferred_lft forever
```

查看 pod 信息

```
➜ kubectl get pod -o wide
NAME                     READY   STATUS    RESTARTS   AGE     IP            NODE    NOMINATED NODE   READINESS GATES
nginx-6c8b449b8f-hxgqm   1/1     Running   0          3h29m   10.233.96.3   node2   <none>           <none>
```

查看 ipvs 信息

```
➜ ipvsadm|grep -A 1 31438
TCP  node1:31438 rr
  -> 10.233.96.3:http             Masq    1      0          0         
TCP  node1.cluster.local:31438 rr
  -> 10.233.96.3:http             Masq    1      0          0         
--
TCP  node1:31438 rr
  -> 10.233.96.3:http             Masq    1      0          0  
```

