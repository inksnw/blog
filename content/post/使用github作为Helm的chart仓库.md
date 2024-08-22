---
title: "使用github作为Helm的chart仓库"
date: 2022-09-27T16:56:13+08:00
tags: ["工程化"]
---

创建一个**github**仓库

- 如果不想生成的`tar`包让代码目录变的混乱,可新建一个分支,专门存储chart文件

- 想自动化完成,可以了解基本原理后直接看下方的 [自动化构建](#自动化构建)

## 初始化项目

```bash
git clone https://github.com/inksnw/mychart.git
# 创建测试helm文件
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
➜ helm package test/
➜ ls
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
git push
```

## 设置GitHub Pages

<img src="https://inksnw.asuscomm.com:3001/blog/使用github作为Helm的chart仓库_4702eb9bb1298308ea430cd67a2d0c70.png" alt="image-20220927172720598" style="zoom:50%;" />

> 设置`_config.yml`文件可以配置主题, 可选

```bash
remote_theme: pages-themes/cayman@v0.2.0
plugins:
- jekyll-remote-theme
```

## 使用源

添加使用chart仓库

```bash
➜ helm repo add myrepo https://inksnw.github.io/mychart
➜ helm repo list
NAME  	URL                                   
myrepo	https://inksnw.github.io/mychart
# -l 显示所有版本
➜ helm search repo myrepo -l 
NAME       	CHART VERSION	APP VERSION	DESCRIPTION                
myrepo/test	0.1.3        	1.16.3     	A Helm chart for Kubernetes
myrepo/test	0.1.2        	1.16.2     	A Helm chart for Kubernetes
```

安装chart包

```bash
➜ helm install xxx myrepo/test
```

## 发布多个 release

更新`Chart.yaml`文件中 

```yaml
version: 0.1.2
appVersion: "1.16.2"
```

打包推送
```bash
➜ helm package test/ 
➜ helm repo index --url https://inksnw.github.io/mychart/ .
➜ git add .
➜ git commit -m "0.1.2"
➜ git push
```

更新查看helm源

```bash
➜ helm repo update
➜ helm search repo myrepo -l
NAME       	CHART VERSION	APP VERSION	DESCRIPTION                
myrepo/test	0.1.3        	1.16.3     	A Helm chart for Kubernetes
myrepo/test	0.1.2        	1.16.2     	A Helm chart for Kubernetes
```

## 同步分支数据

实际使用中,每次更新手动复制文件到 chart分支会很麻烦,git 提供了只合并部分文件的功能

```bash
## 首先切换到chart分支
git checkout chart
## 合并
git checkout main test/** 
```

合并好了文件后,就是上文中的生成tar包,生成索引,上传的过程了,就不再写一遍了

## 自动化构建

helm repo的本质就是一个网址,只要能够提供`index.yaml` 发行版的下载信息就可以,每次更新上面的步骤都是一样的,有没有办法自动化呢,可以,使用github ci就能实现自动化发布

### 创建目录

chart文件应该放在根目录下的`/charts`目录中

### GitHub 操作流

在主分支创建一个GitHub操作流文件 `.github/workflows/release.yml`

```yaml
name: Release Charts
on:
  push:
    branches:
      - main
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v2
        with:
          fetch-depth: 0
      - name: Configure Git
        run: |
          git config user.name "$GITHUB_ACTOR"
          git config user.email "$GITHUB_ACTOR@users.noreply.github.com"
      - name: Run chart-releaser
        uses: helm/chart-releaser-action@v1.1.0
        env:
          CR_TOKEN: "${{ secrets.GITHUB_TOKEN }}"
```

创建另一个分支 `gh-pages` 用于发布chart,这个分支的`readme` 文件就是github pages的页面,`index.yaml` 就是helm repo的索引文件

主分支推送后`chart-releaser-action`会检查项目中的每个chart来执行次操作,每当有新的chart版本时,就会在`gh-pages` 分支下创建或更新`index.yaml`文件,同时发会布一个release,打一个tag.

这样就实现了使用github来当作helm的仓库,推荐使用自动化构建的方法

官方文档: https://helm.sh/zh/docs/howto/chart_releaser_action/
