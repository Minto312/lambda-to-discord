# Lambda to Discord Webhook

このリポジトリには、AWS Lambda から Discord Webhook へ通知を送信するための
Go 実装とテストが含まれています。

## 使い方

1. `DISCORD_WEBHOOK_URL` 環境変数に Discord Webhook の URL を設定します。
2. Lambda に渡すイベントは以下のような JSON オブジェクトで、`content` または
   `message` を含めてください。

```json
{
  "content": "通知本文",
  "username": "任意の表示名",
  "avatar_url": "任意のアイコン URL",
  "embeds": []
}
```

`message` キーは `content` のエイリアスとして扱われます。

## デプロイ

Go ランタイムを利用するために、Linux 向けにビルドしたバイナリをアップロードします。
エントリーポイントは `lambda` ビルドタグの下に配置しているため、ビルド時にタグを指定します。

```bash
GOOS=linux GOARCH=amd64 go build -tags lambda -o bootstrap
zip function.zip bootstrap
```

作成した `function.zip` を Lambda 関数にデプロイし、ランタイムに
「Amazon Linux 2023 を対象としたカスタムランタイム (provided.al2023)」を選択してください。

## ローカルテスト

```bash
go test ./...
```

`DISCORD_WEBHOOK_URL` が設定されていない場合、ハンドラーは実行時にエラーを返します。
本番環境では必ず設定してください。
