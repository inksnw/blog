---
title: "Containerlab基础使用"
date: 2023-12-30T10:06:06+08:00

---

### 基本概念

Containerlab根据用户传递给它的拓扑信息来建立实验,构建网络交换和路由, 是一个很方便的学习工具

### 安装

官方文档(https://containerlab.dev/quickstart/)

```
bash -c "$(curl -sL https://get.containerlab.dev)"
```

### veth直连

定义拓扑文件, 创建clab执行脚本

```shell
#!/bin/bash
set -v
cat <<EOF>clab.yaml | clab deploy -t clab.yaml -
name: veth
topology:
  nodes:
    server1:
      kind: linux
      image: nicolaka/netshoot
      exec:
      - ip addr add 10.1.5.10/24 dev net0
      
    server2:
      kind: linux
      image: nicolaka/netshoot
      exec:
      - ip addr add 10.1.5.11/24 dev net0
      
  links:
    - endpoints: ["server1:net0","server2:net0"]
EOF
```

查看信息

```bash
root@node4:~# clab inspect -a
+---+-----------+----------+-------------------+--------------+-------------------+-------+---------+----------------+----------------------+
| # | Topo Path | Lab Name |       Name        | Container ID |       Image       | Kind  |  State  |  IPv4 Address  |     IPv6 Address     |
+---+-----------+----------+-------------------+--------------+-------------------+-------+---------+----------------+----------------------+
| 1 | clab.yaml | veth     | clab-veth-server1 | 66a991710bdc | nicolaka/netshoot | linux | running | 172.20.20.3/24 | 2001:172:20:20::3/64 |
| 2 |           |          | clab-veth-server2 | 2cdfbebe238f | nicolaka/netshoot | linux | running | 172.20.20.2/24 | 2001:172:20:20::2/64 |
+---+-----------+----------+-------------------+--------------+-------------------+-------+---------+----------------+----------------------+
```

销毁

```bash
clab destroy -t clab.yaml 
INFO[0000] Parsing & checking topology file: clab.yaml  
INFO[0000] Destroying lab: veth                         
INFO[0000] Removed container: clab-veth-server2         
INFO[0000] Removed container: clab-veth-server1         
INFO[0000] Removing containerlab host entries from /etc/hosts file 
INFO[0000] Removing ssh config for containerlab nodes   
root@node4:~# clab inspect -a
INFO[0000] no containers found 
```

### 网桥连接

```bash
set -v
brctl addbr  br-pool0
ifconfig br-pool0 up
cat <<EOF>br.yaml | clab deploy -t br.yaml -
name: bridge
topology: 
  nodes:
    br-pool0:
      kind: bridge
    server1:
      kind: linux
      image: nicolaka/netshoot
      exec:
      - ip addr add 10.1.5.10/24 dev net0
    server2:
      kind: linux
      image: nicolaka/netshoot
      exec: 
      - ip addr add 10.1.5.11/24 dev net0
      
  links: 
    - endpoints: ["br-pool0:eth1", "server1:net0"]
    - endpoints: ["br-pool0:eth2", "server2:net0"]
EOF
```

测试

```
docker exec -it cfeaba2f8bb2  /bin/bash
server2:~# ping 10.1.5.10
PING 10.1.5.10 (10.1.5.10) 56(84) bytes of data.
64 bytes from 10.1.5.10: icmp_seq=1 ttl=64 time=0.255 ms
64 bytes from 10.1.5.10: icmp_seq=2 ttl=64 time=0.106 ms
64 bytes from 10.1.5.10: icmp_seq=3 ttl=64 time=0.112 ms
```

可以发现ttl的值一直是64,表示网络走的是交换，如果走的路由，ttl会每走一次路由 -1

销毁

```
root@node4:~# ifconfig br-pool0 down
root@node4:~# brctl delbr br-pool0
clab destroy -t br.yaml
```

### 路由连接

创建vyos配置`gw0-boot.cfg`

```
interfaces {
    ethernet eth1 {
        address 10.1.5.1/24  # 设置接口 eth1 的 IP 地址和子网掩码
        duplex auto  # 自动设置双工模式
        smp-affinity auto  # 自动设置 SMP 亲和性
        speed auto  # 自动设置速度
    }
    ethernet eth2 {
        address 10.1.8.1/24  # 设置接口 eth2 的 IP 地址和子网掩码
        duplex auto  # 自动设置双工模式
        smp-affinity auto  # 自动设置 SMP 亲和性
        speed auto  # 自动设置速度
    }
    loopback lo {
    }
}

nat {
    source {
        rule 100 {
            outbound-interface eth0  # 设置 NAT 出口接口为 eth0
            source {
                address 10.1.0.0/16   # 源地址为 10.1.0.0/16
            }
            translation {
                address masquerade  # 使用地址伪装进行 NAT
            }
        }
    }
}

system {
    host-name "vyos"  # 设置主机名为 "vyos"
    login {
        user vyos {
            authentication {
                encrypted-password "$6$QxPS.uk6mfo$9QBSo8u1FkH16gMyAVhus6fU3LOzvLR9Z9.82m3tiHFAxTtIkhaZSWssSgzt4v4dGAL8rhVQxTg0oAG9/q11h/"
                plaintext-password ""  # 设置用户 vyos 的密码
            }
            level "admin"  # 用户权限级别为管理员
        }
    }
    syslog {
        global {
            facility all {
                level "info"  # 设置全局日志级别为 info
            }
            facility protocols {
                level "debug"  # 设置协议日志级别为 debug
            }
        }
    }
    ntp {
        server "0.pool.ntp.org"
        server "1.pool.ntp.org"
        server "2.pool.ntp.org"  # 配置 NTP 服务器
    }
    config-management {
        commit-revisions "100"  # 保留最近 100 次配置更改
    }
    console {
        device ttyS0 {
            speed 9600  # 设置控制台速度为 9600
        }
    }
}

interfaces {
    loopback "lo"  # 设置回环接口 lo
}

/* Warning: Do not remove the following line. */
/* === vyatta-config-version: "broadcast-relay@1:cluster@1:config-management@1:conntrack@1:conntrack-sync@1:dhcp-relay@2:dhcp-server@5:dns-forwarding@1:firewall@5:ipsec@5:l2tp@1:mdns@1:nat@4:ntp@1:pppoe-server@2:pptp@1:qos@1:quagga@7:snmp@1:ssh@1:system@10:vrrp@2:wanloadbalance@3:webgui@1:webproxy@1:zone-policy@1" === */
/* Release version: 1.2.8 */
```

拓扑

```bash
#!/bin/bash
set -v
cat <<EOF> route.yaml | clab deploy -t route.yaml -
name: routing 
topology:
  nodes:
    gw0:       #gw0 是一个路由器配置
      kind: linux
      image: vyos/vyos:1.2.8
      cmd: sbin/init
      binds:
        - /lib/modules:/lib/modules
        - ./gw0-boot.cfg:/opt/vyatta/etc/config/config.boot
    server1: 
      kind: linux
      image: nicolaka/netshoot
      exec:
      - ip addr add 10.1.5.10/24 dev net0
      - ip route replace default via 10.1.5.1
      
    server2:
      kind: linux
      image: nicolaka/netshoot
      exec:
      - ip addr add 10.1.8.10/24 dev net0
      - ip route replace default via 10.1.8.1
      
  links:
    - endpoints: ["gw0:eth1", "server1:net0"]
    - endpoints: ["gw0:eth2", "server2:net0"]
EOF
```

创建完成

```
+---+----------------------+--------------+-------------------+-------+---------+----------------+----------------------+
| # |         Name         | Container ID |       Image       | Kind  |  State  |  IPv4 Address  |     IPv6 Address     |
+---+----------------------+--------------+-------------------+-------+---------+----------------+----------------------+
| 1 | clab-routing-gw0     | c71e42b886ad | vyos/vyos:1.2.8   | linux | running | 172.20.20.4/24 | 2001:172:20:20::4/64 |
| 2 | clab-routing-server1 | 68db0b748fd0 | nicolaka/netshoot | linux | running | 172.20.20.3/24 | 2001:172:20:20::3/64 |
| 3 | clab-routing-server2 | 66bd60e72778 | nicolaka/netshoot | linux | running | 172.20.20.2/24 | 2001:172:20:20::2/64 |
+---+----------------------+--------------+-------------------+-------+---------+----------------+----------------------+
```

验证, ttl会每走一次路由 -1

```
docker exec -it 66bd60e72778 /bin/bash
server2:~# ping 10.1.5.10
PING 10.1.5.10 (10.1.5.10) 56(84) bytes of data.
64 bytes from 10.1.5.10: icmp_seq=1 ttl=63 time=0.292 ms
64 bytes from 10.1.5.10: icmp_seq=2 ttl=63 time=0.137 ms
64 bytes from 10.1.5.10: icmp_seq=3 ttl=63 time=0.109 ms
```

销毁

```
clab destroy -a
```

### vxlan

准备拓扑文件

```bash
#!/bin/bash
set -v
cat <<EOF> vxlan.yaml | clab deploy -t vxlan.yaml -
name: vxlan
topology:
  nodes:
    gw0:      
      kind: linux
      image: vyos/vyos:1.2.8
      cmd: /sbin/init
      binds:
        - /lib/modules:/lib/modules
        - ./gw0.cfg:/opt/vyatta/etc/config/config.boot
        
    gw1:      
      kind: linux
      image: vyos/vyos:1.2.8
      cmd: /sbin/init
      binds:
        - /lib/modules:/lib/modules
        - ./gw1.cfg:/opt/vyatta/etc/config/config.boot
         
    server1: 
      kind: linux
      image: nicolaka/netshoot
      exec:
      - ip addr add 10.1.5.10/24 dev net0
      - ip route replace default via 10.1.5.1
      
    server2:
      kind: linux
      image: nicolaka/netshoot
      exec:
      - ip addr add 10.1.8.10/24 dev net0
      - ip route replace default via 10.1.8.1
      
  links:
    - endpoints: ["gw0:eth1", "server1:net0"]
    - endpoints: ["gw1:eth1", "server2:net0"]
    - endpoints: ["gw0:eth2", "gw1:eth2"]
    
EOF
```

路由配置文件

`gw0.cfg  `

```
interfaces {
    ethernet eth1 {
    	address 10.1.5.1/24  
    	duplex auto
    	smp-affinity auto
    	speed auto
	}
	ethernet eth2 {
    	address 172.12.1.10/24
    	duplex auto
    	smp-affinity auto
    	speed auto
	}
	loopback lo {
    }
	vxlan vxlan0 {
        address 1.1.1.1/24
        remote 172.12.1.11
        vni 10
    }
}
protocols {
    static {
    	route 10.1.8.0/24{
    		next-hop 1.1.1.2 {  
			}
		}
	}
}

system {
    host-name "vyos"
    login {
        user vyos {
            authentication {
                encrypted-password "$6$QxPS.uk6mfo$9QBSo8u1FkH16gMyAVhus6fU3LOzvLR9Z9.82m3tiHFAxTtIkhaZSWssSgzt4v4dGAL8rhVQxTg0oAG9/q11h/"
                plaintext-password ""
            }
            level "admin"
        }
    }
    syslog {
        global {
            facility all {
                level "info"
            }
            facility protocols {
                level "debug"
            }
        }
    }
    ntp {
        server "0.pool.ntp.org"
        server "1.pool.ntp.org"
        server "2.pool.ntp.org"
    }
    config-management {
        commit-revisions "100"
    }
    console {
        device ttyS0 {
            speed 9600
        }
    }
}
interfaces {
    loopback "lo"
}


/* Warning: Do not remove the following line. */
/* === vyatta-config-version: "broadcast-relay@1:cluster@1:config-management@1:conntrack@1:conntrack-sync@1:dhcp-relay@2:dhcp-server@5:dns-forwarding@1:firewall@5:ipsec@5:l2tp@1:mdns@1:nat@4:ntp@1:pppoe-server@2:pptp@1:qos@1:quagga@7:snmp@1:ssh@1:system@10:vrrp@2:wanloadbalance@3:webgui@1:webproxy@1:zone-policy@1" === */
/* Release version: 1.2.8 */

```

`gw1.cfg`

```
interfaces {
    ethernet eth1 {
    	address 10.1.8.1/24  
    	duplex auto
    	smp-affinity auto
    	speed auto
	}
	ethernet eth2 {
    	address 172.12.1.11/24
    	duplex auto
    	smp-affinity auto
    	speed auto
	}
	loopback lo {
    }
	vxlan vxlan0 {
        address 1.1.1.2/24
        remote 172.12.1.10
        vni 10
    }
}
protocols {
    static {
    	route 10.1.5.0/24{
    		next-hop 1.1.1.1 {  
			}
		}
	}
}

system {
    host-name "vyos"
    login {
        user vyos {
            authentication {
                encrypted-password "$6$QxPS.uk6mfo$9QBSo8u1FkH16gMyAVhus6fU3LOzvLR9Z9.82m3tiHFAxTtIkhaZSWssSgzt4v4dGAL8rhVQxTg0oAG9/q11h/"
                plaintext-password ""
            }
            level "admin"
        }
    }
    syslog {
        global {
            facility all {
                level "info"
            }
            facility protocols {
                level "debug"
            }
        }
    }
    ntp {
        server "0.pool.ntp.org"
        server "1.pool.ntp.org"
        server "2.pool.ntp.org"
    }
    config-management {
        commit-revisions "100"
    }
    console {
        device ttyS0 {
            speed 9600
        }
    }
}
interfaces {
    loopback "lo"
}


/* Warning: Do not remove the following line. */
/* === vyatta-config-version: "broadcast-relay@1:cluster@1:config-management@1:conntrack@1:conntrack-sync@1:dhcp-relay@2:dhcp-server@5:dns-forwarding@1:firewall@5:ipsec@5:l2tp@1:mdns@1:nat@4:ntp@1:pppoe-server@2:pptp@1:qos@1:quagga@7:snmp@1:ssh@1:system@10:vrrp@2:wanloadbalance@3:webgui@1:webproxy@1:zone-policy@1" === */
/* Release version: 1.2.8 */
```

查看

```bash
clab inspect -t vxlan.yaml 
INFO[0000] Parsing & checking topology file: vxlan.yaml 
+---+--------------------+--------------+-------------------+-------+---------+----------------+----------------------+
| # |        Name        | Container ID |       Image       | Kind  |  State  |  IPv4 Address  |     IPv6 Address     |
+---+--------------------+--------------+-------------------+-------+---------+----------------+----------------------+
| 1 | clab-vxlan-gw0     | 6117c8f4a823 | vyos/vyos:1.2.8   | linux | running | 172.20.20.3/24 | 2001:172:20:20::3/64 |
| 2 | clab-vxlan-gw1     | 4b7510fc2a27 | vyos/vyos:1.2.8   | linux | running | 172.20.20.2/24 | 2001:172:20:20::2/64 |
| 3 | clab-vxlan-server1 | 548291d7e5bd | nicolaka/netshoot | linux | running | 172.20.20.5/24 | 2001:172:20:20::5/64 |
| 4 | clab-vxlan-server2 | 8b4c72fe856a | nicolaka/netshoot | linux | running | 172.20.20.4/24 | 2001:172:20:20::4/64 |
+---+--------------------+--------------+-------------------+-------+---------+----------------+----------------------+
```

测试

```
ping 10.1.8.10
PING 10.1.8.10 (10.1.8.10) 56(84) bytes of data.
64 bytes from 10.1.8.10: icmp_seq=1 ttl=62 time=0.197 ms
64 bytes from 10.1.8.10: icmp_seq=2 ttl=62 time=0.068 ms
```

ttl值 `-2` 与我们的拓扑一致

登陆gw0 容器，在eth2 网卡上进行抓包： ` `

1. `-p`: 禁用混杂模式。在混杂模式下，网络接口将接收通过网络传输的所有数据包，而不仅仅是目标地址是本地主机的数据包。使用 `-p` 选项会禁用混杂模式，只捕获发送到本地主机的数据包。
2. `-n`: 禁用对IP地址和端口的网络解析。使用此选项，`tcpdump`将以数字形式显示IP地址和端口，而不会尝试将它们解析为主机名或服务名称。
3. `-e`: 在输出中显示以太网头部信息。这包括源和目标MAC地址，以及以太网帧的类型字段。

```bash
docker exec -it clab-vxlan-gw0 /bin/bash
cd ~
tcpdump -pne -i eth2 -w vxlan_clab.cap
exit
docker cp clab-vxlan-gw0:/root/vxlan_clab.cap ./
```

>  在vyos中抓包，使用tcpdump -w xxx.cap 可能会报错： `permission denied`, 可以通过cd 命令切换到家目录或者cd /tmp 进行抓包

待续..
