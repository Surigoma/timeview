# 依存ライブラリの選定

2026年9月時点で公式ドキュメントとリリース情報を確認し、Go 1.27、CGO無効、Windows・macOS・Linuxという条件で選定した。

| 用途 | 採用 | 理由 |
| --- | --- | --- |
| HTTP | Gin v1.12 | ルート、ミドルウェア、JSON応答・バインド、静的配信を一つの小さなAPIで扱える |
| 設定 | Koanf v2 | providerとparserを必要なものだけ組み合わせられ、JSONと構造体の相互変換を分離できる。Viperのリモート設定や監視等は今回不要 |
| 検証 | go-playground/validator v10 | 構造体タグで範囲、列挙、件数、色形式をまとめて宣言でき、Ginの依存とも共通化できる |
| 流量制限 | golang.org/x/time/rate v0.16 | Go公式の拡張パッケージにある並行安全なトークンバケット実装 |
| システムログ | tint v1.1.3 | 標準 `log/slog` 用の小さなカラー対応ハンドラー。ログ形式とレベル判定を一か所へ集約できる |

React側は固定3画面、単一タイマー、SSE接続一つという規模のため、React Routerや汎用状態管理ライブラリを追加しても既存コードは短くならない。Reactの状態とブラウザ標準のEventSourceを継続使用する。タイマーの楽観的同時実行制御やIdempotency-Keyは製品固有なので、汎用HTTPクライアントへ隠さず`useTimer`に集約する。

UIのエラーメッセージはi18nextの同梱リソースで管理する。現時点では日本語だけを同梱し、ブラウザや通信ライブラリが返す英語エラーを画面へ直接表示しない。動的な翻訳取得や言語検出は行わない。

設定ファイルの置換処理はOS間で挙動を揃える必要がある。検討したrenameioはWindows非対応のため採用せず、同一ディレクトリへの一時書込、同期、renameを短い専用処理として残した。

## 参照資料

- [Gin releases](https://github.com/gin-gonic/gin/releases)
- [Koanf](https://github.com/knadh/koanf)
- [go-playground/validator](https://github.com/go-playground/validator)
- [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate)
- [tint](https://github.com/lmittmann/tint)
- [google/renameio](https://github.com/google/renameio)
