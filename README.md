# CAMEO 백엔드

커플 전용 iOS 앱 CAMEO의 서버. 전화번호 OTP 로그인, 6자리 코드로 상대 연결, 1:1 음성 통화(서버 중계와 녹음), 통화 전사와 AI 제목/요약, 통화 중 하이라이트, 공유 사진 라이브러리, APNs/PushKit 푸시를 제공한다.

- API 명세: [docs/openapi.yaml](docs/openapi.yaml) (OpenAPI 3.1)
- 통화 시그널링, 상태 머신, 푸시 페이로드: [docs/signaling.md](docs/signaling.md)

## 스택

| 영역 | 사용 기술 |
|---|---|
| 언어/HTTP | Go 1.27, chi v5 + go-chi/cors + go-chi/httprate |
| DB | PostgreSQL 18, ent(포크) + pgx/v5, Atlas 마이그레이션(기동 시 자동 적용) |
| 인증 | Twilio Verify(미설정 시 fake OTP), DB 세션 + FWT 서명 bearer 토큰(90일 슬라이딩) |
| 통화 | Pion WebRTC v4를 프로세스 안에서 SFU-lite로 운용(Opus 중계, 참여자별 Ogg 녹음), coder/websocket 시그널링, 단일 UDP 포트 |
| 후처리 | ffmpeg(트랙 인코딩/믹스), S3 업로드, OpenAI 전사(whisper-1) + Responses API 제목/요약 |
| 저장소 | S3 호환 스토리지(minio-go), presigned PUT/GET |
| 푸시 | APNs 토큰 인증(sideshow/apns2), 미설정 시 로그 전용 |
| 결제 | RevenueCat(REST v1 + 웹훅), 미설정 시 모두 프리미엄 |

## 준비물

