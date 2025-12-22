#!/bin/bash
# download_and_process.sh
# 定义文件名
DATA_PATH="/usr/share/logstash/data/"
DEBUG_LOG="${DATA_PATH}debug_detailed.log"
CATEGORIES_FILE="recom-categories.xml.gz"
CATALOG_FILE="recom-catalog.xml.gz"

# 定义SFTP连接信息(考虑使用环境变量更安全)
USER="mkt_cnwebsite"
PASSWORD="Ht0jzAxMpnouKrCvdatcrmftU"
HOST="webdisk.recom.at"
REMOTE_PATH="Production"

# 创建临时目录用于比较
TEMP_PATH="/tmp/logstash_download/"
mkdir -p "$TEMP_PATH"

CATEGORIES_CHANGED=false
CATALOG_CHANGED=false

# 初始化日志记录（追加模式）
echo "=== $(date) ===" >> "$DEBUG_LOG"

# 处理 recom-categories.xml.gz
echo "Processing $CATEGORIES_FILE..." | tee -a "$DEBUG_LOG"

if /usr/bin/curl --insecure --ftp-ssl -u "$USER:$PASSWORD" "sftp://$HOST/$REMOTE_PATH/$CATEGORIES_FILE" -o "${TEMP_PATH}${CATEGORIES_FILE}"; then
    echo "Successfully downloaded $CATEGORIES_FILE" | tee -a "$DEBUG_LOG"
    
    # 解压到临时位置
    if /usr/bin/gzip -d -c "${TEMP_PATH}${CATEGORIES_FILE}" > "${TEMP_PATH}recom-categories.xml"; then
        echo "Successfully decompressed $CATEGORIES_FILE" | tee -a "$DEBUG_LOG"
        
        # 比较新旧文件内容
        if [ ! -f "${DATA_PATH}recom-categories.xml" ] || ! cmp -s "${TEMP_PATH}recom-categories.xml" "${DATA_PATH}recom-categories.xml"; then
            echo "Categories file changed, updating..." | tee -a "$DEBUG_LOG"
            mv "${TEMP_PATH}recom-categories.xml" "${DATA_PATH}recom-categories.xml"
            CATEGORIES_CHANGED=true
        else
            echo "Categories file unchanged, skipping update" | tee -a "$DEBUG_LOG"
        fi
    else
        echo "Failed to decompress $CATEGORIES_FILE" >&2 | tee -a "$DEBUG_LOG"
        rm -f "${TEMP_PATH}${CATEGORIES_FILE}" "${TEMP_PATH}recom-categories.xml"
        exit 1
    fi
else
    echo "Failed to download $CATEGORIES_FILE" >&2 | tee -a "$DEBUG_LOG"
    rm -f "${TEMP_PATH}${CATEGORIES_FILE}"
    exit 1
fi

# 处理 recom-catalog.xml.gz
echo "Processing $CATALOG_FILE..." | tee -a "$DEBUG_LOG"

if /usr/bin/curl --insecure --ftp-ssl -u "$USER:$PASSWORD" "sftp://$HOST/$REMOTE_PATH/$CATALOG_FILE" -o "${TEMP_PATH}${CATALOG_FILE}"; then
    echo "Successfully downloaded $CATALOG_FILE" | tee -a "$DEBUG_LOG"
    
    # 解压到临时位置
    if /usr/bin/gzip -d -c "${TEMP_PATH}${CATALOG_FILE}" > "${TEMP_PATH}recom-catalog.xml"; then
        echo "Successfully decompressed $CATALOG_FILE" | tee -a "$DEBUG_LOG"
        
        # 比较新旧文件内容
        if [ ! -f "${DATA_PATH}recom-catalog.xml" ] || ! cmp -s "${TEMP_PATH}recom-catalog.xml" "${DATA_PATH}recom-catalog.xml"; then
            echo "Catalog file changed, updating..." | tee -a "$DEBUG_LOG"
            mv "${TEMP_PATH}recom-catalog.xml" "${DATA_PATH}recom-catalog.xml"
            CATALOG_CHANGED=true
        else
            echo "Catalog file unchanged, skipping update" | tee -a "$DEBUG_LOG"
        fi
    else
        echo "Failed to decompress $CATALOG_FILE" >&2 | tee -a "$DEBUG_LOG"
        rm -f "${TEMP_PATH}${CATALOG_FILE}" "${TEMP_PATH}recom-catalog.xml"
        exit 1
    fi
else
    echo "Failed to download $CATALOG_FILE" >&2 | tee -a "$DEBUG_LOG"
    rm -f "${TEMP_PATH}${CATALOG_FILE}"
    exit 1
fi

# 清理临时文件
rm -f "${TEMP_PATH}${CATEGORIES_FILE}" "${TEMP_PATH}${CATALOG_FILE}"

# 根据文件变化情况决定执行哪些处理脚本
echo "Categories changed: $CATEGORIES_CHANGED" | tee -a "$DEBUG_LOG"
echo "Catalog changed: $CATALOG_CHANGED" | tee -a "$DEBUG_LOG"

if [ "$CATEGORIES_CHANGED" = true ] || [ "$CATALOG_CHANGED" = true ]; then
    echo "FILES_CHANGED" | tee -a "$DEBUG_LOG"
    
    # 根据具体哪个文件发生了变化来决定执行哪些脚本
    if [ "$CATEGORIES_CHANGED" = true ]; then
        echo "Running load_categories as categories file changed..." | tee -a "$DEBUG_LOG"
        if ! /usr/share/logstash/scripts/load_categories >> "$DEBUG_LOG" 2>&1; then
            echo "load_categories failed" >&2 | tee -a "$DEBUG_LOG"
            exit 1
        fi
        echo "load_categories completed successfully" | tee -a "$DEBUG_LOG"
    fi
    
    if [ "$CATALOG_CHANGED" = true ]; then
        echo "Running catalog processing scripts as catalog file changed..." | tee -a "$DEBUG_LOG"
        if ! /usr/share/logstash/scripts/load_attributes >> "$DEBUG_LOG" 2>&1; then
            echo "load_attributes failed" >&2 | tee -a "$DEBUG_LOG"
            exit 1
        fi
        echo "load_attributes completed successfully" | tee -a "$DEBUG_LOG"
        
        if ! /usr/share/logstash/scripts/load_products >> "$DEBUG_LOG" 2>&1; then
            echo "load_products failed" >&2 | tee -a "$DEBUG_LOG"
            exit 1
        fi
        echo "load_products completed successfully" | tee -a "$DEBUG_LOG"
        
        if ! /usr/share/logstash/scripts/load_variations >> "$DEBUG_LOG" 2>&1; then
            echo "load_variations failed" >&2 | tee -a "$DEBUG_LOG"
            exit 1
        fi
        echo "load_variations completed successfully" | tee -a "$DEBUG_LOG"
    fi
    
    echo "All required processing steps completed" | tee -a "$DEBUG_LOG"
else
    echo "FILES_UNCHANGED" | tee -a "$DEBUG_LOG"
    echo "Skipping all processing scripts" | tee -a "$DEBUG_LOG"
fi