# 新しい中間言語(slash [Stream oriented Llm bourne Again Shell])

## 新しいシェル仕様ドラフト

### 概要
このドキュメントは、新しいシェルの仕様ドラフトです。
目的は llmcmd の内部中間言語としての shell like 機能を提供し、
LLM が直感的に利用できるようにすることです。

#### ファイル安全性
入出力ファイルは基本操作できません。
例外は下記のみ
 - stdin
 - stdout
 - stderr
 - `-i` オプションで指定されたファイル
 - `-o` オプションで指定されたファイル

仮想ファイルは扱えますが、基本は名前付きパイプと同じような機能となります。
出力先として指定された場合は、仮想ファイルが自動的に作成されます。
上記の例外ファイルも扱いは仮想ファイルと同じになります。

list > {仮想ファイル名}
tee {仮想ファイル名1} {仮想ファイル名2} ...

入力元は存在する仮想ファイルのみとなります。

cat {仮想ファイル名1} {仮想ファイル名2} ...
tail -10 < {仮想ファイル名}

内部では、pipe で作成された fd が入ります。

外部コマンドは一切利用できません。
内部コマンドのみで構成されます。

### 内部コマンド
以下は、内部コマンドの一覧です。
ls, cat, head, tail, tee, grep, sed, sort, uniq, wc, cut, tr, find, xargs
read, env, echo, printf, sleep, exit, kill, wait, export, source, alias, unalias, history, jobs, fg, bg, killall, :, [, test, true, false, time, seq, bc, sed, exec, eval, unset, exit, return, continue, break, trap, set, shopt, declare, typeset, local

制御構文
if, then, else, elif, fi, for, in, do, done, while, until, case, esac

関数定義
function, return, local

変数展開
$VAR, ${VAR}, $((expression)), $((VAR)), $((VAR + 1)), $((VAR * 2))

特殊な変数展開
$?, $$, $!, $#, $*, $@, ${#VAR}, ${VAR#pattern}, ${VAR##pattern}, ${VAR%pattern}, ${VAR%%pattern}, ${VAR/pattern/replacement}, ${VAR//pattern/replacement}, ${VAR:offset:length}, ${VAR:offset}, ${VAR:0:length}, ${VAR:0}, ${var:-default}, ${VAR:=default}, ${VAR:+value}, ${VAR:?error_message}

構文
{ ... } # ブロック
; # コマンド区切り (改行も可)
&& # 論理積
|| # 論理和
| # パイプ
& # バックグラウンド実行
() # サブシェル
NAME=VALUE # 変数代入

リダイレクト
list > file
list >> file
list < file
list 2> file
list 2>> file
list &> file
list &>> file
list 2>&1
exec {fd}>file
exec {fd}>>file
exec {fd}<file
exec {fd}<&0 # stdin
exec {fd}>&1 # stdout
exec {fd}>&2 # stderr   
exec {fd}>&- # close fd
exec {fd}<&- # close fd
exec {fd}>&{fd2} # duplicate fd
exec {fd}<&{fd2} # duplicate fd

