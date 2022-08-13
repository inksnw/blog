---
title: "Iptables与ipvs"
date: 2022-05-25T11:07:01+08:00
tags: ["k8s"]
---

## filter表

```bash
iptables -F #清空（flush）
iptables -X #删除指定表中用户自定义的规则链（delete-chain）
iptables -nvL -t filter --line-number
```

```bash
#追加
iptables -A INPUT -p tcp --dport 8000 -j DROP 
#删除
iptables -D INPUT -p tcp --dport 8000 -j DROP 
#按行号删除
iptables -D INPUT 1
```

```bash
#开启关闭日志
iptables -A INPUT -j LOG
iptables -D INPUT -j LOG
```

## nat表

```bash
iptables -nvL -t nat --line-number
iptables -t nat -A PREROUTING -p tcp -d 192.168.50.18 --dport 80 -j DNAT --to-destination 192.168.50.18:8000
iptables -t nat -D PREROUTING 1 #按num删除
```

### 负载均衡

```bash
iptables -A PREROUTING -t nat -p tcp -d 192.168.50.18 --dport 80 -m statistic --mode random --probability 0.5 -j DNAT --to-destination 192.168.50.18:8000
```

probability公式
probability=1/(n-i+1)
n:后端的数量
i:序号(从1开始)

### docker端口映射

```bash
iptables -t nat -A PREROUTING -p tcp -d 192.168.50.18 --dport 80 -j DNAT --to-destination 172.17.0.2:80
iptables -A FORWARD -i enp1s0 -o docker0 -j ACCEPT
```

### 限流

```bash
iptables -A INPUT -p tcp --dport 9090 -m connlimit --connlimit-above 2 -j REJECT --reject-with tcp-reset
```

## 自定义链

```bash
iptables -t filter -N test
iptables -A INPUT -p tcp --dport 8000 -j test 
iptables -A test -p tcp --dport 8000 -j DROP
#删除引用计数为0并且不包含任何规则的自定义链
iptables -X test 
```

