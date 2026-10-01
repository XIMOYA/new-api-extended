<div align="center">

![new-api](/web/public/logo.png)

# New API

🍥 **次世代大規模モデルゲートウェイとAI資産管理システム**

<p align="center">
  <a href="./README.zh_CN.md">简体中文</a> |
  <a href="./README.zh_TW.md">繁體中文</a> |
  <a href="./README.md">English</a> |
  <a href="./README.fr.md">Français</a> |
  <strong>日本語</strong>
</p>

<p align="center">
  <a href="https://raw.githubusercontent.com/XIMOYA/new-api-extended/main/LICENSE">
    <img src="https://img.shields.io/github/license/XIMOYA/new-api-extended?color=brightgreen" alt="license">
  </a><!--
  --><a href="https://github.com/XIMOYA/new-api-extended/releases/latest">
    <img src="https://img.shields.io/github/v/release/XIMOYA/new-api-extended?color=brightgreen&include_prereleases" alt="release">
  </a><!--
  --><a href="https://hub.docker.com/r/CalciumIon/new-api">
    <img src="https://img.shields.io/badge/docker-dockerHub-blue" alt="docker">
  </a>
  <a href="https://atomgit.com/QuantumNous/new-api" target="_blank">
    <img alt="AtomGit G-Star" src="https://atomgit.com/QuantumNous/new-api/star/badge.svg"/>
  </a>
</p>

<p align="center">
  <a href="https://trendshift.io/repositories/20180" target="_blank">
    <img src="https://trendshift.io/api/badge/repositories/20180" alt="QuantumNous%2Fnew-api | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/>
  </a>
  <br>
  <a href="https://hellogithub.com/repository/QuantumNous/new-api" target="_blank">
    <img src="https://api.hellogithub.com/v1/widgets/recommend.svg?rid=539ac4217e69431684ad4a0bab768811&claim_uid=tbFPfKIDHpc4TzR" alt="Featured｜HelloGitHub" style="width: 250px; height: 54px;" width="250" height="54" />
  </a><!--
  -->
  <a href="https://atomgit.com/QuantumNous/new-api" target="_blank">
    <img alt="AtomGit G-Star" src="https://atomgit.com/QuantumNous/new-api/star/new_badge.svg" width="250" height="55" />
  </a>
</p>

<p align="center">
  <a href="#-このフォークについて">フォークについて</a> •
  <a href="#-このフォークの変更点">変更点</a> •
  <a href="#-クイックスタート">クイックスタート</a> •
  <a href="#-主な機能">主な機能</a> •
  <a href="#-デプロイ">デプロイ</a> •
  <a href="#-ドキュメント">ドキュメント</a> •
  <a href="#-ヘルプサポート">ヘルプ</a>
</p>

</div>

## 📝 プロジェクト説明

> [!IMPORTANT]
> - 本プロジェクトは、合法的に許可された AI API ゲートウェイ、組織レベルの認証、マルチモデル管理、利用量分析、コスト管理、プライベートデプロイのシナリオのみを対象としています。
> - ユーザーは、上流の API キー、アカウント、モデルサービス、インターフェース権限を合法的に取得し、上流のサービス利用規約および適用される法律法規を遵守する必要があります。
> - ユーザーは、利用方法が上流のサービス利用規約および適用される法律法規に準拠していることを確認してください。
> - 生成 AI サービスを公衆に提供する場合、ユーザーは適用される規制要件を遵守し、管轄区域で求められる届出、ライセンス、コンテンツセキュリティ、本人確認、ログ保持、税務、上流認可などのすべての義務を履行してください。

---

## 🔀 このフォークについて

