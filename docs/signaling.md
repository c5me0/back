# CAMEO 통화 시그널링 가이드 (iOS 클라이언트용)

이 문서는 통화 한 건의 전체 수명(발신, VoIP 푸시, WebSocket 시그널링, WebRTC, 종료, 녹음 후처리)을 클라이언트 입장에서 설명한다. REST 엔드포인트의 요청/응답 스키마는 [openapi.yaml](./openapi.yaml)을 따른다. 코드 기준 출처는 `server/services/call/{room,signaling,participant,api}.go`와 `server/handlers/v1/call/*.go`다.

## 1. 구조 요약

- 서버가 통화의 WebRTC 상대(peer)다. 두 사용자가 직접 연결하지 않고, 각자 서버와 `RTCPeerConnection` 하나를 맺는다. 서버는 한쪽의 Opus RTP를 다른 쪽으로 그대로 중계하고, 통화가 `active`인 동안 참여자별로 녹음한다.
- 통화 상태 전이는 전부 서버가 결정한다. 클라이언트는 `state`/`ended` 메시지와 REST 응답으로 상태를 받아 UI와 CallKit에 반영한다.
- 오퍼는 항상 클라이언트가 만든다. 서버는 answer만 보내고, ICE 후보를 trickle하지 않는다.

## 2. 연결

### URL과 인증

```
GET /v1/calls/{call_id}/signal        (WebSocket 업그레이드)
로컬: ws://localhost:18080/v1/calls/{call_id}/signal
운영: wss://<host>/v1/calls/{call_id}/signal
```

인증은 둘 중 하나다.

- 헤더 `Authorization: Bearer <token>` (`URLSessionWebSocketTask`는 `URLRequest`에 헤더를 넣을 수 있으므로 이 방식을 권장)
- 쿼리 `?token=<token>` (헤더를 못 넣는 환경용)

Origin 검사는 하지 않는다.

### 업그레이드 전 오류 (일반 HTTP 오류 봉투)

| 상태 | code | 의미 |
|---|---|---|
| 401 | `unauthenticated` | 토큰 없음/무효/만료 |
| 404 | `not_found` | ID 형식 오류이거나 우리 커플의 통화가 아님 |
| 404 | `couple:not_connected` | 연결된 상대가 없음 |
| 409 | `call:invalid_state` | 통화가 `ringing`/`active`가 아니거나 이 서버 프로세스에 살아 있지 않음. `GET /v1/calls/{id}`로 최종 상태를 확인할 것 |

### 소켓 규칙

- 프레임은 모두 JSON 텍스트 한 개이며 `type` 필드로 구분한다.
- 서버가 15초마다 ping을 보낸다. 10초 안에 pong이 없으면 서버가 연결을 끊는다. `URLSessionWebSocketTask`는 pong을 자동으로 보낸다.
- 수신 프레임 최대 크기는 64 KiB다. 넘으면 서버가 소켓을 닫는다(1009).
- 서버의 송신 큐는 32개다. 클라이언트가 읽지 못해 큐가 차면 이후 메시지는 버려진다.

### Close 코드

| 코드 | reason | 언제 | 클라이언트 동작 |
|---|---|---|---|
| 1000 | `call ended` | `ended` 메시지 직후 | 통화 종료 처리. 재접속하지 않음 |
| 1000 | `call is not live` | 접속 직후 통화가 이미 끝난 경합 | `GET /v1/calls/{id}`로 상태 확인 |
| 1000 | (빈 문자열) | 서버가 정상적으로 소켓을 정리 | 통화가 진행 중이면 재접속 |
| 1008 | `not a participant` | 통화 참여자가 아님(정상 흐름에서는 발생하지 않음) | 재접속하지 않음 |
| 4000 | `replaced` | 같은 사용자가 새 소켓으로 접속해 이 소켓을 대체 | 재접속하지 않음(다른 소켓이 주인) |

## 3. 메시지 카탈로그

모든 UUID는 문자열, 시각은 UTC RFC 3339다.

### 클라이언트 → 서버

**offer**: 새 `RTCPeerConnection`의 오퍼. 보낼 때마다 서버는 기존 PeerConnection을 닫고 새로 만든다(재협상/재접속 겸용).

```json
{"type": "offer", "sdp": "v=0\r\no=- 4611731400430051336 2 IN IP4 127.0.0.1\r\n..."}
```

