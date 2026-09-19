# TimeView

上映イベント向けの、暗転できる単一タイマー。Go + React / TypeScript。状態はメモリに保持し、分離LAN内で認証なしのHTTPを使用します。

## 起動

Windowsでは `dist/windows-amd64/timeview.exe` を実行して、[操作画面](http://127.0.0.1:8080/)を開きます。終了は実行したターミナルでCtrl+C。

```powershell
.\dist\windows-amd64\timeview.exe
# 分離LANの別端末から接続する場合
.\dist\windows-amd64\timeview.exe -listen 0.0.0.0:8080
```

macOS / Linuxでは対象CPUの `dist/darwin-arm64/timeview` 等を実行します。

```sh
chmod +x ./timeview
./timeview -listen 0.0.0.0:8080
```

別端末からは `http://<サーバーPCのLAN側IP>:8080/`、演台は同じホストの `/display` へアクセスします。`0.0.0.0` は待受指定なので、ブラウザには実際のIPを入力してください。OSのファイアウォールで会場LANから指定ポートへの接続を許可してください。

起動時は10分・待機・**暗転ON**です。設定を確認し、操作画面で「暗転解除」を押してください。上映時は演台を全画面にし、実際の表示を確認してから暗転します。暗転中も計測を継続します。再起動でタイマー・定型文・設定は消え、再接続後も暗転ONになります。

## 開発

検証環境: Go 1.25.3、Node.js 24.16.0 / npm 11.13.0、Task 3.45.4（Taskfile v3）。依存はpackage-lock.jsonで固定しています。

```sh
task setup
# 別ターミナルでそれぞれ起動
task dev:backend
task dev:frontend
```

Viteはターミナルに表示される開発URLで開きます。`/api` はGoの8080ポートへプロキシします。開発用Go起動前にもフロントをビルドし、embed対象を準備します。

```sh
task fmt
task lint
task test
task build
task release
```

- `task build`: 現在のOS / CPU向けバイナリを生成。
- `task release`: Windows amd64、macOS amd64 / arm64、Linux amd64 / arm64を生成。
- 本番実行時はGo・Node.js・Task・DBは不要。配布先に実行ファイルを渡します。
- 依存取得は開発環境で事前に行います。会場内でインターネットへ接続しません。
- Go検証の対象は `. ./internal/... ./web`。`node_modules` 内の他社Goコードを対象に含めません。

## Keypad

操作画面でKeypadをONにし、その画面を前面にしてください。フォーカスが外れるとOFFになります。ブラウザ再読み込みでカスタム割り当ても既定に戻ります。

| キー | 操作 |
| --- | --- |
| テンキーEnter / 小数点 | 開始・再開 / 一時停止 |
| テンキー＋ / − | +60秒 / −60秒 |
| Ctrl + テンキー0 | リセット |
| Ctrl + テンキー− / ＋ | 暗転 / 暗転解除 |
| テンキー1～9 | 定型文を即時送信 |
| テンキー0 / × / ÷ | カンペ非表示 / 再表示 / 消去 |

## API例

PowerShell:

```powershell
$base = 'http://127.0.0.1:8080/api/v1/timer'
Invoke-RestMethod "$base/commands" -Method Post -ContentType 'application/json' -Body '{"command":"start"}'
Invoke-RestMethod "$base/blackout" -Method Put -ContentType 'application/json' -Body '{"enabled":true}'
$headers = @{ 'Idempotency-Key' = [guid]::NewGuid().ToString() }
Invoke-RestMethod "$base/commands" -Method Post -Headers $headers -ContentType 'application/json' -Body '{"command":"adjust","deltaSeconds":60}'
```

curl:

```sh
curl http://127.0.0.1:8080/api/v1/timer
curl -X PUT http://127.0.0.1:8080/api/v1/timer/blackout -H 'Content-Type: application/json' -d '{"enabled":false}'
curl -N http://127.0.0.1:8080/api/v1/timer/events
```

設定変更はGET応答のETagを`If-Match`で送ります。加減算は`Idempotency-Key`が必須。同じ要求の再送では同じキー・本文を使ってください。記録は同一起動中の10分間有効です。任意の`X-Timeview-Instance`に取得したinstanceIdを付けると、再起動前の操作を409で拒否します。

## ドキュメント

- [仕様書](docs/SPECIFICATION.md)
- [OpenAPI](docs/openapi.json)
- [動作確認結果と未確認項目](docs/VALIDATION.md)

ローカルGitリポジトリとして管理しています。GitHub等へのリモート作成・pushは行っていません。
