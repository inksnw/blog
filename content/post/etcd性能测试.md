---
title: "Etcd性能测试"
date: 2023-07-17T18:10:12+08:00
tags: ["k8s"]
---

## 基础配置

```bash
alias etcdctl='etcdctl --key=/etc/ssl/etcd/ssl/admin-node1-key.pem --cert=/etc/ssl/etcd/ssl/admin-node1.pem --cacert=/etc/ssl/etcd/ssl/ca.pem --endpoints 192.168.50.50:2379'
etcdctl -w table member list
+------------------+---------+------------+----------------------------+----------------------------+------------+
|        ID        | STATUS  |    NAME    |         PEER ADDRS         |        CLIENT ADDRS        | IS LEARNER |
+------------------+---------+------------+----------------------------+----------------------------+------------+
| ae12e8bc550d8bfc | started | etcd-node1 | https://192.168.50.50:2380 | https://192.168.50.50:2379 |      false |
+------------------+---------+------------+----------------------------+----------------------------+------------+
```

## 压测

```bash
# 不同大小的集群负载，支持参数为：s,m,l,xl
etcdctl check perf --load=s
etcdctl check perf --load=l
# 测试失败。可以看到，集群可执行 3934/s 的写操作
FAIL: Throughput too low: 3934 writes/s
PASS: Slowest request took 0.461817s
PASS: Stddev is 0.046747s
FAIL
```
## 信息查看

### lsof

```basic
root@node1:~# lsof -p $(pgrep etcd)|egrep 'wal|COMMAND'
COMMAND PID USER   FD      TYPE             DEVICE  SIZE/OFF    NODE NAME
etcd    593 root   10r      DIR              252,2      4096  917520 /var/lib/etcd/member/wal
etcd    593 root  581w      REG              252,2  64000000  917576 /var/lib/etcd/member/wal/0000000000000011-00000000000d4a35.wal
etcd    593 root  601w      REG              252,2  64000000  917589 /var/lib/etcd/member/wal/0.tmp
```

在etcd中，'wal'文件包含了所有待写入磁盘的数据，这样即使系统突然崩溃，也可以从'wal'文件中恢复数据。

以下是每一列的含义：

- `COMMAND`: 进程名，这里是etcd。
- `PID`: 进程ID，这里是593。
- `USER`: 执行这个进程的用户，这里是root。
- `FD`: 文件描述符编号+类型，代表进程与文件之间的接口。这里包括10r，581w和601w，其中'r'代表读，'w'代表写。
- `TYPE`: 文件类型，这里包括DIR（目录）和REG（普通文件）。
- `DEVICE`: 设备号，是设备在系统中的标识。
- `SIZE/OFF`: 文件的大小（对普通文件）或文件在文件系统中的偏移量（对目录）。
- `NODE`: 节点，代表文件在文件系统中的位置。
- `NAME`: 文件路径。

### iostat

```bash
iostat 2 -h
```

```bash
avg-cpu:  %user   %nice %system %iowait  %steal   %idle
          54.4%    0.0%   26.9%    8.2%    0.0%   10.5%

      tps    kB_read/s    kB_wrtn/s    kB_dscd/s    kB_read    kB_wrtn    kB_dscd Device
   322.50         0.0k        15.2M         0.0k       0.0k      30.5M       0.0k vda
```

先来解释一下CPU统计的部分：

- `%user`: 用户态CPU时间百分比。CPU花在执行用户进程（用户态）的时间百分比，不包括优先级调整为负的进程。
- `%nice`: 用户态的低优先级CPU时间百分比。CPU花在执行优先级调整为负的用户进程的时间百分比。
- `%system`: 内核态CPU时间百分比。CPU花在系统管理（内核态）的时间百分比。
- `%iowait`: I/O等待时间百分比。CPU因为等待I/O操作（如读取磁盘数据）而空闲的时间百分比。
- `%steal`: 被强占的CPU时间百分比。在虚拟环境中，一台物理机上的一个虚拟机（guest）可能会使用其他虚拟机的CPU资源，这个数值表示这种情况发生的时间百分比。
- `%idle`: CPU空闲时间百分比。CPU没有执行任何任务，也没有等待I/O操作的时间百分比。

再来看看设备统计的部分：

- `tps`: 每秒传输的事务数。显示了每秒发生的读取和写入操作的总次数。
- `kB_read/s`: 每秒读取的数据量（单位为KB）。
- `kB_wrtn/s`: 每秒写入的数据量（单位为KB）。
- `kB_dscd/s`: 每秒丢弃的数据量（单位为KB）。这通常涉及到虚拟内存和交换空间。
- `kB_read`: 读取的总数据量（单位为KB）。
- `kB_wrtn`: 写入的总数据量（单位为KB）。
- `kB_dscd`: 丢弃的总数据量（单位为KB）。
- `Device`: 设备名称。

所以从这个输出中，我们可以看出：

