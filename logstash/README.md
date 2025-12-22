# Logstash

- 需要先构建镜像
  ```
  docker build -t my-logstash:1.0
  ```
- scripts文件夹下的几个可执行文件应该是改了api地址然后编译了的，如果不是，请先创建Dockerfile文件（需要在Ubuntu20.04环境中编译）：
  ```
  # Dockerfile
    FROM ubuntu:20.04

    # 安装Go
    RUN apt-get update && \
        apt-get install -y wget tar gcc && \
        wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz && \
        tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz

    ENV PATH=$PATH:/usr/local/go/bin

    WORKDIR /src
    COPY . .

    # 编译时指定兼容的链接模式
    RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=gcc go build -ldflags="-linkmode external -extldflags -static" -o load_categories load_categories.go
    RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=gcc go build -ldflags="-linkmode external -extldflags -static" -o load_attributes load_attributes.go
    RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=gcc go build -ldflags="-linkmode external -extldflags -static" -o load_products load_products.go
    RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=gcc go build -ldflags="-linkmode external -extldflags -static" -o load_variations load_variations.go
    ```
    然后执行以下命令：
    ```
    # 构建Docker镜像
    docker build -t compatible-builder .

    # 运行容器并将编译好的文件复制出来
    docker run --rm -v $(pwd):/output compatible-builder cp load_categories load_attributes load_products load_variations /output/
    ```
    然后把编译出来的文件替换src/scripts中的文件

- 运行容器：
  ```
  docker compose up -d
  ```
- 日志文件存储位置：
  ```
  /var/lib/docker/volumes/logstash_31dgm/_data/
  ```
- 如果要修改文件检查更新间隔，请修改 **src/pipeline/download_and_process.conf** 文件中的 **interval** 项