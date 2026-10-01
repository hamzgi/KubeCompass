# 초기 아키텍처

## MVP 흐름

1. 사용자는 웹 대시보드에서 자신이 등록한 클러스터의 상태를 확인합니다.
2. 클러스터 내부 Agent는 읽기 전용 권한으로 Kubernetes API를 조회합니다.
3. Agent는 리소스 관계와 점검 결과를 중앙 API로 전송합니다.
4. 웹 대시보드는 사용자별 클러스터 정보와 쉬운 설명을 제공합니다.

## MVP 범위

- Pod, Service, Namespace, Deployment, ServiceAccount, RBAC, NetworkPolicy 조회
- 클러스터 구조 시각화
- 대표적인 설정 위험 점검
- 리소스 변경 및 삭제 기능 제외
