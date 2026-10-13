# KubeCompass 진행 현황 및 개발 계획

기준일: 2026-10-06

## 1. 프로젝트 소개

KubeCompass는 Kubernetes를 처음 접하는 사람도 자신의 클러스터 구성을 쉽게 이해하고, 대표적인 보안 설정을 확인할 수 있도록 돕는 읽기 전용 대시보드입니다. 초기에는 Minikube 환경을 지원하고, 이후에는 VMware 기반 클러스터와 AWS 환경으로 연결 대상을 확장합니다.

> 핵심 목표: 복잡한 Kubernetes 리소스를 한눈에 이해하고, 기본 보안 위험을 빠르게 발견할 수 있게 합니다.

## 2. 지금까지 구현한 내용

현재 `main` 브랜치에는 아래 두 커밋이 반영되어 있습니다.

- `b971fb0 chore: initialize project structure`
- `8bff5bd feat: Minikube 대시보드 연동 초기 구현`

두 번째 커밋에서 처음으로 화면과 API를 연결했습니다. 웹 화면은 Next.js와 TypeScript로 구현했으며, Namespace·Pod·Service 개수와 API 연결 상태를 보여 줍니다. Go API는 Gin과 `client-go`를 사용하며, 로컬 개발에서는 kubeconfig를 통해, Kubernetes 안에서 실행할 때는 ServiceAccount를 통해 클러스터에 연결합니다.

현재 API는 다음 두 주소를 제공합니다.

- `GET /healthz`: API와 Kubernetes 연결 상태를 확인합니다.
- `GET /api/v1/summary`: Namespace, Pod, Service 개수와 조회 시각을 반환합니다.

조회 로직은 Namespace, Pod, Service만 읽습니다. Secret의 실제 값은 의도적으로 읽지 않습니다. 이 읽기 전용 원칙은 이후 Agent와 보안 점검 기능을 구현할 때도 유지합니다.

초기 구현 단계에서 Go 테스트, Next.js 린트, 프로덕션 빌드를 모두 통과했습니다. GitHub 원격 저장소의 `main`과 로컬 `main`도 동기화되어 있습니다.

## 3. 현재 개발 환경

Windows 로컬에는 Docker Desktop 드라이버를 사용하는 Minikube 프로필이 있습니다. 이 환경은 VMware VM 세 대로 구성한 클러스터와 별개의 개발용 클러스터입니다. Minikube에서는 하나의 노드가 Control Plane과 Worker 역할을 함께 수행합니다.

기본 네임스페이스에는 기존 Nginx Deployment와 Pod 세 개가 남아 있습니다. 프로젝트 작업 과정에서 해당 리소스를 수정하거나 삭제하지 않았습니다.

다만 2026-10-06 기준으로 `kubectl`은 `minikube` context를 가리키지만 Kubernetes API에 연결되지 않습니다. Docker Desktop 또는 Minikube가 중지된 상태로 판단됩니다. 다음 작업을 시작하기 전에 아래 순서로 상태를 복구하고 확인합니다.

```powershell
minikube start --driver=docker
kubectl get nodes
```

클러스터가 준비되면 API와 웹을 실행해 실제 리소스 개수가 화면에 표시되는지 확인합니다.

## 4. 현재 저장소 구성

```text
apps/web                 Next.js 기반 대시보드
services/api             Go, Gin, client-go 기반 조회 API
deploy/helm              Agent용 Helm Chart 위치 (현재 비어 있음)
docs                     아키텍처 및 진행 문서
```

## 5. 향후 구현 범위

현재 구현 이후에는 다음 기능을 순차적으로 추가합니다.

- Pod, Service, Namespace 관계 그래프
- 보안 점검 규칙과 이슈 상세 화면
- PostgreSQL과 점검 이력
- Dockerfile, Docker Compose
- 로그인, 사용자 Workspace, 여러 클러스터 관리
- Kubernetes Agent와 실제 Helm Chart
- GitHub Actions CI/CD와 클라우드 배포

## 6. 개발 로드맵

### 1단계. 실행 환경을 확인합니다

Minikube를 시작한 뒤 `kubectl get nodes`, `kubectl get pods -A`로 클러스터 상태를 확인합니다. 이어서 Go API와 웹 화면을 실행해 현재 클러스터의 리소스 개수가 정상적으로 표시되는지 검증합니다.

### 2단계. 전용 데모 리소스를 구성합니다

