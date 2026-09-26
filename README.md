# TimeView

上映イベント向けの、暗転できる単一タイマー。Go + React / TypeScript。タイマー状態はメモリ、設定はJSONファイルに保持し、分離LAN内で認証なしのHTTPを使用します。

## 起動

Windowsでは `dist/windows-amd64/timeview.exe` を実行して、[操作画面](http://127.0.0.1:8080/)を開きます。終了は実行したターミナルでCtrl+C。

```powershell
.\dist\windows-amd64\timeview.exe
# 分離LANの別端末から接続する場合
.\dist\windows-amd64\timeview.exe -listen 0.0.0.0:8080
# 設定ファイルの保存先を指定する場合
.\dist\windows-amd64\timeview.exe -config C:\TimeView\config.json
```

macOS / Linuxでは対象CPUの `dist/darwin-arm64/timeview` 等を実行します。

```sh
chmod +x ./timeview
./timeview -listen 0.0.0.0:8080
```

別端末からは `http://<サーバーPCのLAN側IP>:8080/`、演台は同じホストの `/display` へアクセスします。`0.0.0.0` は待受指定なので、ブラウザには実際のIPを入力してください。OSのファイアウォールで会場LANから指定ポートへの接続を許可してください。

初回起動時は10分・待機・**暗転ON**です。設定を確認し、操作画面で「暗転解除」を押してください。演台は警告段階に応じて背景色が変わり、画面下部のゲージで通常・第1警告・第2警告の領域、経過率、次の警告までの時間を示します。上映時は演台を全画面にし、実際の表示を確認してから暗転します。暗転中も計測を継続します。

## 設定ファイル

初回起動時にカレントディレクトリへ `timeview-config.json` を作成します。`-config`で別のパスを指定できます。指定先の親フォルダーは先に作成してください。持ち時間、警告、配色、表示モード、点滅、定型文、Keypad割り当てを画面から変更すると、UTF-8のJSONへ自動保存します。

JSONを手作業で編集する場合はTimeViewを終了してから変更し、再起動してください。不明な項目、値の不整合、未対応の設定バージョンがある場合は、安全のため起動を中止してエラーを表示します。再起動後も設定は復元しますが、残り時間、計測状態、暗転解除、送信中のカンペは復元せず、待機・暗転ONから始まります。

設定画面の「ブラウザからの操作だけを許可」を有効にすると、外部APIのGET・SSEは利用できますが、変更要求は403になります。操作画面と、その画面で有効にしたKeypadからは引き続き操作できます。この機能は分離LAN内の誤操作防止用で、認証機能ではありません。

## 開発

検証環境: Go 1.27.1、Node.js 24.16.0 / npm 11.13.0、Task 3.45.4（Taskfile v3）。Go依存は`go.mod`と`go.sum`、フロント依存は`package-lock.json`で固定しています。

バックエンドはGin v1.12でHTTPルーティングとミドルウェア、Koanf v2でJSON設定、go-playground/validatorで構造体検証、`golang.org/x/time/rate`でAPI流量制限を実装しています。タイマー状態は引き続きメモリ内に保持し、CGOは使用しません。選定理由と適用範囲は[依存ライブラリの選定](docs/DEPENDENCIES.md)を参照してください。

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

操作画面でKeypadをONにし、その画面を前面にしてください。フォーカスが外れるとOFFになります。カスタム割り当てはJSON設定へ保存され、ブラウザ再読み込みやサーバー再起動後も復元されます。安全のためKeypadのON/OFF状態は保存せず、常にOFFから始まります。

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

ブラウザ操作限定が有効な間、上記PowerShell・curlの変更例は403になります。解除はTimeViewの設定画面から行います。操作画面が使えない場合はTimeViewを終了し、設定JSONの`browserOnly`を`false`へ変更して再起動してください。

## ドキュメント

- [仕様書](docs/SPECIFICATION.md)
- [OpenAPI](docs/openapi.json)
- [動作確認結果と未確認項目](docs/VALIDATION.md)

ローカルGitリポジトリとして管理しています。GitHub等へのリモート作成・pushは行っていません。