**candidate**: 클라이언트 로컬 ICE 후보(trickle). `sdp_mid`, `sdp_mline_index`는 생략하거나 `null` 가능. `candidate`가 빈 문자열이면(end-of-candidates) 무시된다. 서버가 아직 offer를 적용하기 전에 도착한 후보는 큐에 쌓였다가 적용된다.

```json
{"type": "candidate", "candidate": "candidate:842163049 1 udp 1677729535 203.0.113.7 61234 typ srflx raddr 0.0.0.0 rport 0 generation 0", "sdp_mid": "0", "sdp_mline_index": 0}
```

**hangup**: 통화 종료. `POST /v1/calls/{id}/end`와 같다. 발신자가 `ringing` 중에 보내면 `missed`, 그 외에는 `ended`.

```json
{"type": "hangup"}
```

알 수 없는 `type`은 `error` 메시지로 응답한다.

### 서버 → 클라이언트

**state**: 통화 상태 스냅샷. 다음 시점에 보낸다.
- 소켓이 붙은 직후(접속/재접속마다 항상 첫 메시지)
- 어느 한쪽이 offer를 보내 PeerConnection을 교체할 때(양쪽 모두에게)
- 어느 한쪽 PeerConnection이 `connected`/`failed`/`closed`가 될 때(양쪽 모두에게, `ringing → active` 전환 포함)

`peer_connected`는 상대방의 **미디어(PeerConnection)** 가 서버와 연결돼 있는지다(상대 소켓 여부가 아님). `started_at`은 `active`가 된 시각이며 그 전에는 `null`이다.

```json
{"type": "state", "status": "ringing", "started_at": null, "peer_connected": false}
```

```json
{"type": "state", "status": "active", "started_at": "2026-10-01T12:00:05.123456789Z", "peer_connected": true}
```

**answer**: offer에 대한 서버 answer. 서버 ICE 수집이 끝난 뒤 모든 후보를 포함해 한 번에 보낸다(non-trickle). 수집이 10초 안에 끝나지 않으면 answer 대신 `error`가 온다.

```json
{"type": "answer", "sdp": "v=0\r\no=- 7309104218432946145 1727784005 IN IP4 0.0.0.0\r\n...a=candidate:... 203.0.113.10 50000 typ host\r\n..."}
```

**highlight_added**: 상대가 하이라이트를 찍었다(`POST /v1/calls/{id}/highlights`). 찍은 본인에게는 오지 않는다.

```json
{
  "type": "highlight_added",
  "highlight": {
    "id": "0192a1b2-0000-7000-8000-000000000001",
    "user_id": "0192a1b2-0000-7000-8000-0000000000aa",
    "offset_seconds": 83.4,
    "created_at": "2026-10-01T12:01:28.500Z"
  }
}
```

**photo_shared**: 상대가 통화 중 사진을 공유했다(`POST /v1/calls/{id}/photos`). `photo`는 REST의 `Photo`와 같은 모양이며 `is_favorite`는 받는 사람 기준이다. URL은 1시간 뒤 만료된다.

```json
{
  "type": "photo_shared",
  "photo": {
    "id": "0192a1b2-0000-7000-8000-000000000101",
    "uploader_id": "0192a1b2-0000-7000-8000-0000000000aa",
    "call_id": "0192a1b2-0000-7000-8000-000000000c01",
    "content_type": "image/jpeg",
    "size_bytes": 2483112,
    "width": 3024,
    "height": 4032,
    "taken_at": "2026-09-30T08:12:00Z",
    "is_favorite": false,
    "url": "https://s3.example/cameo/photos/.../original?X-Amz-...",
    "thumbnail_url": "https://s3.example/cameo/photos/.../thumbnail?X-Amz-...",
    "url_expires_at": "2026-10-01T13:02:00Z",
    "created_at": "2026-10-01T11:58:40Z"
  }
}
```

**ended**: 통화가 끝났다. 최종 상태(`ended`/`missed`/`declined`/`failed`)를 담고, 서버는 곧바로 1000 `call ended`로 소켓을 닫는다.

```json
{"type": "ended", "status": "ended"}
```

**error**: 처리할 수 없는 메시지. 소켓은 유지된다.

```json
{"type": "error", "code": "invalid_request", "message": "failed to negotiate the offer"}
```

```json
{"type": "error", "code": "invalid_request", "message": "unknown message type \"ping\""}
```

## 4. 통화 상태 머신

