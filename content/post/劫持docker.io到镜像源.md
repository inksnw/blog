---
title: "劫持docker.io到镜像源"
date: 2024-01-11T18:07:55+08:00

---

dockerhub限流真是烦人, 配置镜像源又得一台台的改docker配置, 有没有其它办法, 比如对截持一下, 直接转发到镜像源

## 自签证书

###  创建 CA 根证书

```bash
openssl genrsa -out ca.key 4096
openssl req -x509 -new -nodes -sha512 -days 3650 \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=example/OU=Personal" \
    -key ca.key \
    -out ca.crt
```

### 创建 `*.docker.io` 域名证书

- 生成私钥

```bash
openssl genrsa -out docker.io.key 4096
```

- 生成证书签名请求 CSR

```bash
openssl req -sha512 -new \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=example/OU=Personal/CN=*.docker.io" \
    -key docker.io.key \
    -out docker.io.csr
```

- 生成 x509 v3 扩展

```bash
cat > v3.ext <<-EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1=docker.io
DNS.2=*.docker.io
EOF
```

- 生成 `*.docker.io` 域名证书

```bash
openssl x509 -req -sha512 -days 3650 \
    -extfile v3.ext \
    -CA ca.crt -CAkey ca.key -CAcreateserial \
    -in docker.io.csr \
    -out docker.io.crt
```

查看生成的全部文件

```bash
root@node4:~/d# tree
.
├── ca.crt
├── ca.key
├── ca.srl
├── docker.io.crt
├── docker.io.csr
├── docker.io.key
└── v3.ext
```

## Nginx 代理转发

这里可以找到可用的镜像源

```
https://gist.github.com/y0ngb1n/7e8f16af3242c7815e7ca2f0833d3ea6
```

配置一个nginx

```bash
mkdir -p /usr/local/nginx/cert/
cp docker.io.crt /usr/local/nginx/cert/
cp docker.io.key  /usr/local/nginx/cert/
apt install nginx -y
vi  /etc/nginx/nginx.conf
```

配置如下`nginx.conf`

```
user root;
worker_processes auto;
error_log /var/log/nginx/error.log;
pid /run/nginx.pid;

include /usr/share/nginx/modules/*.conf;
events {
    worker_connections 1024;
}
http {
    log_format main '$remote_addr [$time_local] "$request" '
                    '"$http_user_agent"  $http_x_forwarded_for ' ;
    access_log /var/log/nginx/access.log main;

    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    types_hash_max_size 4096;
    include /etc/nginx/mime.types;
    default_type application/octet-stream;
   
    server {
        listen       443 ssl;
        server_name  docker.io;
        
        ssl_certificate     /usr/local/nginx/cert/docker.io.crt;
        ssl_certificate_key  /usr/local/nginx/cert/docker.io.key;
        
        ssl_session_timeout  5m;
        ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE:ECDH:AES:HIGH:!NULL:!aNULL:!MD5:!ADH:!RC4;
        ssl_protocols TLSv1 TLSv1.1 TLSv1.2;
        ssl_prefer_server_ciphers on;

        location / {
            proxy_pass https://docker.m.daocloud.io;
        }
    }

}
```

启动nginx

```
systemctl start nginx
```

## 添加证书信任

ubuntu系统如下配置

```bash
cp ca.crt /usr/local/share/ca-certificates
update-ca-certificates
#重启 Docker，才会重新加载根证书
systemctl restart docker 
```

> 不重启docker会提示证书错误
>
> Using default tag: latest
> Error response from daemon: Get "https://registry-1.docker.io/v2/": tls: failed to verify certificate: x509: certificate signed by unknown authority

### 配置hosts

```
127.0.0.1 docker.io
127.0.0.1 registry-1.docker.io
```

## 验证

```bash
root@node4:~# curl -I https://registry-1.docker.io/v2/
HTTP/1.1 200 OK
Server: nginx/1.18.0 (Ubuntu)
Date: Thu, 11 Jan 2024 10:44:01 GMT
Content-Type: application/json; charset=utf-8
Content-Length: 2
Connection: keep-alive

docker pull nginx
```

另开一个终端查看日志

```
tail -f /var/log/nginx/access.log
127.0.0.1 [11/Jan/2024:10:24:32 +0000] "GET /v2/ HTTP/1.1" "docker/24.0.2 go/go1.20.4 git-commit/659604f kernel/5.4.0-169-generic os/linux arch/amd64 UpstreamClient(Docker-Client/24.0.2 \x5C(linux\x5C))"  - 
127.0.0.1 [11/Jan/2024:10:24:32 +0000] "HEAD /v2/library/nginx/manifests/latest HTTP/1.1" "docker/24.0.2 go/go1.20.4 git-commit/659604f kernel/5.4.0-169-generic os/linux arch/amd64 UpstreamClient(Docker-Client/24.0.2 \x5C(linux\x5C))"  - 
127.0.0.1 [11/Jan/2024:10:24:32 +0000] "GET /v2/library/nginx/manifests/sha256:2bdc49f2f8ae8d8dc50ed00f2ee56d00385c6f8bc8a8b320d0a294d9e3b49026 HTTP/1.1" "docker/24.0.2 go/go1.20.4 git-commit/659604f kernel/5.4.0-169-generic os/linux arch/amd64 UpstreamClient(Docker-Client/24.0.2 \x5C(linux\x5C))"  - 
```

## 参考文档

https://www.chenshaowen.com/blog/hijack-docker-io-req-to-private-repository.html
