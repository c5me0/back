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
| `storage.endpoint` | 필수 | | 서버가 접속하는 S3 엔드포인트(`host:port`, 스킴 없음) |
| `storage.public_endpoint` | 선택 | `endpoint` | presigned URL에 들어가는 엔드포인트. 클라이언트가 도달할 수 있어야 함 |
| `storage.bucket` | 필수 | | 버킷. 없으면 기동 시 생성 |
| `storage.access_key` / `storage.secret_key` | 필수 | | S3 자격 증명 |
| `storage.region` | 선택 | `us-east-1` | 서명 리전 |
| `storage.insecure` | 선택 | `false` | `true`면 HTTP로 접속하고 presigned URL도 `http://` |
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

## Make 타깃

| 타깃 | 동작 |
|---|---|
| `make up` / `make down` | 로컬 PostgreSQL, S3(versitygw), 버킷 초기화 컨테이너 기동/정지 |
| `make run` | 로컬 설정으로 서버 실행(`:18080`) |
| `make build` | `bin/cameo` 빌드 |
| `make generate` | ent 코드 생성 + DDL 스냅샷(`internal/ent/migrate/schema.sql`) 갱신 |
| `make migration` | 스냅샷과 `extra.sql`로부터 Atlas 마이그레이션 생성(Docker로 dev DB를 띄움) |
| `make fmt` / `make lint` | golangci-lint 포맷/린트 |

## 스키마 변경 워크플로

1. `internal/ent/schema/*.go`(enum은 `internal/ent/schema_types/`)를 수정한다.
2. `make generate`로 ent 코드와 `schema.sql` 스냅샷을 갱신하고 빌드를 확인한다. 스키마를 다듬는 동안에는 이 단계만 반복한다.
3. 스키마가 확정되면 `make migration`으로 `internal/ent/migrate/migrations/`에 SQL과 `atlas.sum`을 만든다. ent가 표현하지 못하는 DDL은 `internal/ent/migrate/extra.sql`에 둔다.
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

## 저장소 구조

```
.
├── main.go                  진입점 (healthcheck 하위 명령 포함)
├── docs/                    openapi.yaml, signaling.md
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