상태 값: `ringing`, `active`, `ended`, `missed`, `declined`, `failed`. `ringing`/`active`만 진행 중이며 커플당 동시에 하나만 존재할 수 있다(두 번째 `POST /v1/calls`는 `409 call:busy`).

```
             POST /calls
                 │
                 ▼
            ┌─────────┐  양쪽 PeerConnection 모두 connected   ┌────────┐
            │ ringing │ ───────────────────────────────────▶ │ active │
            └─────────┘                                       └────────┘
   decline(수신자) │ 발신자 hangup/end,     수신자 hangup/end,       │ 누구든 hangup/end,
                 │ 60초 링 타임아웃        커플 해제                │ 30초 grace 초과, 커플 해제
                 ▼        ▼                   ▼                    ▼
            declined    missed              ended                ended
   (ringing/active 어디서든) 서버 종료, 부팅 시 스윕, active 전환 DB 저장 실패 → failed
```

| 전이 | 누가/무엇이 | 비고 |
|---|---|---|
| (없음) → `ringing` | 발신자 `POST /v1/calls` | 수신자에게 VoIP `incoming_call` 푸시, 60초 링 타이머 시작 |
| `ringing` → `active` | 서버: 두 참여자의 PeerConnection이 모두 `connected` | `started_at` 기록, 녹음 시작, 양쪽에 `state` 브로드캐스트 |
| `ringing` → `declined` | 수신자 `POST /v1/calls/{id}/decline` | 발신자가 호출하면 `403 forbidden`, ringing이 아니면 `409 call:invalid_state` |
| `ringing` → `missed` | 발신자 `hangup` 또는 `POST /end` | |
| `ringing` → `missed` | 링 타이머 60초 만료 | 수신자가 WS에 붙었어도 미디어가 60초 안에 연결되지 않으면 동일 |
| `ringing` → `ended` | 수신자 `hangup` 또는 `POST /end` | 거절 의도라면 `decline`을 쓸 것 |
| `ringing`/`active` → `ended` | 누구든 `DELETE /v1/couple` | 강제 종료 |
| `active` → `ended` | 누구든 `hangup` 또는 `POST /end` | |
| `active` → `ended` | grace 타이머 30초 만료 | 한 참여자가 소켓 **또는** 미디어 연결을 30초 넘게 잃은 경우 |
| `ringing`/`active` → `failed` | 서버 종료, 재시작 후 부팅 스윕, `active` 저장 실패 | |

종료 시 서버가 하는 일:

1. 양쪽 소켓에 `ended{status}` 전송 후 1000으로 close, PeerConnection 종료.
2. `ended_at` 기록. 녹음 세그먼트가 있으면 `transcript_status = pending`, 없으면 `none`.
3. 한 번도 `active`가 되지 못했고 `declined`가 아니면(= `missed`, ringing 중 `ended`/`failed`) 수신자에게 VoIP `call_ended` 푸시.
4. `missed`이고 수신자의 `call_alert`가 켜져 있으면 수신자에게 alert `missed_call` 푸시.

### 타이머와 타임아웃

| 이름 | 값 | 동작 |
|---|---|---|
| 링 타이머 | 60초 (`POST /calls` 시점부터) | 여전히 `ringing`이면 `missed` |
| grace 타이머 | 30초 (참여자별) | `active` 중 해당 참여자의 소켓이 없거나 PeerConnection이 연결되지 않은 상태가 30초 지속되면 `ended`. 복구되면 해제 |
| ICE disconnected / failed / keepalive | 5초 / 15초 / 2초 | 서버 PeerConnection의 ICE 타임아웃 |
| ICE 수집 | 10초 | 넘으면 answer 대신 `error` |
| WS ping / 쓰기 타임아웃 | 15초 / 10초 | |

`ringing` 중에는 grace가 없다. 소켓을 끊었다 다시 붙여도 링 타이머만 적용된다.

## 5. 권장 클라이언트 시퀀스

### 발신자

1. `POST /v1/calls` → `201 {call, ice_servers}`. `409 call:busy`면 통화 중 안내.
2. CallKit에 발신 통화 보고(`CXStartCallAction`, UUID는 `call.id`로 매핑).
3. WS 접속 → 첫 메시지 `state{status: "ringing", peer_connected: false}`.
4. `RTCPeerConnection(iceServers: ice_servers)` 생성, 오디오 트랙 1개(sendrecv) 추가, `createOffer` → `setLocalDescription` → `offer` 전송. 이후 생성되는 로컬 후보를 `candidate`로 전송.
5. `answer` 수신 → `setRemoteDescription`. 발신자 미디어는 서버와 먼저 연결되지만 상태는 수신자가 연결될 때까지 `ringing`.
6. `state{status: "active"}` 수신 → CallKit에 연결 보고(`reportOutgoingCall(with:connectedAt:)`), 경과 시간은 `started_at` 기준으로 계산.
7. 종료는 `hangup` 전송(또는 `POST /end`). `ended` 수신 후 CallKit 종료 보고.

