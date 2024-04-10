---
title: "本地签发证书代理https"
date: 2024-04-10T10:12:54+08:00

---

## mac下使用nginx

一些配置

```bash
brew install nginx
brew services start nginx
brew services list 
brew services stop nginx
brew services restart nginx
```

重新加载配置文件

```
nginx -s reload
```

验证nginx配置文件是否正确

```
nginx -t
```

配置文件位置

```
/usr/local/etc/nginx/nginx.conf
```

## 创建证书

```bash
openssl genrsa -out ca.key 4096
openssl req -x509 -new -nodes -sha512 -days 3650 \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=example/OU=Personal" \
    -key ca.key \
    -out ca.crt
```

创建 `*.my.com` 域名证书

- 生成私钥

```bash
openssl genrsa -out my.key 4096
```

- 生成证书签名请求 CSR

```bash
openssl req -sha512 -new \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=example/OU=Personal/CN=*.my.com" \
    -key my.key \
    -out my.csr
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
DNS.1=my.com
DNS.2=*.my.com
EOF
```

- 生成 `*.my.com` 域名证书

```bash
openssl x509 -req -sha512 -days 3650 \
    -extfile v3.ext \
    -CA ca.crt -CAkey ca.key -CAcreateserial \
    -in my.csr \
    -out my.crt
```

查看生成的全部文件

```bash
.
├── ca.crt
├── ca.key
├── ca.srl
├── my.crt
├── my.csr
├── my.key
└── v3.ext

1 directory, 7 files
```

## 配置nginx

```bash
worker_processes  1;
events {
    worker_connections  1024;
}
http {

    log_format main '$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent" "$http_authorization"';
    include       mime.types;
    default_type  application/octet-stream;
    sendfile        on;
    keepalive_timeout  65;


    server {
        listen 443 ssl;
        server_name www.my.com;

        ssl_certificate /usr/local/etc/nginx/certs/my.crt; # Path to your SSL certificate
        ssl_certificate_key /usr/local/etc/nginx/certs/my.key; # Path to your SSL certificate private key

        location / {
            proxy_pass http://127.0.0.1:8000;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }

        access_log /usr/local/etc/nginx/access.log;
        
        error_page 500 502 503 504 /50x.html;
        location = /50x.html {
            root html;
        }
    }
}
```

## 验证

```bash
d➜ brew services start nginx
d➜ brew services list
Name  Status  User   File
nginx started inksnw ~/Library/LaunchAgents/homebrew.mxcl.nginx.plist
```

配置hosts

```
127.0.0.1 www.my.com
```

本地启动一个服务

```
python3 -m http.server
```

访问

```bash
https://www.my.com/
```

<img src="http://inksnw.asuscomm.com:3001/blog/本地签发证书代理https_c2e63853f1d7b3601b4581e4fd824f59.png" alt="image-20240410103958436" style="zoom:50%;" />

此时,证书不受信任

双击ca.crt加入到系统信任中

<img src="http://inksnw.asuscomm.com:3001/blog/本地签发证书代理https_79584d3e26dfab47d53144b237082679.png" alt="image-20240410104731767" style="zoom:50%;" />

再次访问

<img src="http://inksnw.asuscomm.com:3001/blog/本地签发证书代理https_879c2a3bf8d89a98fba411408abce9c5.png" alt="image-20240410104818353" style="zoom:50%;" />

