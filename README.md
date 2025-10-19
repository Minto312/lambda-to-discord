# Lambda to Discord Webhook

このリポジトリは、AWS Lambda から Discord Webhook へ通知を送信するための Go 実装です。入力経路は CloudWatch Alarm/SNS と任意 JSON の直接入力の 2 系統をサポートしており、アダプタ層で共通の通知モデルに変換した上で Discord へ配信します。

## アーキテクチャ概要

- `adapter/` には入力形式ごとのアダプタを実装しています。
  - `cloudwatch_sns.go`: CloudWatch Alarm → SNS → Lambda で受信した固定スキーマを解析します。SNS からは raw message delivery を利用する想定です。
  - `direct.go`: 任意の JSON ペイロードを直接変換します。
- `domain/notification.go` に通知ドメインモデルを定義し、`discord/client.go` で送信ロジックを一元管理しています。
- Lambda デプロイ時に環境変数 `ADAPTER_TYPE` を `cloudwatch` または `direct` に設定することで、起動時に利用するアダプタを切り替えます。
- CloudWatch 系統では追加で環境変数 `WEBHOOK_URL` に送信先 Discord Webhook を設定してください。Direct 系統ではイベント内の `webhookURL` で送信先を指定します (未指定の場合は `WEBHOOK_URL` が利用されます)。

## イベント形式

### Direct アダプタ

Lambda に渡すイベントは以下のような JSON オブジェクトです。`content` または `message` のいずれかを含め、`webhookURL` (または `webhook_url`) に送信先を指定します。イベント内で送信先が指定されていない場合は、環境変数 `WEBHOOK_URL` がフォールバックとして使用されます。

```json
{
  "webhookURL": "https://discord.com/api/webhooks/...",
  "content": "通知本文",
  "username": "任意の表示名",
  "avatar_url": "任意のアイコン URL",
  "embeds": [],
  "allowed_mentions": {
    "parse": []
  }
}
```

`allowed_mentions` は Discord の仕様に従った構造で指定できます。

### CloudWatch/SNS アダプタ

CloudWatch Alarm から SNS 経由で Lambda に届くメッセージ (raw message) をそのまま渡すことを想定しています。アラームの状態遷移・メトリクス・ディメンションなどを Embed として整形し、`WEBHOOK_URL` で指定した Discord へ通知します。必要に応じて SNS 側で raw message delivery を有効化してください。

## エラー通知

環境変数 `ERROR_WEBHOOK_URL` を設定すると、リクエスト処理中にエラーが発生した際に元のリクエスト内容とエラーメッセージを含む通知を送信します。通知が不要な場合は未設定のままにしてください。

## デプロイ

Go ランタイムを利用するため、Linux 向けにビルドしたバイナリをアップロードします。エントリーポイントは `lambda` ビルドタグの下に配置しているため、ビルド時にタグを指定します。

```bash
make build-amd64
make package-amd64
```

Arm64 (Graviton) ランタイム向けのバイナリを生成する場合は以下のターゲットを利用してください。

```bash
make build-arm64
make package-arm64
```

あるいは、任意のアーキテクチャでビルドしたい場合は `GOARCH` 変数を指定して `make build`/`make package` を実行できます。

```bash
make build GOARCH=arm64
make package GOARCH=arm64
```

デプロイ後、Lambda 関数に `ADAPTER_TYPE` および必要な Webhook URL 系の環境変数を設定し、`cloudwatch` と `direct` のエイリアスを付与してそれぞれのトリガーに割り当ててください。

## ローカルテスト

```bash
go test ./...
```

すべてのアダプタおよび送信ロジックには単体テストが用意されており、上記コマンドで一括実行できます。
