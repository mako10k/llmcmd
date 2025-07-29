# Raspberry Pi用のインストール手順

Raspberry Pi 5 (ARM64)で llmcmd をインストールする方法：

## 方法1: 直接ダウンロード（推奨）

```bash
# ARM64バイナリを直接ダウンロード
curl -sL "https://raw.githubusercontent.com/mako10k/llmcmd/main/dist/llmcmd-linux-arm64" -o llmcmd
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
export OPENAI_API_KEY="your-api-key-here"
# または
echo 'export OPENAI_API_KEY="your-api-key-here"' >> ~/.bashrc
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
