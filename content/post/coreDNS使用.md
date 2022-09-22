---
title: "CoreDNS使用"
date: 2022-09-22T15:56:54+08:00
tags: ["k8s"]
---

### 下载二进制文件

```bash
https://github.com/coredns/coredns/releases/tag/v1.10.0
```

### 创建配置文件

```
.:53 { 
 forward . 114.114.114.114
 log
 } 
```

### 目录结构

```bash
.
├── Corefile
└── coredns

0 directories, 2 files
```

### 启动

```bash
./coredns
```

dig命令

> 查询DNS包括NS记录，A记录，MX记录等相关信息的工具
>
> @<服务器地址>：指定进行域名解析的域名服务器；
>
> -b<ip地址>：当主机具有多个IP地址，指定使用本机的哪个IP地址向域名服务器发送域名查询请求； 
>
> -f<文件名称>：指定dig以批处理的方式运行，指定的文件中保存着需要批处理查询的DNS任务信息；
>
> -P：指定域名服务器所使用端口号；
>
> -t<类型>：指定要查询的DNS数据类型；
>
> -x<IP地址>：执行逆向域名查询；
>
> -**4**：使用IPv4
>
> -**6**：使用IPv6；
>
> -h：显示指令帮助信息

### 测试

```bash
dig @localhost www.baidu.com
# coredns日志
[INFO] [::1]:53726 - 37910 "A IN www.baidu.com. udp 42 false 4096" NOERROR qr,rd,ra 149 0.040773854s
```

### resolv.conf 

/etc/resolv.conf  是DNS客户机配置文件，用于设置DNS服务器的IP地址及DNS域名，还包含了主机的域名搜索顺序

resolv.conf的关键字主要有四个，分别是：

- nameserver  //定义DNS服务器的IP地址

- domain    //定义本地域名

- search    //定义域名的搜索列表

- sortlist    //对返回的域名进行排序

### 修改配置文件

```
.:53 {
  hosts {
    192.168.50.40 inksnw.com
  }
 forward . /etc/resolv.conf
 reload 5s
 log
 } 
```

执行测试

```bash
dig @localhost inksnw.b.c
nslookup inksnw.b.c localhost
```

可以看到`inksnw.b.c`都解析到了 `192.168.50.40`上

dns缓存清除方法

```bash
# mac
sudo dscacheutil -flushcache; sudo killall -HUP mDNSResponder
# ubuntu
systemctl status systemd-resolved #查看服务状态
# 检查DNS缓存统计信息
systemd-resolve --statistics
# 清空缓存
systemd-resolve --flush-caches
systemctl restart systemd-resolved
```

短域名的配置方式 

在 `/etc/resolv.conf`中添加,就会如果找不到xxx域名会自动寻找xx.b.c域名

```
search b.c
```

