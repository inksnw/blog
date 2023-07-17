---
title: "K8s基础知识"
date: 2022-05-25T10:36:08+08:00
tags: ["k8s"]
---

<!--more-->

# 1 快速入门

## 1.1基础知识

### 1.11组件解析

**基本架构**

master组件

> Master是整个k8s集群中的控制节点,负整个集群中资源的调度和控制,至少包括四个组件.
> Api Server,Schedule,Controller,etcd
>
> Api Server:
>
> ​		是用户使用k8s的入口,封装了核对对象的curd,提供了RESTFul风格的API.
>
> ​		APIServer总体上由两部分组成: HTTP/HTTPS服务和一些功能性插件.这些功能性插件又分为两种:一部分与Iaas平台相关;另一部分与资源控制(Admission Control)相关.
>
> Schedule:
>
> ​		Scheduler的作用是根据特定的调度算法把pod调度到node节点上,输入是待调度的pod和可用的工作节点列表,输出则是一个已经绑定了pod的节点
>
> Controller:
>
> ​		重点实现service Endpoint的动态更新,管理着k8s集群中的各种控制节点,包括replication Controller和node Controller
>
> ​		与APIServer相比,APIServer接收请求,根据请求的配置文件,定制资源的期望状态,而Controller Manager对资源的期望状态和当前状态进行比较,通过重启,迁移,删除等,保证资源在达到预期状态.

node节点组件

> Docker,kubelet,kube-proxy,Fluentd,flannel
>
> kubelet:
>
> ​		相当于集群中的一个Agent,负责与Master节点通信,同时Kubelet负责工作结点间的状态同步,对工作节点的所有资源(主要是容器)进行管理.kubelet与cAdvisor交互来抓取docker容器和主机的资源信息.
>
> kubectl垃圾回收机制,负责容器垃圾和镜像垃圾的回收.
>
> Kube-proxy:
>
> ​		运行在Node节点,主要提供两种功能:
>
>   - 管理service并提供service通信和负载均衡功能
>      提供算法将客户端流量负载均衡到service对应的一组后端pod
>   - 使用etcd的watch机制,实现服务发现功能
>      维护一张从service到endpoint的映射关系
>      		- 保证后端pod的ip变化不对访问者的访问造成影响
>
> Flannel:
>
> ​	flannel是针对k8s设计的一个覆盖网络(Overlay Network)工具,使不同节点的容器能够获得同一内网且不重复的ip地址,并且可通过内网ip通信.

扩展组件

> 网络和网络策略
>
> ​		ACI通过Cisco ACI提供集成的容器网络和安全网络.
>
> ​		Antrea在第3/4层工作,为k8s提供网络连接和安全服务,Antrea利用Open vSwitch作为网络的数据面
>
> ​		Calico是一个安全的L3网络和网络策略驱动.
>
> ​		Canal结合Flannel和Calico提供网络和网络策略
>
> 服务发现
>
> ​		CoreDNS是一种灵活的,可扩展的DNS服务器
>
> 服务管理
>
> ​		Ingress Controller(入栈流量控制器)k8s场景中应用级别的负载均衡解决方案
>
> 

## 1.2环境部署

### 1.2.1基础配置

```bash
#免密登录,略
#禁用swap,略
```

网络参数

```bash
#配置iptables参数,使用流经网桥的流量也经过iptables
cat >> /etc/sysctl.d/k8s.conf <<EOF
vm.swappiness=0
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
EOF

#配置生效
modprobe br_netfilter
modprobe overlay
sysctl -p /etc/sysctl.d/k8s.conf
```

安装docker

```bash
apt-get update
apt-get install apt-transport-https ca-certificates curl gnupg lsb-release -y

#配置软件源
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
add-apt-repository \
   "deb [arch=amd64] https://mirrors.tuna.tsinghua.edu.cn/docker-ce/linux/ubuntu \
   $(lsb_release -cs) \
   stable"
apt-get update
apt-get install docker-ce docker-ce-cli containerd.io -y

#国内源
cat >> /etc/docker/daemon.json <<EOF
{
    "registry-mirrors": [
        "https://hub-mirror.c.163.com"
    ],
    "exec-opts": [
        "native.cgroupdriver=systemd"
    ]
}
EOF
# 启动
systemctl restart docker
systemctl enable docker
#检查
docker info|grep Cgroup
```

