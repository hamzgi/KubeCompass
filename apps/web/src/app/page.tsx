"use client";

import { useEffect, useState } from "react";

type ClusterSummary = {
  namespaces: number;
  pods: number;
  services: number;
  observedAt: string;
};

type LoadState = "loading" | "ready" | "error";

const apiBaseUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export default function Home() {
  const [summary, setSummary] = useState<ClusterSummary | null>(null);
  const [loadState, setLoadState] = useState<LoadState>("loading");
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    const controller = new AbortController();

    async function loadSummary() {
      setLoadState("loading");

      try {
        const response = await fetch(`${apiBaseUrl}/api/v1/summary`, {
          signal: controller.signal,
          cache: "no-store",
        });

        if (!response.ok) {
          throw new Error("The cluster summary could not be loaded.");
        }

        const nextSummary = (await response.json()) as ClusterSummary;
        setSummary(nextSummary);
        setLoadState("ready");
    } catch {
        if (!controller.signal.aborted) {
          setSummary(null);
          setLoadState("error");
        }
      }
    }

    void loadSummary();

    return () => controller.abort();
  }, [reloadKey]);

  const cards = [
    { label: "작업 공간", englishLabel: "Namespace", value: summary?.namespaces },
    { label: "실행 중인 앱", englishLabel: "Pod", value: summary?.pods },
    { label: "서비스 접속 주소", englishLabel: "Service", value: summary?.services },
  ];

  const observedAt = summary
    ? new Intl.DateTimeFormat("ko-KR", {
        dateStyle: "medium",
        timeStyle: "medium",
      }).format(new Date(summary.observedAt))
    : null;

  return (
    <main className="min-h-screen bg-slate-50 px-6 py-10 text-slate-950 sm:px-10">
      <div className="mx-auto max-w-6xl">
        <header className="flex flex-col gap-5 border-b border-slate-200 pb-8 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="text-sm font-semibold tracking-[0.2em] text-blue-700">KUBECOMPASS</p>
            <h1 className="mt-3 text-3xl font-bold tracking-tight sm:text-4xl">내 클러스터 한눈에 보기</h1>
            <p className="mt-3 max-w-2xl text-base leading-7 text-slate-600">
              Kubernetes 리소스를 쉬운 이름으로 보여주는 읽기 전용 학습 대시보드입니다.
            </p>
          </div>
          <button
            className="w-fit rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold shadow-sm transition hover:border-blue-500 hover:text-blue-700 disabled:cursor-wait disabled:opacity-60"
            disabled={loadState === "loading"}
            onClick={() => setReloadKey((key) => key + 1)}
          >
            새로고침
          </button>
        </header>

        <section className="mt-8" aria-labelledby="cluster-summary-title">
          <div className="flex items-center justify-between gap-4">
            <div>
              <h2 id="cluster-summary-title" className="text-xl font-bold">클러스터 현황</h2>
              <p className="mt-1 text-sm text-slate-500">
                {loadState === "ready" && observedAt
                  ? `마지막 확인: ${observedAt}`
                  : "Minikube에서 리소스 정보를 읽는 중입니다."}
              </p>
            </div>
            <span
              className={`rounded-full px-3 py-1 text-sm font-semibold ${
                loadState === "ready"
                  ? "bg-emerald-100 text-emerald-800"
                  : loadState === "error"
                    ? "bg-rose-100 text-rose-800"
                    : "bg-amber-100 text-amber-800"
              }`}
            >
              {loadState === "ready" ? "연결됨" : loadState === "error" ? "연결 확인 필요" : "연결 중"}
            </span>
          </div>

          {loadState === "error" ? (
            <div className="mt-6 rounded-xl border border-rose-200 bg-rose-50 p-5 text-rose-900">
              <p className="font-semibold">클러스터 정보를 불러오지 못했습니다.</p>
              <p className="mt-1 text-sm leading-6">Go API와 Kubernetes 클러스터가 실행 중인지 확인한 뒤 다시 시도해 주세요.</p>
            </div>
          ) : (
            <div className="mt-6 grid gap-4 md:grid-cols-3">
              {cards.map((card) => (
                <article key={card.englishLabel} className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-sm font-semibold text-slate-500">{card.label}</p>
                  <p className="mt-2 text-xs font-medium uppercase tracking-wide text-blue-700">{card.englishLabel}</p>
                  <p className="mt-6 text-5xl font-bold tracking-tight text-slate-950">
                    {loadState === "loading" ? "-" : card.value}
                  </p>
                </article>
              ))}
            </div>
          )}
        </section>

        <section className="mt-8 rounded-xl border border-blue-100 bg-blue-50 p-6">
          <h2 className="text-lg font-bold text-blue-950">다음에 볼 내용</h2>
          <p className="mt-2 leading-7 text-blue-900">
            다음 단계에서는 실행 중인 앱과 서비스가 어떻게 연결되는지 클러스터 지도로 보여주고, 권한과 네트워크 설정에서 발견한 보안 이슈를 쉬운 설명으로 안내합니다.
          </p>
        </section>
      </div>
    </main>
  );
}
