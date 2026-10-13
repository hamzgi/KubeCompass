"use client";

import {
  Background,
  Controls,
  MarkerType,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useMemo, useState } from "react";

type ClusterSummary = {
  namespaces: number;
  pods: number;
  services: number;
  observedAt: string;
};

type ClusterTopologyNode = {
  id: string;
  kind: "Namespace" | "Pod" | "Service" | "ServiceAccount";
  name: string;
  namespace?: string;
  description: string;
  status?: string;
};

type ClusterTopology = {
  nodes: ClusterTopologyNode[];
  edges: {
    id: string;
    source: string;
    target: string;
    relationship: string;
  }[];
  observedAt: string;
};

type SecurityIssue = {
  id: string;
  ruleId: string;
  severity: "high" | "medium" | "low";
  title: string;
  resource: {
    kind: string;
    namespace: string;
    name: string;
    container?: string;
  };
  summary: string;
  cause: string;
  impact: string;
  recommendation: string;
  exampleYaml: string;
};

type SecurityReport = {
  issues: SecurityIssue[];
  observedAt: string;
};

type LoadState = "loading" | "ready" | "error";
type IssueFilter = "all" | SecurityIssue["severity"];
type DashboardView = "home" | "cluster-map" | "security";

const severityMeta: Record<
  SecurityIssue["severity"],
  { label: string; className: string }
> = {
  high: { label: "높음", className: "bg-rose-100 text-rose-800" },
  medium: { label: "보통", className: "bg-amber-100 text-amber-800" },
  low: { label: "낮음", className: "bg-sky-100 text-sky-800" },
};

const topologyMeta: Record<
  ClusterTopologyNode["kind"],
  {
    term: string;
    friendlyLabel: string;
    helpText: string;
    className: string;
    badgeClassName: string;
  }
> = {
  Namespace: {
    term: "Namespace",
    friendlyLabel: "작업 공간",
    helpText: "클러스터 안의 리소스를 팀·환경·용도별로 나누는 공간입니다.",
    className: "border-violet-200 bg-violet-50 text-violet-950",
    badgeClassName: "bg-violet-100 text-violet-700",
  },
  Service: {
    term: "Service",
    friendlyLabel: "접속 주소",
    helpText:
      "Pod가 바뀌어도 같은 주소로 앱에 연결할 수 있게 해 주는 리소스입니다.",
    className: "border-blue-200 bg-white text-slate-800",
    badgeClassName: "bg-blue-100 text-blue-700",
  },
  Pod: {
    term: "Pod",
    friendlyLabel: "실행 중인 앱",
    helpText: "컨테이너가 실제로 실행되는 가장 작은 Kubernetes 단위입니다.",
    className: "border-[#bfe2d6] bg-white text-[#18483b]",
    badgeClassName: "bg-[#e0f6ed] text-[#197357]",
  },
  ServiceAccount: {
    term: "ServiceAccount",
    friendlyLabel: "앱 계정",
    helpText: "Pod 안의 앱이 Kubernetes API를 사용할 때 쓰는 전용 계정입니다.",
    className: "border-[#fed9a3] bg-white text-[#624116]",
    badgeClassName: "bg-[#fff1d6] text-[#a46214]",
  },
};

async function fetchJSON<T>(path: string, signal: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    signal,
    cache: "no-store",
  });

  if (!response.ok) {
    throw new Error(`The ${path} request failed.`);
  }

  return (await response.json()) as T;
}

function formatObservedAt(observedAt?: string) {
  if (!observedAt) {
    return null;
  }

  return new Intl.DateTimeFormat("ko-KR", {
    dateStyle: "medium",
    timeStyle: "medium",
  }).format(new Date(observedAt));
}

function resourceLabel(issue: SecurityIssue) {
  const container = issue.resource.container
    ? ` · 컨테이너 ${issue.resource.container}`
    : "";
  return `${issue.resource.namespace} / ${issue.resource.name}${container}`;
}