- CPU的大部分时间都花在执行用户进程和系统管理上，有一部分时间在等待I/O操作，有一小部分时间是空闲的。
- 设备`vda`每秒进行约322.5次读写操作，读取数据的速度几乎为0，写入数据的速度约为15.2MB/s，没有丢弃数据，总共写入了约30.5MB的数据。

## 使用Fio测试

```bash
# trace=write 只追踪特定的系统调用。表达式可以是signal, abbrev, verbose, raw, read 和 write等几种类型。
strace -p $(pgrep etcd) -e trace=write
```

### bs参数

```bash
strace: Process 572 attached
write(79, "\27\3\3\0%\0\0\0\0\0\0\1!\306\0CG\344\215\262=\31\30W\245\213gQ\354\356\274_"..., 42) = 42
write(79, "\27\3\3\0)\0\0\0\0\0\0\1\"\340\217\267^t%W\214%\272\376,\254\201\33\26\227\25\361"..., 46) = 46
write(79, "\27\3\3\0C\0\0\0\0\0\0\1#<\371\341\205U^!I\254\306\234\341\203L\234\312\314\266Y"..., 72) = 72
```

在`strace`的输出中，`write`系统调用后面的数字，比如`42`，`46`和`72`，表示的是写入的字节数, write系统调用的原型如下

```c
ssize_t write(int fd, const void *buf, size_t count);
```

- `fd`是文件描述符，代表要写入的文件或设备。
- `buf`是一个指向要写入的数据的指针。
- `count`是要写入的字节数。

结果显示 WAL 文件写入大小几乎都在 100 字节这个范围， 客户环境中在 2200-2400 范围，因此设置参数`--bs=2300`

### fdatasync参数

`fdatasync`函数的原型如下：

```c
int fdatasync(int fd);
```

追踪etcd使用

```bash
strace -p $(pgrep etcd) -e trace=fdatasync
strace: Process 572 attached
fdatasync(9)                            = 0
```

每当客户端添加或更新键值对数据时，etcd 会向 WAL 文件添加一条入库记录条目。再进一步处理之前，etcd 必须 100% 确保 WAL 条目已经被持久化。 要在 Linux 实现这一点，仅使用**write**系统调用是不够的，因为对物理存储的写入操作可能会发生延迟。比如， Linux 可能会将写入的 WAL 条目在内核内存缓存中保留一段时间（例如，页缓存）。如果要确保数据被写入持久化存储，你必须在 write 系统调用之后调用 fdatasync 系统调用

### 测试

下面使用 fio 来查看 fdatasync 的延迟：

```bash
fio --rw=write --ioengine=sync --fdatasync=1 --directory=benchmark --size=22m --bs=2300 --name=sandbox
sandbox: (g=0): rw=write, bs=(R) 2300B-2300B, (W) 2300B-2300B, (T) 2300B-2300B, ioengine=sync, iodepth=1
fio-3.16
Starting 1 process
Jobs: 1 (f=1): [W(1)][100.0%][w=490KiB/s][w=218 IOPS][eta 00m:00s]
sandbox: (groupid=0, jobs=1): err= 0: pid=5038: Tue Jul 18 02:33:32 2023
  write: IOPS=294, BW=661KiB/s (677kB/s)(21.0MiB/34072msec); 0 zone resets
    clat (nsec): min=1873, max=5726.1k, avg=61779.17, stdev=139361.02
     lat (usec): min=2, max=5727, avg=62.25, stdev=139.53
    clat percentiles (usec):
     |  1.00th=[    3],  5.00th=[    4], 10.00th=[    4], 20.00th=[    5],
     | 30.00th=[    6], 40.00th=[    7], 50.00th=[   17], 60.00th=[   33],
     | 70.00th=[   45], 80.00th=[   73], 90.00th=[  111], 95.00th=[  412],
     | 99.00th=[  529], 99.50th=[  553], 99.90th=[  701], 99.95th=[ 1123],
     | 99.99th=[ 4621]
```

可以看到延迟的 99th 百分比为 `529 usec` (或0.5ms)，说明集群足够快 (etcd 官方建议最小 10ms)。IOPS 为 `w=218 IOPS`

- `--rw=write`: 这个选项指定了I/O的类型，`write`表示只进行写操作。
- `--ioengine=sync`: 这个选项指定了I/O引擎类型。`sync`表示使用基于系统调用的同步I/O。
- `--fdatasync=1`: 这个选项指定了在每次写操作后是否调用`fdatasync`来强制把数据写入存储设备。
- `--directory=benchmark`: 这个选项指定了测试文件存放的目录。
- `--size=22m`: 这个选项指定了每个线程写入的数据量，`22m`表示每个线程写入22MB的数据。
- `--bs=2300`: 这个选项指定了块大小，`2300`表示每次写入2300字节的数据。
- `--name=sandbox`: 这个选项指定了测试的名称，可以用来区分不同的测试。

在这个命令中没有指定线程数，所以默认使用一个线程进行测试。如果你想指定线程数，可以添加`--numjobs=N`选项，其中`N`是线程数。
