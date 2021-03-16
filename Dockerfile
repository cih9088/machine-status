FROM nvidia/cuda:10.1-base-ubuntu18.04
MAINTAINER Andy Cho <cih9088@gmail.com>

RUN apt-get update && apt-get install -y wget gawk tzdata binutils && rm -rf /var/lib/apt/lists/*

ENV TZ='Asia/Seoul'
RUN ln -snf /usr/share/zoninfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

RUN wget https://dl.google.com/go/go1.14.2.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.14.2.linux-amd64.tar.gz
ENV PATH="${PATH}:/usr/local/go/bin"
RUN mkdir /app
WORKDIR /app
RUN go mod init github.com/cih9088/machine-status

ADD . ./

ENTRYPOINT [ "go", "run", "main.go" ]
