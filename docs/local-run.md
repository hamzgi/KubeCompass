# KubeCompass 로컬 실행 및 브라우저 접속

KubeCompass는 Kubernetes 클러스터(Minikube), Go API, Next.js 웹 서버가 모두 실행되어야 실제 리소스 정보를 화면에 표시합니다. 아래 순서는 Windows의 Git Bash 기준입니다.

## 실행 전 준비

- Docker Desktop을 실행합니다.
- `minikube`, `kubectl`, `go`, `node`, `npm`이 설치되어 있어야 합니다.
- 저장소 위치는 `~/Desktop/멋사/KubeCompass`를 기준으로 설명합니다.

## 1. Minikube 시작

첫 번째 터미널에서 다음 명령어를 실행합니다.

```bash
cd ~/Desktop/멋사/KubeCompass
minikube start
minikube status
```

아래처럼 `host`, `kubelet`, `apiserver`, `kubeconfig`가 모두 정상 상태여야 합니다.

```text
host: Running
kubelet: Running
apiserver: Running
kubeconfig: Configured
```

`kubeconfig: Misconfigured` 또는 `kubectl is pointing to stale minikube-vm` 메시지가 표시되면 다음 명령어를 실행한 뒤 상태를 다시 확인합니다.

```bash
minikube update-context
minikube status
```

## 2. Go API 실행

두 번째 터미널에서 API를 실행합니다.

```bash
cd ~/Desktop/멋사/KubeCompass/services/api
go run ./cmd/api
```

다음 메시지가 나타나고 터미널이 실행 상태로 유지되면 정상입니다.

```text
Listening and serving HTTP on :8080
```

API가 정상인지 별도 터미널에서 확인할 수 있습니다.

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/api/v1/summary
```

## 3. 웹 대시보드 실행

세 번째 터미널에서 Next.js 웹 서버를 실행합니다.

```bash
cd ~/Desktop/멋사/KubeCompass/apps/web
npm run dev
```

`npm run dev`는 저장소 최상위 폴더에서 실행하면 안 됩니다. 최상위 폴더에는 `package.json`이 없으므로 반드시 `apps/web` 폴더로 이동해야 합니다.

실행이 끝나면 다음과 비슷한 주소가 출력됩니다.

```text
Local: http://localhost:3000
```

3000번 포트를 이미 다른 프로그램이 사용 중이면 Next.js가 다른 포트를 제안하거나 사용합니다. 이때는 터미널에 표시된 `Local` 주소를 사용합니다.

## 4. 브라우저에서 열기

브라우저 주소창에 웹 서버가 출력한 주소를 입력합니다. 일반적인 기본 주소는 다음과 같습니다.

```text
http://localhost:3000
```

홈 화면에 Namespace, Pod, Service 수치가 표시되면 정상적으로 연결된 것입니다. 상단 메뉴에서 클러스터 맵과 보안 설정 확인 화면으로 이동할 수 있습니다.

## 자주 발생하는 문제

### `npm error enoent Could not read package.json`

현재 위치가 저장소 최상위 폴더인 경우입니다. 아래처럼 웹 프로젝트 폴더로 이동합니다.

```bash
cd ~/Desktop/멋사/KubeCompass/apps/web
npm run dev
```

### `listen tcp :8080: bind: Only one usage ...`

이미 다른 프로세스가 API 기본 포트인 8080을 사용하고 있다는 뜻입니다. 먼저 해당 프로세스를 확인합니다.

```bash
netstat -ano | grep ":8080"
```

출력의 마지막 숫자가 PID입니다. 프로세스 이름과 실행 경로는 다음 명령어로 확인합니다.

```bash
powershell.exe -NoProfile -Command "Get-CimInstance Win32_Process -Filter 'ProcessId=PID' | Select-Object ProcessId,Name,CommandLine | Format-List"
```

위 명령어의 `PID`를 실제 숫자로 바꿉니다. KubeCompass의 기존 `api.exe` 또는 `go.exe`라면 이미 API가 실행 중일 수 있으므로 그대로 사용해도 됩니다. 완전히 다시 시작하려면 해당 PID를 종료한 뒤 API 명령어를 다시 실행합니다.

```bash
taskkill /PID PID /F
```

### 웹 화면이 `연결 중` 또는 `확인 중`에서 멈춤

아래 순서로 확인합니다.

1. `minikube status`에서 Kubernetes 구성 요소가 모두 `Running`인지 확인합니다.
2. `curl http://127.0.0.1:8080/api/v1/summary`가 JSON을 반환하는지 확인합니다.
3. API와 웹 서버 터미널을 모두 종료한 뒤, 이 문서의 1~4단계를 순서대로 다시 실행합니다.
4. 브라우저에서 `Ctrl + Shift + R`로 강력 새로고침합니다.

## 종료 방법

웹 서버와 API는 각각 실행 중인 터미널에서 `Ctrl + C`를 눌러 종료합니다. Kubernetes 실습 환경까지 중지하려면 다음 명령어를 실행합니다.

```bash
minikube stop
```

`minikube stop`은 클러스터를 삭제하지 않고 중지합니다. 다시 사용할 때는 `minikube start`를 실행하면 됩니다.
