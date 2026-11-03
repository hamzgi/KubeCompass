package cluster

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	kubernetesPSSURL      = "https://kubernetes.io/docs/concepts/security/pod-security-standards/"
	kubernetesRBACURL     = "https://kubernetes.io/docs/concepts/security/rbac-good-practices/"
	kubernetesResourceURL = "https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/"
	awsEKSNetworkURL      = "https://docs.aws.amazon.com/eks/latest/best-practices/network-security.html"
	awsEKSDataPlaneURL    = "https://docs.aws.amazon.com/eks/latest/best-practices/data-plane.html"
	cisKubernetesURL      = "https://www.cisecurity.org/benchmark/kubernetes"
	cvssNotApplicable     = "해당 없음 — CVE 기반 취약점이 아닌 Kubernetes 구성 위험입니다."
)

// SecurityReport contains explainable, read-only checks. The rules describe a
// setting and its impact instead of changing any cluster resource.
type SecurityReport struct {
	Issues     []SecurityIssue `json:"issues"`
	ObservedAt time.Time       `json:"observedAt"`
}

// SecurityReference explains the public standard or vendor guidance used to
// design a check. It does not claim complete framework certification.
type SecurityReference struct {
	Framework string `json:"framework"`
	Control   string `json:"control"`
	URL       string `json:"url"`
}

type SecurityIssue struct {
	ID             string              `json:"id"`
	RuleID         string              `json:"ruleId"`
	Severity       string              `json:"severity"`
	Category       string              `json:"category"`
	Assessment     string              `json:"assessment"`
	CVSS           string              `json:"cvss"`
	References     []SecurityReference `json:"references"`
	Title          string              `json:"title"`
	Resource       ResourceReference   `json:"resource"`
	Summary        string              `json:"summary"`
	Cause          string              `json:"cause"`
	Impact         string              `json:"impact"`
	Recommendation string              `json:"recommendation"`
	ExampleYAML    string              `json:"exampleYaml"`
}

type ResourceReference struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Container string `json:"container,omitempty"`
}

// SecurityIssues checks application resources outside system namespaces. It
// separates confirmed settings from exposure reviews because cloud networking
// cannot be proven from Kubernetes manifests alone.
func (c *Client) SecurityIssues(ctx context.Context) (SecurityReport, error) {
	podList, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return SecurityReport{}, fmt.Errorf("list pods for security checks: %w", err)
	}
	networkPolicyList, err := c.clientset.NetworkingV1().NetworkPolicies("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return SecurityReport{}, fmt.Errorf("list network policies for security checks: %w", err)
	}
	serviceList, err := c.clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return SecurityReport{}, fmt.Errorf("list services for security checks: %w", err)
	}
	ingressList, err := c.clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return SecurityReport{}, fmt.Errorf("list ingresses for security checks: %w", err)
	}
	roleList, err := c.clientset.RbacV1().Roles("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return SecurityReport{}, fmt.Errorf("list roles for security checks: %w", err)
	}
	clusterRoleList, err := c.clientset.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return SecurityReport{}, fmt.Errorf("list cluster roles for security checks: %w", err)
	}
	roleBindingList, err := c.clientset.RbacV1().RoleBindings("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return SecurityReport{}, fmt.Errorf("list role bindings for security checks: %w", err)
	}
	clusterRoleBindingList, err := c.clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return SecurityReport{}, fmt.Errorf("list cluster role bindings for security checks: %w", err)
	}

	issues := make([]SecurityIssue, 0)
	workloadNamespaces := make(map[string]struct{})
	for _, pod := range podList.Items {
		if isSystemNamespace(pod.Namespace) {
			continue
		}
		workloadNamespaces[pod.Namespace] = struct{}{}
		issues = append(issues, defaultServiceAccountIssue(pod)...)
		issues = append(issues, hostPathIssues(pod)...)
		for _, container := range workloadContainers(pod) {
			if container.SecurityContext != nil && container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
				issues = append(issues, privilegedContainerIssue(pod, container.Name))
			}
			if allowsPrivilegeEscalation(container) {
				issues = append(issues, privilegeEscalationIssue(pod, container.Name))
			}
			if !runsAsNonRoot(pod, container) {
				issues = append(issues, runAsRootIssue(pod, container.Name))
			}
			if missing := missingResourceSettings(container.Resources); len(missing) > 0 {
				issues = append(issues, missingResourcesIssue(pod, container.Name, missing))
			}
		}
	}

	issues = append(issues, missingNetworkPolicyIssues(workloadNamespaces, networkPolicyList.Items)...)
	issues = append(issues, serviceExposureIssues(serviceList.Items)...)
	issues = append(issues, ingressExposureIssues(ingressList.Items)...)
	issues = append(issues, rbacIssues(roleList.Items, clusterRoleList.Items, roleBindingList.Items, clusterRoleBindingList.Items)...)

	sort.Slice(issues, func(i, j int) bool {
		leftRank, rightRank := severityRank(issues[i].Severity), severityRank(issues[j].Severity)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if issues[i].Resource.Namespace != issues[j].Resource.Namespace {
			return issues[i].Resource.Namespace < issues[j].Resource.Namespace
		}
		return issues[i].ID < issues[j].ID
	})
	return SecurityReport{Issues: issues, ObservedAt: time.Now().UTC()}, nil
}

