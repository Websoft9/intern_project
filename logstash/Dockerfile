FROM docker.elastic.co/logstash/logstash:8.11.0

USER root

# 安装curl和其他必要工具
RUN apt-get update && \
    apt-get install -y curl gzip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# 切换回logstash用户（如果需要）
USER logstash
