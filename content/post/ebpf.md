---
title: "Ebpf简单使用"
date: 2023-06-17T14:04:53+08:00
tags: ["ebpf"]
---

## 第一个 eBPF 程序
安装ecli

```bash
wget https://aka.pw/bpf-ecli 
```

```c
#define BPF_NO_GLOBAL_DATA
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

typedef unsigned int u32;
typedef int pid_t;
const pid_t pid_filter = 0;

char LICENSE[] SEC("license") = "Dual BSD/GPL";

SEC("tp/syscalls/sys_enter_write")
int handle_tp(void *ctx)
{
 pid_t pid = bpf_get_current_pid_tgid() >> 32;
 if (pid_filter && pid != pid_filter)
  return 0;
 bpf_printk("BPF triggered from PID %d.\n", pid);
 return 0;
}
```

编译程序：

```bash
docker run -it -v `pwd`/:/src/ yunwei37/ebpm:latest
```

然后使用 ecli 运行编译后的程序

```bash
tree
.
├── minimal.bpf.c
├── minimal.bpf.o
├── minimal.skel.json
└── package.json
# 运行
ecli run /root/Desktop/quickstart/package.json
Runing eBPF program...
```

查看日志

```bash
cat /sys/kernel/debug/tracing/trace_pipe
cat-38623   [004] d..31  7645.145301: bpf_trace_printk: BPF triggered from PID 38623.
cat-38623   [004] d..31  7645.145304: bpf_trace_printk: BPF triggered from PID 38623.
```

## 监控 Go 程序写入

写一个简单go程序

```go
package main

import (
	"fmt"
	"os"
	"time"
)

func writeFile() {
	f, err := os.OpenFile("test.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	f.WriteString(time.Now().String())
}

func main() {
	fmt.Println("当前的PID是：", os.Getpid())
	for {
		writeFile()
		fmt.Println("写入成功", time.Now())
		time.Sleep(time.Second * 5)
	}
}
```

启动 Go 程序执行写入操作。

```bash
当前的PID是： 39228
写入成功 2023-06-17 16:48:56.009876885 +0800 CST m=+0.001414184
写入成功 2023-06-17 16:49:01.011167331 +0800 CST m=+5.002704636
```

修改 tracepoint/test.bpf.c 文件，只对指定 PID 的程序进行跟踪。

```c
#define BPF_NO_GLOBAL_DATA
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

typedef unsigned int u32;
typedef int pid_t;
const pid_t pid_filter = 0;
const int myappid=39228; // 替换成 Go 程序的 PID
char LICENSE[] SEC("license") = "Dual BSD/GPL";

SEC("tracepoint/syscalls/sys_enter_write")
int handle_tp(void *ctx)
{
 pid_t pid = bpf_get_current_pid_tgid() >> 32;
 if (pid_filter && pid != pid_filter)
    return 0;
 if(pid!=myappid){
    return 0;
  }
 bpf_printk("triggered from PID %d.\n", pid);
 return 0;
}
```

使用 ecc 编译程序：

```bash
#编译
docker run -it -v `pwd`/:/src/ yunwei37/ebpm:latest
#执行
ecli run /root/Desktop/quickstart/package.json
#查看效果
cat /sys/kernel/debug/tracing/trace_pipe
# 输出
___go_build_mai-39234   [000] d..31  8010.976013: bpf_trace_printk: triggered from PID 39228.
___go_build_mai-39234   [000] d..31  8010.976240: bpf_trace_printk: triggered from PID 39228.
```

## kprobe 监控 Go 程序

kprobes ：动态内核跟踪技术，可以定义自己的回调函数，在内核几乎所有的函数中动态地插入探测点


继续使用 kprobe 来监控上面运行的 Go 程序，修改 tracepoint/test.bpf.c 文件，只对指定 PID 的程序进行跟踪。

```c
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <linux/limits.h> // 定义了NAME_MAX
#include <stddef.h> // size_t 无符号整数类型，通常被用来表示内存块的大小或元素的数量
char LICENSE[] SEC("license") = "Dual BSD/GPL";
typedef unsigned int u32;
struct data_t {
    u32 pid;
    char comm[NAME_MAX];  //NAME_MAX 文件名的最大长度，通常也可以用于进程或线程名称的最大长度
};
SEC("kprobe/__x64_sys_write")
int kprobe_write(struct pt_regs *ctx, int fd, const void *buf, size_t count)
{
    struct data_t data = {};
    data.pid = bpf_get_current_pid_tgid() >> 32;
    if (data.pid!=39228){ // 替换成 Go 程序的 PID
        return 0;
    }
    bpf_get_current_comm(&data.comm, sizeof(data.comm));
    bpf_printk("pid= %d,name:%s. writing data\n",  data.pid, data.comm);
    return 0;
}
```

运行查看

