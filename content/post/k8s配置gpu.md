---
title: "K8s配置gpu"
date: 2024-05-29T11:02:12+08:00
tags: ["ai"]
---

## 又不是不能用

一直想研究一下k8s配置 gpu, 但是现在的显卡也太贵了, 家里服务器的显卡是张 GT710 使用 nvidia-smi 不认, 今天才发现驱动版本太高, 降低就可以了, 1G 显存也是显存, 又不是不能用 -_-

## 安装 nvidia GPU 驱动

查看显卡信息

```bash
root@server:~# lspci | grep -i nvidia
01:00.0 VGA compatible controller: NVIDIA Corporation GK208B [GeForce GT 710] (rev a1)
01:00.1 Audio device: NVIDIA Corporation GK208 HDMI/DP Audio Controller (rev a1)
```

这个仓库提供了一个安装脚本, 还挺好用

```bash
wget https://github.com/XuehaiPan/nvitop/blob/main/install-nvidia-driver.sh
```

```bash
# 查看驱动列表
install-nvidia-driver.sh
# 安装驱动, 注意要保证你的gpu与支持的驱动版本匹配
install-nvidia-driver.sh --package=nvidia-driver-470
```
```bash
# nvidia-smi
root@server:~# nvidia-smi
Wed May 29 03:19:01 2024       
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 470.239.06   Driver Version: 470.239.06   CUDA Version: 11.4     |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
|                               |                      |               MIG M. |
|===============================+======================+======================|
|   0  NVIDIA GeForce ...  On   | 00000000:01:00.0 N/A |                  N/A |
| 40%   37C    P8    N/A /  N/A |      0MiB /   981MiB |     N/A      Default |
|                               |                      |                  N/A |
+-------------------------------+----------------------+----------------------+
                                                                               
+-----------------------------------------------------------------------------+
| Processes:                                                                  |
|  GPU   GI   CI        PID   Type   Process name                  GPU Memory |
|        ID   ID                                                   Usage      |
|=============================================================================|
|  No running processes found                                                 |
+-----------------------------------------------------------------------------+
```

## 安装 nvidia-container-toolkit

官方文档 https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html#installing-with-apt

```bash
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg \
  && curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
    sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
    sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
apt-get update
apt-get install -y nvidia-container-toolkit
```

## 配置 runtime

```bash
# containerd
nvidia-ctk runtime configure --runtime=containerd
systemctl restart containerd
```

## 验证

cuda 驱动版本可以 `nvidia-smi` 查看右上角, 具体小版本号可以到dockerhub上搜一下

```bash
nerdctl run --rm --gpus all nvidia/cuda:11.4.3-base-ubuntu20.04 nvidia-smi
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 470.239.06   Driver Version: 470.239.06   CUDA Version: 11.4     |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
|                               |                      |               MIG M. |
|===============================+======================+======================|
|   0  NVIDIA GeForce ...  Off  | 00000000:01:00.0 N/A |                  N/A |
| 40%   42C    P0    N/A /  N/A |      0MiB /   981MiB |     N/A      Default |
|                               |                      |                  N/A |
+-------------------------------+----------------------+----------------------+
                                                                               
+-----------------------------------------------------------------------------+
| Processes:                                                                  |
|  GPU   GI   CI        PID   Type   Process name                  GPU Memory |
|        ID   ID                                                   Usage      |
|=============================================================================|
|  No running processes found                                                 |
+-----------------------------------------------------------------------------+
```

## 启用 k8s gpu支持

