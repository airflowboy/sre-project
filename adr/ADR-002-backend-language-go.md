# ADR-002: 백엔드 언어 = Go

- **Date**: 2026-05-15
- **Status**: Accepted
- **Phase**: Ch10 마스터 (캡스톤 백엔드 전반)
- **Decider**: geontae

## Context
발급 API는 100만 동시접속 처리 + 원자적 재고 차감(KV 호출) + 이벤트 큐 produce + RDBMS 쿼리. **요구**: 동시성, 낮은 지연, 운영 친화성 (컨테이너), 클라우드 생태계.

## Options
| 언어 | 동시성 모델 | 컨테이너 친화 | 학습 비용 | 생태계 |
|------|------------|:---:|:---:|:---:|
| **Go** | goroutine (저렴) | 단일 정적 바이너리 (~10MB) | ↓ (Ch 07 워밍업) | 클라우드 네이티브 표준어 (K8s/Docker/Terraform) |
| Rust | async + ownership | 단일 정적 바이너리 | ↑↑ | 성장 중, 작음 |
| Java (Spring) | thread + Loom | JVM 메모리 ↑ (~200MB+) | △ (성숙) | 엔터프라이즈 표준, 클라우드는 무거움 |
| Node.js | async I/O (single-thread) | npm 의존성 ↑ | △ | 풍부하지만 CPU-bound 약함 |

## Decision
**Go**

## Consequences
**+**:
- goroutine으로 요청별 동시 처리 자연스러움 (channel + sync 패턴)
- 단일 정적 바이너리 + distroless 이미지 = 공격면 최소 (Ch 09 Trivy 0건 검증)
- 빠른 빌드 (Java 대비), 빠른 cold start (JVM 대비)
- AWS SDK for Go v2 + Redis client + Kafka client + pgx (PostgreSQL) 성숙
- Ch 07-A 미니 앱(/healthz, /version)으로 워밍업 완료 — 학습 비용 ↓

**−**:
- 새로 배워야 할 것: HTTP 미들웨어, context propagation, structured logging (zap/zerolog)
- 에러 핸들링이 verbose (panic 안 쓰고 명시적 error return)
- 제네릭은 Go 1.18+ (우리 1.25라 무관)

## 검토 일정
Phase C(API 작성) 끝나고 — 실제 작성 경험으로 "다음 마이크로서비스는 Go? Rust로 갈까?" 재평가.
