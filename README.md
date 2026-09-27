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
# 操作ログの保存先を指定する場合
.\dist\windows-amd64\timeview.exe -audit-log C:\TimeView\operations.jsonl
```

macOS / Linuxでは対象CPUの `dist/darwin-arm64/timeview` 等を実行します。

```sh
chmod +x ./timeview
./timeview -listen 0.0.0.0:8080
```

### Docker

Docker Composeを使う場合は次のコマンドだけで起動できます。

```sh
docker compose up --build -d
```

操作画面は `http://127.0.0.1:8080/` です。実行イメージは単一バイナリだけを含む `scratch` ベースで、非rootユーザーとして動作します。設定ファイルと操作ログは `/data` に保存され、Composeの名前付きボリューム `timeview-data` へ永続化されます。`docker compose down` ではデータを残し、`docker compose down -v` を実行するとボリュームも削除します。

別端末からは `http://<サーバーPCのLAN側IP>:8080/`、タッチパネルは `/touch`、演台は `/display` へアクセスします。`0.0.0.0` は待受指定なので、ブラウザには実際のIPを入力してください。OSのファイアウォールで会場LANから指定ポートへの接続を許可してください。

初回起動時は10分・待機・**暗転ON**です。設定を確認し、操作画面で「暗転解除」を押してください。演台は警告段階に応じて背景色が変わり、画面下部のゲージで通常・第1警告・第2警告の領域、経過率、次の警告までの時間を示します。上映時は演台を全画面にし、実際の表示を確認してから暗転します。暗転中も計測を継続します。

## 設定ファイル

初回起動時にカレントディレクトリへ `timeview-config.json` を作成します。`-config`で別のパスを指定できます。指定先の親フォルダーは先に作成してください。持ち時間、警告、配色、表示モード、表示言語、システムログレベル、点滅、定型文、Keypad割り当てを画面から変更すると、UTF-8のJSONへ自動保存します。表示言語は設定画面で日本語または英語へ切り替えます。

JSONを手作業で編集する場合はTimeViewを終了してから変更し、再起動してください。不明な項目、値の不整合、未対応の設定バージョンがある場合は、安全のため起動を中止してエラーを表示します。再起動後も設定は復元しますが、残り時間、計測状態、暗転解除、送信中のカンペは復元せず、待機・暗転ONから始まります。

設定画面の「ブラウザからの操作だけを許可」を有効にすると、外部APIのGET・SSEは利用できますが、変更要求は403になります。操作画面と、その画面で有効にしたKeypadからは引き続き操作できます。この機能は分離LAN内の誤操作防止用で、認証機能ではありません。

## 操作ログ

起動、終了、異常終了、操作の成否を既定の`timeview-operations.jsonl`へJSON Lines形式で記録します。操作画面の「操作ログ」から最新500件を確認できます。カンペ本文は記録しません。保存先は`-audit-log`で変更できます。

コンソールへ出すシステムログは、HTTPアクセスを含めてカラー対応の構造化形式へ統一しています。設定画面で `DEBUG`、`INFO`、`WARN`、`ERROR` を選ぶと、その場で出力レベルが切り替わり、JSON設定へ保存されます。

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

## ドキュメント

- [開発ガイドとAPI利用例](docs/DEVELOPMENT.md)
- [仕様書](docs/SPECIFICATION.md)
- [OpenAPI](docs/openapi.json)
- [動作確認結果と未確認項目](docs/VALIDATION.md)