kubeadm配置

```bash
#配置软件源
apt-get update && apt-get install -y 
curl https://mirrors.aliyun.com/kubernetes/apt/doc/apt-key.gpg | apt-key add - 
cat <<EOF >/etc/apt/sources.list.d/kubernetes.list
deb https://mirrors.aliyun.com/kubernetes/apt/ kubernetes-xenial main
EOF
apt-get update

#查看支持的软件版本
apt-cache madison kubeadm
#安装
apt-get install -y kubelet kubeadm kubectl 
# 会同时安装以下软件
# ebtables 是linux下网络数据包过滤的配置工具
# cni是容器网络通信的软件
# socat是kubelet的依赖
# cri-tools是cri容器运行时接口的命令行工具
```

```bash
# 服务自启动,在kubeadm init前,暂不用启动
systemctl enable kubelet
```

信息检查

```bash
# 查看默认配置
kubeadm config print init-defaults
# 检查kubeadm所依赖的镜像版本
kubeadm config images list 
# 拉取镜像文件
kubeadm config images pull
```

集群初始化(主节点)

```bash
kubeadm init --kubernetes-version=1.23.6 --apiserver-advertise-address=192.168.50.40 \
--service-cidr=10.96.0.0/12 --pod-network-cidr=10.244.0.0/16 --ignore-preflight-errors=Swap
#注意:
# --apiserver-advertise-address要设定为当前集群的master地址
# --pod-network-cidr选项用于指定pod分配时使用的地址,通常应与使用的网络插件(flannel,calico)的默认保持一致
# 10.244.0.0/16是flannel的默认网络
# --service-cidr用于指定为service分配时使用的网络地址,默认为10.96.0.0/12
```

出现以下字样表示启动成功

```bash
kubeadm join 192.168.50.40:6443 --token 8hbz0g.5wg7qy6641bx5msw \
        --discovery-token-ca-cert-hash sha256:b1bdb64e7f2d647515039f54017a086896a68e85a3d246786280333dff4ee317 
```

查看日志

```bash
journalctl -u kubelet -f
```

日志解析

```
[kubelet]生成kubelet的配置文件 /var/lib/kubelet/config.yaml
[certificates]生成相关的各种证书
[kubeconfig]生成相关的kubeconfig文件
[bootstraptoken]生成的token记录下来,后续使用kubeadm join添加节点
```

配置用户访问

```bash
mkdir -p ~/.kube
cp -i /etc/kubernetes/admin.conf ~/.kube/config
chown $(id -u):$(id -g) ~/.kube/config
```

检查效果

```
kubectl get cs
kubectl get nodes
```

网络现状

```bash
journalctl -u kubelet -f
#报错 Unable to update cni config
```

配置网络

```bash
cd ➜ && mkdir flannel && cd flannel
wget https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml
kubectl apply -f kube-flannel.yml
```

```bash
#检查效果
kubectl get nodes
kubectl get ns
kubectl get pod -n kube-system
```

配置节点

```bash
kubeadm join 192.168.50.40:6443 --token 8hbz0g.5wg7qy6641bx5msw \
        --discovery-token-ca-cert-hash sha256:b1bdb64e7f2d647515039f54017a086896a68e85a3d246786280333dff4ee317 
```

### 1.2.2其它功能

kubectl taint nodes k8s-master node-role.kubernetes.io/master:NoSchedule-bash补全

```bash
echo "source <(kubectl completion bash)" >>~/.bashrc
```

污点处理

```bash
#获取污点信息
kubectl describe nodes|grep -i taint
#清除master上的污点,最后一个"-"代表删除
kubectl taint nodes node1 node-role.kubernetes.io/master:NoSchedule-
#添加污点
kubectl taint nodes node1 node-role.kubernetes.io/master:NoSchedule
```

token信息

```bash
#查看token时效
kubeadm token list
#重新生成
kubeadm token create --print-join-command
```

节点删除

```bash
#在master执行
kubectl drain node2 --delete-local-data --force --ignore-daemonsets
kubectl delete node node2
#在node上执行
kubeadm reset
systemctl stop kubelet docker
rm -rf /etc/kubernetes/
ifconfig cnio down
```

### 1.2.3命令实践

应用实践

