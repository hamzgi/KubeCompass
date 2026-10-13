# KubeCompass 데모 리소스

이 디렉터리는 기존 클러스터 리소스를 변경하지 않고 KubeCompass 화면을 확인하기 위한 전용 실습 환경입니다.

```powershell
kubectl apply -f deploy/demo/kubecompass-demo.yaml
```

`kubecompass-demo` 네임스페이스에는 두 가지 리소스가 만들어집니다.

- `demo-web`: ServiceAccount와 CPU·메모리 기준을 갖춘 정상 예시 앱입니다.
- `security-lab`: 보안 점검 화면을 확인하기 위한 의도적인 취약 설정 예시입니다. 로컬 학습용 Minikube에서만 사용합니다.

실습이 끝난 뒤에는 아래 명령으로 전용 네임스페이스만 삭제할 수 있습니다.

```powershell
kubectl delete -f deploy/demo/kubecompass-demo.yaml
```