func workloadContainers(pod corev1.Pod) []corev1.Container {
	containers := make([]corev1.Container, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers))
	containers = append(containers, pod.Spec.InitContainers...)
	containers = append(containers, pod.Spec.Containers...)
	return containers
}

func standard(framework, control, url string) []SecurityReference {
	return []SecurityReference{{Framework: framework, Control: control, URL: url}}
}

func defaultServiceAccountIssue(pod corev1.Pod) []SecurityIssue {
	if pod.Spec.ServiceAccountName != "" && pod.Spec.ServiceAccountName != "default" {
		return nil
	}
	return []SecurityIssue{{
		ID: fmt.Sprintf("default-service-account:%s/%s", pod.Namespace, pod.Name), RuleID: "default-service-account", Severity: "low", Category: "RBAC 최소 권한", Assessment: "검토 필요", CVSS: cvssNotApplicable,
		References: []SecurityReference{{Framework: "Kubernetes RBAC Good Practices", Control: "최소 권한 및 강한 권한 토큰 배포 최소화", URL: kubernetesRBACURL}, {Framework: "CIS Kubernetes Benchmark", Control: "RBAC·서비스 계정 권한 검토 기준", URL: cisKubernetesURL}},
		Title:      "기본 ServiceAccount를 사용합니다", Resource: ResourceReference{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name},
		Summary: "이 Pod는 별도 계정 대신 default ServiceAccount를 사용하고 있습니다.", Cause: "Pod의 serviceAccountName이 비어 있거나 default로 설정되어 있습니다.",
		Impact: "기본 계정에 권한이 추가되면 의도하지 않은 앱도 같은 권한을 가질 수 있습니다.", Recommendation: "앱 전용 ServiceAccount를 만들고 필요한 권한만 Role과 RoleBinding으로 부여합니다.",
		ExampleYAML: "apiVersion: v1\nkind: ServiceAccount\nmetadata:\n  name: app-reader\n---\nspec:\n  serviceAccountName: app-reader",
	}}
}