このリポジトリは、[New API](https://github.com/QuantumNous/new-api) を独立して改修した **New API Extended** です。[XIMOYA/new-api-extended](https://github.com/XIMOYA/new-api-extended) で個別に保守されています。New API の公式リリースではなく、上流プロジェクトのサポートを意味するものでもありません。

元の New API が提供する大規模モデルゲートウェイ、マルチモデルリレー、課金、認証、デプロイ、データ互換性の機能は引き続き利用できます。このフォークは独自の開発サイクルで保守されるため、動作、設定、リリース時期が上流版と異なる場合があります。

## ✨ このフォークの変更点

### 📊 NarraFork クォータとイベントシステム

- クォータ残高イベントに、クォータ、トークン、課金元、レイテンシ、キャッシュヒットの詳細を追加。
- キャッシュヒット率のスコープ、キャッシュヒットトークンの統計、関連するダッシュボード表示に対応。
- グローバルポリシー、ユーザー単位・チャネル単位の表示設定、ユーザー上書き設定に対応。
- NarraFork のシステム設定を、検証済みの単一バッチリクエストで保存。

### 🧾 リクエストコンテンツ監査

- JSON、raw body、multipart、マルチモーダルリソースの構造化キャプチャに対応。
- Zstandard 圧縮と AES-GCM 暗号化による保存、完全性検証、保持期間に基づくクリーンアップに対応。
- 権限に応じたマスキング、完全コンテンツのストリーミング読み取り、プレビュー、リソース表示、重複排除に対応。
- 権限のないユーザーに監査データを公開せず、構造化されたリクエスト記録を確認できる画面を追加。

### 📈 ダッシュボードと利用状況の可観測性

- キャッシュヒット率カードと、キャッシュ/トークン利用量の詳細表示を追加。
- 利用ログで推論の強度と推論トークンを確認可能。
- リクエスト記録、クォータ、運用状態の診断情報を拡充。

### 🛡️ 安定性とエンジニアリング

- リクエスト記録の重要なレート制限バケットを分離する独自修正を追加。
- フォーク固有のバックエンド・フロントエンド機能について、回帰テストと開発ドキュメントを追加。

上流プロジェクトは元の機能の参照先です。New API との互換性を確認する際は、このリポジトリの変更履歴とソースコードを確認してください。

---

## 🤝 信頼できるパートナー

<p align="center">
  <em>順不同</em>
</p>

<p align="center">
  <a href="https://www.cherry-ai.com/" target="_blank">
    <img src="./docs/images/cherry-studio.png" alt="Cherry Studio" height="80" />
  </a><!--
  --><a href="https://github.com/iOfficeAI/AionUi/" target="_blank">
    <img src="./docs/images/aionui.png" alt="Aion UI" height="80" />
  </a><!--
  --><a href="https://bda.pku.edu.cn/" target="_blank">
    <img src="./docs/images/pku.png" alt="北京大学" height="80" />
  </a><!--
  --><a href="https://www.compshare.cn/?ytag=GPU_yy_gh_newapi" target="_blank">
    <img src="./docs/images/ucloud.png" alt="UCloud 優刻得" height="80" />
  </a><!--
  --><a href="https://www.aliyun.com/" target="_blank">
    <img src="./docs/images/aliyun.png" alt="Alibaba Cloud" height="80" />
  </a><!--
  --><a href="https://io.net/" target="_blank">
    <img src="./docs/images/io-net.png" alt="IO.NET" height="80" />
  </a>
</p>

---

## 🙏 特別な感謝

<p align="center">
  <a href="https://www.jetbrains.com/?from=new-api" target="_blank">
    <img src="https://resources.jetbrains.com/storage/products/company/brand/logos/jb_beam.png" alt="JetBrains Logo" width="120" />
  </a>
</p>

<p align="center">
  <strong>感謝 <a href="https://www.jetbrains.com/?from=new-api">JetBrains</a> が本プロジェクトに無料のオープンソース開発ライセンスを提供してくれたことに感謝します</strong>
</p>

---

## 🚀 クイックスタート

### Docker Composeを使用（推奨）

```bash
# プロジェクトをクローン
git clone https://github.com/XIMOYA/new-api-extended.git
cd new-api-extended

# docker-compose.yml 設定を編集
nano docker-compose.yml

# サービスを起動
docker-compose up -d
```

<details>
<summary><strong>Dockerコマンドを使用</strong></summary>

```bash
# 最新のイメージをプル
docker pull calciumion/new-api:latest

# SQLiteを使用（デフォルト）
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  calciumion/new-api:latest

# MySQLを使用
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e SQL_DSN="root:123456@tcp(localhost:3306)/oneapi" \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  calciumion/new-api:latest
```

> **💡 ヒント:** `-v ./data:/data` は現在のディレクトリの `data` フォルダにデータを保存します。絶対パスに変更することもできます：`-v /your/custom/path:/data`

</details>

---

🎉 デプロイが完了したら、`http://localhost:3000` にアクセスして使用を開始してください！

> [!WARNING]
> 本プロジェクトを公衆向け生成 AI サービスまたは API 再販サービスとして運営する場合、ユーザーは届出、コンテンツセキュリティ、本人確認、ログ保持、税務、決済、上流認可などの必要なコンプライアンス義務を先に完了してください。

📖 その他のデプロイ方法については[デプロイガイド](https://docs.newapi.pro/ja/docs/installation)を参照してください。

---

## 🛠️ ソースからのビルド

このフォークは上流プロジェクトと同じビルドフローを使用します。まずフロントエンドをビルドし、その後リポジトリのルートから Go バックエンドをビルドしてください。

```bash
cd web
bun install --frozen-lockfile
bun run build
cd ..
go build -ldflags "-s -w -X github.com/QuantumNous/new-api/common.Version=$(tr -d '\r\n' < VERSION)" -o new-api
```

### Windows AMD64 テストビルド

Windows の Git Bash で次のコマンドを実行してください。

```bash
cd web
bun install --frozen-lockfile
VERSION=$(tr -d '\r\n' < ../VERSION)
DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION="$VERSION" bun run build
cd ..
go build -ldflags "-s -w -X github.com/QuantumNous/new-api/common.Version=$VERSION" -o new-api-windows-amd64-test.exe
```

ビルド済みの実行ファイルを再配布する場合は、対応するソースコード、`LICENSE`、`NOTICE`、`THIRD-PARTY-LICENSES.md` を保持してください。

---

## 📚 ドキュメント

<div align="center">

### 📖 [公式ドキュメント](https://docs.newapi.pro/ja/docs) | [![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/QuantumNous/new-api)

</div>

**クイックナビゲーション:**

| カテゴリ | リンク |
|------|------|
| 🚀 デプロイガイド | [インストールドキュメント](https://docs.newapi.pro/ja/docs/installation) |
| ⚙️ 環境設定 | [環境変数](https://docs.newapi.pro/ja/docs/installation/config-maintenance/environment-variables) |
| 📡 APIドキュメント | [APIドキュメント](https://docs.newapi.pro/ja/docs/api) |
| ❓ よくある質問 | [FAQ](https://docs.newapi.pro/ja/docs/support/faq) |
| 💬 コミュニティ交流 | [交流チャネル](https://docs.newapi.pro/ja/docs/support/community-interaction) |

---

## ✨ 主な機能

> 詳細な機能については[機能説明](https://docs.newapi.pro/ja/docs/guide/wiki/basic-concepts/features-introduction)を参照してください。

### 🎨 コア機能

| 機能 | 説明 |
|------|------|
| 🎨 新しいUI | モダンなユーザーインターフェースデザイン |
| 🌍 多言語 | 簡体字中国語、繁体字中国語、英語、フランス語、日本語をサポート |
| 🔄 データ互換性 | オリジナルのOne APIデータベースと完全に互換性あり |
| 📈 データダッシュボード | ビジュアルコンソールと統計分析 |
| 🔒 権限管理 | トークングループ化、モデル制限、ユーザー管理 |

### 💰 認可済み利用量とコスト管理

- ✅ 合法的に許可されたシナリオでの内部チャージとクォータ割り当て（EPay、Stripe）
- ✅ 組織レベルのリクエスト単位、使用量ベース、キャッシュヒットのコスト会計
- ✅ OpenAI、Azure、DeepSeek、Claude、Qwen などのモデルのキャッシュ課金統計
- ✅ 内部管理または認可済み企業顧客向けの柔軟な課金ポリシー

### 🔐 認証とセキュリティ

- 😈 Discord認証ログイン
- 🤖 LinuxDO認証ログイン
- 📱 Telegram認証ログイン
- 🔑 OIDC統一認証
- 🔍 Key使用量クォータ照会（[new-api-key-tool](https://github.com/Calcium-Ion/new-api-key-tool)と併用）



### 🚀 高度な機能

**APIフォーマットサポート:**
- ⚡ [OpenAI Responses](https://docs.newapi.pro/ja/docs/api/ai-model/chat/openai/create-response)
- ⚡ [OpenAI Realtime API](https://docs.newapi.pro/ja/docs/api/ai-model/realtime/create-realtime-session)（Azureを含む）
- ⚡ [Claude Messages](https://docs.newapi.pro/ja/docs/api/ai-model/chat/create-message)
- ⚡ [Google Gemini](https://doc.newapi.pro/ja/api/google-gemini-chat)
- 🔄 [Rerankモデル](https://docs.newapi.pro/ja/docs/api/ai-model/rerank/create-rerank)（Cohere、Jina）

**インテリジェントルーティング:**
- ⚖️ チャネル重み付けランダム
- 🔄 失敗自動リトライ
- 🚦 ユーザーレベルモデルレート制限

**フォーマット変換:**
- 🔄 **OpenAI Compatible ⇄ Claude Messages**
- 🔄 **OpenAI Compatible → Google Gemini**
- 🔄 **Google Gemini → OpenAI Compatible** - テキストのみ、関数呼び出しはまだサポートされていません
- 🚧 **OpenAI Compatible ⇄ OpenAI Responses** - 開発中
- 🔄 **思考からコンテンツへの機能**

**Reasoning Effort サポート:**

<details>
<summary>詳細設定を表示</summary>

**OpenAIシリーズモデル:**
- `o3-mini-high` - 高思考努力
- `o3-mini-medium` - 中思考努力
- `o3-mini-low` - 低思考努力
- `gpt-5-high` - 高思考努力
- `gpt-5-medium` - 中思考努力
- `gpt-5-low` - 低思考努力

**Claude思考モデル:**
- `claude-3-7-sonnet-20250219-thinking` - 思考モードを有効にする

**Google Geminiシリーズモデル:**
- `gemini-2.5-flash-thinking` - 思考モードを有効にする
- `gemini-2.5-flash-nothinking` - 思考モードを無効にする
- `gemini-2.5-pro-thinking` - 思考モードを有効にする
- `gemini-2.5-pro-thinking-128` - 思考モードを有効にし、思考予算を128トークンに設定する
- Gemini モデル名の末尾に `-low` / `-medium` / `-high` を付けることで推論強度を直接指定できます（追加の思考予算サフィックスは不要です）。

</details>

---

## 🤖 モデルサポート

> 詳細については[APIドキュメント - ゲートウェイインターフェース](https://docs.newapi.pro/ja/docs/api)

| モデルタイプ | 説明 | ドキュメント |
|---------|------|------|
| 🤖 OpenAI-Compatible | OpenAI互換モデル | [ドキュメント](https://docs.newapi.pro/ja/docs/api/ai-model/chat/openai/createchatcompletion) |
| 🤖 OpenAI Responses | OpenAI Responsesフォーマット | [ドキュメント](https://docs.newapi.pro/ja/docs/api/ai-model/chat/openai/createresponse) |
| 🎨 Midjourney-Proxy | [Midjourney-Proxy(Plus)](https://github.com/novicezk/midjourney-proxy) | [ドキュメント](https://doc.newapi.pro/api/midjourney-proxy-image) |
| 🎵 Suno-API | [Suno API](https://github.com/Suno-API/Suno-API) | [ドキュメント](https://doc.newapi.pro/api/suno-music) |
| 🔄 Rerank | Cohere、Jina | [ドキュメント](https://docs.newapi.pro/ja/docs/api/ai-model/rerank/creatererank) |
| 💬 Claude | Messagesフォーマット | [ドキュメント](https://docs.newapi.pro/ja/docs/api/ai-model/chat/createmessage) |
| 🌐 Gemini | Google Geminiフォーマット | [ドキュメント](https://docs.newapi.pro/ja/docs/api/ai-model/chat/gemini/geminirelayv1beta) |
| 🔧 Dify | ChatFlowモード | - |
| 🎯 カスタム上流 | 合法的に許可された上流エンドポイントの設定をサポート | - |

### 📡 サポートされているインターフェース

<details>
<summary>完全なインターフェースリストを表示</summary>

- [チャットインターフェース (Chat Completions)](https://docs.newapi.pro/ja/docs/api/ai-model/chat/openai/createchatcompletion)
- [レスポンスインターフェース (Responses)](https://docs.newapi.pro/ja/docs/api/ai-model/chat/openai/createresponse)
- [イメージインターフェース (Image)](https://docs.newapi.pro/ja/docs/api/ai-model/images/openai/post-v1-images-generations)
- [オーディオインターフェース (Audio)](https://docs.newapi.pro/ja/docs/api/ai-model/audio/openai/create-transcription)
- [ビデオインターフェース (Video)](https://docs.newapi.pro/ja/docs/api/ai-model/audio/openai/createspeech)
- [エンベッドインターフェース (Embeddings)](https://docs.newapi.pro/ja/docs/api/ai-model/embeddings/createembedding)
- [再ランク付けインターフェース (Rerank)](https://docs.newapi.pro/ja/docs/api/ai-model/rerank/creatererank)
- [リアルタイム対話インターフェース (Realtime)](https://docs.newapi.pro/ja/docs/api/ai-model/realtime/createrealtimesession)
- [Claudeチャット](https://docs.newapi.pro/ja/docs/api/ai-model/chat/createmessage)
- [Google Geminiチャット](https://docs.newapi.pro/ja/docs/api/ai-model/chat/gemini/geminirelayv1beta)

</details>

---

## 🚢 デプロイ

> [!TIP]
> **上流の最新Dockerイメージ:** `calciumion/new-api:latest`
>
> このフォークは現在、独自のDockerイメージを公開していません。上流イメージは参考用であり、`XIMOYA/new-api-extended` の変更が自動的に含まれるわけではありません。このフォークの機能を利用する場合は、ソースからビルドしてください。

### 📋 デプロイ要件

| コンポーネント | 要件 |
|------|------|
| **ローカルデータベース** | SQLite（Dockerは `/data` ディレクトリをマウントする必要があります）|
| **リモートデータベース** | MySQL ≥ 5.7.8 または PostgreSQL ≥ 9.6 |
| **コンテナエンジン** | Docker / Docker Compose |
| **システムアーキテクチャ** | 64ビットのみ対応（amd64 / arm64）。32ビットシステムは非対応 |

### ⚙️ 環境変数設定

<details>
<summary>一般的な環境変数設定</summary>

| 変数名 | 説明 | デフォルト値 |
|--------|------|--------|
| `SESSION_SECRET` | 認証署名シークレット。すべてのノードで同じ値が必要 | - |
| `SESSION_COOKIE_SECURE` | `false`/未設定ではローカル HTTP 開発プロキシ向けに refresh/logout の OriginGuard を無効化し、`true` では Secure Cookie と厳格な Origin 検証を有効化 | `false` |
| `SESSION_COOKIE_TRUSTED_URL` | Secure モードでは必須。refresh/logout を許可する完全一致の HTTPS Origin をカンマ区切りで指定。relay CORS 設定ではありません | - |
| `TRUSTED_PROXIES` | 未設定/空ではループバック、RFC 1918、IPv6 ULA を信頼して起動時に警告し、`none` ではすべて無効、明示的なプロキシ IP/CIDR リストは既定値を完全に置き換えます | `127.0.0.0/8, ::1, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7` |
| `USER_SESSION_ACTIVE_LIMIT` | 1 ユーザーあたりの有効なログイン Session 上限 | `50` |
| `USER_SESSION_ISSUANCE_LIMIT` | カウント期間内に作成できる Session 数の上限（取り消し済みを含む） | `100` |
| `USER_SESSION_ISSUANCE_WINDOW_SECONDS` | Session 発行のカウント期間（秒）。取り消し済み Session の保持期間を超える場合は自動的に制限 | `86400` |
| `USER_SESSION_REVOKED_RETENTION_DAYS` | 監査と発行数計算のため取り消し済み Session を保持する日数 | `7` |
| `USER_SESSION_HOURLY_ALERT_THRESHOLD` | 1 時間あたりのグローバル Session 発行数の警告閾値。ログインは拒否しません | `5000` |
| `CRYPTO_SECRET` | キャッシュキー用 HMAC シークレット。Redis を共有するノードでは同じ実効値が必要 | デフォルトは `SESSION_SECRET` |
| `SQL_DSN** | データベース接続文字列 | - |
| `REDIS_CONN_STRING` | Redis接続文字列 | - |
| `STREAMING_TIMEOUT` | ストリーミング応答のタイムアウト時間（秒） | `300` |
| `STREAM_SCANNER_MAX_BUFFER_MB` | ストリームスキャナの1行あたりバッファ上限（MB）。4K画像など巨大なbase64 `data:` ペイロードを扱う場合は値を増加させてください | `64` |
| `MAX_REQUEST_BODY_MB` | リクエストボディ最大サイズ（MB、**解凍後**に計測。巨大リクエスト/zip bomb によるメモリ枯渇を防止）。超過時は `413` | `32` |
| `AZURE_DEFAULT_API_VERSION` | Azure APIバージョン | `2025-04-01-preview` |
| `ERROR_LOG_ENABLED` | エラーログスイッチ | `false` |
| `PYROSCOPE_URL` | Pyroscopeサーバーのアドレス | - |
| `PYROSCOPE_APP_NAME` | Pyroscopeアプリ名 | `new-api` |
| `PYROSCOPE_BASIC_AUTH_USER` | Pyroscope Basic Authユーザー | - |
| `PYROSCOPE_BASIC_AUTH_PASSWORD` | Pyroscope Basic Authパスワード | - |
| `PYROSCOPE_MUTEX_RATE` | Pyroscope mutexサンプリング率 | `5` |
| `PYROSCOPE_BLOCK_RATE` | Pyroscope blockサンプリング率 | `5` |
| `HOSTNAME` | Pyroscope用のホスト名タグ | `new-api` |

📖 **完全な設定:** [環境変数ドキュメント](https://docs.newapi.pro/ja/docs/installation/config-maintenance/environment-variables)

</details>

### 🔧 デプロイ方法

<details>
<summary><strong>方法 1: Docker Compose（推奨）</strong></summary>

```bash
# プロジェクトをクローン
git clone https://github.com/XIMOYA/new-api-extended.git
cd new-api-extended

# 設定を編集
nano docker-compose.yml

# サービスを起動
docker-compose up -d
```

</details>

<details>
<summary><strong>方法 2: Dockerコマンド</strong></summary>

**SQLiteを使用:**
```bash
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  calciumion/new-api:latest
```

**MySQLを使用:**
```bash
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e SQL_DSN="root:123456@tcp(localhost:3306)/oneapi" \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  calciumion/new-api:latest
```

> **💡 パス説明:**
> - `./data:/data` - 相対パス、データは現在のディレクトリのdataフォルダに保存されます
> - 絶対パスを使用することもできます：`/your/custom/path:/data`

</details>

<details>
<summary><strong>方法 3: 宝塔パネル</strong></summary>

1. 宝塔パネル（**9.2.0バージョン**以上）をインストールし、アプリケーションストアで**New-API**を検索してインストールします。

📖 [画像付きチュートリアル](./docs/BT.md)

</details>

### ⚠️ マルチマシンデプロイの注意事項

> [!WARNING]
> - すべてのノードで同じプライマリデータベースと同じ `SESSION_SECRET` を使用してください。異なる場合、Access Token、Refresh セッション、一時認証フローを一貫して検証できません。
> - 同じ Redis に接続するノードでは同じ `CRYPTO_SECRET` も設定してください。異なる場合、キャッシュキーのダイジェストが一致せず、共有エントリを正しく再利用できません。

ログイン Session とユーザー単位の有効数／発行数制限では、データベースが信頼できる唯一の情報源です。Redis の Session エントリは短期キャッシュであり、TTL は `SYNC_FREQUENCY`（デフォルト 60 秒）に従い、Session の残り有効期間を超えません。

| Redis トポロジー | Session 状態の伝播 | レート制限 |
| --- | --- | --- |
| すべてのノードで Redis を共有 | 取り消しとバージョン更新は通常即時に伝播 | Redis の制限枠はノード間で共有 |
| ノードごとに独立した Redis | 有効な `SYNC_FREQUENCY` 以内にデータベースへフォールバックして収束。バージョンローテーション直後の新しい Token は、古いキャッシュを持つノードで一時的に 401 になる場合があります | ノードごとに独立して計数するため、クラスター全体では設定値の約ノード数倍まで許可される可能性があります |
| Redis なし | Session の検証ごとにデータベースを直接参照 | メモリ内の制限枠はノードごとに独立 |

`SYNC_FREQUENCY` を短くすると独立 Redis のキャッシュ陳腐化時間は短くなりますが、有効な SID ごと、ノードごと、TTL ごとにデータベースへの主キー照会が 1 回増えます。この保証は Session 認証の陳腐化時間を限定するものです。レート制限や Redis を使うその他のコントロールプレーンキャッシュは、引き続きトポロジーに依存します。

Token、Origin 検証、PAT の契約については[ユーザー認証とログインセッション](./docs/authentication.md)を参照してください。

### 🔄 チャネルリトライとキャッシュ

**リトライ設定:** `設定 → 運営設定 → 一般設定 → 失敗リトライ回数`

**キャッシュ設定:**
- `REDIS_CONN_STRING`：Redisキャッシュ（推奨）
- `MEMORY_CACHE_ENABLED`：メモリキャッシュ

---

## 🔗 関連プロジェクト

### 上流プロジェクト

| プロジェクト | 説明 |
|------|------|
| [One API](https://github.com/songquanpeng/one-api) | オリジナルプロジェクトベース |
| [Midjourney-Proxy](https://github.com/novicezk/midjourney-proxy) | Midjourneyインターフェースサポート |

### 補助ツール

| プロジェクト | 説明 |
|------|------|
| [new-api-key-tool](https://github.com/Calcium-Ion/new-api-key-tool) | キー使用量クォータ照会ツール |
| [new-api-horizon](https://github.com/Calcium-Ion/new-api-horizon) | New API高性能最適化版 |

---

## 💬 ヘルプサポート

### 📖 ドキュメントリソース

| リソース | リンク |
|------|------|
| 📘 よくある質問 | [FAQ](https://docs.newapi.pro/ja/docs/support/faq) |
| 💬 コミュニティ交流 | [交流チャネル](https://docs.newapi.pro/ja/docs/support/community-interaction) |
| 🐛 問題のフィードバック | [問題フィードバック](https://docs.newapi.pro/ja/docs/support/feedback-issues) |
| 📚 完全なドキュメント | [公式ドキュメント](https://docs.newapi.pro/ja/docs) |

### 🤝 貢献ガイド

あらゆる形の貢献を歓迎します！

- 🐛 バグを報告する
- 💡 新しい機能を提案する
- 📝 ドキュメントを改善する
- 🔧 コードを提出する

---

## 📜 ライセンス

このプロジェクトは [GNU Affero General Public License v3.0 (AGPLv3)](./LICENSE) の下でライセンスされています。このリポジトリは New API の独立した改変版であり、もともとは MIT ライセンスの [One API](https://github.com/songquanpeng/one-api) をベースに開発されました。

AGPLv3 第 7 条の追加条件が適用されます。改変版では、次の作者表示を保持してください。

```text
Frontend design and development by New API contributors.
```

ユーザーインターフェースを提供する改変版では、元のプロジェクトへの見えるリンクも保持してください：<https://github.com/QuantumNous/new-api>。

Windows 実行ファイル、Docker イメージ、その他のバイナリ形式を配布する場合は、そのバージョンに対応するソースコードとビルド手順を提供してください。該当する場合は、配布物に `LICENSE`、`NOTICE`、`THIRD-PARTY-LICENSES.md` を含めてください。

改変版を公開ネットワークサービスとして運用する場合は、実行中のバージョンに対応するソースコードの取得方法をリモートユーザーに明確に示してください。改変版であることを明示し、New API 公式版であるかのように表示しないでください。

お客様の組織のポリシーがAGPLv3ライセンスのソフトウェアの使用を許可していない場合、またはAGPLv3のオープンソース義務を回避したい場合は、こちらまでお問い合わせください：[support@quantumnous.com](mailto:support@quantumnous.com)

---

## 🌟 スター履歴

<div align="center">

[![スター履歴チャート](https://api.star-history.com/svg?repos=Calcium-Ion/new-api&type=Date)](https://star-history.com/#Calcium-Ion/new-api&Date)

</div>

---

<div align="center">

### 💖 New APIをご利用いただきありがとうございます

このプロジェクトがあなたのお役に立てたなら、ぜひ ⭐️ スターをください！

**[公式ドキュメント](https://docs.newapi.pro/ja/docs)** • **[フォークの問題報告](https://github.com/XIMOYA/new-api-extended/issues)** • **[フォークのリリース](https://github.com/XIMOYA/new-api-extended/releases)**

<sub>❤️ で構築された QuantumNous</sub>

</div>