```bash
#创建
kubectl create deployment nginx --image=nginx
kubectl get deployments.apps
#扩容
kubectl scale --replicas=3 deploy nginx
#缩容
kubectl scale --replicas=1 deploy nginx
#pod的版本更新
kubectl set image deploy/nginx nginx='nginx:1.11.0'
#查看版本更新状态和历史
kubectl rollout status deploy nginx
kubectl rollout history deploy nginx
#回滚更改
kubectl rollout undo deploy nginx
#回到指定版本
kubectl rollout undo --to-revision=2 deploy nginx
```

## 1.3细节解析

### 1.3.1初始化流程

简单流程

>master节点启动:
>
>​	生成对外提供服务需要的各种私钥以及数据证书
>
>​	生成控制组件的kubeconfig文件及相关配置文件
>
>​	生成控制组件的pod对象需要的manifest文件
>
>​	为主节点添加标识,不参与node角色的工作
>
>​	生成统一认证token信息,方便节点加入
>
>​	基于TLS的安全引导相关配置,角色策略,签名请求,自动配置策略
>
>​	为集群安装DNS和kube-proxy插件
>
>node节点加入
>
>​	环境检查,读取配置信息
>
>​	获取集群相关数据后启动kubelet服务
>
>​	获取认证信息后,基于证书方式进行通信

```bash
#查看ca的文件
ls /etc/kubernetes/pki/
#生成控制组件的kubeconfig文件及相关配置文件
ls /etc/kubernetes
#kubelet属性信息
cat /var/lib/kubelet/config.yaml
#kubeadm启动参数
cat /var/lib/kubelet/kubeadm-flags.env
```

生成控制组件的pod对象需要的manifest文件

```bash
#查看默认启动的容器
ls /etc/kubernetes/manifests
#查看配置信息
kubectl describe cm kubelet-config-1.23 -n kube-system|grep staticPodPath
#查看etcd信息
kubectl get pods -n kube-system|grep etcd
#安装etcd客户端
apt install etcd-client
#起个别名
alias etcdctl='etcdctl --key=/etc/kubernetes/pki/etcd/server.key --cert=/etc/kubernetes/pki/etcd/server.crt --cacert=/etc/kubernetes/pki/etcd/ca.crt --endpoints 192.168.50.40:2379'
#查看节点
ETCDCTL_API=3 etcdctl -w table member list
```

查看证书相关信息

```bash
#查看角色信息
kubectl get role -n kube-system
```

为集群安装DNS和kube-proxy插件

```bash
kubectl get pod -n kube-system|egrep 'proxy|core'
```

Node注册解析

```bash
kubectl -n kube-system get cm kubeadm-config -o yaml
```

### 1.3.2组件解析

> ​		k8s关于节点的控制,是通过Node Controller进行统一管理的.默认每10s节点上的kubelet向master汇报一次信息,如果超过4次master节点没有收到信息,就会将该节点置为NotReady状态.
>
> ​		由于10s一次大量信息传递有些浪费,1.13版以后,引入了与节点配套使用的资源对象:节点租约
>
> ​		kubelet每10s向master的apiserver同步一下节点状态,每次更新的是租约的有效期限,租约的同步间隔默认40s,默认采集到的信息保存到租约对象中,一旦租约存储的数据发生改变或每5分钟才将node的信息上报给master节点

```bash
kubectl get ns kube-node-lease
kubectl get lease -n kube-node-lease
```

# 2核心知识

### 2.1Pods

#### 2.1.1概述

| 关系      | 解析                                                         |
| --------- | ------------------------------------------------------------ |
| pod&容器  | 一个pod可以有多个容器,共享网络和存储<br />每个pod中有一个pause容器保存所有容器状态,通过管理pause容器实现管理pod中所有容器的效果 |
| pod&节点  | 同一Pod中的容器会被调度到相同node节点<br />不同节点间的Pod通信是通过虚拟二级网络技术实现的 |
| pod&pod   | 普通pod和静态pod                                             |
| pod&应用  | 每个pod都是应用的一个实例,有专门的Ip,与容器暴露的端口组合为一个service的endpoint地址<br />每个service由kube-proxy转换成本地的ipvs或iptables规则 |
| 应用&应用 | 每个service的endpoint地址由coredns组件解析为对应的服务名称,其它service的pod通过访问该名称实现应用间通信的效果 |
| 外部&pod  | 外部流量通过节点网卡进入K8s集群,基于ipvs或者iptables规则找到对应的service,进而找到对应的Pod地址 |

