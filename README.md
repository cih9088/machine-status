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
$ docker run -p 9200:9200 --detach --pid=host --hostname=$(hostname) \
    --name mstat-exporter --restart always --gpus all \
    cih9088/machine-status:0.3 exporter

# use another port rather than default one
$ docker run -p 9999:9999 --detach --pid=host --hostname=$(hostname) \
    --name mstat-exporter --restart always --gpus all \
    cih9088/machine-status:0.3 exporter --port 9999

# change timezone
$ docker run -p 9200:9200 --detach --pid=host --hostname=$(hostname) \
    --name mstat-exporter --restart always --gpus all \
    --env TZ="Europe/London" \
    cih9088/machine-status:0.3 exporter

# help for exporter
$ docker run --rm cih9088/machine-status:0.3 exporter -h
```

#### Server
Change user, pass, machine, and etc. as you wish.
```bash
# simple authenticated web server
$ docker run -p 80:80 --detach --name mstat-server --restart always \
    cih9088/machine-status:0.3 server-simple \
        --fqdn $(hostname --fqdn) \
        --user user1,user2 \
        --pwd pass1,pass2 \
        --machine machine1.example.com:9200 \
        --machine machine2.example.com:9200 \
        --machine machine3.example.com:9200

# simple authenticated web server with pre-generated tls
$ docker run -p 80:80 --detach --name mstat-server --restart always \
    --volume path/where/certs/are/in:/tmp/certs \
    cih9088/machine-status:0.3 server-simple \
        --fqdn $(hostname --fqdn) \
        --wss \
        --https-key key_name_in_path/wehre/certs/are/in \
        --https-crt certificate_name_in_path/where/certs/are/in \
        --port 443 \
        --user user1,user2 \
        --pwd pass1,pass2 \
        --machine machine1.example.com:9200 \
        --machine machine2.example.com:9200 \
        --machine machine3.example.com:9200

# simple authenticated web server with letsencrypt tls
$ docker run -p 80:80 --detach --name mstat-server --restart always \
    --volume path/where/certs/are/in:/tmp/certs \
    cih9088/machine-status:0.3 server-simple \
        --fqdn $(hostname --fqdn) \
        --wss \
        --letsencrypt \
        --port 443 \
        --user user1,user2 \
        --pwd pass1,pass2 \
        --machine machine1.example.com:9200 \
        --machine machine2.example.com:9200 \
        --machine machine3.example.com:9200

# keycloak authenticated web server with letsencrypt tls
$ docker run -p 80:80 --detach --name mstat-server --restart always \
    --volume path/where/certs/are/in:/tmp/certs \
    --volume path/to/keycloak.json:/app/web/keycloak/keycloak.json
    cih9088/machine-status:0.3 server-simple \
        --fqdn $(hostname --fqdn) \
        --wss \
        --letsencrypt \
        --port 443 \
        --keycloak-server https://keycloak.server:8443 \
        --keycloak-client client_name \
        --keycloak-realm realm_name \
        --machine machine1.example.com:9200 \
        --machine machine2.example.com:9200 \
        --machine machine3.example.com:9200

# help for simple server
$ docker run --rm cih9088/machine-status:0.3 server-simple -h
# help for keycloak server
$ docker run --rm cih9088/machine-status:0.3 server-keycloak -h
```

#### Docker compose example
```bash
# simple server
$ docker-compose -f examples/docker-compose-simple.yml up -d
# keycloak server
$ docker-compose -f examples/docker-compose-keycloak.yml up -d

# go to 'http://localhost:9201' and login with id of 'user' and password of 'pwd'

# clean up simple server
docker-compose -f examples/docker-compose-simple.yml down
# clean up keycloak server
docker-compose -f examples/docker-compose-keycloak.yml down
```


### For Kubernetes
```bash
# Edit examples/k8s-example.yaml first then
kubectl create -f examples/k8s-example.yaml

# set label for node you want to export
kubectl label node ${NODE} mstat-exporter=true
```