func privilegedContainerIssue(pod corev1.Pod, containerName string) SecurityIssue {
	return SecurityIssue{
		ID: fmt.Sprintf("privileged-container:%s/%s/%s", pod.Namespace, pod.Name, containerName), RuleID: "privileged-container", Severity: "high", Category: "Pod 보안", Assessment: "확인됨", CVSS: cvssNotApplicable,
		References: []SecurityReference{{Framework: "Kubernetes Pod Security Standards", Control: "Baseline/Restricted: privileged 컨테이너 금지", URL: kubernetesPSSURL}, {Framework: "CIS Kubernetes Benchmark", Control: "워크로드 하드닝 참고 기준", URL: cisKubernetesURL}},
		Title:      "privileged 컨테이너가 실행 중입니다", Resource: ResourceReference{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name, Container: containerName},
		Summary: "컨테이너가 privileged 모드로 실행되고 있습니다.", Cause: "securityContext.privileged가 true로 설정되어 있습니다.",
		Impact: "컨테이너가 노드의 장치와 커널 기능에 넓게 접근할 수 있어 침해 시 영향 범위가 커질 수 있습니다.", Recommendation: "특별한 이유가 없다면 privileged를 제거하고 필요한 Linux capability만 개별적으로 허용합니다.",
		ExampleYAML: "securityContext:\n  privileged: false\n  allowPrivilegeEscalation: false",
	}
}

func privilegeEscalationIssue(pod corev1.Pod, containerName string) SecurityIssue {
	return SecurityIssue{
		ID: fmt.Sprintf("allow-privilege-escalation:%s/%s/%s", pod.Namespace, pod.Name, containerName), RuleID: "allow-privilege-escalation", Severity: "medium", Category: "Pod 보안", Assessment: "확인됨", CVSS: cvssNotApplicable,
		References: standard("Kubernetes Pod Security Standards", "Restricted: allowPrivilegeEscalation은 false", kubernetesPSSURL),
		Title:      "권한 상승 차단이 명시되지 않았습니다", Resource: ResourceReference{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name, Container: containerName},
		Summary: "컨테이너의 allowPrivilegeEscalation이 false로 설정되지 않았습니다.", Cause: "securityContext.allowPrivilegeEscalation이 true이거나 비어 있습니다.",
		Impact: "컨테이너 내부 프로세스가 추가 권한을 얻을 수 있는 여지가 커질 수 있습니다.", Recommendation: "필요한 예외가 없다면 allowPrivilegeEscalation을 false로 설정합니다.",
		ExampleYAML: "securityContext:\n  allowPrivilegeEscalation: false",
	}
}

func runAsRootIssue(pod corev1.Pod, containerName string) SecurityIssue {
	return SecurityIssue{
		ID: fmt.Sprintf("run-as-non-root:%s/%s/%s", pod.Namespace, pod.Name, containerName), RuleID: "run-as-non-root", Severity: "medium", Category: "Pod 보안", Assessment: "확인됨", CVSS: cvssNotApplicable,
		References: standard("Kubernetes Pod Security Standards", "Restricted: runAsNonRoot는 true", kubernetesPSSURL),
		Title:      "비루트 실행이 보장되지 않습니다", Resource: ResourceReference{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name, Container: containerName},
		Summary: "Pod 또는 컨테이너에 runAsNonRoot: true 설정이 없습니다.", Cause: "컨테이너 보안 컨텍스트에서 비루트 실행을 명시하지 않았습니다.",
		Impact: "이미지 기본 사용자가 root라면 컨테이너 내부 권한이 불필요하게 커질 수 있습니다.", Recommendation: "이미지가 비루트 사용자를 지원하는지 확인한 뒤 runAsNonRoot와 runAsUser를 설정합니다.",
		ExampleYAML: "securityContext:\n  runAsNonRoot: true\n  runAsUser: 1000",
	}
}

func hostPathIssues(pod corev1.Pod) []SecurityIssue {
	issues := make([]SecurityIssue, 0)
	for _, volume := range pod.Spec.Volumes {
		if volume.HostPath == nil {
			continue
		}
		issues = append(issues, SecurityIssue{
			ID: fmt.Sprintf("hostpath-volume:%s/%s/%s", pod.Namespace, pod.Name, volume.Name), RuleID: "hostpath-volume", Severity: "high", Category: "Pod 보안", Assessment: "확인됨", CVSS: cvssNotApplicable,
			References: standard("Kubernetes Pod Security Standards", "Restricted: hostPath 볼륨 금지", kubernetesPSSURL),
			Title:      "노드 파일 시스템을 마운트합니다", Resource: ResourceReference{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name},
			Summary: fmt.Sprintf("%q hostPath 볼륨이 노드 경로 %q를 참조합니다.", volume.Name, volume.HostPath.Path), Cause: "Pod volume에 hostPath가 설정되어 있습니다.",
			Impact: "컨테이너가 노드 파일에 접근하거나 변경할 수 있어 노드 침해로 이어질 수 있습니다.", Recommendation: "가능하면 PersistentVolumeClaim, ConfigMap, Secret 등 목적에 맞는 볼륨으로 대체합니다.",
			ExampleYAML: "volumes:\n  - name: app-data\n    persistentVolumeClaim:\n      claimName: app-data-pvc",
		})
	}
	return issues
}

