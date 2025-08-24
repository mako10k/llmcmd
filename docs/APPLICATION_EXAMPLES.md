# 応用例集

このドキュメントでは、`llmcmd` を活用した実践的な使い方をいくつか紹介します。基本的な機能やインストール方法については `README.md` を参照してください。

## ログ分析

大量のログファイルからエラー情報だけを抽出したい場合、`data_proc` プリセットを利用すると効率的です。

```bash
llmcmd --preset data_proc -i /var/log/app.log "エラー行のみを抽出して要約してください"
```

## データ前処理

CSV ファイルの集計やフィルタリングなど、シェルスクリプトでは煩雑になりがちな処理も、LLM に自然言語で指示できます。

```bash
llmcmd --preset data_proc -i sales.csv "月別売上の合計を計算し、降順で表示してください"
```

## コードレビュー補助

`code_review` プリセットを使用すると、既存コードの問題点洗い出しや改善案の提案を得られます。

```bash
llmcmd --preset code_review -i main.go "このコードの改善点を教えてください"
```

## パッチ生成と適用

2 つのファイル差分からパッチを生成したり、既存パッチを適用する際は `diff_patch` プリセットが便利です。

```bash
# パッチ生成
llmcmd --preset diff_patch -i old.txt -i new.txt -o update.patch "差分からパッチを作成してください"

# パッチ適用
llmcmd --preset diff_patch -i update.patch -i old.txt "このパッチを適用してください"
```

## 複数ファイルの一括処理

ワイルドカードや `find` コマンドと組み合わせることで、多数のファイルをまとめて処理できます。

```bash
find ./logs -name "*.log" | llmcmd --preset data_proc "各ログファイルのエラー数を集計してください"
```

## 大規模データの概要取得

巨大な CSV などを丸ごと読み込むとトークン数が膨大になるため、まずはファイル情報のみを渡して概要を把握し、必要に応じて部分読み込みを行う使い方が推奨されます。

```bash
llmcmd --preset data_proc -i large_dataset.csv "このデータセットの構造を教えてください。必要な箇所だけ読み込む方法も提案してください"
```

