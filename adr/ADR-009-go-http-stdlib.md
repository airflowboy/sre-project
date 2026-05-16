# ADR-009: Go HTTP 프레임워크 = net/http (stdlib only)

- **Date**: 2026-05-17
- **Status**: Accepted
- **Phase**: Ch10 Phase C
- **Decider**: geontae

## Context
발급 API는 단순한 4개 라우트:
- `POST /issue` — 한정 발급 (핵심)
- `GET  /healthz` — LB 헬스 체크
- `GET  /version` — 빌드 메타
- `GET  /stock` — 현재 재고 (관찰/디버깅)

프레임워크 선택.

## Options
| 옵션 | 의존성 | 학습 비용 | 성능 | 생태계 |
|------|:---:|:---:|:---:|:---:|
| **net/http (stdlib)** | 0 | 낮음 (Go 핵심) | 표준 | 표준 |
| chi | 1 | 낮음 (net/http 호환) | 표준 | 큼 (미들웨어 풍부) |
| gin | 1 | 중간 (자체 API) | 빠름 | 매우 큼 (가장 인기) |
| fiber (fasthttp 기반) | 1 | 중간 | **가장 빠름** | 중간 (비표준) |
| echo | 1 | 중간 | 빠름 | 큼 |

## Decision
**net/http (stdlib only)**

## Consequences
**+**:
- **의존성 0** — go.mod에 외부 패키지 없음(redis 클라이언트만 추가). distroless 이미지 최소화
- **Go 1.22+의 새 라우터** — `mux.HandleFunc("POST /issue", handler)` 메서드별 라우팅 가능. gin·chi 없어도 충분
- 컨테이너 cold start 빠름 (의존성 적음)
- 모든 Go 개발자가 이해 가능 — 프레임워크 학습 비용 0
- 미래에 chi·gin으로 *마이그레이션 쉬움* (net/http 인터페이스가 표준)

**−**:
- 미들웨어를 *직접* 구현해야 함 (로깅·panic recover·request ID). 100줄 안 넘음
- 자동 validation(struct tag → 검증) 없음 — 직접 검증
- swagger·OpenAPI 통합은 별도 도구(swaggo 등)
- 라우트가 100+개면 chi/gin이 *명백히* 편함. 우리는 ~4개라 무관

## 만약 다른 시나리오면?
- **REST API 100개+, 빠른 개발**: gin (가장 인기, generator 풍부)
- **gRPC 위주**: connectrpc + gin/chi
- **극한 성능**: fiber (fasthttp 기반 — 단 표준 net/http 호환 안 됨)
- **AWS Lambda**: chi가 가장 호환 좋음

## 면접 답
"라우트 5개 미만이면 net/http로 충분. Go 1.22의 ServeMux가 메서드 라우팅도 지원 → 외부 프레임워크 의존이 *학습 부담만* 됨. 100+ 라우트가 되면 gin 도입 검토. 지금은 의존성 0 + distroless 최소 이미지 = 보안·cold start 최적화."

## 작성 패턴 (예시)
```go
mux := http.NewServeMux()
mux.HandleFunc("POST /issue", handleIssue)
mux.HandleFunc("GET /healthz", handleHealthz)
mux.HandleFunc("GET /version", handleVersion)
mux.HandleFunc("GET /stock", handleStock)

// 미들웨어 chain
handler := loggingMiddleware(recoverMiddleware(mux))
http.ListenAndServe(":8080", handler)
```

## 검토 일정
Phase F(봇 탐지) — Python ML 서버와 통신 시 gRPC 도입 검토. 그때 gRPC + net/http 분리 필요할 수 있음.