- Go 1.27
- Docker + docker compose (로컬 PostgreSQL과 S3, 마이그레이션 생성 시 dev DB)
- [Atlas](https://atlasgo.io) CLI (`make migration`에 필요. 서버 기동 시 마이그레이션은 atlas 바이너리를 캐시에서 찾고 없으면 내려받는다)
- golangci-lint v2 (`make fmt`, `make lint`)
- ffmpeg (통화 녹음 후처리)

## 빠른 시작

```sh
make up                                             # PostgreSQL(5432), S3(10900), 버킷 초기화
mkdir -p local/config
cp local/config.example.json local/config/config.json
make run                                            # http://localhost:18080, WebRTC UDP 50000
curl -i localhost:18080/readyz
```

- Twilio를 설정하지 않으면 fake OTP가 쓰인다. SMS는 보내지 않고 모든 번호에 대해 `000000`(`otp.fake_code`)을 받아 준다.
  ```sh
  curl -X POST localhost:18080/v1/auth/phone/start  -H 'Content-Type: application/json' -d '{"phone":"+821012345678"}'
  curl -X POST localhost:18080/v1/auth/phone/verify -H 'Content-Type: application/json' -d '{"phone":"+821012345678","code":"000000"}'
  ```
- 마이그레이션은 기동 시 `MIGRATIONS_DIR`에서 자동 적용된다.
- `make down`은 컨테이너를 내린다(볼륨은 유지).

## 설정

`${CONFIG_DIR:-/config}/config.json`을 읽는다. 필수 섹션이 없거나 값이 잘못되면 기동하지 않는다.

환경 변수:

| 변수 | 기본값 | 설명 |
|---|---|---|
| `CONFIG_DIR` | `/config` | `config.json`이 있는 디렉터리 (`make run`은 `./local/config`) |
| `LISTEN_ADDRESS` | `:80` | HTTP 주소 (`make run`은 `:18080`) |
| `MIGRATIONS_DIR` | `assets/migrations` | Atlas 마이그레이션 디렉터리 (`make run`은 `internal/ent/migrate/migrations`) |

`config.json` 키:

| 키 | 필수 | 기본값 | 효과 |
|---|---|---|---|
| `log_level` | 선택 | `info` | zerolog 레벨(`debug`, `info`, `warn`, ...). 잘못된 값도 `info` |
| `database.host` | 필수 | | PostgreSQL 호스트 |
| `database.port` | 필수 | | PostgreSQL 포트 |
| `database.user` / `database.password` | 선택 | | 접속 계정 |
| `database.database` | 선택 | | DB 이름 |
| `database.tls` | 선택 | `false` | `true`면 `sslmode=require`, 아니면 `disable` |
| `token.secret` | 필수 | | 토큰 서명 키(hex). ed25519는 32바이트(64 hex), ed448은 57바이트 |
| `token.algorithm` | 필수 | | `ed25519`, `ed448`, `hmac-sha256`, `hmac-sha512`, `blake2b-256`, `blake2b-512`, `blake3` |
| `service.cors_allowed_origins` | 선택 | `[]` | CORS 허용 Origin 목록 |
| `service.trusted_proxies` | 선택 | `0` | 앞단 리버스 프록시 수. 1 이상이면 `X-Forwarded-For`의 오른쪽에서 N번째 항목을 클라이언트 IP로 쓴다(rate limit 키). `0`이면 연결의 remote address. 프록시 없이 1 이상으로 두면 클라이언트가 IP를 위조할 수 있다 |
| `storage.endpoint` | 필수 | | 서버가 접속하는 S3 엔드포인트(`host:port`, 스킴 없음) |
| `storage.public_endpoint` | 선택 | `endpoint` | presigned URL에 들어가는 엔드포인트. 클라이언트가 도달할 수 있어야 함 |
| `storage.bucket` | 필수 | | 버킷. 없으면 기동 시 생성 |
| `storage.access_key` / `storage.secret_key` | 필수 | | S3 자격 증명 |
| `storage.region` | 선택 | `us-east-1` | 서명 리전 |
| `storage.insecure` | 선택 | `false` | `true`면 서버가 `endpoint`에 HTTP로 접속 |
| `storage.public_insecure` | 선택 | `false` (`public_endpoint`가 없으면 `insecure`) | `true`면 presigned URL이 `http://` |
| `webrtc.udp_port` | 필수 | | 모든 통화가 공유하는 WebRTC UDP 포트(1-65535, IPv4) |
| `webrtc.public_ips` | 선택 | `[]` | 서버 host ICE 후보를 이 IP로 바꿔 광고(NAT 뒤 서버). 운영에서는 공인 IP 필수 |
| `webrtc.ice_servers[]` | 선택 | `[]` | `POST /v1/calls` 응답으로 클라이언트에 전달. 각 항목 `urls`(필수), `username`, `credential` |
| `recording.dir` | 필수 | | 통화 녹음 세그먼트와 후처리 임시 파일 디렉터리 |
| `recording.ffmpeg_path` | 선택 | `ffmpeg` | ffmpeg 실행 파일 |
| `recording.concurrency` | 선택 | `2` | 동시에 도는 후처리 파이프라인 수(1 이상) |
| `twilio.account_sid` / `auth_token` / `verify_service_sid` | 섹션 선택, 있으면 모두 필수 | | 있으면 Twilio Verify로 SMS 발송 |
| `otp.fake_code` | 선택 | `000000` | Twilio가 없을 때 받아 주는 코드 |
| `otp.allow_fake` | 선택 | `false` | 로컬이 아닌 빌드(`VERSION != local`)에서 fake OTP를 허용. 없으면 Twilio 없이 기동 거부 |
| `openai.api_key` | 섹션 선택, 있으면 필수 | | 없으면 녹음만 업로드하고 전사는 `skipped` |
| `openai.transcription_model` | 선택 | `whisper-1` | 전사 모델 |
| `openai.summary_model` | 선택 | `gpt-5-mini` | 제목/요약 모델 |
| `openai.language` | 선택 | | 전사 언어 힌트(ISO-639-1, 예: `ko`) |
| `push.apns.key_path` / `key_id` / `team_id` / `bundle_id` | 섹션 선택, 있으면 모두 필수 | | APNs .p8 키 경로와 식별자. VoIP topic은 `<bundle_id>.voip`. 없으면 푸시를 로그로만 남김 |
| `push.apns.production` | 선택 | `false` | `true`면 운영 APNs, 아니면 sandbox |
| `revenuecat.api_key` | 섹션 선택, 있으면 필수 | | RevenueCat REST v1 `GET /v1/subscribers/{app_user_id}`에 Bearer로 쓰는 키(public SDK 키 또는 secret 키). 섹션이 없으면 결제 비활성: 모두 프리미엄, 복원 무료, sync는 no-op, 웹훅 미제공 |
| `revenuecat.webhook_secret` | 선택 | | RevenueCat 웹훅 서명 secret. 비어 있으면 `POST /v1/webhooks/revenuecat`을 제공하지 않음 |
| `revenuecat.premium_entitlement` | 선택 | `premium` | 프리미엄 entitlement 식별자 |
| `revenuecat.restore_product_id` | 선택 | `restore` | 데이터 복원 소모성 상품 ID |

## Make 타깃

| 타깃 | 동작 |
|---|---|
| `make up` / `make down` | 로컬 PostgreSQL, S3(versitygw), 버킷 초기화 컨테이너 기동/정지 |
| `make run` | 로컬 설정으로 서버 실행(`:18080`) |
| `make build` | `bin/cameo` 빌드 |
| `make generate` | ent 코드 생성 + DDL 스냅샷(`internal/ent/migrate/schema.sql`) 갱신 |
| `make migration NAME=<이름>` | 스냅샷과 `extra.sql`로부터 `<타임스탬프>_<이름>.sql` Atlas 마이그레이션 생성(Docker로 dev DB를 띄움) |
| `make fmt` / `make lint` | golangci-lint 포맷/린트 |
| `make prod` | 운영 호스트에서 루트 `compose.yml`로 빌드 후 기동(`VERSION`은 git short hash) |
| `make prod-logs` | 운영 `cameo` 컨테이너 로그 |

## 스키마 변경 워크플로

1. `internal/ent/schema/*.go`(enum은 `internal/ent/schema_types/`)를 수정한다.
2. `make generate`로 ent 코드와 `schema.sql` 스냅샷을 갱신하고 빌드를 확인한다. 스키마를 다듬는 동안에는 이 단계만 반복한다.
3. 스키마가 확정되면 `make migration NAME=<이름>`으로 `internal/ent/migrate/migrations/`에 SQL과 `atlas.sum`을 만든다. ent가 표현하지 못하는 DDL은 `internal/ent/migrate/extra.sql`에 둔다.
4. 생성된 마이그레이션을 커밋한다. 서버는 다음 기동 때 자동으로 적용한다.

## Docker

```sh
docker build --build-arg VERSION=0.1.0 -t cameo .
```

- `local/compose.yml`의 `cameo` 서비스는 같은 방식으로 빌드하고 `local/config`를 `/config`에, 녹음 볼륨을 `/data/recordings`에 마운트한다. 컨테이너에서 돌릴 때는 설정의 호스트를 `database`, `s3:10900`, `recording.dir`을 `/data/recordings`로 바꾼다.
- 이미지: debian trixie-slim, `ffmpeg`와 Atlas 바이너리 포함, UID 1000 `cameo` 사용자, `EXPOSE 80`, `50000/udp`, 헬스체크 `/app/cameo healthcheck`(`/readyz` 호출).

## 배포 메모

- WebRTC UDP 포트(`webrtc.udp_port`, 기본 예시 50000)를 방화벽과 컨테이너 포트 매핑(`50000:50000/udp`)에서 열어야 한다. 모든 통화가 이 포트 하나를 쓴다.
- `webrtc.public_ips`에 서버 공인 IPv4를 넣는다. 없으면 컨테이너 내부 IP가 후보로 나가 통화가 연결되지 않는다.
- 통화 상태는 프로세스 메모리에 있다. 인스턴스는 하나만 운영하고, 재시작하면 진행 중 통화는 `failed`가 된다. 중단된 후처리는 재기동 때 이어서 처리하므로 `recording.dir`은 영속 볼륨에 둔다.
- `/tmp/drain` 파일이 있으면 `/livez`, `/readyz`가 503을 반환한다. 롤아웃 전에 만들어 트래픽을 먼저 빼낸다.
- APNs .p8 키는 이미지에 넣지 말고 볼륨으로 마운트한 뒤 `push.apns.key_path`로 가리킨다(예: `/config/AuthKey_XXXX.p8`).
- `storage.public_endpoint`는 앱이 도달할 수 있는 주소여야 한다(presigned URL 호스트).
- 운영 빌드(`VERSION != local`)는 Twilio 또는 `otp.allow_fake`가 없으면 기동하지 않고, 500 응답 메시지에 내부 오류를 노출하지 않는다.

## 배포 (oxygen)

oxygen은 Traefik v3가 유일한 리버스 프록시인 Docker 호스트다(외부 네트워크 `proxy`, 엔트리포인트 `https`, 인증서 리졸버 `dnsresolver`, `http`는 https로 리다이렉트). 루트 `compose.yml`이 `cameo`, `database`(PostgreSQL 18), `s3`(versitygw), `bucket-init`을 띄운다. `cameo`와 `s3`만 `proxy` 네트워크에 붙어 Traefik 라벨로 노출되고, DB와 S3는 호스트 포트를 열지 않는다.

### 디렉터리

```
~/services/cameo/            git checkout (https://github.com/c5me0/back)
├── compose.yml
├── .env                     .env.example을 복사해 채운다 (gitignore)
└── config/config.json       deploy/config.example.json을 복사해 채운다 (gitignore, /config로 마운트)
```

```sh
git clone https://github.com/c5me0/back ~/services/cameo && cd ~/services/cameo
cp .env.example .env
mkdir -p config && cp deploy/config.example.json config/config.json
```

- `.env`: `CAMEO_API_HOST`, `CAMEO_S3_HOST`(각 도메인), `POSTGRES_PASSWORD`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`. `VERSION`은 `make prod`가 덮어쓴다.
- `config/config.json`: `database.password`는 `POSTGRES_PASSWORD`, `storage.access_key`/`secret_key`는 `S3_ACCESS_KEY`/`S3_SECRET_KEY`와 같게, `storage.public_endpoint`는 `CAMEO_S3_HOST`(스킴 없이), `token.secret`은 `openssl rand -hex 32`, `webrtc.public_ips`는 공인 IPv4.
- `service.trusted_proxies: 1`은 Traefik 뒤에서 필수다. `0`이면 모든 요청이 Traefik IP로 보여 rate limit이 전체 사용자에 공유된다. 반대로 프록시 없이 노출하면서 `1`로 두면 IP 위조가 가능하다.
- `storage.public_insecure: false`: presigned URL은 Traefik이 TLS를 종료하는 `https://<CAMEO_S3_HOST>`로 나간다. 서버 내부 접속(`endpoint: s3:10900`)은 `insecure: true`로 평문 HTTP.

### DNS와 포트

- `CAMEO_API_HOST`, `CAMEO_S3_HOST` 두 개를 호스트를 가리키는 DNS only(프록시 끔) CNAME으로 만든다. TLS는 Traefik이 `dnsresolver`로 발급한 인증서로 종료한다.
- 공유기에서 `50000/udp`를 호스트로 포트 포워딩한다(WebRTC 미디어, 모든 통화가 공유). HTTP/HTTPS는 Traefik이 이미 받는다.

### 기동과 갱신

```sh
git pull
make prod         # VERSION=$(git rev-parse --short HEAD) docker compose up -d --build --remove-orphans
make prod-logs
```

마이그레이션은 기동 시 자동 적용된다.

### 선택 기능 추가

`config/config.json`에 섹션을 추가하고 `docker compose restart cameo`로 재시작한다.

- Twilio Verify(실제 SMS): `"twilio": { "account_sid": "...", "auth_token": "...", "verify_service_sid": "..." }`. 설정 후 `otp.allow_fake`를 `false`로 바꾼다.
- OpenAI(전사, 제목/요약): `"openai": { "api_key": "...", "language": "ko" }`. 없으면 녹음만 업로드한다.
- APNs 푸시: `.p8` 키를 `config/`에 두고 `"push": { "apns": { "key_path": "/config/AuthKey_XXXX.p8", "key_id": "...", "team_id": "...", "bundle_id": "...", "production": true } }`.
- RevenueCat(결제): 아래 [RevenueCat](#revenuecat) 참고. 없으면 모두 프리미엄으로 취급한다.

## RevenueCat

결제는 RevenueCat으로 처리한다. 상품은 둘이다.

- entitlement `premium`: 구독 상품들에 붙인다. 앱 전체(통화, 사진)를 연다.
- 소모성 상품 `restore`: 이전 커플 데이터 복원 1회.

둘 다 커플 단위다. 둘 중 한 명이라도 `premium`이 활성이면 커플이 프리미엄이고, 복원 크레딧은 두 사람의 미사용 `restore` 구매 합계다.

### 대시보드

1. 프로젝트를 만들고 iOS 앱을 추가한다. In-App Purchase Key(.p8)를 등록한다.
2. entitlement `premium`을 만들고 구독 상품들을 붙인다.
3. 소모성 상품 `restore`를 만든다(entitlement에 붙이지 않는다).
4. offering을 만들어 구독 패키지와 `restore`를 넣는다.
5. 개발 중에는 Test Store 키로도 동작한다.

### 웹훅

- URL: `https://<api host>/v1/webhooks/revenuecat`
- 서명 secret을 `revenuecat.webhook_secret`에 넣는다. 서버는 `X-RevenueCat-Webhook-Signature: t=<unix>,v1=<hex>`(`HMAC-SHA256(secret, "<t>.<raw body>")`, 허용 오차 5분)를 검증한다.
- 환경(sandbox/production) 필터는 필요에 맞게 고른다.
- 이벤트를 받으면 관련 사용자(`app_user_id`, `original_app_user_id`, `aliases`, `transferred_from/to`)를 RevenueCat에서 다시 읽는다. 실패하면 500을 돌려 RevenueCat이 재시도한다.

### 설정

```json
"revenuecat": {
  "api_key": "<REVENUECAT_API_KEY>",
  "webhook_secret": "<REVENUECAT_WEBHOOK_SECRET>",
  "premium_entitlement": "premium",
  "restore_product_id": "restore"
}
```

섹션이 없으면 결제가 꺼진다(로컬 개발용): 모두 프리미엄, 복원 무료, `POST /v1/me/purchases/sync`는 아무것도 하지 않고 웹훅은 제공하지 않는다.

### 클라이언트 책임

- SDK의 App User ID는 CAMEO 사용자 `id`다. 로그인 직후 그 ID로 SDK를 설정하고 `logOut`은 호출하지 않는다.
- SDK에서 구매나 복원이 끝나면, 그리고 앱 실행 시 `POST /v1/me/purchases/sync`를 호출한다(갱신된 `User` 반환).
- 기능 게이팅은 SDK의 CustomerInfo가 아니라 `GET /v1/me`의 `premium.active`를 따른다. 상대가 구매한 경우 내 CustomerInfo에는 entitlement가 없다.
- 연결 시(`POST /v1/couple`)와 `GET /v1/couple`의 `restorable`이 0이 아니면 `restore` 구매를 제안하고, 구매와 sync 후 `POST /v1/couple/restore`를 호출한다. 이미 복원한 커플은 0을 반환하고 크레딧을 쓰지 않는다. 상대는 `data_restored` 푸시를 받는다.

### 서버 강제 규칙

- 프리미엄 필요: `/v1/calls` 아래 전부(`/signal` WebSocket 포함), `/v1/photos` 아래 전부, `POST /v1/couple/restore`.
  - 커플이 없으면 `404 couple:not_connected`, 프리미엄이 아니면 `402 purchase:required` + `meta: {"required":"premium"}`.
  - `POST /v1/couple/restore`에서 옮길 데이터가 있는데 복원 크레딧이 0이면 `402 purchase:required` + `meta: {"required":"restore"}`.
- 프리미엄 불필요: `/v1/auth/*`, `GET`/`PATCH /v1/me`, `POST /v1/me/purchases/sync`, `/v1/devices*`, `GET`/`POST`/`DELETE /v1/couple`, `POST /v1/couple/code`.

## 저장소 구조

```
.
├── main.go                  진입점 (healthcheck 하위 명령 포함)
├── docs/                    openapi.yaml, signaling.md
├── compose.yml              운영 compose (Traefik 라벨)
├── deploy/                  운영 config.example.json
├── local/                   compose.yml, config.example.json, config/ (gitignore)
├── internal/
│   ├── config/              설정 로딩과 검증
│   ├── protocol/            오류 코드와 HTTP 상태 매핑
│   ├── tools/               ro(핸들러 래퍼), pgconfig, pairing
│   └── ent/                 스키마, enum, 생성 코드, migrate/(마이그레이션)
└── server/
    ├── server*.go           초기화, 라우팅, 기동/종료
    ├── route_*.go           도메인별 라우트
    ├── middleware/          request id, 로깅, rate limit
    ├── handlers/            probe, v1/{auth,account,couple,device,call,photo,models}
    └── services/            session, otp, storage, push, call(WebRTC/시그널링/녹음), transcript
```