func missingResourcesIssue(pod corev1.Pod, containerName string, missing []string) SecurityIssue {
	return SecurityIssue{
		ID: fmt.Sprintf("missing-resource-settings:%s/%s/%s", pod.Namespace, pod.Name, containerName), RuleID: "missing-resource-settings", Severity: "medium", Category: "가용성·운영 안정성", Assessment: "확인됨", CVSS: cvssNotApplicable,
		References: []SecurityReference{{Framework: "Kubernetes Resource Management", Control: "컨테이너 CPU·메모리 요청량과 제한값 관리", URL: kubernetesResourceURL}, {Framework: "AWS EKS Best Practices", Control: "워크로드별 리소스 요청량·제한값 구성", URL: awsEKSDataPlaneURL}},
		Title:      "필수 CPU·메모리 기준이 비어 있습니다", Resource: ResourceReference{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name, Container: containerName},
		Summary: fmt.Sprintf("%s 설정이 없습니다.", strings.Join(missing, ", ")), Cause: "CPU request 또는 메모리 request·limit가 모두 설정되지 않았습니다.",
		Impact: "스케줄링 예측이 어려워지고 메모리 압박 상황에서 Pod가 종료되거나 다른 워크로드에 영향을 줄 수 있습니다.", Recommendation: "실제 사용량을 관찰한 뒤 CPU request와 메모리 request·limit를 설정합니다. AWS EKS 가이드는 CPU limit을 일률적으로 요구하지 않습니다.",
		ExampleYAML: "resources:\n  requests:\n    cpu: 100m\n    memory: 128Mi\n  limits:\n    memory: 256Mi",
	}
}

func missingResourceSettings(resources corev1.ResourceRequirements) []string {
	checks := []struct {
		name   string
		values corev1.ResourceList
		key    corev1.ResourceName
	}{
		{name: "CPU request", values: resources.Requests, key: corev1.ResourceCPU},
		{name: "메모리 request", values: resources.Requests, key: corev1.ResourceMemory},
		{name: "메모리 limit", values: resources.Limits, key: corev1.ResourceMemory},
	}
	missing := make([]string, 0, len(checks))
	for _, check := range checks {
		if _, exists := check.values[check.key]; !exists {
			missing = append(missing, check.name)
		}
	}
	return missing
}

func missingNetworkPolicyIssues(workloadNamespaces map[string]struct{}, policies []networkingv1.NetworkPolicy) []SecurityIssue {
	namespacesWithPolicy := make(map[string]struct{})
	for _, policy := range policies {
		namespacesWithPolicy[policy.Namespace] = struct{}{}
	}
	issues := make([]SecurityIssue, 0)
	for namespace := range workloadNamespaces {
		if _, exists := namespacesWithPolicy[namespace]; exists {
			continue
		}
		issues = append(issues, SecurityIssue{
			ID: fmt.Sprintf("missing-network-policy:%s", namespace), RuleID: "missing-network-policy", Severity: "medium", Category: "네트워크 격리", Assessment: "검토 필요", CVSS: cvssNotApplicable,
			References: standard("AWS EKS Best Practices", "Kubernetes NetworkPolicy로 Pod 간 트래픽 제한", awsEKSNetworkURL),
			Title:      "Namespace에 NetworkPolicy가 없습니다", Resource: ResourceReference{Kind: "Namespace", Name: namespace},
			Summary: "이 Namespace에는 하나 이상의 Pod가 있지만 NetworkPolicy가 없습니다.", Cause: "NetworkPolicy 리소스가 정의되지 않았습니다.",
			Impact: "CNI가 NetworkPolicy를 지원하는 환경에서는 기본 허용 통신으로 인해 Pod 간 트래픽이 과도하게 열릴 수 있습니다.", Recommendation: "먼저 default-deny 정책을 검토하고, 필요한 DNS·Service·Pod 통신만 명시적으로 허용합니다.",
			ExampleYAML: "apiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata:\n  name: default-deny\nspec:\n  podSelector: {}\n  policyTypes:\n    - Ingress\n    - Egress",
		})
	}
	return issues
}

