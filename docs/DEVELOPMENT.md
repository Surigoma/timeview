# TimeView 開発ガイド

## 開発環境

検証環境はGo 1.27.1、Node.js 24.16.0、npm 11.13.0、Task 3.45.4（Taskfile v3）。Go依存は`go.mod`と`go.sum`、フロント依存は`package-lock.json`で固定する。

バックエンドはGinでHTTPルーティング、KoanfでJSON設定、go-playground/validatorで入力検証、`golang.org/x/time/rate`でAPI流量制限を実装している。タイマー状態はメモリ内に保持し、サーバーはCGOを使用しない。UIなしリモコンのビルドにはCGOとC/C++ツールチェーンが必要になる。詳細は[依存ライブラリの選定](DEPENDENCIES.md)を参照。

UIなしリモコンをローカルビルドする場合、WindowsはMinGW-w64、macOSはXcode Command Line Tools、LinuxはC/C++コンパイラーに加えてALSA、X11、Xtst、X11-xcb、xcb-xkb、xkbcommonの開発パッケージを用意する。配布用CIは対象OSごとにこれらを準備する。

```sh
task setup
# 別ターミナルでそれぞれ起動
task dev:backend
task dev:frontend
# UIなしリモコン（例）
task dev:remote -- -server http://127.0.0.1:8080 status
```

Viteは表示された開発URLで開く。`/api`とSSEはGoの8080ポートへプロキシされる。GoサーバーはReact成果物を埋め込むため、`dev:backend`でも先にフロントをビルドする。

ブラウザのMIDI入力は標準のWeb MIDI APIを直接使用する。UIなしリモコンの常駐入力はgohookでグローバルキー、gomidi/rtmididrvで各OSのMIDI入力を監視する。WindowsはWinMM、macOSはCoreMIDI、LinuxはALSAとX11を利用し、Note OnとControl Changeだけを対象とする。macOSではアクセシビリティの入力監視許可が必要で、LinuxのWaylandネイティブ環境は対象外とする。

## 検証とビルド

```sh
task fmt
task lint
task test
task build
task release
task docker:build
task docker:up
```

- `task build`: 現在のOS／CPU向けバイナリを生成する。
- `task release`: 実行中のOS／CPU向けにサーバーとUIなしリモコンを生成する。Windows amd64、macOS amd64／arm64、Linux amd64／arm64の全成果物はGitHub Actionsが各OSのネイティブランナーで生成する。
- `task remote:build`: 現在のOS／CPU向けUIなしリモコンだけをCGO有効で生成する。
- `task dev:remote -- <引数>`: UIなしリモコンをソースから実行する。
- Go検証の対象は `. ./cmd/... ./internal/... ./web`。`node_modules`内の他社Goコードは対象外。
- 本番実行時はGo、Node.js、Task、DBを必要としない。
- 生成物は`dist/`へ出力し、Gitには追加しない。
- APIはJSONのみを扱うため、Goの実行・検証・配布ビルドにはGin公式の`nomsgpack`ビルドタグを付ける。未使用のMsgPack実装をリンクせず、機能を変えずにバイナリを小さくする。
- DockerイメージはNodeとGoのビルドステージから、単一バイナリだけを`/timeview`へコピーした`scratch`イメージを作る。実行時の設定と操作ログはイメージへ含めず、`/data`ボリュームへ保存する。

## GitHub Actionsとリリース

`.github/workflows/ci.yml`はpushとpull requestでフロントエンドのlint・テスト・ビルド、Goのvet・テスト・カバレッジ確認、Dockerビルドに加え、Windows amd64、macOS amd64／arm64、Linux amd64／arm64のネイティブビルドを行う。

リリースはRelease Pleaseがv0.1.0を起点に管理する。mainへ入れる変更にはConventional Commit形式のタイトルを付け、squash mergeする。`fix: ...`はpatch、`feat: ...`は1.0未満ではminor更新になる。

1. mainへの変更からRelease workflowがバージョン・CHANGELOGを更新するリリースPRを作成する。
2. リリースPRを検証してマージする。`GITHUB_TOKEN`で作ったPRのCIが自動開始しない場合は、ActionsのCIからRun workflowを選び、リリースPRのブランチを指定する。ローカルでは`task release:check`を使える。
3. Release PleaseがタグとGitHub Releaseを作成し、同じworkflowから既存CIを呼び出す。対象タグでテスト・Dockerビルド・5種類のネイティブビルドを行い、各アーカイブと`SHA256SUMS.txt`を既存Releaseへ添付する。

`GITHUB_TOKEN`が作ったタグでは別workflowが自動起動しないため、`workflow_call`でCIを再利用する。追加secretやPATは不要。Settings → Actions → GeneralでGitHub ActionsによるPR作成を許可する。手動のタグ作成・pushでリリースしない。

## API利用例

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
# 演台一覧と操作ログも同じ接続で受け取る
curl -N 'http://127.0.0.1:8080/api/v1/timer/events?topics=displays,logs'
```

設定変更はGET応答のETagを`If-Match`で送る。加減算は`Idempotency-Key`が必須で、再送時は同じキーと本文を使う。任意の`X-Timeview-Instance`へ取得済みの`instanceId`を付けると、サーバー再起動前の操作を409で拒否できる。完全な契約は[OpenAPI](openapi.json)を参照。

ブラウザ操作限定が有効な間、外部クライアントの変更要求は403になる。解除はTimeViewの設定画面から行う。操作画面が使えない場合はTimeViewを終了し、設定JSONの`timer.browserOnly`を`false`へ変更して再起動する。

## 関連資料

- [仕様書](SPECIFICATION.md)
- [HTTP API・SSE仕様](specification/API.md)
- [運用ガイド](OPERATIONS.md)
- [動作確認結果](VALIDATION.md)
- [依存ライブラリの選定](DEPENDENCIES.md)