### 수신자

1. PushKit VoIP `incoming_call{call_id, caller_id, caller_name}` 수신 → **즉시** `reportNewIncomingCall` (8장 참고).
2. 사용자가 받음(`CXAnswerCallAction`) → `GET /v1/calls/{id}`로 `ice_servers` 획득 → WS 접속 → `state{ringing}` → 발신자와 같은 방식으로 offer/candidate → `answer`.
3. 양쪽 미디어가 연결되면 `state{active}`. 경과 시간은 `started_at` 기준으로 계산한다.
4. 사용자가 거절 → `POST /v1/calls/{id}/decline` (WS 불필요).
5. `call_ended` VoIP 푸시 또는 `ended` 메시지 → CallKit 종료 보고.

```
발신자                         서버                               수신자
  │ POST /v1/calls ─────────────▶│                                   │
  │◀──── 201 {call, ice_servers} │── VoIP incoming_call ────────────▶│ reportNewIncomingCall
  │ WS connect ─────────────────▶│                                   │
  │◀──────────── state(ringing) │                                   │
  │ offer, candidate… ──────────▶│                                   │ (사용자 수락)
  │◀──────────────────── answer │◀───────────────────── WS connect  │
  │                              │ state(ringing) ──────────────────▶│
  │                              │◀──────────── offer, candidate…   │
  │                              │ answer ──────────────────────────▶│
  │◀──────── state(active, started_at, peer_connected: true) ───────▶│
  │◀══════════════ Opus RTP 중계 + 녹음 ════════════════════════════▶│
  │ hangup ─────────────────────▶│                                   │
  │◀────────── ended(ended) ────│──────────── ended(ended) ────────▶│
  │◀──────────── close 1000 ────│──────────── close 1000 ──────────▶│
```

앱이 푸시로 깨어난 직후 통화가 이미 끝났을 수 있다. WS 접속이 `409 call:invalid_state`이면 CallKit 통화를 종료하고 `GET /v1/calls/{id}`로 상태를 확인한다.

## 6. 재접속 규칙

- **소켓만 끊긴 경우**(네트워크 전환 등): 같은 URL로 WS를 다시 연다. 첫 메시지 `state`로 현재 상태를 받는다. 기존 PeerConnection이 살아 있으면 다시 offer할 필요가 없다.
- **미디어가 끊긴 경우**(`iceConnectionState`가 failed/disconnected에서 회복 안 됨): 새 `RTCPeerConnection`을 만들고 새 `offer`를 보낸다. 서버는 해당 참여자의 기존 PeerConnection을 닫고 새로 연결하며, 녹음은 새 세그먼트로 이어진다. ICE restart가 아니라 **새 PeerConnection**을 만들어야 한다(서버 쪽 ICE/DTLS 자격이 새로 만들어진다).
- `active` 중에는 소켓과 미디어를 둘 다 30초 안에 회복해야 한다. 못 하면 서버가 `ended`로 끝낸다.
- 같은 사용자가 두 번째 소켓을 열면 이전 소켓은 4000 `replaced`로 닫힌다. 4000을 받은 쪽은 재접속하지 않는다.
- `ended` 후 1000 `call ended`, 또는 재접속 시 `409 call:invalid_state`면 재접속을 멈춘다.
- 서버가 재시작되면 진행 중 통화는 모두 `failed`가 된다(종료 중인 서버는 `ended{failed}`를 보낸다).

## 7. 통화 중 기능

### 하이라이트

1. 사용자가 탭하면 `offset_seconds = 지금 - started_at`(초, 소수 허용)을 계산.
2. `POST /v1/calls/{id}/highlights {"offset_seconds": 83.4}` → `201 Highlight`. `active`가 아니면 `409 call:invalid_state`.
3. 상대는 `highlight_added`를 받는다.
4. 하이라이트는 통화 후 요약에서 강조되고 `GET /v1/calls/{id}`의 `highlights`(offset 오름차순)에 남는다.

