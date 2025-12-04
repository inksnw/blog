---
title: "Buildx自签证书构建"
date: 2025-12-04T21:39:01+08:00
---

一直没搞定buildx信任私有证书, 这里总结一下

### 构建报错

```bash
docker buildx build --platform=linux/amd64 --load -t  foo:1.0 -f Dockerfile .
[+] Building 1.3s (2/2) FINISHED                                                            => ERROR [internal] load metadata for cr-ee.registry.cn-neimeng-236e-d01.env236.cloud.arm.com/foo/busybox:1.0                                          1.2s
Dockerfile:1
--------------------
   1 | >>> FROM cr-ee.registry.cn-neimeng-236e-d01.env236.cloud.arm.com/foo/busybox:1.0
--------------------
ERROR: failed to build: failed to solve: cr-ee.registry.cn-neimeng-236e-d01.env236.cloud.arm.com/foo/busybox:1.0: failed to resolve source metadata for cr-ee.registry.cn-neimeng-236e-d01.env236.cloud.arm.com/foo/busybox:1.0: failed to do request: Head "https://cr-ee.registry.cn-neimeng-236e-d01.env236.cloud.arm.com/v2/foo/busybox/manifests/1.0": tls: failed to verify certificate: x509: certificate signed by unknown authority
```
### 宿主机信任根证书

从浏览器导出证书,  经验证, 在我的mac系统上只导入根证书不行, 需要把**每一层证书**都下载信任了
![微信图片_20251204214055_4_63](http://inksnw.asuscomm.com:3001/blog/buildx自签证书构建_4d7c2f89cf75123f69e4f0abb2f35294.png)
双击添加到系统信任证书中, 并配置始终信任

<img src="http://inksnw.asuscomm.com:3001/blog/buildx自签证书构建_af8b9b671ae745b73cd15838e82911df.png" alt="微信图片_20251204214050_2_63" style="zoom:50%;" />

可以这样判断是否是根证书( 我的mac上只导入根证书不行, 建议都导入 )

在https://myssl.com/cert_decode.html 这个网站贴入证书

![微信图片_20251204214053_3_63](http://inksnw.asuscomm.com:3001/blog/buildx自签证书构建_373100b23329c2d2989313c80f735c97.png)

### 验证curl(非必须)

验证curl不通过

```bash
curl https://cr-ee.registry.cn-wulan-env148-d01.inter.env148.shuguang.com/auth
curl: (60) SSL certificate OpenSSL verify result: self-signed certificate in certificate chain (19)
More details here: https://curl.se/docs/sslcerts.html

curl failed to verify the legitimacy of the server and therefore could not
establish a secure connection to it. To learn more about this situation and
how to fix it, please visit the webpage mentioned above.
```

> 使用brew 安装的curl 走的好像不是钥匙串里配置的证书, 始终验证不过

使用系统的curl , 还是不行

```bash
/usr/bin/curl https://cr-ee.registry.cn-wulan-env148-d01.inter.env148.shuguang.com/auth
```

把根证书追加到`/etc/ssl/cert.pem`再使用**系统curl**测试, 可以了

```bash
/usr/bin/curl https://cr-ee.registry.cn-wulan-env148-d01.inter.env148.shuguang.com/auth
{"IsSuccess":false,"Code":"InvalidService","Message":"The Service field is required","RequestId":"curl/8.7.1"}
```

显式的指定证书使用, 也可以

```bash
curl --cacert env148.pem https://cr-ee.registry.cn-wulan-env148-d01.inter.env148.shuguang.com/auth
{"IsSuccess":false,"Code":"InvalidService","Message":"The Service field is required","RequestId":"curl/8.17.0"}
```

### 构建容器信任根证书

```bash
docker buildx create  --name newctx --driver docker-container --use
docker buildx inspect --bootstrap   
```

拷贝进容器

```bash
# 我这里有两层证书, 实际有几个就导入几个
docker cp env148.pem 18af24e5b6f3:/usr/local/share/ca-certificates/ 
docker cp cr-ee.registry.cn-wulan-env148-d01.inter.env148.shuguang.com.pem 18af24e5b6f3:/usr/local/share/ca-certificates/ 
# 进入容器更新证书
docker exec -it 18af24e5b6f3 sh
apk add ca-certificates
# 更新证书
update-ca-certificates
docker restart 18af24e5b6f3
```

### 再次构建

```bash
docker buildx build --load -t  foo:1.0 -f Dockerfile .
# 构建成功
➜  Desktop docker buildx build --load -t  foo:1.0 -f Dockerfile .
[+] Building 1.5s (7/7) FINISHED                                                                              docker-container:newctx
 => [internal] load build definition from Dockerfile                                       ...
 => => exporting config sha256:2b9f132b6470740b5b7c55a11ae83b936bdd17ac5b3796d8398c401b722d3ee5                                  0.0s
 => => sending tarball                                                                                                           0.3s
 => importing to docker  
```
