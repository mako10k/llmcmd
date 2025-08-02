#!/bin/bash
# Product Backlog File Monitor Script
# ファイル変更を監視し、新しいBacklogアイテムを処理する

BACKLOG_FILE="/home/mako10k/llmcmd/product-backlog-requests.md"
PROCESSED_LOG="/home/mako10k/llmcmd/.backlog-processed.log"
LOCK_FILE="/home/mako10k/llmcmd/.backlog-monitor.lock"

# ロックファイルチェック（重複実行防止）
if [ -f "$LOCK_FILE" ]; then
    echo "Monitor already running. Exiting..."
    exit 1
fi

# ロックファイル作成
touch "$LOCK_FILE"

# 終了時のクリーンアップ
cleanup() {
    rm -f "$LOCK_FILE"
    exit
}
trap cleanup EXIT INT TERM

echo "Starting Product Backlog File Monitor..."
echo "Monitoring: $BACKLOG_FILE"

# 初回処理ログ作成
if [ ! -f "$PROCESSED_LOG" ]; then
    echo "$(date): Monitor started" > "$PROCESSED_LOG"
fi

# ファイル監視ループ
while true; do
    if [ -f "$BACKLOG_FILE" ]; then
        # ファイルの最終更新時刻を取得
        CURRENT_MTIME=$(stat -f "%m" "$BACKLOG_FILE" 2>/dev/null || stat -c "%Y" "$BACKLOG_FILE" 2>/dev/null)
        
        # 前回処理時刻を取得
        LAST_PROCESSED=$(tail -1 "$PROCESSED_LOG" | cut -d'|' -f2 2>/dev/null || echo "0")
        
        # ファイルが更新されている場合
        if [ "$CURRENT_MTIME" != "$LAST_PROCESSED" ]; then
            echo "$(date): File updated, processing new items..."
            
            # 新しいアイテムをカウント
            NEW_ITEMS=$(grep -c "**ステータス**: NEW" "$BACKLOG_FILE" 2>/dev/null || echo "0")
            
            if [ "$NEW_ITEMS" -gt 0 ]; then
                echo "$(date): Found $NEW_ITEMS new items"
                
                # Python処理スクリプトを実行
                echo "$(date): Triggering Python backlog processor..."
                python3 /home/mako10k/llmcmd/scripts/backlog-processor.py
                
                if [ $? -eq 0 ]; then
                    echo "$(date): Processing completed successfully"
                else
                    echo "$(date): Processing failed with error code $?"
                fi
                
                # 処理ログ更新
                echo "$(date)|$CURRENT_MTIME|$NEW_ITEMS items processed" >> "$PROCESSED_LOG"
            else
                echo "$(date): No new items found"
            fi
        fi
    else
        echo "$(date): Backlog file not found: $BACKLOG_FILE"
    fi
    
    # 30秒間隔で監視
    sleep 30
done
