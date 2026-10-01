# KubeCompass

Kubernetes 초보자가 자신의 클러스터 구조와 대표적인 보안 설정을 웹 대시보드에서 이해할 수 있도록 돕는 관찰 도구입니다.

## 초기 프로젝트 구조

- `apps/web`: Next.js 기반 웹 대시보드
- `services/api`: Kubernetes 리소스 수집 및 분석 API
- `deploy/helm`: Agent와 서비스 배포를 위한 Helm Chart
- `docs`: 아키텍처와 개발 문서

## 개발 원칙

- 초기 버전은 읽기 전용으로 Kubernetes 리소스를 관찰합니다.
- 클러스터 인증 정보와 Secret 값은 중앙 서버에 저장하지 않습니다.
- Minikube, 자체 구성 Kubernetes, EKS에서 공통 Kubernetes API 기반 기능을 제공합니다.

## 현재 상태

프로젝트 골격과 Git 저장소를 준비했습니다. 다음 단계에서 웹, API, Helm Chart를 순서대로 구성합니다.