func serviceExposureIssues(services []corev1.Service) []SecurityIssue {
	issues := make([]SecurityIssue, 0)
	for _, service := range services {
		if isSystemNamespace(service.Namespace) || (service.Spec.Type != corev1.ServiceTypeLoadBalancer && service.Spec.Type != corev1.ServiceTypeNodePort) {
			continue
		}
		issues = append(issues, SecurityIssue{
			ID: fmt.Sprintf("service-exposure:%s/%s", service.Namespace, service.Name), RuleID: "service-exposure-review", Severity: "medium", Category: "네트워크 노출", Assessment: "검토 필요", CVSS: cvssNotApplicable,
			References: standard("AWS EKS Best Practices", "클러스터 내부·외부 네트워크 접근 제어", awsEKSNetworkURL),
			Title:      "외부 노출 가능 Service를 검토하세요", Resource: ResourceReference{Kind: "Service", Namespace: service.Namespace, Name: service.Name},
			Summary: fmt.Sprintf("Service type이 %s입니다.", service.Spec.Type), Cause: "NodePort 또는 LoadBalancer Service는 클러스터 외부 경로를 만들 수 있습니다.",
			Impact: "실제 노출 범위는 클라우드 보안 그룹, 방화벽, Load Balancer 설정에 따라 달라집니다.", Recommendation: "인터넷 공개가 필요한지 확인하고, 필요한 CIDR·포트만 허용하며 인증과 TLS를 적용합니다.",
			ExampleYAML: "spec:\n  type: ClusterIP",
		})
	}
	return issues
}

func ingressExposureIssues(ingresses []networkingv1.Ingress) []SecurityIssue {
	issues := make([]SecurityIssue, 0)
	for _, ingress := range ingresses {
		if isSystemNamespace(ingress.Namespace) {
			continue
		}
		issues = append(issues, SecurityIssue{
			ID: fmt.Sprintf("ingress-exposure:%s/%s", ingress.Namespace, ingress.Name), RuleID: "ingress-exposure-review", Severity: "medium", Category: "네트워크 노출", Assessment: "검토 필요", CVSS: cvssNotApplicable,
			References: standard("AWS EKS Best Practices", "클러스터 내부·외부 네트워크 접근 제어", awsEKSNetworkURL),
			Title:      "Ingress 외부 노출 범위를 검토하세요", Resource: ResourceReference{Kind: "Ingress", Namespace: ingress.Namespace, Name: ingress.Name},
			Summary: "Ingress가 HTTP(S) 요청을 Service로 라우팅합니다.", Cause: "Ingress 리소스가 정의되어 있습니다.",
			Impact: "인터넷 노출 여부와 접근 범위는 Ingress Controller와 Load Balancer 설정에 따라 달라집니다.", Recommendation: "TLS, 인증, 허용 CIDR, WAF 및 Ingress Controller 설정을 함께 점검합니다.",
			ExampleYAML: "spec:\n  ingressClassName: nginx\n  tls:\n    - hosts:\n        - app.example.com",
		})
	}
	return issues
}

