# machine-status
![screenshot](https://imgur.com/kFTAvDS.png)

A web interface for GPU machines that is largely inspired by [gpustat-web](https://github.com/wookayin/gpustat-web)

## How to use

### For Docker
#### Exporter
```bash
# on each machine you want to export status
docker run -p 9200:9200 -d --pid=host --hostname=$(hostname) \
       --name mstat-exporter --restart always cih9088/machine-status:v0.1 \
       exporter

# use another port rather than default one
docker run -p ${custom_port}:${custom_port} -d --pid=host --hostname=$(hostname) \
       --name mstat-exporter --restart always cih9088/machine-status:v0.1 \
       exporter --port ${custom_port}

# help for exporter
docker run --rm cih9088/machine-status:v0.1 exporter -h
```

#### Server
Change user, pass, machine, and etc. as you wish.
```bash
# on web server hosting machine
docker run -p 80:80 \
       --name mstat-server --restart always cih9088/machine-status:v0.1 \
       server --host $(hostname --fqdn) \
              --user user1,user2 \
              --pass pass1,pass2 \
              --machine machine1.example.com:9200 \
              --machine machine2.example.com:9200 \
              --machine machine3.example.com:9200

# help for server
docker run --rm cih9088/machine-status:v0.1 server -h
```

### For Kubernetes
```bash
# Edit deployments/k8s_example.yaml first then
kubectl create -f deployments/k8s_example.yaml

# set label for node you want to export
kubectl label node ${NODE} mstat-exporter=true
```
