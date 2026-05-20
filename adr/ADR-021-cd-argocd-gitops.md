# ADR-021: CD = ArgoCD GitOps (수동 helm install 대체)

- **Date**: 2026-05-21
- **Status**: Accepted
- **Phase**: Ch10 Phase G

## Context
A~F 동안 클러스터를 재생성할 때마다 사람이 `helm install`을 5번씩 쳤다 (ALB Controller, Strimzi, issue-api, issuance-consumer, bot-detector). 문제:
- **Git이 진실의 원천이 아니다** — 클러스터의 실제 상태는 *그때 누가 친 명령*에 달림. helm chart는 Git에 있지만 *배포 행위*는 Git 밖
- **drift 감지 없음** — 누가 `kubectl edit`로 replica를 바꿔도 아무도 모름
- **롤백이 수동** — 이전 버전으로 되돌리려면 사람이 helm rollback
- **재현성** — "어떤 차트가 어떤 값으로 떠 있나"가 셸 히스토리에만 존재

CI(GitHub Actions → ECR, ADR-015)는 이미 있다. 빠진 건 **CD** — 빌드된 이미지를 *Git 상태에 따라 클러스터에 자동 반영*하는 고리.

## Options
| 옵션 | GitOps | UI | 학습 | 함정 |
|---|---|---|---|---|
| **ArgoCD** | pull 기반, Application CR | 있음 (sync/health 가시화) | GitOps 표준, 자료 풍부 | 클러스터 리소스 점유 |
| Flux v2 | pull 기반, 순수 | 없음 (CLI only) | 순수하지만 가시성 ↓ | 디버깅 시 UI 부재 |
| `helm install` 유지 | push 기반 (사람) | 없음 | 현 상태 | drift·재현성 문제 그대로 |
| Helm + CI에서 `helm upgrade` | push 기반 (CI) | 없음 | 간단 | self-heal 없음, CI가 클러스터 자격증명 보관 |

## Decision
**ArgoCD on EKS + 개별 Application 3개 + auto-sync(prune·selfHeal)**.

```
GitHub Actions (CI) ──build──> ECR
        Git repo (helm/*)  ──watch──> ArgoCD ──sync──> EKS
```

- ArgoCD가 이 repo의 `helm/issue-api`, `helm/issuance-consumer`, `helm/bot-detector`를 watch
- `syncPolicy.automated`: `prune: true`(Git에서 지운 리소스 삭제) + `selfHeal: true`(drift 자동 복구)
- repo가 **public**이라 ArgoCD repo credential 불필요

### GitOps의 진실 원천과 동적 값의 경계
ArgoCD는 "Git = 진실"이지만, IRSA role ARN·Redis endpoint·WAF IPSet ID는 `terraform apply`가 *런타임에* 만드는 값이라 Git에 미리 못 박는다. 그래서 두 층을 분리한다:

| 무엇 | 진실 원천 | 이유 |
|---|---|---|
| helm chart 구조·템플릿·로직, replica, 리소스 한도 | **Git** (`helm/*`) | 진짜 GitOps — push가 곧 배포 |
| IRSA ARN, endpoint, WAF ID 등 *apply마다 바뀌는 값* | Application CR의 `helm.parameters` (부트스트랩 시 terraform output 주입) | Git에 박을 수 없는 인프라 바인딩 |

→ Application CR 3개는 `scripts/argocd-bootstrap.sh`가 `terraform output`을 읽어 placeholder를 치환한 뒤 `kubectl apply`. *부트스트랩 1회*, 그 후 helm chart 변경은 ArgoCD가 Git에서 자동 sync.

### 왜 App of Apps를 안 쓰나
앱이 3개뿐이라 root Application 한 겹을 더 두는 이득이 없다. 앱이 10개를 넘거나 multi-env가 생기면 App of Apps / ApplicationSet으로 — Phase J 이후 검토.

### 닭-달걀: ArgoCD 자체는 누가 배포하나
GitOps 도구 자신은 GitOps로 못 깐다. ArgoCD는 `helm install`로 *부트스트랩*하고 (ALB Controller·Strimzi와 동일하게 CLI), 그 다음부터 ArgoCD가 *애플리케이션*을 관리한다. terraform helm provider를 안 쓰는 이유: terraform이 클러스터 API에 의존하면 apply/destroy가 더 깨지기 쉽다 (E~F destroy 교훈) — 인프라(terraform)와 클러스터 부트스트랩(helm CLI)을 분리 유지.

## Consequences
**+**:
- **Git push = 배포** — 수동 `helm install` 5연타 제거. replica를 바꾸려면 Git을 고치고 push
- **self-heal** — `kubectl edit`로 누가 drift를 만들어도 ArgoCD가 Git 상태로 되돌림
- **가시화** — ArgoCD UI에서 각 앱의 Synced/Healthy, 어느 커밋이 떠 있는지 한눈에
- **롤백 = `git revert`** — 배포 이력이 곧 Git 이력
- CI는 클러스터 자격증명을 갖지 않는다 — ArgoCD가 *클러스터 안에서 pull*. CI 탈취 시 클러스터 직접 접근 불가

**−**:
- ArgoCD 자체가 클러스터 리소스를 먹는다 (~수백 MB) — t3.small 2노드엔 부담, 학습 동안만
- **동적 값이 Application CR에 있어 순수 GitOps가 아니다** — 부트스트랩 스크립트 의존. 운영은 External Secrets로 빼야 함
- **image tag** — `latest` + `imagePullPolicy: Always`로 단순화. GitOps 정통은 CI가 values의 tag를 bump 커밋하거나 ArgoCD Image Updater. 학습 범위 밖
- 닭-달걀 — ArgoCD 부트스트랩 자체는 여전히 수동 (helm CLI 1회)

## 운영 시 변경 사항 (현재는 학습용 단순화)
| 항목 | 학습 (현재) | 운영 |
|---|---|---|
| 동적 값 (ARN·endpoint) | Application CR `helm.parameters` (부트스트랩 주입) | External Secrets Operator + ConfigMap(terraform), CR엔 동적값 0 |
| image tag | `latest` + Always | CI가 values tag bump 커밋 / ArgoCD Image Updater |
| 앱 추가 | Application YAML 수동 추가 | App of Apps / ApplicationSet generator |
| ArgoCD 자체 | helm install 1회 | terraform 또는 cluster bootstrap 파이프라인, HA 모드 |
| 접근 제어 | admin 단일 계정 | SSO(OIDC) + ArgoCD RBAC, project별 격리 |
| 환경 | 단일 (main → cluster) | ApplicationSet으로 staging/prod 분리 |

## 검토 일정
Phase H — Observability 스택(kube-prometheus-stack)도 ArgoCD Application으로 관리할지. Phase J 이후 — 앱 수 증가 시 App of Apps 전환.