기존 Nginx 리소스에 의존하지 않고 `kubecompass-demo` 네임스페이스를 구성합니다. Deployment, Service, ServiceAccount 등 화면과 보안 점검에 필요한 최소 리소스를 YAML로 관리합니다. 이 YAML은 `deploy/demo`에 추가해 누구나 동일한 데모 환경을 구성할 수 있게 합니다.

### 3단계. 클러스터 지도 화면을 구현합니다

단순 개수 화면을 Namespace, Pod, Service 관계를 보여 주는 클러스터 지도 화면으로 확장합니다. API는 리소스 이름·라벨·소유 관계를 정리해 반환하고, 웹은 React Flow를 사용해 Namespace 내부의 Pod와 Service 연결을 시각화합니다. 화면에는 Kubernetes 용어뿐 아니라, 예를 들어 Pod 옆에 “실행 중인 앱”과 같은 쉬운 설명을 함께 제공합니다.

### 4단계. 핵심 보안 점검 3가지를 구현합니다

처음부터 모든 규칙을 구현하기보다, 이해하기 쉬운 세 가지 보안 점검부터 제공합니다.

- privileged 컨테이너 사용 여부를 확인합니다.
- CPU·메모리 요청 또는 제한 누락 여부를 확인합니다.
- 기본 ServiceAccount 사용 여부를 확인합니다.

각 이슈에는 위험도뿐 아니라 원인, 영향, 권장 조치, 수정 예시 YAML을 제공합니다. 이후 외부 노출 Service·Ingress, hostPath, 과도한 RBAC, NetworkPolicy 부재로 점검 범위를 넓힙니다.

### 5단계. 테스트와 로컬 실행 방식을 정리합니다

API의 리소스 변환 및 보안 규칙에 단위 테스트를 추가하고, 웹에도 핵심 화면 테스트를 추가합니다. 이후 웹과 API용 Dockerfile, Docker Compose를 구성해 한 번의 명령으로 로컬 개발 환경을 실행할 수 있게 합니다.

### 6단계. CI를 구축합니다

GitHub Actions에서 Pull Request와 `main` 브랜치 푸시마다 다음 검증을 수행합니다.

- 웹: 의존성 설치, 린트, 빌드를 수행합니다.
- API: `go test ./...`를 수행합니다.
- 배포 파일 추가 후: Docker 이미지 빌드, `helm lint`, `helm template`을 수행합니다.

이 단계에서는 자동 배포보다 변경이 기존 기능을 손상시키지 않는지 확인하는 CI를 우선합니다.

### 7단계. 데이터 저장과 사용자 구분을 추가합니다

PostgreSQL에 점검 결과와 이력을 저장합니다. 이후 로그인, Workspace, 클러스터 등록 기능을 추가해 사용자별로 자신의 클러스터 결과만 확인할 수 있게 합니다. 브라우저에는 kubeconfig를 저장하지 않습니다.

### 8단계. Agent와 Helm Chart를 구현합니다

사용자 클러스터에 설치되는 Agent를 별도 서비스로 구현합니다. Agent는 읽기 전용 ServiceAccount 권한으로 필요한 Kubernetes 정보만 조회하고, 중앙 API와 HTTPS로 통신합니다. Secret의 실제 값은 수집하지 않습니다.

Helm Chart에는 ServiceAccount, 최소 권한 RBAC, Agent Deployment, 필요한 Secret 참조를 포함합니다. 사용자는 자신이 관리하는 Minikube, VMware, EC2 기반 Kubernetes 또는 EKS 환경에 이 Chart를 설치할 수 있습니다.

### 9단계. 중앙 서비스를 배포합니다

웹, API, PostgreSQL을 외부에서 접근 가능한 환경에 배포합니다. 이후 GitHub Actions가 버전별 Docker 이미지와 Helm Chart를 레지스트리에 올리고, 중앙 서비스의 스테이징 환경만 자동으로 업데이트하도록 구성합니다. 사용자 클러스터에 Agent를 설치하는 작업은 클러스터 관리자가 직접 승인하고 실행합니다.

## 7. 바로 다음 작업

가장 먼저 Minikube를 다시 시작해 현재 구현한 API와 화면이 실제 클러스터 데이터를 정상적으로 읽는지 확인합니다. 확인이 완료되면 기존 리소스를 수정하지 않는 `kubecompass-demo` 네임스페이스와 데모 YAML을 추가합니다.
