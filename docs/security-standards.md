# KubeCompass 보안 점검 기준

KubeCompass는 Kubernetes 리소스를 읽기 전용으로 조회하여 설정 위험을 설명합니다. 이 문서는 각 규칙의 기준과 한계를 명확히 하기 위한 문서입니다.

## 적용 원칙

- Kubernetes Pod Security Standards, Kubernetes RBAC Good Practices, AWS EKS Best Practices를 규칙 설계의 근거로 사용합니다.
- CIS Kubernetes Benchmark는 클러스터 하드닝 기준을 참고하는 기준선으로 표시합니다. 개별 규칙 통과만으로 전체 CIS 준수 인증을 의미하지는 않습니다.
- CVSS는 CVE처럼 소프트웨어·하드웨어·펌웨어 취약점의 특성을 점수화하는 체계입니다. Kubernetes 설정 위험에는 임의 CVSS 점수나 벡터를 만들지 않고 `해당 없음`으로 표시합니다.
- `확인됨`은 Kubernetes API의 설정값만으로 판정할 수 있는 항목입니다. `검토 필요`는 클라우드 방화벽, CNI, Load Balancer, 실제 트래픽 등 클러스터 밖의 정보가 필요한 항목입니다.

## 현재 규칙

| Rule ID | 점검 대상 | 위험도 | 판정 | 주요 기준 |
| --- | --- | --- | --- | --- |
| `privileged-container` | `securityContext.privileged: true` | High | 확인됨 | Kubernetes Pod Security Standards Baseline/Restricted |
| `allow-privilege-escalation` | `allowPrivilegeEscalation`이 false가 아님 | Medium | 확인됨 | Kubernetes Pod Security Standards Restricted |
| `run-as-non-root` | `runAsNonRoot: true` 미지정 | Medium | 확인됨 | Kubernetes Pod Security Standards Restricted |
| `hostpath-volume` | `hostPath` 볼륨 사용 | High | 확인됨 | Kubernetes Pod Security Standards Restricted |
| `default-service-account` | 기본 ServiceAccount 사용 | Low | 검토 필요 | Kubernetes RBAC Good Practices, CIS 참고 |
| `rbac-wildcard-permission` | ServiceAccount에 `*` 권한 부여 | High | 확인됨 | Kubernetes RBAC Good Practices, CIS 참고 |
| `cluster-admin-service-account` | ServiceAccount에 `cluster-admin` 부여 | High | 확인됨 | Kubernetes RBAC Good Practices, CIS 참고 |
| `rbac-broad-secret-access` | Secret `get`, `list`, `watch` 권한 | High | 확인됨 | Kubernetes Secrets Good Practices |
| `missing-network-policy` | 워크로드 Namespace에 NetworkPolicy 없음 | Medium | 검토 필요 | AWS EKS Network Security |
| `service-exposure-review` | NodePort 또는 LoadBalancer Service | Medium | 검토 필요 | AWS EKS Network Security |
| `ingress-exposure-review` | Ingress 존재 | Medium | 검토 필요 | AWS EKS Network Security |
| `missing-resource-settings` | CPU request 또는 메모리 request·limit 없음 | Medium | 확인됨 | Kubernetes Resource Management, AWS EKS Data Plane |

## 리소스 설정 규칙의 해석

KubeCompass는 CPU request, 메모리 request, 메모리 limit이 없는 경우를 표시합니다. AWS EKS Best Practices는 CPU limit을 모든 워크로드에 일률적으로 요구하지 않으므로, CPU limit 미설정만으로는 이슈를 만들지 않습니다. 실제 값은 워크로드의 사용량과 서비스 요구사항을 관찰한 뒤 결정해야 합니다.

## CVSS 및 이미지 취약점 범위

CVSS 점수는 특정 CVE와 취약한 패키지·이미지 버전에 연결될 때만 표시해야 합니다. 따라서 현재의 구성 점검 결과에는 CVSS를 사용하지 않습니다. 향후 Trivy 같은 이미지 취약점 스캐너를 연동하면 다음 정보를 별도 취약점 영역에 추가할 수 있습니다.

- CVE ID
- CVSS v4 점수와 벡터
- 영향을 받는 이미지·패키지·버전
- 수정 버전과 공급자 권고

## 참고 문서

- [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
- [Kubernetes RBAC Good Practices](https://kubernetes.io/docs/concepts/security/rbac-good-practices/)
- [Kubernetes Secrets Good Practices](https://kubernetes.io/docs/concepts/security/secrets-good-practices/)
- [Kubernetes Resource Management](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)
- [AWS EKS Network Security Best Practices](https://docs.aws.amazon.com/eks/latest/best-practices/network-security.html)
- [AWS EKS Data Plane Best Practices](https://docs.aws.amazon.com/eks/latest/best-practices/data-plane.html)
- [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes)
- [FIRST CVSS v4.0 Specification](https://www.first.org/cvss/v4.0/specification-document)