### 사진 공유

1. 새 사진이면 먼저 업로드: `POST /v1/photos/upload-url` → 원본과 썸네일을 각 URL에 `PUT` → `POST /v1/photos/{photo_id}/complete`. 라이브러리의 기존 사진은 바로 2번으로.
2. `POST /v1/calls/{id}/photos {"photo_id": "..."}` → `200 Photo`(보낸 사람 기준). 조건: 통화 `active`(아니면 `409 call:invalid_state`), 사진 업로드 완료(아니면 `409 photo:invalid_state`).
3. 상대는 WS `photo_shared`와 alert 푸시 `photo_shared`를 둘 다 받는다. 포그라운드에서 통화 화면이 떠 있으면 푸시 배너는 앱에서 숨기면 된다.
4. 사진이 아직 어느 통화에도 묶이지 않았다면 이 통화에 묶여 `GET /v1/calls/{id}`의 `photos`와 `GET /v1/photos?call_id=...`에 나타난다. 이미 다른 통화에 묶인 사진은 기존 `call_id`를 유지한다.

## 8. 푸시 알림

서버는 APNs 토큰 인증으로 보낸다. 기기 등록은 `POST /v1/devices`이며, 일반 APNs 토큰은 `apns`, PushKit 토큰은 `apns_voip`로 등록한다. 같은 사용자의 해당 플랫폼 기기 전부에 전송되며, APNs가 무효 토큰(410, BadDeviceToken, Unregistered)을 알리면 서버가 기기 행을 지운다. APNs 설정이 없는 서버는 푸시를 로그로만 남긴다.

### VoIP (PushKit, topic `<bundle_id>.voip`)

페이로드에 `aps`가 없고 데이터 키만 최상위에 있다.

| `type` | 받는 사람 | 필드 | 언제 |
|---|---|---|---|
| `incoming_call` | 수신자 | `call_id`, `caller_id`, `caller_name`(표시 이름, 없으면 `""`) | `POST /v1/calls` 직후, 항상 |
| `call_ended` | 수신자 | `call_id` | `active`가 되지 못하고 끝났을 때(`missed`, ringing 중 `ended`/`failed`). `declined`에는 보내지 않음 |

```json
{"type": "incoming_call", "call_id": "0192a1b2-0000-7000-8000-000000000c01", "caller_id": "0192a1b2-0000-7000-8000-0000000000aa", "caller_name": "지민"}
```

```json
{"type": "call_ended", "call_id": "0192a1b2-0000-7000-8000-000000000c01"}
```

**CallKit 의무**: iOS 13 이상은 VoIP 푸시를 받을 때마다 `reportNewIncomingCall`을 호출하지 않으면 앱이 종료되고 이후 VoIP 푸시가 막힌다. `call_ended`를 포함한 **모든 VoIP 푸시를 CallKit에 보고**해야 한다.

- `incoming_call`: `call_id`로 CallKit UUID를 만들고 즉시 수신 통화로 보고.
- `call_ended`: 같은 `call_id`의 통화가 CallKit에 있으면 `reportCall(with:endedAt:reason: .remoteEnded)`(부재중이면 `.unanswered`)로 종료. 없으면(앱이 새로 깨어났거나 `incoming_call`보다 먼저 도착) 수신 통화로 보고한 직후 바로 종료 보고한다.
- 두 푸시는 백그라운드에서 독립적으로 전송되므로 순서가 뒤바뀔 수 있다. `call_ended`를 먼저 받은 `call_id`는 기억해 두고 뒤늦은 `incoming_call`은 보고 직후 종료한다.
- 수신자가 `decline`하면 `call_ended`가 오지 않는다. 수신자의 다른 기기에서 울리는 CallKit은 앱이 스스로 정리해야 한다(예: `GET /v1/calls/{id}` 확인).

### Alert (topic `<bundle_id>`)

`aps.alert`에 제목/본문, `sound: "default"`, 데이터 키는 최상위 커스텀 키로 들어간다.

```json
{"aps": {"alert": {"title": "부재중 전화", "body": "지민님의 전화를 받지 못했어요"}, "sound": "default"}, "type": "missed_call", "call_id": "0192a1b2-0000-7000-8000-000000000c01"}
```