func rbacIssues(roles []rbacv1.Role, clusterRoles []rbacv1.ClusterRole, roleBindings []rbacv1.RoleBinding, clusterRoleBindings []rbacv1.ClusterRoleBinding) []SecurityIssue {
	rulesByReference := make(map[string][]rbacv1.PolicyRule)
	for _, role := range roles {
		rulesByReference[roleKey(role.Namespace, role.Name)] = role.Rules
	}
	for _, clusterRole := range clusterRoles {
		rulesByReference[clusterRoleKey(clusterRole.Name)] = clusterRole.Rules
	}
	issues := make([]SecurityIssue, 0)
	for _, binding := range roleBindings {
		issues = append(issues, bindingIssues("RoleBinding", binding.Namespace, binding.Name, binding.RoleRef, binding.Subjects, binding.Namespace, rulesByReference)...)
	}
	for _, binding := range clusterRoleBindings {
		issues = append(issues, bindingIssues("ClusterRoleBinding", "", binding.Name, binding.RoleRef, binding.Subjects, "", rulesByReference)...)
	}
	return issues
}

func bindingIssues(bindingKind, bindingNamespace, bindingName string, roleRef rbacv1.RoleRef, subjects []rbacv1.Subject, defaultNamespace string, rulesByReference map[string][]rbacv1.PolicyRule) []SecurityIssue {
	referenceKey := clusterRoleKey(roleRef.Name)
	if roleRef.Kind == "Role" {
		referenceKey = roleKey(bindingNamespace, roleRef.Name)
	}
	rules, issues := rulesByReference[referenceKey], make([]SecurityIssue, 0)
	for _, subject := range subjects {
		if subject.Kind != rbacv1.ServiceAccountKind {
			continue
		}
		subjectNamespace := subject.Namespace
		if subjectNamespace == "" {
			subjectNamespace = defaultNamespace
		}
		if subjectNamespace == "" || isSystemNamespace(subjectNamespace) {
			continue
		}
		resource := ResourceReference{Kind: bindingKind, Namespace: subjectNamespace, Name: bindingName}
		if roleRef.Kind == "ClusterRole" && roleRef.Name == "cluster-admin" {
			issues = append(issues, SecurityIssue{
				ID: fmt.Sprintf("cluster-admin-binding:%s/%s:%s/%s", bindingKind, bindingName, subjectNamespace, subject.Name), RuleID: "cluster-admin-service-account", Severity: "high", Category: "RBAC 최소 권한", Assessment: "확인됨", CVSS: cvssNotApplicable,
				References: []SecurityReference{{Framework: "Kubernetes RBAC Good Practices", Control: "cluster-admin은 필요한 경우에만 사용", URL: kubernetesRBACURL}, {Framework: "CIS Kubernetes Benchmark", Control: "RBAC 최소 권한 검토 기준", URL: cisKubernetesURL}},
				Title:      "ServiceAccount에 cluster-admin이 부여되었습니다", Resource: resource, Summary: fmt.Sprintf("%s/%s가 cluster-admin ClusterRole을 사용합니다.", subjectNamespace, subject.Name),
				Cause: "ClusterRoleBinding 또는 RoleBinding이 cluster-admin을 ServiceAccount에 연결합니다.", Impact: "Pod가 탈취되면 클러스터 전체 리소스를 변경하거나 삭제할 수 있습니다.",
				Recommendation: "전용 Role 또는 ClusterRole에 필요한 API·리소스·동작만 명시하고, 가능하면 Namespace 범위 RoleBinding을 사용합니다.", ExampleYAML: "roleRef:\n  apiGroup: rbac.authorization.k8s.io\n  kind: Role\n  name: app-reader",
			})
			continue
		}
		if hasWildcardRule(rules) {
			issues = append(issues, SecurityIssue{
				ID: fmt.Sprintf("rbac-wildcard:%s/%s:%s/%s", bindingKind, bindingName, subjectNamespace, subject.Name), RuleID: "rbac-wildcard-permission", Severity: "high", Category: "RBAC 최소 권한", Assessment: "확인됨", CVSS: cvssNotApplicable,
				References: []SecurityReference{{Framework: "Kubernetes RBAC Good Practices", Control: "와일드카드 권한 사용 회피", URL: kubernetesRBACURL}, {Framework: "CIS Kubernetes Benchmark", Control: "RBAC 최소 권한 검토 기준", URL: cisKubernetesURL}},
				Title:      "ServiceAccount에 와일드카드 RBAC 권한이 있습니다", Resource: resource, Summary: fmt.Sprintf("%s/%s가 * API·리소스 또는 동작을 포함한 권한을 받습니다.", subjectNamespace, subject.Name),
				Cause: "연결된 Role 또는 ClusterRole에 와일드카드 규칙이 있습니다.", Impact: "새 API 리소스가 추가돼도 권한이 자동으로 확대되어 최소 권한 원칙을 지키기 어렵습니다.",
				Recommendation: "apiGroups, resources, verbs를 필요한 값으로 각각 명시합니다.", ExampleYAML: "rules:\n  - apiGroups: [\"\"]\n    resources: [\"configmaps\"]\n    verbs: [\"get\", \"list\"]",
			})
			continue
		}
		if hasBroadSecretRead(rules) {
			issues = append(issues, SecurityIssue{
				ID: fmt.Sprintf("rbac-secret-read:%s/%s:%s/%s", bindingKind, bindingName, subjectNamespace, subject.Name), RuleID: "rbac-broad-secret-access", Severity: "high", Category: "민감 정보 접근", Assessment: "확인됨", CVSS: cvssNotApplicable,
				References: []SecurityReference{{Framework: "Kubernetes Secrets Good Practices", Control: "Secret get·list·watch 권한 제한", URL: "https://kubernetes.io/docs/concepts/security/secrets-good-practices/"}, {Framework: "Kubernetes RBAC Good Practices", Control: "최소 권한", URL: kubernetesRBACURL}},
				Title:      "ServiceAccount가 Secret을 넓게 읽을 수 있습니다", Resource: resource, Summary: fmt.Sprintf("%s/%s가 Secret의 get, list 또는 watch 권한을 받습니다.", subjectNamespace, subject.Name),
				Cause: "연결된 Role 또는 ClusterRole이 Secret 읽기 권한을 포함합니다.", Impact: "Pod가 탈취되면 같은 범위의 자격 증명과 토큰이 노출될 수 있습니다.",
				Recommendation: "정말 필요한 Secret 이름과 동작만 허용하고 list·watch는 가능한 한 피합니다.", ExampleYAML: "rules:\n  - apiGroups: [\"\"]\n    resources: [\"secrets\"]\n    resourceNames: [\"app-config\"]\n    verbs: [\"get\"]",
			})
		}
	}
	return issues
}