pod通信

> pod内 多容器通信 :容器间通信(容器模型)
>
> 单节点内多Pod通信:主机间容器通信(host模型)
>
> 多节点内多pod通信: 跨主机网络解决方案(overlay模型)

>工作负载型资源
>
>​	  Pod,Deplyment,DaemonSet,Replica,StateFulSet,Job,CronJob
>
>服务发现和负载均衡资源
>
>​	Svc,Ingress
>
>配置和存储
>
>​	cm,Secret,pv,pvc,DownwardAPI
>
>动态调整资源
>
>​	HPA,VPA
>
>资源隔离,权限控制
>
>​	namespace,nodes,cluserroles,Roles	

#### 2.2.2流程解析

>用户向master节点上的apiserver发起一个创建pod的请求
>
>apiserver将该信息写入etcd
>
>scheduluer检测到apiserver上有建立pod请求,开始调度该pod到指定的node,同时更新信息到etcd
>
>kubelet检测到有新的Pod调度过来,通过docker引擎运行pod对象
>
>kubelet通过container runtime取到pod状态,同步信息到apiserver,由它更新到etcd

三种模式

>sideCar模式:	为主容器提供辅助功能
>
>Adapater模式:	帮助主容器通信过程中的信息修正
>
>Ambassador模式: 	利用pod多容器共享数据的特性,代理后端容器和其它应用进行交流

流程解析

>顺序1:	init容器
>
>​		初始化容器,独立于主容器之外,pod可有任意数量的init容器,init顺序执行,最后一个执行完成后再启动主容器.主要是为主容器准备功能,如向存储卷写入数据,然后挂载到主容器上
>
>顺序2:	生命周期钩子
>
>​		启动后钩子:	与主进程并行运行
>
>​		运行中钩子:	Liveiness,Readiness
>
>​		停止前钩子:	先执行钩子,完成后向容器发送SIGTERM信号,无效则会被杀死,未成功会告警
>
>顺序3:	pod关闭
>
>​		执行停止前钩子,等待执行完毕
>
>​		向容器的主进程发送SIGTERM信号
>
>​		等待容器优雅关闭或者等待终止宽限期超时
>
>​		如未优雅关闭,使用SIGKILL信号强制终止进程
>
>​	设置终止宽限期
>
>​		sepc.terminationGracePeriod 默认为30s
>
>​		kubectl delete pod mypod --grace-period=0 --force

Pod状态

>正常情况:
>
>​		pod一旦绑定到Node节点,就永远不会移动,即使节点发生重启
>
>pod在整个周期中会出现5种状态:
>
>​		pending,running,succeeded,failed,unknown
>
>异常情况:
>
>​		业务运行过程中不可避免会出现意外,这时有三种策略对pod进行管理
>
>​		OnFailure,Never,和Always(默认)


pod状态

| 状态             | 描述                                                         |
| ---------------- | ------------------------------------------------------------ |
| Pending          | Apiserver已创建该server,但pod内有一个或多个镜像还未创建,可能还在下载中 |
| Running          | Pod内所有容器已创建,且至少有一个容器处于运行状态,正在启动或重启状态 |
| Succeeded        | 所有容器均成功执行退出,且不会再重启                          |
| Failed           | 所有容器均成功执行退出,且不会再重启                          |
| Unknown          | 由于某些原因无法获取pod的状态,如网络不通                     |
| CrashLookBackOff | 曾启动成功,后来重启次数过多                                  |
| Error            | 因集群配置,安全限制,资源等原因导致pod启动过程发生了异常      |
| Evited           | 系统内存或磁盘资源不足,导致Pod异常                           |

容器状态

| 容器状态   | 描述                                  |
| ---------- | ------------------------------------- |
| Waiting    | 容器处于Running和Terminated之前的状态 |
| Running    | 容器能够正常运行的状态                |
| Terminated | 容器已经被成功关闭了                  |

流程状态

| 流程状态        | 描述                             |
| --------------- | -------------------------------- |
| PodScheduled    | Pod被调度到某一个节点            |
| Ready           | 准备就绪,pod可处理请求           |
| Initialized     | Pod中所有初始init容器启动完毕    |
| Unschedulable   | 由于资源等限制,导致pod无法被调度 |
| ContainersReady | Pod中所有的容器都启动完毕了      |

