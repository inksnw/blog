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
# 安装驱动
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

cuda 驱动版本可以 `nvidia-smi` 查看右上角, 具体镜像可以到dockerhub上搜一下

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

