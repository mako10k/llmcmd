# Raspberry Pi用のインストール手順

Raspberry Pi 5 (ARM64)で llmcmd をインストールする方法：

## 方法1: 直接ダウンロード（推奨）

```bash
# ARM64バイナリを直接ダウンロード
curl -sL "https://raw.githubusercontent.com/mako10k/llmcmd/main/releases/llmcmd-linux-arm64" -o llmcmd
chmod +x llmcmd

# システムワイドにインストール
sudo mv llmcmd /usr/local/bin/

# テスト
echo "Hello Raspberry Pi" | llmcmd "Convert to uppercase"
```

## 方法2: 手動ビルド

```bash
# Go 1.21+がインストールされている場合
git clone https://github.com/mako10k/llmcmd.git
cd llmcmd
go build -o llmcmd ./cmd/llmcmd
sudo mv llmcmd /usr/local/bin/
```

## 設定

OpenAI API キーを設定：

```bash
# 方法1: 環境変数（推奨）
export OPENAI_API_KEY="your-api-key-here"
echo 'export OPENAI_API_KEY="your-api-key-here"' >> ~/.bashrc

# 方法2: 設定ファイル
curl -sL https://raw.githubusercontent.com/mako10k/llmcmd/main/.llmcmdrc.example -o ~/.llmcmdrc
nano ~/.llmcmdrc  # API キーを設定
```

### 設定ファイルのオプション

`~/.llmcmdrc` で以下の設定が可能：

```ini
# OpenAI API設定
openai_api_key=your-api-key-here
model=gpt-4o-mini
max_tokens=4096
temperature=0.1

# セキュリティ設定
max_api_calls=50
timeout_seconds=300
max_file_size=10485760    # 10MB
read_buffer_size=4096     # 4KB

# 再試行設定  
max_retries=3
retry_delay_ms=1000
```

## テスト例

```bash
# 基本的なテスト
echo "apple banana cherry" | llmcmd "Sort alphabetically"

# ファイル処理
cat /etc/passwd | llmcmd "Count the number of users"

# 複雑な処理
echo -e "line1\nline2\nline3" | llmcmd "Number each line"
```

## 注意事項

- Raspberry Pi OS (64-bit) が必要
- OpenAI API キーが必要
- インターネット接続が必要（API呼び出しのため）