重启策略

| 重启策略  | 描述                             |
| --------- | -------------------------------- |
| Always    | 容器失效时,即重启                |
| OnFailure | 容器终止运行,且退出码不为0时重启 |
| Never     | Pod不重启                        |

#### 2.2.3流程实践

pod实践

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: init-pod
  labels:
    app: init
spec:
  containers:
    - name: busybox
      image: busybox
      command: ["sh","-c","echo the app is running! && sleep 3600"]
  initContainers:
    - name: init-myservice
      image: busybox
      command: ["sh","-c","until nslookup myservice;do echo waiting for myservice;sleep 2; done;"]

```

```bash
#查看效果
kubectl get pod
kubectl describe pod init-pod
kubectl los init-pod -c init-myservice
```

service实践

```yaml
apiVersion: v1
kind: Service
metadata:
  name: myservice
spec:
  ports:
    - protocol: TCP
      port: 80
      targetPort: 9257
```

```bash
#查看效果
kubectl get pod -w
kubectl get svc
```

sidecar实践

>在一个pod启动两个容器,在访问B容器的时候都要经过A容器

poststart实践

>对于pod的启动流程,主要有两种钩子
>
>​		poststart,容器创建完成后立即即行,不保证于容器entrypoint之前运行
>
>​		preStop,容器终止前运行,会阻塞删除容器的操作
>
>钩子容器的实现方式: Exec和HTTP
>
>​		Exec:	在钩子事件触发时,直接在当前容器运行由用户定义的命令
>
>​		HTTP:	在当前容器向某url发起HTTP请求

poststop实践

>Pod对象移除前,做一些事情
>
>实现方式exec,httpGet,tcpSocket
>
>钩子处理的日志不会在pod事件中,如果失败postStart是FailedPostStartHook事件,prestop是FailedPreStopHook事件

### 2.2Pod策略

>容器是基于6个命名空间隔离的
>
>	- 内部所有容器共享pause容器的UTS|Network
>	- 可选共享命名空间:IPC,PID
>	- MNT和USER默认没有共享

#### 2.2.1安全相关

略

#### 2.2.2探测机制

>对于k8s内部的pod,常见的api接口有
>
>	- process health 状态健康检测接口
>	- metrics 监控指标接口
>	- readiness 容器可读状态接口
>	- liveness 容器存储状态接口
>	- tracing 链路监控的埋点接口
>	- logs 日志接口

>LivenessProbe
>
>​		周期性检测,检测未通过时,kubelet会根据restartPllicy的定义来决定是否重启,未定义时,容器未终止即为健康
>
>ReadinessProbe:
>
>​		周期性检测,检测未通过时,与该pod关联的Service会将该pod从可用端点列表中删除,直到再次就绪再添加回来,未定义时,只要容器未终止,即为就绪
>
>StartupProbe:
>
>​		用户自定义的一些探测机制,效果等同LivenessProbe

| 探针类型        | 解析                                         |
| --------------- | -------------------------------------------- |
| ExecAction      | 直接执行命令,成功返回表示探测成功            |
| TCPSocketAction | 端口正常打开,即成功                          |
| HTTPGetAction   | 向指定path发送http请求,2xx,3xx状态码表示成功 |

相关的属性

```yaml
	# livenessProbe,ReadinessProbe的属性一样
spec:
	containers:
	- name: ...
		image: ...
		livenessProbe:
		exec #命令式擦针
		httpGet #http get类型探针
		tcpSocket #tcp socket类型的探针
		initialDelaySecondss #发起初次探测的延后时长
		periodSeconds #请求周期
		timeoutSeconds #超时时长,默认1s
		successThreshold #连续成功几次才正常,默认1次
		failureThreshold #连接失败几次,才表示异常,默认3次
```

#### 2.2.3资源限制

略

#### 2.2.4服务质量

>高优先级:Guaranteed:
>
>​		pod内的每个容器同时设置了CPU和内存的requests和limits而且值必须相等
>
>中优先级:Burstable:
>
>​		pod至少有一个容器设置了cpu或内存的requests和limits
>
>低优先级: BestEffort
>
>​		没有任何一个容器设置了requests和limits

