---
title: "Tekton和argocd使用"
date: 2022-09-24T20:05:03+08:00
---

Tekton是一个谷歌开源的kubernetes原生CI/CD框架 

部分重要组件

- Tekton Pipelines:  定义流水线（包含task、 task-run、pipeline等）

- Trigger Trigger: pipeline 触发器，可以在 git 推送后，触发 pipeline

- Tekton CLI:  Tekton 交互的命令行工具。

- Tekton Dashboard: Tekton Pipelines 的 Web 图形界面

## 安装

文档: https://github.com/tektoncd/pipeline/blob/main/docs/install.md

**tekton/pipelines**

```bash
kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
kubectl get pods --namespace tekton-pipelines
```

**dashboard**

```bash
kubectl apply --filename https://storage.googleapis.com/tekton-releases/dashboard/latest/tekton-dashboard-release.yaml
```

开放svc访问

```bash
#修改为nodePort
kubectl edit svc tekton-dashboard -n  tekton-pipelines 
```

**tkn cli**

```
brew install tektoncd-cli
```

**基本概念**

- Task: 表示执行命令的一系列步骤，task 里可以定义一系列的 steps，例如编译代码、构建镜像、推送镜像等，每个 step 实际由一个 Pod 执行。
- TaskRun: task定义了一个模板,taskrun代表了一次实际的运行
- Pipeline: 一个或多个task,PipelineResource以及各种参数的集合
- PipelineRun: 类似task和taskRun的关系，pipelineRun 也表示某一次实际运行的 pipeline，下发一个 pipelineRun CRD 实例到 Kubernetes后，同样也会触发一次 pipeline 的构建。
- PipeLineResource: 表示pipeline 输入资源,如git源码,或者 pipeline 输出资源，例如一个容器镜像或者构建生成的 jar 包等

## 运行

创建一个`Task`与`TaskRun`

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: hello
spec:
  steps:
    - name: echo
      image: alpine
      script: |
        #!/bin/sh
        echo "Hello World"
---
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: hello-task-run
spec:
  taskRef:
    name: hello
```

查看信息

```bash
$ kubectl get task
$ kubectl get taskrun
$ tkn taskrun list
$ tkn taskrun logs hello-task-run
[echo] Hello World
$ tkn task describe  hello
Name:        hello
Namespace:   default
🦶 Steps
 ∙ echo
🗂  Taskruns
NAME             STARTED         DURATION   STATUS
hello-task-run   6 minutes ago   6s         Succeeded
```

查看图形界面

<img src="http://inksnw.asuscomm.com:3001/blog/Tekton和argocd使用_0e87c8f1eecf273adae9217db1ba60e8.png" alt="image-20220924211406813" style="zoom: 50%;" />

我们可以通过 `kubectl describe` 命令来查看任务运行的过程，当任务执行完成后， Pod 就会变成 `Completed` 状态了：

```bash
$ kubectl get pod                        
NAME                 READY   STATUS      RESTARTS   AGE
hello-task-run-pod   0/1     Completed   0          36m
```

可以查看容器的日志信息来了解任务的执行结果信息：

```bash
$ kubectl logs hello-task-run-pod --all-containers
2022/09/24 13:12:10 Entrypoint initialization
2022/09/24 13:12:11 Decoded script /tekton/scripts/script-0-nrkts
Hello World
```