func roleKey(namespace, name string) string { return fmt.Sprintf("role:%s/%s", namespace, name) }
func clusterRoleKey(name string) string     { return "clusterrole:" + name }

func hasWildcardRule(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		if includes(rule.APIGroups, "*") || includes(rule.Resources, "*") || includes(rule.Verbs, "*") {
			return true
		}
	}
	return false
}

func hasBroadSecretRead(rules []rbacv1.PolicyRule) bool {
	for _, rule := range rules {
		if (!includes(rule.APIGroups, "") && !includes(rule.APIGroups, "*")) || (!includes(rule.Resources, "secrets") && !includes(rule.Resources, "*")) {
			continue
		}
		if includes(rule.Verbs, "get") || includes(rule.Verbs, "list") || includes(rule.Verbs, "watch") || includes(rule.Verbs, "*") {
			return true
		}
	}
	return false
}

func includes(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func allowsPrivilegeEscalation(container corev1.Container) bool {
	return container.SecurityContext == nil || container.SecurityContext.AllowPrivilegeEscalation == nil || *container.SecurityContext.AllowPrivilegeEscalation
}

func runsAsNonRoot(pod corev1.Pod, container corev1.Container) bool {
	if container.SecurityContext != nil && container.SecurityContext.RunAsNonRoot != nil {
		return *container.SecurityContext.RunAsNonRoot
	}
	return pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.RunAsNonRoot != nil && *pod.Spec.SecurityContext.RunAsNonRoot
}

func severityRank(severity string) int {
	switch severity {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}