| `type` | 받는 사람 | 제목 / 본문 | 데이터 필드 | 설정 게이트 |
|---|---|---|---|---|
| `missed_call` | 수신자 | `부재중 전화` / `{이름}님의 전화를 받지 못했어요` (이름 없으면 `받지 못한 전화가 있어요`) | `call_id` | 수신자 `call_alert` |
| `photo_shared` | 공유받은 상대 | `{보낸 사람 이름}`(없으면 빈 문자열) / `사진을 공유했어요` | `call_id`, `photo_id` | 없음 |
| `summary_ready` | 두 참여자 각각 | `통화 요약이 준비됐어요` / 생성된 제목(발화가 없으면 빈 문자열) | `call_id` | 각자의 `highlight_alert` |
| `partner_connected` | 코드를 입력당한 쪽(코드 주인) | `연결됐어요` / `{이름}님과 연결됐어요` (이름 없으면 `새로운 상대와 연결됐어요`) | 없음 | 없음 |
| `partner_disconnected` | 해제당한 상대 | `연결이 해제됐어요` / 빈 문자열 | 없음 | 없음 |

## 9. 녹음과 전사 상태

녹음은 `active`가 된 순간부터 종료까지 참여자별로 기록된다(재접속마다 세그먼트 추가). 종료 후 `GET /v1/calls/{id}`와 목록의 `transcript_status`가 다음처럼 바뀐다.

```
none  (녹음 없음: active가 되지 못한 통화)
pending → processing → completed
                     → skipped   (OpenAI 미설정: 녹음 파일만 업로드)
                     → failed    (ffmpeg/업로드/전사/요약 실패, 또는 재시작 후 녹음 파일 유실)
```

- `processing` 중 순서: 참여자별 트랙 인코딩 → 믹스 → S3 업로드 → `recording_url` 사용 가능 → 전사 → 제목/요약 → `completed`. 따라서 `recording_url`은 `processing` 도중부터 non-null일 수 있고, `skipped`에서도 채워진다.
- `recording_url`은 믹스된 m4a(AAC 모노, `audio/mp4`)의 presigned URL이며 `url_expires_at`(1시간)에 만료된다. 만료되면 `GET /v1/calls/{id}`를 다시 호출한다.
- `completed`면 `transcript`에 `{speaker_user_id, start, end, text}` 배열이 시작 시각 순으로 들어간다. `start`/`end`는 `started_at` 기준 초라서 하이라이트 `offset_seconds`와 같은 축이다. 발화가 하나도 없으면 `transcript`는 빈 배열이고 `title`/`summary`는 `null`로 남는다.
- 완료 알림은 `summary_ready` 푸시(각자 `highlight_alert`가 켜진 경우)다. 푸시를 못 받는 경우를 대비해 통화 상세 화면에서는 `pending`/`processing`인 동안 주기적으로 `GET /v1/calls/{id}`를 다시 읽는다.
- 서버 재시작 시 `pending`/`processing`은 자동으로 재개된다.

## 10. WebRTC 클라이언트 요구사항

- **오디오 전용, Opus만.** 서버는 Opus(48 kHz, 2채널, `minptime=10;useinbandfec=1`, payload type 111)만 협상한다. 비디오 m-line을 넣지 않는다.
- **오디오 트랜시버 1개, `sendrecv`.** 서버는 각 PeerConnection에 상대 음성을 보내는 트랙 하나를 붙인다.
- **ICE 서버는 응답의 `ice_servers`를 쓴다.** 발신자는 `POST /v1/calls` 응답, 수신자는 `GET /v1/calls/{id}` 응답의 값을 쓴다. 서버는 공인 IP의 host 후보(UDP 50000 단일 포트, IPv4)를 answer에 넣으므로 대부분의 네트워크에서 STUN 없이도 연결된다.
- **클라이언트 후보는 trickle로 보낸다.** `onicecandidate`마다 `candidate` 메시지를 보낸다. offer 전송 후에 보내는 것을 권장한다(먼저 도착해도 큐잉되지만 순서를 지키는 편이 안전하다).
- **서버 answer는 non-trickle이다.** answer SDP에 서버 후보가 모두 들어 있고 별도 `candidate` 메시지는 오지 않는다. `addIceCandidate`로 받을 것이 없다.
- **재협상은 새 PeerConnection으로.** 한 참여자에게 PeerConnection은 항상 하나이며 새 offer가 이전 것을 대체한다.
- 오디오 세션은 CallKit의 `provider(_:didActivate:)`에서 활성화한다(WebRTC iOS SDK의 `RTCAudioSession` 수동 모드 사용 권장).