```bash
#编译
docker run -it -v `pwd`/:/src/ yunwei37/ebpm:latest
#执行
ecli run /root/Desktop/quickstart/package.json
#查看效果
cat /sys/kernel/debug/tracing/trace_pipe
# 输出
 ___go_build_mai-39232   [000] d..31  8306.304318: bpf_trace_printk: pid= 39228,name:___go_build_mai. writing data
 ___go_build_mai-39232   [000] d..31  8306.304379: bpf_trace_printk: pid= 39228,name:___go_build_mai. writing data
```

## XDP 拦截 ICMP 协议

<img src="http://inksnw.asuscomm.com:3001/blog/ebpf_79a9c2792be062eb095fa7e0146a80cf.webp" alt="Untitled.png" style="zoom:50%;" />

xdp_md 在头文件 /usr/include/linux/bpf.h 有定义：
- data： 数据包数据的地址。 指向数据包数据的开头。
- data_end： 数据包数据的结束地址。指向数据包的结尾。
- data_meta： 数据包元数据的地址。存储有关数据包的附加信息。
- ingress_ifindex： 接收数据包的网络接口的索引。
- rx_queue_index： 接收数据包的接收队列的索引。

头文件中定义 if_ether.h。它代表以太网链路层报头。其主要目的是定义以太网报头的结构，其中包括源 MAC 地址和目的 MAC 地址，以及以太网协议类型。

```c
struct ethhdr
{
unsigned char h_dest[ETH_ALEN]; //目的MAC地址
unsigned char h_source[ETH_ALEN]; //源MAC地址
__u16 h_proto ; //网络层所使用的协议类型
}__attribute__((packed))
```

头文件 iphdr 中定义 <linux/ip.h>。它用于描述 IPv4 数据包的 IP 标头。该结构包括 IP 版本、报头长度、服务类型、数据包总长度、标识号、标志、生存时间、协议、校验和、源 IP 地址和目标 IP 地址等字段。协议常见数值：
- 1：ICMP（Internet 控制报文协议）
- 2：IGMP（Internet 组管理协议）
- 6：TCP（传输控制协议）
- 17：UDP（用户数据报协议）
- 41：IPv6 封装的 IPv6 数据报
- 47：GRE（通用路由封装）
- 50：ESP（封装安全载荷）
- 51：AH（认证头部）
- 89：OSPF（开放式最短路径优先协议）
- 132：SCTP（流控制传输协议）

修改代码

```c
#include <linux/bpf.h> 
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/in.h>

SEC("xdp")
int my_pass(struct xdp_md* ctx) {
    void *data = (void*)(long)ctx->data;
    void *data_end = (void*)(long)ctx->data_end;
    int pkt_sz = data_end - data;
    struct ethhdr *eth = data;  // 链路层
    if ((void*)eth + sizeof(*eth) > data_end) {  //如果包不完整、或者被篡改， 我们直接DROP
        bpf_printk("Invalid ethernet header\n");
        return XDP_DROP;
    }
    struct iphdr *ip = data + sizeof(*eth); // 得到了 ip层
    if ((void*)ip + sizeof(*ip) > data_end) {
        bpf_printk("Invalid IP header\n");
        return XDP_DROP;
    }
   if (ip->protocol == IPPROTO_ICMP) {
        // 拦截 ICMP Ping 请求
        bpf_printk("Drop ICMP packets\n");
        return XDP_DROP;
    }
    bpf_printk("output:Packet size is %d, protocol is %d\n", pkt_sz, ip->protocol);
    return XDP_PASS;
}

char __license[] SEC("license") = "GPL";
```

使用 ecc 编译程序：

```bash
docker run -it -v `pwd`/:/src/ yunwei37/ebpm:latest
```

启用 XDP 之前先创建一个容器。

```bash
docker run -itd --name nginx nginx
```

获取容器的 IP 地址。此时 curl 和 ping 容器都是可以成功访问的。

```bash
docker inspect -f "{{ .NetworkSettings.IPAddress }}" nginx
# 输出
172.17.0.2
```

启用 XDP, 试了下直接操作虚拟机的网卡会报错 Error: virtio_net: Can't set XDP while host is implementing GRO_HW/CSUM, disable GRO_HW/CSUM first. 所以操作docker0 试一下

```bash
# ip link set dev <网卡> xdp obj <文件名> sec xdp verbose
ip link set dev docker0 xdp obj xdp.bpf.o sec xdp verbose
```

执行 ip link 命令可以看到 docker0 网卡上挂载了 XDP 程序。

```bash
docker0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 xdpgeneric qdisc noqueue state UP mode DEFAULT group default 
    link/ether 02:42:c9:e3:96:da brd ff:ff:ff:ff:ff:ff
    prog/xdp id 81 tag 0720394bf900dfe8 jited 
```

此时再次访问容器，会发现无法通过 Ping 访问容器了，但是 curl 还是可以访问容器。查看 trace 日志。

```bash
cat /sys/kernel/debug/tracing/trace_pipe

ping-6367    [003] d.s31   511.259245: bpf_trace_printk: Drop ICMP packets
curl-6380    [003] d.s31   513.619792: bpf_trace_printk: output:Packet size is 74, protocol is 6
```

卸载 XDP：

```bash
ip link set dev docker0 xdp off
```