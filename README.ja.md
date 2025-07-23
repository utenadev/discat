# discat

CLIの出力をDiscordチャンネルにWebhookで簡単に送信するためのツールです。
> 実は、これも私が作った[slackcat](https://github.com/dwisiswant0/slackcat)のフォーク版です！

## インストール

- [リリースページ](https://github.com/utenadev/discat/releases/latest)からビルド済みのバイナリをダウンロードし、展開して実行してください！ または
- go1.21+のコンパイラがインストールされている場合: `go install github.com/utenadev/discat@latest`

## 設定

**ステップ1:** [こちら](https://support.discord.com/hc/en-us/articles/228383668-Intro-to-Webhooks)からあなたのDiscordチャンネルのWebhook URLを取得してください。

**ステップ2** _(オプション)_**:** 環境変数 `DISCORD_WEBHOOK_URL` を設定します。
```bash
export DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/xnxx/xxx-xxx"
```

## 使い方

とてもシンプルです！

```bash
▶ echo -e "Hello,\nworld!" | discat
```

### フラグ

```
Usage of discat:
  -1    1行ごとにメッセージを送信
  -c string
        設定ファイルのパス
  -u string
        Discord Webhook URL
  -v    詳細モード
```

### 応用例

目的は、興味深い情報について自動でアラートを受け取ることです！

```bash
▶ assetfinder twitter.com | anew | discat -u https://hooks.discord.com/services/xxx/xxx/xxx
```

環境変数 `DISCORD_WEBHOOK_URL` を定義している場合、`-u` フラグはオプションです。

Discatは標準入力からANSIカラーコードを除去してメッセージを送信するため、Discordではクリーンなメッセージを受信できます！

```bash
▶ nuclei -l urls.txt -t cves/ | discat
```

![Proof](https://user-images.githubusercontent.com/25837540/108782401-1571e380-759e-11eb-8d20-dfcc9294a30a.png)

### 1行ごとの送信

以前に実行したプログラムの終了を待つ代わりに、メッセージを1行ずつ送信したい場合は `-1` フラグを使用してください _(デフォルト: false)_。

```bash
▶ amass track -d domain.tld | discat -1
```

## 変更履歴

### v0.0.2

#### ✨ 新機能

*   **設定ファイル**: YAMLファイルによる設定に対応しました (`-c` フラグ)。
*   **グレースフルシャットダウン**: `SIGINT`および`SIGTERM`シグナル受信時に、正常にシャットダウンする処理を実装しました。
*   **レートリミット**: APIの過剰な使用を防ぐためのレートリミッターを追加しました。
*   **リトライ処理**: メッセージ送信失敗時に、指数バックオフ付きのリトライ処理を実装しました。
*   **メトリクス**: 送信メッセージ数、エラー数、バイト数のメトリクスを追加し、詳細モードで表示するようにしました。
*   **メッセージ分割**: Discordの2000文字の制限を超えるメッセージを自動的に分割するようになりました。

#### ♻️ リファクタリング・改善

*   **構造化ロギング**: `fmt`による出力から、`log/slog`パッケージを使用した構造化ロギングに変更しました。
*   **コード構造**: `DiscordSender`構造体の導入や、設定・シグナル処理を専用関数に分離することで、モジュール性とテスト容易性を向上させる大幅なリファクタリングを実施しました。
*   **エラーハンドリング**: エラーの伝播とコンテキストを改善しました。

## ライセンス

`discat` はMITライセンスで配布されています。
