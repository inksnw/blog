---
title: "使用github作为Helm的chart仓库"
date: 2022-09-27T16:56:13+08:00
tags: ["工程化"]
---

创建一个**github**仓库

略

## 初始化项目

```bash
git clone https://github.com/inksnw/mychart.git
# 使用helm初始化
helm create test
```

目录结构

```
test
├── Chart.yaml
├── charts
├── templates
│   ├── NOTES.txt
│   ├── _helpers.tpl
│   ├── deployment.yaml
│   ├── hpa.yaml
│   ├── ingress.yaml
│   ├── service.yaml
│   ├── serviceaccount.yaml
│   └── tests
│       └── test-connection.yaml
└── values.yaml
```

## 打包上传

```bash
$ helm package test/
$ ls
test           test-0.1.0.tgz
```

生成索引文件

```bash
helm repo index --url https://inksnw.github.io/mychart/ .
```

提交到github

```bash
git add .
git commit -m "my first chart"
```

## 设置GitHub Pages

<img src="http://inksnw.asuscomm.com:3001/blog/使用github作为Helm的chart仓库_4702eb9bb1298308ea430cd67a2d0c70.png" alt="image-20220927172720598" style="zoom:50%;" />

设置`_config.yml`文件可以配置主题

```bash
remote_theme: pages-themes/cayman@v0.2.0
plugins:
- jekyll-remote-theme # add this line to the plugins list if you already have one
```

## 使用源

添加使用chart仓库

```bash
$ helm repo add myrepo https://inksnw.github.io/mychart
$ helm repo list
NAME  	URL                                   
myrepo	https://inksnw.github.io/mychart
# -l 显示所有版本
$ helm search repo myrepo -l 
NAME       	CHART VERSION	APP VERSION	DESCRIPTION                
myrepo/test	0.1.3        	1.16.3     	A Helm chart for Kubernetes
myrepo/test	0.1.2        	1.16.2     	A Helm chart for Kubernetes
```

安装chart包

```bash
$ helm install xxx myrepo/test
```

## 发布多个 release

更新`Chart.yaml`文件中 

```yaml
version: 0.1.2
appVersion: "1.16.2"
```

打包推送
```bash
$ helm package test/ 
$ helm repo index --url https://inksnw.github.io/mychart/ .
$ git add .
$ git commit -m "0.1.2"
$ git push
```

更新查看helm源

```bash
$ helm repo update myrepo
$ helm search repo myrepo -l
NAME       	CHART VERSION	APP VERSION	DESCRIPTION                
myrepo/test	0.1.3        	1.16.3     	A Helm chart for Kubernetes
myrepo/test	0.1.2        	1.16.2     	A Helm chart for Kubernetes
```

## 使用同一个仓库

实际使用时,不想为helm源单独建一个仓库怎么办呢

**方法一**

设置github pages页,将主页目录由`/(root)`改为`/docs`,并把helm 发布相关的内容移动到这个目录,存在问题: 

- docs里的`readme`文件对项目依然有效,在根目录放一个`readme`文件无效果,显示为空
- 生成`helm repo index`的命令,`--url`参数需要添加`/docs`,保证生成的下载链接正确

**方法二**

新建一个分支专门用于当作helm repo, 配置github pages页的主页分支
