# machine-status
![screenshot](https://imgur.com/kFTAvDS.png)

A web interface for GPU machines that is largely inspired by [gpustat-web](https://github.com/wookayin/gpustat-web)

## Prerequisites
[nvidia-docker2](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html)

## How to use

### For Docker
#### Exporter
```bash
# on each machine you want to export status
docker run -p 9200:9200 --detach --pid=host --hostname=$(hostname) \
    --name mstat-exporter --restart always --gpus all cih9088/machine-status:0.2 \
    exporter

# use another port rather than default one
docker run -p 9999:9999 --detach --pid=host --hostname=$(hostname) \
    --name mstat-exporter --restart always --gpus all cih9088/machine-status:0.2 \
    exporter --port 9999

# change timezone
docker run -p 9200:9200 --detach --pid=host --hostname=$(hostname) \
    --env TZ="Europe/London" \
    --name mstat-exporter --restart always --gpus all cih9088/machine-status:0.2 \
    exporter

# help for exporter
docker run --rm cih9088/machine-status:0.2 exporter -h
```

#### Server
Change user, pass, machine, and etc. as you wish.
```bash
# on web server hosting machine
docker run -p 80:80 --detach \
    --name mstat-server --restart always cih9088/machine-status:0.2 \
    server --host $(hostname --fqdn) \
        --user user1,user2 \
        --pass pass1,pass2 \
        --machine machine1.example.com:9200 \
        --machine machine2.example.com:9200 \
        --machine machine3.example.com:9200

# help for server
docker run --rm cih9088/machine-status:0.2 server -h
```

### For Kubernetes
```bash
# Edit deployments/k8s_example.yaml first then
kubectl create -f deployments/k8s_example.yaml

# set label for node you want to export
kubectl label node ${NODE} mstat-exporter=true
```