> 发现官方使用 nvidia-ctk 配置的方法, nvidia-device-plugin-daemonset 的日志显示有问题
>
> 需要手动修改一下 vi /etc/containerd/config.toml, 改成如下的样式
>
>     [plugins."io.containerd.grpc.v1.cri".containerd]
>       default_runtime_name = "nvidia"
>       [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
>         [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
>           privileged_without_host_devices = false
>           runtime_engine = ""
>           runtime_root = ""
>           runtime_type = "io.containerd.runc.v2"
>           [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
>             BinaryName = "/usr/bin/nvidia-container-runtime"
>             SystemdCgroup = true

安装 k8s-device-plugin

```bash
kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.15.0/deployments/static/nvidia-device-plugin.yml
# 查看日志
kubectl logs -f ds/nvidia-device-plugin-daemonset -n kube-system
I0529 06:16:45.123510       1 main.go:279] Retrieving plugins.
I0529 06:16:45.123736       1 factory.go:104] Detected NVML platform: found NVML library
I0529 06:16:45.123759       1 factory.go:104] Detected non-Tegra platform: /sys/devices/soc0/family file not found
I0529 06:16:45.473066       1 server.go:216] Starting GRPC server for 'nvidia.com/gpu'
I0529 06:16:45.473479       1 server.go:147] Starting to serve 'nvidia.com/gpu' on /var/lib/kubelet/device-plugins/nvidia-gpu.sock
I0529 06:16:45.474646       1 server.go:154] Registered device plugin for 'nvidia.com/gpu' with Kubelet
```

查看节点信息

```bash
kubectl describe node server
# 可以看到类似字样 nvidia.com/gpu
Allocated resources:
  (Total limits may be over 100 percent, i.e., overcommitted.)
  Resource           Requests    Limits
  --------           --------    ------
  cpu                1 (8%)      0 (0%)
  memory             140Mi (0%)  600Mi (1%)
  nvidia.com/gpu     1           1
```

## 测试

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: gpu-pod
spec:
  restartPolicy: Never
  containers:
    - name: cuda-container
      image: nvcr.io/nvidia/k8s/cuda-sample:vectoradd-cuda10.2
      resources:
        limits:
          nvidia.com/gpu: 1 # requesting 1 GPU
  tolerations:
  - key: nvidia.com/gpu
    operator: Exists
    effect: NoSchedule
EOF
```

查看日志

```bash
kubectl logs gpu-pod
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```

## 安装nvidia nim

### 申请访问权限

nim下的镜像需要单独申请访问权限才能拉取

进入如下网址 https://catalog.ngc.nvidia.com/orgs/nim/teams/meta/containers/llama3-8b-instruct, 点击右上角的 `get container`, 填写信息后, 会收到邮件,再次查看此页面就可以看到镜像版本了,

<img src="http://inksnw.asuscomm.com:3001/blog/k8s配置gpu_副本_811f8eab270e84eb1d54aa8c204e0662.png" alt="企业微信截图_76efa6ae-25da-402b-aa91-953a73ebe112" style="zoom:50%;" />

收到允许邮件后, 再次查看tag如图, 此时生成的ncg api key才能拉取镜像, 否则会报权限问题

<img src="http://inksnw.asuscomm.com:3001/blog/k8s配置gpu_副本_28b41536d7a1d9b403c23069effd2dca.png" alt="image-20240606100402704" style="zoom:50%;" />

进入 https://org.ngc.nvidia.com/setup  申请key

<img src="http://inksnw.asuscomm.com:3001/blog/k8s配置gpu_副本_613a7870f03e15455bbcacc0e62cbda2.png" alt="image-20240606100527318" style="zoom:50%;" />

### 安装

```bash
git clone https://github.com/NVIDIA/nim-deploy.git
cd nim-deploy/helm
export NGC_CLI_API_KEY="xxx"
kubectl -n nim create secret generic ngc-api --from-literal=NGC_CLI_API_KEY=$NGC_CLI_API_KEY
kubectl create ns nim
helm --namespace nim install my-nim nim-llm/ --set model.ngcAPIKey=xxx --set persistence.enabled=true
```

查看生成的pod

```bash
root@server:~/nim-deploy/helm# kubectl get pod -A
NAMESPACE     NAME                                           READY   STATUS             RESTARTS      AGE
default       gpu-pod                                        0/1     Completed          0             14m
kube-system   calico-kube-controllers-7b84757b95-p9flq       1/1     Running            0             16m
kube-system   calico-node-jncdg                              1/1     Running            0             16m
kube-system   coredns-565dd4648d-4gcwf                       1/1     Running            0             16m
kube-system   coredns-565dd4648d-x8fpm                       1/1     Running            0             16m
kube-system   kube-apiserver-server                          1/1     Running            0             16m
kube-system   kube-controller-manager-server                 1/1     Running            0             16m
kube-system   kube-proxy-zrn2z                               1/1     Running            0             16m
kube-system   kube-scheduler-server                          1/1     Running            0             16m
kube-system   nvidia-device-plugin-daemonset-dmhpv           1/1     Running            0             15m
kube-system   openebs-localpv-provisioner-64b88c795c-2q6w4   1/1     Running            0             16m
nim           my-nim-0                                       0/1     CrashLoopBackOff   7 (43s ago)   13m
```

## 使用nim

看官方文档, 安装完会生成一个 svc, 可以直接使用api调用, 但是我的远古显卡gt710似乎不支持

```bash
root@server:~/nim-deploy/helm# kubectl logs -f my-nim-0 -n nim
...
2024-06-06 02:14:24,804 [INFO] PyTorch version 2.2.2 available.
2024-06-06 02:14:25,245 [WARNING] [TRT-LLM] [W] Logger level already set from environment. Discard new verbosity: error
2024-06-06 02:14:25,245 [INFO] [TRT-LLM] [I] Starting TensorRT-LLM init.
[TensorRT-LLM][INFO] Set logger level by INFO
2024-06-06 02:14:25,252 [INFO] [TRT-LLM] [I] TensorRT-LLM inited.
[TensorRT-LLM] TensorRT-LLM version: 0.10.1.dev2024053000
Traceback (most recent call last):
...
    gpus = GPUInspect()
  File "/usr/local/lib/python3.10/dist-packages/vllm_nvext/hub/hardware_inspect.py", line 77, in __init__
    GPUInspect._safe_exec(cuda.cuInit(0))
  File "/usr/local/lib/python3.10/dist-packages/vllm_nvext/hub/hardware_inspect.py", line 85, in _safe_exec
    raise RuntimeError(f"Unexpected error: {status.name}")
RuntimeError: Unexpected error: CUDA_ERROR_NO_DEVICE
```

如果可用, 此时应该可以使用api类似如下调用

```python
from openai import OpenAI

client = OpenAI(
  base_url = "https://integrate.api.nvidia.com/v1",
  api_key = "$API_KEY_REQUIRED_IF_EXECUTING_OUTSIDE_NGC"
)

completion = client.chat.completions.create(
  model="meta/llama3-8b-instruct",
  messages=[{"role":"user","content":"你好"}],
  temperature=0.5,
  top_p=1,
  max_tokens=1024,
  stream=True
)

for chunk in completion:
  if chunk.choices[0].delta.content is not None:
    print(chunk.choices[0].delta.content, end="")
```