function TopologyNodeLabel({ node }: { node: ClusterTopologyNode }) {
  const meta = topologyMeta[node.kind];
  const shortKind =
    node.kind === "Service"
      ? "SVC"
      : node.kind === "ServiceAccount"
        ? "SA"
        : "POD";
  const isRunning = node.kind === "Pod" && node.status === "Running";

  return (
    <div className="group relative min-w-0 outline-none" tabIndex={0}>
      <div
        role="tooltip"
        className="pointer-events-none invisible absolute left-0 top-8 z-50 w-60 rounded-lg bg-slate-950 p-3 text-left text-xs leading-5 text-slate-100 opacity-0 shadow-lg transition group-hover:visible group-hover:opacity-100 group-focus:visible group-focus:opacity-100"
      >
        <p className="font-bold text-white">{meta.friendlyLabel}</p>
        <p className="mt-1 text-slate-300">
          {node.description || meta.helpText}
        </p>
      </div>
      <div className="flex items-start gap-2">
        <span
          className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-sm text-[10px] font-black tracking-tight ${meta.badgeClassName}`}
        >
          {shortKind}
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-[10px] font-bold uppercase tracking-[0.12em] opacity-65">
            {meta.term}
          </p>
          <p className="mt-0.5 break-all text-sm font-bold leading-5">
            {node.name}
          </p>
        </div>
      </div>
      <div className="mt-3 flex items-center justify-between gap-2 border-t border-current/10 pt-2 text-[10px] font-medium opacity-75">
        <span className="truncate">{node.namespace}</span>
        {node.status && (
          <span
            className={`shrink-0 rounded-full px-1.5 py-0.5 font-bold ${
              isRunning
                ? "bg-emerald-100 text-emerald-700"
                : "bg-amber-100 text-amber-700"
            }`}
          >
            {node.status}
          </span>
        )}
      </div>
    </div>
  );
}

type NamespaceAreaNodeData = {
  namespace: string;
};

type NamespaceAreaNodeType = Node<NamespaceAreaNodeData, "namespaceArea">;

function NamespaceAreaNode({ data }: NodeProps<NamespaceAreaNodeType>) {
  return (
    <div className="h-full w-full border border-dashed border-blue-200 bg-white/70 p-5 text-slate-700">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-[10px] font-black uppercase tracking-[0.18em] text-blue-700">
            Namespace / 작업 공간
          </p>
          <p className="mt-1 break-all text-base font-bold">{data.namespace}</p>
        </div>
        <span className="rounded-sm border border-blue-200 bg-white px-2 py-1 text-[10px] font-bold uppercase tracking-wide text-blue-700">
          Scope
        </span>
      </div>
      <p className="mt-1 text-[11px] leading-5 text-slate-500">
        같은 Namespace에 속한 리소스만 이 작업 영역에 표시됩니다.
      </p>
    </div>
  );
}

const nodeTypes = {
  namespaceArea: NamespaceAreaNode,
};

export default function Home() {
  const pathname = usePathname();
  const currentView: DashboardView =
    pathname === "/cluster-map"
      ? "cluster-map"
      : pathname === "/security"
        ? "security"
        : "home";
  const [summary, setSummary] = useState<ClusterSummary | null>(null);
  const [topology, setTopology] = useState<ClusterTopology | null>(null);
  const [securityReport, setSecurityReport] = useState<SecurityReport | null>(
    null,
  );
  const [loadState, setLoadState] = useState<LoadState>("loading");
  const [issueFilter, setIssueFilter] = useState<IssueFilter>("all");
  const [selectedMapNamespace, setSelectedMapNamespace] = useState("all");
  const [selectedSecurityNamespace, setSelectedSecurityNamespace] =
    useState("all");
  const [selectedIssueID, setSelectedIssueID] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    const controller = new AbortController();

    async function loadCurrentView() {
      setLoadState("loading");

      try {
        if (currentView === "cluster-map") {
          const nextTopology = await fetchJSON<ClusterTopology>(
            "/api/v1/topology",
            controller.signal,
          );
          setSummary(null);
          setTopology(nextTopology);
          setSecurityReport(null);
        } else if (currentView === "security") {
          const nextSecurityReport = await fetchJSON<SecurityReport>(
            "/api/v1/security-issues",
            controller.signal,
          );
          setSummary(null);
          setTopology(null);
          setSecurityReport(nextSecurityReport);
        } else {
          const nextSummary = await fetchJSON<ClusterSummary>(
            "/api/v1/summary",
            controller.signal,
          );
          setSummary(nextSummary);
          setTopology(null);
          setSecurityReport(null);
        }
        setLoadState("ready");
      } catch {
        if (!controller.signal.aborted) {
          setSummary(null);
          setTopology(null);
          setSecurityReport(null);
          setLoadState("error");
        }
      }
    }

    void loadCurrentView();

    return () => controller.abort();
  }, [currentView, reloadKey]);

  const cards = [
    {
      label: "Namespace",
      helper: "전체 작업 공간",
      value: summary?.namespaces,
    },
    { label: "Pod", helper: "전체 실행 단위", value: summary?.pods },
    { label: "Service", helper: "전체 접속 주소", value: summary?.services },
  ];

  const mapNamespaceNames = useMemo(
    () =>
      (topology?.nodes ?? [])
        .filter((node) => node.kind === "Namespace")
        .map((node) => node.name)
        .sort((left, right) => left.localeCompare(right)),
    [topology],
  );

  const mapTopology = useMemo(() => {
    if (!topology || selectedMapNamespace === "all") {
      return topology;
    }

    const nodes = topology.nodes.filter(
      (node) =>
        node.kind === "Namespace"
          ? node.name === selectedMapNamespace
          : node.namespace === selectedMapNamespace,
    );
    const visibleNodeIDs = new Set(nodes.map((node) => node.id));

    return {
      ...topology,
      nodes,
      edges: topology.edges.filter(
        (edge) =>
          visibleNodeIDs.has(edge.source) && visibleNodeIDs.has(edge.target),
      ),
    };
  }, [selectedMapNamespace, topology]);

  const issues = useMemo(() => securityReport?.issues ?? [], [securityReport]);
  const securityNamespaceNames = useMemo(
    () =>
      [...new Set(issues.map((issue) => issue.resource.namespace))].sort(
        (left, right) => left.localeCompare(right),
      ),
    [issues],
  );
  const securityScopeIssues = useMemo(
    () =>
      selectedSecurityNamespace === "all"
        ? issues
        : issues.filter(
            (issue) => issue.resource.namespace === selectedSecurityNamespace,
          ),
    [issues, selectedSecurityNamespace],
  );
  const filteredIssues = useMemo(
    () =>
      issueFilter === "all"
        ? securityScopeIssues
        : securityScopeIssues.filter(
            (issue) => issue.severity === issueFilter,
          ),
    [issueFilter, securityScopeIssues],
  );
  const selectedIssue = useMemo(
    () =>
      filteredIssues.find((issue) => issue.id === selectedIssueID) ??
      filteredIssues[0] ??
      null,
    [filteredIssues, selectedIssueID],
  );
  const securityMetrics: {
    label: string;
    helper: string;
    value: number;
    valueClassName: string;
    filter: IssueFilter | null;
  }[] = [
    {
      label: "CRITICAL",
      helper: "현재 규칙 없음",
      value: 0,
      valueClassName: "text-rose-600",
      filter: null,
    },
    {
      label: "HIGH",
      helper: "우선 확인",
      value: securityScopeIssues.filter((issue) => issue.severity === "high")
        .length,
      valueClassName: "text-orange-500",
      filter: "high",
    },
    {
      label: "MEDIUM",
      helper: "설정 보완",
      value: securityScopeIssues.filter(
        (issue) => issue.severity === "medium",
      ).length,
      valueClassName: "text-amber-500",
      filter: "medium",
    },
    {
      label: "LOW",
      helper: "학습용 확인",
      value: securityScopeIssues.filter((issue) => issue.severity === "low")
        .length,
      valueClassName: "text-sky-600",
      filter: "low",
    },
    {
      label: "UNKNOWN",
      helper: "분류 불가",
      value: 0,
      valueClassName: "text-slate-400",
      filter: null,
    },
    {
      label: "TOTAL",
      helper: "선택한 범위",
      value: securityScopeIssues.length,
      valueClassName: "text-slate-900",
      filter: "all",
    },
  ];

  const graphNodes = useMemo<Node[]>(() => {
    if (!mapTopology) {
      return [];
    }

    const columnByKind: Record<
      Exclude<ClusterTopologyNode["kind"], "Namespace">,
      number
    > = {
      Service: 54,
      Pod: 330,
      ServiceAccount: 606,
    };
    const nodePositionByID = new Map<string, { x: number; y: number }>();
    const namespaceAreas: Node[] = [];
    const namespaceNodes = mapTopology.nodes
      .filter((node) => node.kind === "Namespace")
      .sort((left, right) => left.name.localeCompare(right.name));
    let rowStartY = 20;

    for (const namespaceNode of namespaceNodes) {
      const resourcesByKind: Record<
        Exclude<ClusterTopologyNode["kind"], "Namespace">,
        ClusterTopologyNode[]
      > = {
        Service: [],
        Pod: [],
        ServiceAccount: [],
      };

      for (const node of mapTopology.nodes) {
        if (
          node.namespace === namespaceNode.name &&
          node.kind !== "Namespace"
        ) {
          resourcesByKind[node.kind].push(node);
        }
      }

      for (const resources of Object.values(resourcesByKind)) {
        resources.sort((left, right) => left.name.localeCompare(right.name));
      }

      // ServiceAccount를 이름순으로 놓으면, 실제로는 서로 다른 계정을 쓰는
      // Pod의 연결선이 교차할 수 있습니다. Pod가 사용하는 계정은 같은 순서로
      // 배치해 각 "계정 사용" 관계를 바로 따라갈 수 있게 합니다.
      const serviceAccountOrderByID = new Map<string, number>();
      resourcesByKind.Pod.forEach((pod, podIndex) => {
        const accountEdge = mapTopology.edges.find(
          (edge) =>
            edge.relationship === "계정 사용" && edge.source === pod.id,
        );

        if (accountEdge && !serviceAccountOrderByID.has(accountEdge.target)) {
          serviceAccountOrderByID.set(accountEdge.target, podIndex);
        }
      });
      resourcesByKind.ServiceAccount.sort((left, right) => {
        const leftOrder =
          serviceAccountOrderByID.get(left.id) ?? Number.MAX_SAFE_INTEGER;
        const rightOrder =
          serviceAccountOrderByID.get(right.id) ?? Number.MAX_SAFE_INTEGER;

        return leftOrder - rightOrder || left.name.localeCompare(right.name);
      });

      const rowItemCount = Math.max(
        1,
        ...Object.values(resourcesByKind).map((resources) => resources.length),
      );
      const rowHeight = Math.max(250, rowItemCount * 140 + 126);
      namespaceAreas.push({
        id: `namespace-area:${namespaceNode.name}`,
        type: "namespaceArea",
        position: { x: 0, y: rowStartY },
        data: { namespace: namespaceNode.name },
        draggable: false,
        selectable: false,
        connectable: false,
        style: {
            width: 900,
          height: rowHeight,
          zIndex: 0,
          pointerEvents: "none",
        },
      });

      (
        Object.entries(resourcesByKind) as [
          Exclude<ClusterTopologyNode["kind"], "Namespace">,
          ClusterTopologyNode[],
        ][]
      ).forEach(([kind, resources]) => {
        resources.forEach((node, index) => {
          nodePositionByID.set(node.id, {
            x: columnByKind[kind],
            y: rowStartY + 98 + index * 140,
          });
        });
      });

      rowStartY += rowHeight + 56;
    }

    const resourceNodes = mapTopology.nodes
      .filter((node) => node.kind !== "Namespace")
      .map((node, index) => {
        const meta = topologyMeta[node.kind];

        return {
          id: node.id,
          position: nodePositionByID.get(node.id) ?? { x: 54, y: index * 140 },
          data: { label: <TopologyNodeLabel node={node} /> },
          sourcePosition: Position.Right,
          targetPosition: Position.Left,
          style: {
            width: 212,
            borderRadius: 5,
            borderWidth: 1,
            boxShadow: "0 3px 10px rgba(15, 58, 65, 0.08)",
            zIndex: 1,
          },
          className: meta.className,
        };
      });

    return [...namespaceAreas, ...resourceNodes];
  }, [mapTopology]);

  const graphEdges = useMemo<Edge[]>(() => {
    if (!mapTopology) {
      return [];
    }

    return mapTopology.edges
      .filter((edge) => edge.relationship !== "포함")
      .map((edge) => {
        const isTrafficEdge = edge.relationship === "트래픽 전달";
        const color = isTrafficEdge ? "#2563eb" : "#ea580c";

        return {
          id: edge.id,
          source: edge.source,
          target: edge.target,
          type: "smoothstep",
          label: isTrafficEdge ? "트래픽" : undefined,
          animated: isTrafficEdge,
          markerEnd: { type: MarkerType.ArrowClosed, color },
          labelStyle: { fill: "#1d4ed8", fontSize: 11, fontWeight: 700 },
          labelBgStyle: { fill: "#eff6ff", fillOpacity: 0.96 },
          style: isTrafficEdge
            ? { stroke: color, strokeWidth: 2 }
            : { stroke: color, strokeWidth: 1.75, strokeDasharray: "6 5" },
        };
      });
  }, [mapTopology]);

  const observedAt = formatObservedAt(summary?.observedAt);
  const mapCounts = useMemo(() => {
    const nodes = mapTopology?.nodes ?? [];
    return {
      namespaces: nodes.filter((node) => node.kind === "Namespace").length,
      services: nodes.filter((node) => node.kind === "Service").length,
      pods: nodes.filter((node) => node.kind === "Pod").length,
      serviceAccounts: nodes.filter((node) => node.kind === "ServiceAccount")
        .length,
    };
  }, [mapTopology]);
  const runningPodCount = useMemo(
    () =>
      (mapTopology?.nodes ?? []).filter(
        (node) => node.kind === "Pod" && node.status === "Running",
      ).length,
    [mapTopology],
  );
  const pageMeta: Record<
    DashboardView,
    { title: string; description: string; eyebrow: string }
  > = {
    home: {
      eyebrow: "CLUSTER OVERVIEW",
      title: "내 클러스터 한눈에 보기",
      description:
        "현재 클러스터의 핵심 수치만 빠르게 확인하고, 필요한 화면으로 이동합니다.",
    },
    "cluster-map": {
      eyebrow: "RESOURCE EXPLORER",
      title: "클러스터 맵",
      description:
        "리소스 사이의 연결 관계를 애플리케이션 트리처럼 탐색합니다.",
    },
    security: {
      eyebrow: "SECURITY SCAN",
      title: "보안 설정 확인",
      description:
        "Kubernetes 설정을 읽기 전용으로 점검하고, 이해하기 쉬운 조치를 안내합니다.",
    },
  };
  const activePageMeta = pageMeta[currentView];
  const navigationItems: { href: string; label: string; view: DashboardView }[] = [
    { href: "/", label: "홈", view: "home" },
    { href: "/cluster-map", label: "클러스터 맵", view: "cluster-map" },
    { href: "/security", label: "보안 설정 확인", view: "security" },
  ];

  return (
    <main className="min-h-screen bg-slate-50 text-slate-950">
      <div className="min-h-screen">
        <header className="flex min-h-16 flex-wrap items-center gap-x-5 gap-y-3 border-b border-slate-200 bg-white px-4 py-3 sm:px-6">
          <Link href="/" className="flex items-center gap-2">
            <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-blue-700 text-sm font-black text-white">
              K
            </span>
            <span className="text-sm font-black tracking-[0.14em] text-slate-900">
              KUBECOMPASS
            </span>
          </Link>
          <span className="hidden h-5 w-px bg-slate-200 sm:block" />
          <p className="text-sm font-bold text-slate-700">{activePageMeta.title}</p>
          <nav
            className="order-3 flex w-full flex-wrap gap-1 sm:order-none sm:ml-auto sm:w-auto"
            aria-label="KubeCompass 주요 화면"
          >
            {navigationItems.map((item) => {
              const isActive = item.view === currentView;

              return (
                <Link
                  key={item.href}
                  href={item.href}
                  aria-current={isActive ? "page" : undefined}
                  className={`rounded-md px-3 py-1.5 text-sm font-semibold transition ${
                    isActive
                      ? "bg-blue-700 text-white"
                      : "text-slate-600 hover:bg-slate-100 hover:text-blue-700"
                  }`}
                >
                  {item.label}
                </Link>
              );
            })}
          </nav>
          <button
            className="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-slate-700 shadow-sm transition hover:border-blue-500 hover:text-blue-700 disabled:cursor-wait disabled:opacity-60"
            disabled={loadState === "loading"}
            onClick={() => setReloadKey((key) => key + 1)}
          >
            새로고침
          </button>
        </header>

        {currentView === "home" && (
          <section
            className="min-h-[calc(100vh-4rem)]"
            aria-labelledby="cluster-summary-title"
          >
            <div className="border-b border-slate-200 bg-white">
              <div className="mx-auto flex w-full max-w-[1440px] flex-col gap-3 px-5 py-4 sm:px-6 lg:flex-row lg:items-center lg:justify-between">
                <div>
                  <p className="text-xs font-black tracking-[0.14em] text-blue-700">
                    {activePageMeta.eyebrow}
                  </p>
                  <p className="mt-1 text-sm text-slate-600">
                    {activePageMeta.description}
                  </p>
                </div>
                <span
                  className={`w-fit rounded-full px-3 py-2 text-xs font-bold ${
                    loadState === "ready"
                      ? "bg-emerald-100 text-emerald-800"
                      : loadState === "error"
                        ? "bg-rose-100 text-rose-800"
                        : "bg-amber-100 text-amber-800"
                  }`}
                >
                  {loadState === "ready"
                    ? "● 클러스터 연결됨"
                    : loadState === "error"
                      ? "● 연결 확인 필요"
                      : "● 연결 중"}
                </span>
              </div>
            </div>

            <div className="mx-auto w-full max-w-[1440px] px-5 py-6 sm:px-6">
        <section aria-labelledby="cluster-summary-title">
          <div>
            <div>
              <h2 id="cluster-summary-title" className="text-xl font-bold">
                클러스터 현황
              </h2>
              <p className="mt-1 text-sm text-slate-500">
                {loadState === "ready" && observedAt
                  ? `마지막 확인: ${observedAt}`
                  : "Minikube에서 리소스 정보를 읽는 중입니다."}
              </p>
              <p className="mt-1 text-xs leading-5 text-slate-500">
                전체 클러스터 기준이며, Kubernetes 시스템 리소스도 포함합니다.
              </p>
            </div>
          </div>

          {loadState === "error" ? (
            <div className="mt-6 rounded-xl border border-rose-200 bg-rose-50 p-5 text-rose-900">
              <p className="font-semibold">
                클러스터 정보를 불러오지 못했습니다.
              </p>
              <p className="mt-1 text-sm leading-6">
                Go API와 Kubernetes 클러스터가 실행 중인지 확인한 뒤 다시 시도해
                주세요.
              </p>
            </div>
          ) : (
            <div className="mt-6 grid gap-4 md:grid-cols-3">
              {cards.map((card) => (
                <article
                  key={card.label}
                  className="rounded-xl border border-slate-200 bg-white p-6 shadow-sm"
                >
                  <p className="text-sm font-semibold text-slate-500">
                    {card.helper}
                  </p>
                  <p className="mt-2 text-xs font-medium uppercase tracking-wide text-blue-700">
                    {card.label}
                  </p>
                  <p className="mt-6 text-5xl font-bold tracking-tight text-slate-950">
                    {loadState === "loading" ? "-" : card.value}
                  </p>
                </article>
              ))}
            </div>
          )}
        </section>

            <section className="mt-6 grid gap-4 md:grid-cols-2" aria-label="주요 기능 바로가기">
              <Link
                href="/cluster-map"
                className="group rounded-xl border border-slate-200 bg-white p-6 shadow-sm transition hover:-translate-y-0.5 hover:border-[#56a8ad] hover:shadow"
              >
                <p className="text-xs font-black tracking-[0.14em] text-[#17737b]">
                  RESOURCE EXPLORER
                </p>
                <h2 className="mt-2 text-xl font-bold text-slate-900">
                  클러스터 맵
                </h2>
                <p className="mt-2 text-sm leading-6 text-slate-600">
                  Namespace별 리소스와 Service, Pod, ServiceAccount의 연결을 탐색합니다.
                </p>
                <p className="mt-5 text-sm font-bold text-[#17737b] transition group-hover:translate-x-1">
                  맵 열기 →
                </p>
              </Link>
              <Link
                href="/security"
                className="group rounded-xl border border-slate-200 bg-white p-6 shadow-sm transition hover:-translate-y-0.5 hover:border-orange-400 hover:shadow"
              >
                <p className="text-xs font-black tracking-[0.14em] text-orange-600">
                  CONFIGURATION SCAN
                </p>
                <h2 className="mt-2 text-xl font-bold text-slate-900">
                  보안 설정 확인
                </h2>
                <p className="mt-2 text-sm leading-6 text-slate-600">
                  현재 설정의 위험 요소와 원인, 영향, 권장 조치를 표 중심으로 확인합니다.
                </p>
                <p className="mt-5 text-sm font-bold text-orange-600 transition group-hover:translate-x-1">
                  점검 결과 열기 →
                </p>
              </Link>
            </section>
            </div>
          </section>
        )}

        {currentView === "cluster-map" && (
        <section
          className="flex h-[calc(100vh-4rem)] min-h-[720px] flex-col overflow-hidden bg-white"
          aria-labelledby="topology-title"
        >
          <div className="shrink-0 border-b border-slate-200 bg-white px-5 py-3 sm:px-6">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <p
                  id="topology-title"
                  className="text-xs font-black tracking-[0.14em] text-blue-700"
                >
                  RESOURCE TOPOLOGY
                </p>
                <p className="mt-1 text-sm text-slate-600">
                  Namespace별 리소스 연결 관계를 확인합니다. 카드에 커서를 올리면 용어 설명을 볼 수 있습니다.
                </p>
              </div>
              <div className="flex flex-wrap items-center gap-2 text-xs font-bold">
                <span
                  className={`rounded-full px-3 py-2 ${
                    mapCounts.pods > 0 && runningPodCount === mapCounts.pods
                      ? "bg-emerald-100 text-emerald-800"
                      : "bg-amber-100 text-amber-800"
                  }`}
                >
                  ● {runningPodCount}/{mapCounts.pods} Pod Running
                </span>
                <span className="rounded-full bg-slate-100 px-3 py-2 text-slate-700">
                  {mapCounts.services} Service · {mapCounts.serviceAccounts} SA
                </span>
              </div>
            </div>
          </div>

          <div className="grid min-h-0 flex-1 lg:grid-cols-[248px_minmax(0,1fr)]">
            <aside className="border-b border-slate-200 bg-white px-4 py-5 text-slate-700 lg:border-b-0 lg:border-r">
              <div className="flex items-center gap-2 border-b border-slate-200 pb-4">
                <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-blue-100 text-sm font-black text-blue-700">
                  K
                </span>
                <div>
                  <p className="text-xs font-black tracking-[0.16em] text-slate-900">
                    KUBECOMPASS
                  </p>
                  <p className="mt-0.5 text-[10px] text-slate-500">
                    RESOURCE EXPLORER
                  </p>
                </div>
              </div>

              <label className="mt-5 block text-[10px] font-black tracking-[0.14em] text-slate-500">
                NAMESPACE
                <select
                  className="mt-2 w-full rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 text-sm font-semibold text-slate-800 outline-none transition focus:border-blue-500 focus:bg-white"
                  value={selectedMapNamespace}
                  onChange={(event) => setSelectedMapNamespace(event.target.value)}
                >
                  <option value="all">전체 Namespace</option>
                  {mapNamespaceNames.map((namespace) => (
                    <option key={namespace} value={namespace}>
                      {namespace}
                    </option>
                  ))}
                </select>
              </label>

              <div className="mt-6 border-t border-slate-200 pt-5">
                <p className="text-[10px] font-black tracking-[0.14em] text-slate-500">
                  RESOURCE FILTERS
                </p>
                <div className="mt-3 space-y-1">
                  {(
                    ["Service", "Pod", "ServiceAccount"] as const
                  ).map((kind) => {
                    const meta = topologyMeta[kind];
                    const count =
                      kind === "Service"
                        ? mapCounts.services
                        : kind === "Pod"
                          ? mapCounts.pods
                          : mapCounts.serviceAccounts;

                    return (
                      <div
                        key={kind}
                        className="flex items-center justify-between rounded-lg px-2 py-2 text-sm text-slate-700"
                      >
                        <span className="flex items-center gap-2">
                          <span className={`h-2 w-2 rounded-sm ${meta.badgeClassName}`} />
                          {meta.term}
                        </span>
                        <span className="rounded bg-slate-100 px-1.5 py-0.5 text-xs font-bold text-slate-600">
                          {count}
                        </span>
                      </div>
                    );
                  })}
                </div>
              </div>

              <div className="mt-6 border-t border-slate-200 pt-5 text-xs leading-5 text-slate-600">
                <p className="font-bold text-slate-900">관계 범례</p>
                <p className="mt-3 flex items-center gap-2">
                  <span className="h-0 w-7 border-t-2 border-blue-600" />
                  Service → Pod
                </p>
                <p className="mt-2 flex items-center gap-2">
                  <span className="h-0 w-7 border-t-2 border-dashed border-orange-500" />
                  Pod → ServiceAccount
                </p>
              </div>
            </aside>

            <div className="flex min-h-0 min-w-0 flex-col bg-slate-50">
              <div className="grid shrink-0 border-b border-slate-200 bg-white sm:grid-cols-3">
                <div className="border-b border-slate-100 px-5 py-4 sm:border-b-0 sm:border-r">
                  <p className="text-[10px] font-black tracking-[0.12em] text-slate-500">
                    APP HEALTH
                  </p>
                  <p className="mt-1 text-lg font-bold text-emerald-600">
                    ♥ Healthy
                  </p>
                  <p className="mt-1 text-xs text-slate-500">
                    실행 중인 Pod 기준 상태
                  </p>
                </div>
                <div className="border-b border-slate-100 px-5 py-4 sm:border-b-0 sm:border-r">
                  <p className="text-[10px] font-black tracking-[0.12em] text-slate-500">
                    MAP SCOPE
                  </p>
                  <p className="mt-1 truncate text-lg font-bold text-blue-700">
                    {selectedMapNamespace === "all"
                      ? "All Namespaces"
                      : selectedMapNamespace}
                  </p>
                  <p className="mt-1 text-xs text-slate-500">
                    Namespace {mapCounts.namespaces}개 표시 중
                  </p>
                </div>
                <div className="px-5 py-4">
                  <p className="text-[10px] font-black tracking-[0.12em] text-slate-500">
                    LAST OBSERVED
                  </p>
                  <p className="mt-1 text-lg font-bold text-slate-800">
                    {formatObservedAt(mapTopology?.observedAt) ?? "확인 중"}
                  </p>
                  <p className="mt-1 text-xs text-slate-500">
                    읽기 전용으로 수집한 현재 상태
                  </p>
                </div>
              </div>

              <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-slate-200 bg-white px-4 py-3 text-xs font-semibold text-slate-600">
                <div className="flex items-center gap-2">
                  <span className="rounded border border-slate-200 bg-slate-50 px-2 py-1 text-blue-700">
                    MAP VIEW
                  </span>
                  <span className="ml-1">캔버스를 드래그하고 왼쪽 하단 컨트롤로 확대/축소합니다.</span>
                </div>
                <span className="rounded border border-slate-200 bg-slate-50 px-2 py-1 uppercase tracking-wide text-slate-500">
                  Resource relationship map
                </span>
              </div>

              <div className="min-h-0 flex-1 overflow-hidden">
                {loadState === "error" ? (
                  <div className="flex h-full items-center justify-center p-6 text-center text-sm leading-6 text-rose-700">
                    클러스터 맵을 불러오지 못했습니다. Kubernetes API 연결을 확인한 뒤 새로고침해 주세요.
                  </div>
                ) : loadState === "loading" ? (
                  <div className="flex h-full items-center justify-center text-sm font-medium text-slate-500">
                    지도를 만드는 중입니다.
                  </div>
                ) : graphNodes.length === 0 ? (
                  <div className="flex h-full items-center justify-center text-center text-sm leading-6 text-slate-500">
                    표시할 애플리케이션 리소스가 없습니다. <br />
                    <code>kubecompass-demo</code> 리소스를 적용하면 연결 관계를 바로
                    확인할 수 있습니다.
                  </div>
                ) : (
                  <ReactFlow
                    key={`${mapTopology?.observedAt}-${selectedMapNamespace}`}
                    nodes={graphNodes}
                    edges={graphEdges}
                    nodeTypes={nodeTypes}
                    fitView
                    fitViewOptions={{ padding: 0.18 }}
                    nodesDraggable={false}
                    nodesConnectable={false}
                    elementsSelectable={false}
                    proOptions={{ hideAttribution: true }}
                  >
                    <Background color="#d8e1ee" gap={18} size={1} />
                    <Controls showInteractive={false} />
                  </ReactFlow>
                )}
              </div>
            </div>
          </div>
        </section>
        )}

        {currentView === "security" && (
          <section
            className="min-h-[calc(100vh-4rem)] bg-slate-50"
            aria-labelledby="security-title"
          >
            <div className="border-b border-slate-200 bg-white">
              <div className="mx-auto flex w-full max-w-[1440px] flex-col gap-3 px-5 py-4 sm:px-6 lg:flex-row lg:items-center lg:justify-between">
                <div>
                  <p
                    id="security-title"
                    className="text-xs font-black tracking-[0.14em] text-blue-700"
                  >
                    {activePageMeta.eyebrow}
                  </p>
                  <p className="mt-1 text-sm text-slate-600">
                    {activePageMeta.description}
                  </p>
                </div>
                <div className="flex flex-wrap items-center gap-2 text-xs font-semibold text-slate-600">
                  <span className="rounded border border-blue-200 bg-blue-50 px-2.5 py-1.5 text-blue-700">
                    READ ONLY
                  </span>
                  <span className="rounded border border-slate-200 bg-slate-50 px-2.5 py-1.5">
                    마지막 확인: {formatObservedAt(securityReport?.observedAt) ?? "확인 중"}
                  </span>
                </div>
              </div>
            </div>

            <div className="mx-auto w-full max-w-[1440px] px-5 py-6 sm:px-6">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
              <div>
                <p className="text-sm font-semibold text-slate-700">
                  Summary of configuration risks
                </p>
                <p className="mt-1 text-sm leading-6 text-slate-500">
                  이미지를 취약점으로 검사하는 화면 형식을 참고하되, KubeCompass는
                  Kubernetes 설정을 읽기 전용으로 점검합니다.
                </p>
              </div>
              <label className="block text-xs font-bold uppercase tracking-wide text-slate-500">
                Namespace
                <select
                  className="ml-2 rounded border border-slate-300 bg-white px-3 py-2 text-sm font-semibold normal-case text-slate-800 outline-none transition focus:border-blue-500"
                  value={selectedSecurityNamespace}
                  onChange={(event) => {
                    setSelectedSecurityNamespace(event.target.value);
                    setSelectedIssueID(null);
                  }}
                >
                  <option value="all">전체 Namespace</option>
                  {securityNamespaceNames.map((namespace) => (
                    <option key={namespace} value={namespace}>
                      {namespace}
                    </option>
                  ))}
                </select>
              </label>
            </div>

            <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
              {securityMetrics.map((metric) => {
                const isActive =
                  metric.filter !== null && issueFilter === metric.filter;

                return (
                  <button
                    key={metric.label}
                    type="button"
                    disabled={metric.filter === null}
                    onClick={() => {
                      if (metric.filter) {
                        setIssueFilter(metric.filter);
                        setSelectedIssueID(null);
                      }
                    }}
                    className={`min-h-36 rounded border bg-white p-4 text-left shadow-sm transition ${
                      metric.filter === null
                        ? "cursor-default"
                        : "hover:-translate-y-0.5 hover:border-slate-400 hover:shadow"
                    } ${
                      isActive
                        ? "border-blue-500 ring-2 ring-blue-100"
                        : "border-slate-200"
                    }`}
                  >
                    <p className="text-sm font-black tracking-wide text-slate-700">
                      {metric.label}
                    </p>
                    <p
                      className={`mt-6 text-5xl font-light tracking-tight ${metric.valueClassName}`}
                    >
                      {metric.value}
                    </p>
                    <p className="mt-2 text-xs text-slate-500">{metric.helper}</p>
                  </button>
                );
              })}
            </div>

            <div className="mt-6 grid gap-4 xl:grid-cols-[minmax(340px,0.85fr)_minmax(0,1.4fr)]">
              <aside className="flex min-h-[620px] flex-col overflow-hidden rounded-lg border border-slate-200 bg-white xl:h-[650px]">
                <div className="border-b border-slate-200 px-4 py-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <p className="text-sm font-bold text-slate-900">
                        Configuration findings
                      </p>
                      <p className="mt-1 text-xs text-slate-500">
                        {filteredIssues.length}개 항목 · 선택하면 오른쪽에서 바로 확인합니다.
                      </p>
                    </div>
                    <span className="rounded bg-slate-100 px-2 py-1 text-xs font-bold text-slate-600">
                      목록
                    </span>
                  </div>
                  <div
                    className="mt-4 flex flex-wrap gap-1.5"
                    aria-label="보안 이슈 위험도 필터"
                  >
                    {(["all", "high", "medium", "low"] as const).map(
                      (filter) => {
                        const isActive = issueFilter === filter;
                        const label =
                          filter === "all"
                            ? "ALL"
                            : filter.toUpperCase();

                        return (
                          <button
                            key={filter}
                            type="button"
                            className={`rounded border px-2.5 py-1.5 text-xs font-bold transition ${
                              isActive
                                ? "border-blue-600 bg-blue-600 text-white"
                                : "border-slate-300 bg-white text-slate-600 hover:border-blue-400 hover:text-blue-700"
                            }`}
                            onClick={() => {
                              setIssueFilter(filter);
                              setSelectedIssueID(null);
                            }}
                          >
                            {label}
                          </button>
                        );
                      },
                    )}
                  </div>
                </div>

                <div className="min-h-0 flex-1 overflow-y-auto p-2">
                  {loadState === "loading" ? (
                    <p className="p-4 text-sm text-slate-500">
                      보안 설정을 확인하는 중입니다.
                    </p>
                  ) : loadState === "error" ? (
                    <p className="p-4 text-sm leading-6 text-rose-700">
                      보안 결과를 불러오지 못했습니다. 연결을 확인한 뒤 새로고침해 주세요.
                    </p>
                  ) : filteredIssues.length === 0 ? (
                    <p className="p-4 text-sm leading-6 text-slate-500">
                      현재 선택한 범위와 위험도에서는 확인할 이슈가 없습니다.
                    </p>
                  ) : (
                    <div className="space-y-2">
                      {filteredIssues.map((issue) => {
                        const severity = severityMeta[issue.severity];
                        const isSelected = selectedIssue?.id === issue.id;

                        return (
                          <button
                            key={issue.id}
                            type="button"
                            aria-pressed={isSelected}
                            onClick={() => setSelectedIssueID(issue.id)}
                            className={`w-full rounded-lg border p-3 text-left outline-none transition ${
                              isSelected
                                ? "border-blue-500 bg-blue-50 shadow-sm ring-1 ring-blue-100"
                                : "border-slate-200 bg-white hover:border-blue-300 hover:bg-slate-50"
                            }`}
                          >
                            <div className="flex items-start justify-between gap-3">
                              <span
                                className={`rounded px-2 py-1 text-[10px] font-black uppercase ${severity.className}`}
                              >
                                {issue.severity}
                              </span>
                              <span className="font-mono text-[10px] text-slate-400">
                                {issue.ruleId}
                              </span>
                            </div>
                            <p className="mt-3 text-sm font-bold leading-5 text-slate-900">
                              {issue.title}
                            </p>
                            <p className="mt-1 text-xs font-medium text-slate-600">
                              {issue.resource.kind} · {issue.resource.namespace} / {issue.resource.name}
                            </p>
                            <p className="mt-2 text-xs leading-5 text-slate-500">
                              {issue.summary}
                            </p>
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
              </aside>

              <section className="min-h-[620px] overflow-hidden rounded-lg border border-slate-200 bg-white xl:h-[650px]">
                {selectedIssue ? (
                  <div className="h-full overflow-y-auto p-5 sm:p-6">
                    <div className="flex flex-col gap-3 border-b border-slate-200 pb-5 sm:flex-row sm:items-start sm:justify-between">
                      <div>
                        <p className="text-xs font-black tracking-[0.12em] text-blue-700">
                          SELECTED FINDING
                        </p>
                        <h3 className="mt-1 text-xl font-bold text-slate-900">
                          {selectedIssue.title}
                        </h3>
                        <p className="mt-2 text-sm text-slate-600">
                          {resourceLabel(selectedIssue)}
                        </p>
                      </div>
                      <span
                        className={`w-fit rounded px-2.5 py-1 text-xs font-bold uppercase ${severityMeta[selectedIssue.severity].className}`}
                      >
                        {selectedIssue.severity}
                      </span>
                    </div>

                    <div className="mt-5 grid gap-3 sm:grid-cols-2">
                      <div className="rounded-lg bg-slate-50 p-3">
                        <p className="text-[10px] font-black tracking-[0.12em] text-slate-500">
                          RESOURCE
                        </p>
                        <p className="mt-1 text-sm font-bold text-slate-900">
                          {selectedIssue.resource.kind} · {selectedIssue.resource.name}
                        </p>
                        <p className="mt-1 text-xs text-slate-500">
                          Namespace: {selectedIssue.resource.namespace}
                        </p>
                      </div>
                      <div className="rounded-lg bg-slate-50 p-3">
                        <p className="text-[10px] font-black tracking-[0.12em] text-slate-500">
                          POLICY ID
                        </p>
                        <p className="mt-1 font-mono text-sm font-bold text-slate-900">
                          {selectedIssue.ruleId}
                        </p>
                        <p className="mt-1 text-xs text-slate-500">
                          읽기 전용 설정 점검 규칙
                        </p>
                      </div>
                    </div>

                    <div className="mt-5 grid gap-5 text-sm leading-6 text-slate-700 md:grid-cols-3">
                      <div>
                        <p className="font-bold text-slate-950">원인</p>
                        <p className="mt-1">{selectedIssue.cause}</p>
                      </div>
                      <div>
                        <p className="font-bold text-slate-950">영향</p>
                        <p className="mt-1">{selectedIssue.impact}</p>
                      </div>
                      <div>
                        <p className="font-bold text-slate-950">권장 조치</p>
                        <p className="mt-1">{selectedIssue.recommendation}</p>
                      </div>
                    </div>

                    <div className="mt-6">
                      <div className="flex items-center justify-between gap-3">
                        <p className="text-sm font-bold text-slate-900">
                          수정 예시 YAML
                        </p>
                        <span className="text-xs text-slate-500">
                          복사 전 환경에 맞게 값을 확인하세요.
                        </span>
                      </div>
                      <pre className="mt-3 overflow-x-auto rounded-lg bg-slate-950 p-4 text-xs leading-5 text-slate-100">
                        <code>{selectedIssue.exampleYaml}</code>
                      </pre>
                    </div>
                  </div>
                ) : (
                  <div className="flex h-full items-center justify-center p-8 text-center text-sm leading-6 text-slate-500">
                    왼쪽 목록에서 확인할 보안 항목을 선택하세요.
                  </div>
                )}
              </section>
            </div>
            </div>
          </section>
        )}
      </div>
    </main>
  );
}
