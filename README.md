# machine-status
![screenshot](https://imgur.com/kFTAvDS.png)

A web interface for several machines with GPUs. It is largely inspired by [gpustat](https://github.com/wookayin/gpustat-web)

## How to use

### For Docker
#### Exporter
```bash
# on each machines you want to monitor
docker run -p 9200:9200 -d --pid=host --hostname=$(hostname) \
       --name mstat-exporter --restart always cih9088/machine-status:v0.1 \
       exporter

# if you want another port to use
docker run -p ${custom_port}:${custom_port} -d --pid=host --hostname=$(hostname) \
       --name mstat-exporter --restart always cih9088/machine-status:v0.1 \
       exporter --port ${custom_port}

# check exporter parameter
docker run --rm cih9088/machine-status:v0.1 exporter -h
```

#### Server
Change user, pass and machine as you wish.
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

# check server parameter
docker run --rm cih9088/machine-status:v0.1 server -h
```

### For Kubernetes
```bash
# Edit deployments/k8s_example.yaml first then
kubectl create -f deployments/k8s_example.yaml
```
