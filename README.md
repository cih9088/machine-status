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
    cih9088/machine-status:0.3.2 exporter

# use another port rather than default one
$ docker run -p 9999:9999 --detach --pid=host --hostname=$(hostname) \
    --name mstat-exporter --restart always --gpus all \
    cih9088/machine-status:0.3.2 exporter \
        --port 9999

# show user name and pid on each GPUs
# note that, to query username from UID,
# one should bind /etc/passwd, /etc/group, /etc/shadow to the container
$ docker run -p 9200:9200 --detach --pid=host --hostname=$(hostname) \
    --volume /etc/passwd:/etc/passwd:ro \
    --volume /etc/group:/etc/group:ro \
    --volume /etc/shadow:/etc/shadow:ro \
    --name mstat-exporter --restart always --gpus all \
    cih9088/machine-status:0.3.2 exporter \
        --show-user --show-pid

# or create explicit mapping between UID and username
$ docker run -p 9200:9200 --detach --pid=host --hostname=$(hostname) \
    --name mstat-exporter --restart always --gpus all \
    cih9088/machine-status:0.3.2 exporter \
        --show-user --show-pid \
        --mapping="$(getent passwd | awk -F':' '{ if ($3 >= 1000) printf "%s:%s ", $1, $3; }')"

# change timezone
$ docker run -p 9200:9200 --detach --pid=host --hostname=$(hostname) \
    --name mstat-exporter --restart always --gpus all \
    --env TZ="Europe/London" \
    cih9088/machine-status:0.3.2 exporter

# help for exporter
$ docker run --rm cih9088/machine-status:0.3.2 exporter -h
```

<!-- ##### Environment variables -->
<!-- - **MSTAT_PORT**: Port to serve. Defaults to `9200`. -->
<!-- - **MSTAT_SHOW_USER**: Show user name of process. Defaults to `false`. -->
<!-- - **MSTAT_SHOW_PID**: Show PID. Defaults to `false`. -->
<!-- - **MSTAT_SHOW_POWER**: Show power consumption. Defaults to `false`. -->
<!-- - **MSTAT_SHOW_CMD**: Show command. Defaults to `true`. -->
<!-- - **MSTAT_SHOW_FAN**: Show fan speed. Defaults to `false`. -->
<!-- - **MSTAT_MAPPING**: Mapping between username and UID. Defualts to ``. -->

#### Server
Change user, pass, machine, and etc. as you wish.
```bash
# simple authenticated web server
$ docker run -p 80:80 --detach --name mstat-server --restart always \
    cih9088/machine-status:0.3.2 server-simple \
        --fqdn $(hostname --fqdn) \
        --user user1,user2 \
        --pwd pass1,pass2 \
        --machine machine1.example.com:9200 \
        --machine machine2.example.com:9200 \
        --machine machine3.example.com:9200

# simple authenticated web server with non trivial port
$ docker run -p 8080:8080 --detach --name mstat-server --restart always \
    cih9088/machine-status:0.3.2 server-simple \
        --fqdn $(hostname --fqdn):8080 \
        --user user1,user2 \
        --pwd pass1,pass2 \
        --machine machine1.example.com:9200 \
        --machine machine2.example.com:9200 \
        --machine machine3.example.com:9200

# simple authenticated web server with alias
$ docker run -p 80:80 --detach --name mstat-server --restart always \
    cih9088/machine-status:0.3.2 server-simple \
        --fqdn $(hostname --fqdn) \
        --user user1,user2 \
        --pwd pass1,pass2 \
        --machine "machine1.example.com:9200->alias" \
        --machine "machine2.example.com:9200->alias" \
        --machine "machine3.example.com:9200->alias"

# simple authenticated web server with pre-generated tls
$ docker run -p 443:443 --detach --name mstat-server --restart always \
    --volume path/where/certs/are/in:/tmp/certs \
    cih9088/machine-status:0.3.2 server-simple \
        --fqdn $(hostname --fqdn):443 \
        --wss \
        --https-key key_name_in_path/wehre/certs/are/in \
        --https-crt certificate_name_in_path/where/certs/are/in \
        --user user1,user2 \
        --pwd pass1,pass2 \
        --machine machine1.example.com:9200 \
        --machine machine2.example.com:9200 \
        --machine machine3.example.com:9200

# simple authenticated web server with self-signed tls
$ mkdir certs && pushd certs
$ openssl req -new -subj "/C=US/ST=Utah/CN=localhost" -newkey rsa:2048 -nodes -keyout localhost.key -out localhost.csr
$ openssl x509 -req -days 365 -in localhost.csr -signkey localhost.key -out localhost.crt
$ popd
$ docker run -p 8080:8080 --detach --name mstat-server --restart always \
    $(pwd)/certs:/tmp/certs \
    cih9088/machine-status:0.3.2 server-simple \
        --fqdn $(hostname --fqdn):8080 \
        --wss \
        --https-key localhost.key \
        --https-crt localhost.crt \
        --user user1,user2 \
        --pwd pass1,pass2 \
        --machine machine1.example.com:9200

# simple authenticated web server with letsencrypt tls
$ docker run -p 443:443 --detach --name mstat-server --restart always \
    --volume path/where/certs/are/in:/tmp/certs \
    cih9088/machine-status:0.3.2 server-simple \
        --fqdn $(hostname --fqdn):443 \
        --wss \
        --letsencrypt \
        --user user1,user2 \
        --pwd pass1,pass2 \
        --machine machine1.example.com:9200 \
        --machine machine2.example.com:9200 \
        --machine machine3.example.com:9200

# keycloak authenticated web server with letsencrypt tls
$ docker run -p 443:443 --detach --name mstat-server --restart always \
    --volume path/where/certs/are/in:/tmp/certs \
    --volume path/to/keycloak.json:/app/web/keycloak/keycloak.json
    cih9088/machine-status:0.3.2 server-simple \
        --fqdn $(hostname --fqdn):443 \
        --wss \
        --letsencrypt \
        --keycloak-server https://keycloak.server:8443 \
        --keycloak-client client_name \
        --keycloak-realm realm_name \
        --machine machine1.example.com:9200 \
        --machine machine2.example.com:9200 \
        --machine machine3.example.com:9200

# help for simple server
$ docker run --rm cih9088/machine-status:0.3.2 server-simple -h
# help for keycloak server
$ docker run --rm cih9088/machine-status:0.3.2 server-keycloak -h
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
