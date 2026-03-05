"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import {
  getVersions,
  startMigration,
  getMigrationProgress,
  pauseMigration,
  resumeMigration,
  updateMigration,
  approveMigration,
  rejectMigration,
  resetMigrations,
  crashServer,
} from "@/lib/api";
import type {
  EmbeddingVersion,
  MigrationProgress,
  ApprovalMigrationProgress,
  StartMigrationRequest,
} from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import {
  Database,
  ExternalLink,
  Play,
  Pause,
  RotateCcw,
  Loader2,
  Cpu,
  Activity,
  CheckCircle2,
  Clock,
  AlertCircle,
  Zap,
  Check,
  X,
  GitPullRequest,
  Timer,
} from "lucide-react";

export default function MigrationsPage() {
  const [versions, setVersions] = useState<EmbeddingVersion[]>([]);
  const [versionsLoading, setVersionsLoading] = useState(true);
  const [versionsError, setVersionsError] = useState(false);
  const [resetting, setResetting] = useState(false);
  const [crashing, setCrashing] = useState(false);
  const hasLoadedRef = useRef(false);

  const loadVersions = useCallback(async () => {
    if (!hasLoadedRef.current) {
      setVersionsLoading(true);
    }
    setVersionsError(false);
    try {
      const data = await getVersions();
      setVersions(data);
      hasLoadedRef.current = true;
    } catch (err) {
      console.error("Failed to load versions:", err);
      if (!hasLoadedRef.current) {
        setVersionsError(true);
      }
    } finally {
      setVersionsLoading(false);
    }
  }, []);

  useEffect(() => {
    loadVersions();
  }, [loadVersions]);

  const handleReset = useCallback(async () => {
    if (
      !confirm("This will delete all migrations and reset to v1. Continue?")
    ) {
      return;
    }
    setResetting(true);
    try {
      await resetMigrations();
      await loadVersions();
    } catch (err) {
      console.error("Reset failed:", err);
    } finally {
      setResetting(false);
    }
  }, [loadVersions]);

  const handleCrash = useCallback(async () => {
    if (!confirm("This will crash the server. Continue?")) {
      return;
    }
    setCrashing(true);
    try {
      await crashServer();
    } catch (err) {
      console.error("crash failed:", err);
    } finally {
      setTimeout(() => setCrashing(false), 3000);
    }
  }, []);

  const activeVersion = versions.find((v) => v.is_active);
  const totalRecords = activeVersion?.total_records ?? 0;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            Migration Dashboard
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Manage embedding model migrations with zero-downtime upgrades.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" asChild>
            <a
              href="/temporal/"
              target="_blank"
              rel="noopener noreferrer"
            >
              <ExternalLink className="h-3.5 w-3.5" />
              Temporal UI
            </a>
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={handleCrash}
            disabled={crashing}
          >
            {crashing ? (
              "Crashed"
            ) : (
              <>
                <RotateCcw className="h-3.5 w-3.5" />
                Crash Server
              </>
            )}
          </Button>
          {versions.length > 1 && (
            <Button
              variant="outline"
              size="sm"
              onClick={handleReset}
              disabled={resetting}
            >
              {resetting ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <RotateCcw className="h-3.5 w-3.5" />
              )}
              Reset to v1
            </Button>
          )}
        </div>
      </div>

      {/* Stats cards */}
      {versionsLoading ? (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : versionsError ? (
        <div className="rounded-lg border border-dashed border-destructive/30 p-8 text-center">
          <AlertCircle className="mx-auto mb-2 h-6 w-6 text-destructive" />
          <p className="text-sm text-muted-foreground">
            Unable to load versions. Please check that the backend is running.
          </p>
          <Button variant="outline" size="sm" className="mt-3" onClick={loadVersions}>
            Try Again
          </Button>
        </div>
      ) : (
      <>
      {/* Stats cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <Card>
          <CardContent className="flex items-center gap-4 pt-6">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-blue-100">
              <Database className="h-5 w-5 text-blue-600" />
            </div>
            <div>
              <p className="text-sm text-muted-foreground">Active Model</p>
              <p className="font-semibold">
                {activeVersion?.model_name ?? "None"}
              </p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex items-center gap-4 pt-6">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-emerald-100">
              <Cpu className="h-5 w-5 text-emerald-600" />
            </div>
            <div>
              <p className="text-sm text-muted-foreground">Dimensions</p>
              <p className="font-semibold">
                {activeVersion?.dimensions ?? "—"}
              </p>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex items-center gap-4 pt-6">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-violet-100">
              <Activity className="h-5 w-5 text-violet-600" />
            </div>
            <div>
              <p className="text-sm text-muted-foreground">Total Records</p>
              <p className="font-semibold">{totalRecords}</p>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Start migration */}
      <StartMigrationForm onStarted={loadVersions} versions={versions} />

      {/* Versions table */}
      <VersionsTable versions={versions} onComplete={loadVersions} />
      </>
      )}
    </div>
  );
}

function StartMigrationForm({
  onStarted,
  versions,
}: {
  onStarted: () => void;
  versions: EmbeddingVersion[];
}) {
  const existingVersions = versions.map((v) => v.version);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [useApprovalWorkflow, setUseApprovalWorkflow] = useState(false);

  const availableModels = [
    {
      model_name: "all-minilm",
      dimensions: 384,
      description: "Fast, lightweight model for semantic search",
      icon: <Zap className="h-5 w-5 text-amber-500" />,
    },
    {
      model_name: "nomic-embed-text",
      dimensions: 768,
      description: "High-quality embeddings for better accuracy",
      icon: <Cpu className="h-5 w-5 text-violet-500" />,
    },
  ];

  const activeVersion = versions.find((v) => v.is_active);

  const handleStartMigration = async (model: (typeof availableModels)[0]) => {
    setError(null);
    setSubmitting(true);

    try {
      const existingNumeric = existingVersions
        .filter((v) => v.match(/^v\d+$/))
        .map((v) => parseInt(v.slice(1), 10))
        .sort((a, b) => b - a);
      const nextNum = existingNumeric.length > 0 ? existingNumeric[0] + 1 : 2;
      const version = `v${nextNum}`;

      const req: StartMigrationRequest = {
        version,
        model_name: model.model_name,
        dimensions: model.dimensions,
        batch_size: 10,
        approval_workflow: useApprovalWorkflow,
        approval_timeout_minutes: 60,
      };
      await startMigration(req);

      let attempts = 0;
      const pollForVersion = async () => {
        while (attempts < 10) {
          await new Promise((r) => setTimeout(r, 500));
          const data = await getVersions();
          if (data.some((v) => v.version === version)) {
            onStarted();
            return;
          }
          attempts++;
        }
        onStarted();
      };
      pollForVersion();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start migration");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Start New Migration</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            id="approval-workflow"
            checked={useApprovalWorkflow}
            onChange={(e) => setUseApprovalWorkflow(e.target.checked)}
            className="h-4 w-4 rounded border-gray-300"
          />
          <label htmlFor="approval-workflow" className="text-sm text-foreground flex items-center gap-2">
            <GitPullRequest className="h-4 w-4" />
            Use approval workflow (human-in-the-loop)
          </label>
        </div>
        
        {useApprovalWorkflow && (
          <div className="rounded-lg bg-amber-50 border border-amber-200 p-3 text-sm text-amber-800">
            <div className="flex items-center gap-2 font-medium">
              <Timer className="h-4 w-4" />
              Approval Workflow
            </div>
            <p className="mt-1 text-amber-700">
              Migration will generate all embeddings first, then pause for human approval. 
              You can approve to switch versions, or reject to rollback. Auto-timeout after 60 minutes.
            </p>
          </div>
        )}

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {availableModels.map((model) => {
            const isThisModelActive = activeVersion?.model_name === model.model_name;
            const isDisabled = isThisModelActive || submitting;

            return (
              <button
                key={model.model_name}
                onClick={() => !isDisabled && handleStartMigration(model)}
                disabled={isDisabled}
                className={`group relative rounded-lg border p-4 text-left transition-all ${
                  isThisModelActive
                    ? "border-emerald-200 bg-emerald-50 opacity-70"
                    : "border-border hover:border-primary/40 hover:bg-accent hover:shadow-sm"
                } ${isDisabled ? "cursor-not-allowed" : "cursor-pointer"}`}
              >
                <div className="flex items-start gap-3">
                  <div className="mt-0.5">{model.icon}</div>
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-foreground">
                        {model.model_name}
                      </span>
                      {isThisModelActive && (
                        <Badge variant="success" className="text-[10px]">
                          Active
                        </Badge>
                      )}
                    </div>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      {model.dimensions} dimensions
                    </p>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {model.description}
                    </p>
                  </div>
                </div>
                {!isThisModelActive && !submitting && (
                  <div className="mt-3 text-xs font-medium text-primary opacity-0 transition-opacity group-hover:opacity-100">
                    Click to start migration &rarr;
                  </div>
                )}
              </button>
            );
          })}
        </div>
        {error && (
          <div className="flex items-center gap-2 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
            <AlertCircle className="h-4 w-4" />
            {error}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function VersionsTable({
  versions,
  onComplete,
}: {
  versions: EmbeddingVersion[];
  onComplete?: () => void;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Embedding Versions</CardTitle>
      </CardHeader>
      <CardContent>
        {versions.length === 0 ? (
          <p className="py-4 text-center text-sm text-muted-foreground">
            No versions found.
          </p>
        ) : (
          <div className="overflow-x-auto rounded-lg border">
            <table className="w-full min-w-[640px] text-sm">
              <thead className="bg-muted/50">
                <tr className="text-left text-xs font-medium text-muted-foreground">
                  <th className="px-4 py-3">Version</th>
                  <th className="px-4 py-3">Model</th>
                  <th className="px-4 py-3">Dims</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-4 py-3">Progress</th>
                  <th className="px-4 py-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {versions.map((v) => (
                  <VersionRow
                    key={v.version}
                    version={v}
                    onComplete={onComplete}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function VersionRow({
  version: v,
  onComplete,
}: {
  version: EmbeddingVersion;
  onComplete?: () => void;
}) {
  const [progress, setProgress] = useState<MigrationProgress | ApprovalMigrationProgress | null>(null);
  const [polling, setPolling] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const isInProgress = v.status === "pending" || v.status === "in_progress";
  const isAwaitingApproval = progress && "awaiting_approval" in progress && progress.awaiting_approval;

  useEffect(() => {
    if (!isInProgress && !isAwaitingApproval) return;

    setPolling(true);
    const poll = async () => {
      try {
        const p = await getMigrationProgress(v.version);
        setProgress(p);
        
        const status = "status" in p ? p.status : "";
        if (status === "completed" || status === "cancelled" || status === "rejected") {
          setPolling(false);
          if (intervalRef.current) clearInterval(intervalRef.current);
          onComplete?.();
        }
      } catch {
        setPolling(false);
        if (intervalRef.current) clearInterval(intervalRef.current);
      }
    };

    poll();
    intervalRef.current = setInterval(poll, 2500);

    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [isInProgress, isAwaitingApproval, v.version]);

  const processed = progress?.processed_records ?? v.processed_records;
  const total = progress?.total_records ?? v.total_records;
  const pct = total > 0 ? Math.round((processed / total) * 100) : 0;
  const displayStatus = progress?.status ?? v.status;

  const handlePause = async () => {
    try {
      await pauseMigration(v.version);
      const p = await getMigrationProgress(v.version);
      setProgress(p);
    } catch (err) {
      console.error("Pause failed:", err);
    }
  };

  const handleResume = async () => {
    try {
      await resumeMigration(v.version);
      const p = await getMigrationProgress(v.version);
      setProgress(p);
    } catch (err) {
      console.error("Resume failed:", err);
    }
  };

  const handleApprove = async () => {
    try {
      await approveMigration(v.version);
      const p = await getMigrationProgress(v.version);
      setProgress(p);
    } catch (err) {
      console.error("Approve failed:", err);
    }
  };

  const handleReject = async () => {
    if (!confirm("Rejecting will rollback all embeddings. Continue?")) {
      return;
    }
    try {
      await rejectMigration(v.version);
      const p = await getMigrationProgress(v.version);
      setProgress(p);
    } catch (err) {
      console.error("Reject failed:", err);
    }
  };

  return (
    <tr className="text-foreground">
      <td className="px-4 py-3">
        <div className="flex items-center gap-2">
          <span className="font-mono font-medium">{v.version}</span>
          {v.is_active && <Badge variant="success">active</Badge>}
          {isAwaitingApproval && (
            <Badge variant="warning" className="gap-1">
              <Timer className="h-3 w-3" />
              Awaiting Approval
            </Badge>
          )}
        </div>
      </td>
      <td className="px-4 py-3 text-muted-foreground">{v.model_name}</td>
      <td className="px-4 py-3 text-muted-foreground">{v.dimensions}</td>
      <td className="px-4 py-3">
        <StatusBadge status={displayStatus} />
      </td>
      <td className="px-4 py-3">
        <div className="flex items-center gap-3">
          <Progress value={pct} className="w-24" />
          <span className="text-xs text-muted-foreground">
            {processed}/{total}
          </span>
        </div>
        {isAwaitingApproval && "pending_update" in progress && progress.pending_update && (
          <p className="mt-1 text-xs text-amber-600">{progress.pending_update}</p>
        )}
      </td>
      <td className="px-4 py-3 text-right">
        {polling && displayStatus === "in_progress" && !isAwaitingApproval && (
          <Button variant="ghost" size="sm" onClick={handlePause}>
            <Pause className="h-3.5 w-3.5" />
            Pause
          </Button>
        )}
        {polling && displayStatus === "paused" && (
          <Button variant="ghost" size="sm" onClick={handleResume}>
            <Play className="h-3.5 w-3.5" />
            Resume
          </Button>
        )}
        {isAwaitingApproval && (
          <div className="flex items-center gap-2 justify-end">
            <Button variant="ghost" size="sm" onClick={handleReject} className="text-red-600 hover:text-red-700">
              <X className="h-3.5 w-3.5" />
              Reject
            </Button>
            <Button variant="ghost" size="sm" onClick={handleApprove} className="text-green-600 hover:text-green-700">
              <Check className="h-3.5 w-3.5" />
              Approve
            </Button>
          </div>
        )}
      </td>
    </tr>
  );
}

function StatusBadge({ status }: { status: string }) {
  const config: Record<
    string,
    { variant: "success" | "info" | "warning" | "destructive" | "secondary"; icon: React.ReactNode }
  > = {
    completed: {
      variant: "success",
      icon: <CheckCircle2 className="h-3 w-3" />,
    },
    in_progress: {
      variant: "info",
      icon: <Loader2 className="h-3 w-3 animate-spin" />,
    },
    awaiting_approval: {
      variant: "warning",
      icon: <Timer className="h-3 w-3" />,
    },
    pending: {
      variant: "warning",
      icon: <Clock className="h-3 w-3" />,
    },
    paused: {
      variant: "warning",
      icon: <Pause className="h-3 w-3" />,
    },
    rejected: {
      variant: "destructive",
      icon: <X className="h-3 w-3" />,
    },
    cancelled: {
      variant: "destructive",
      icon: <X className="h-3 w-3" />,
    },
    timeout: {
      variant: "destructive",
      icon: <Timer className="h-3 w-3" />,
    },
    failed: {
      variant: "destructive",
      icon: <AlertCircle className="h-3 w-3" />,
    },
  };

  const { variant, icon } = config[status] ?? {
    variant: "secondary" as const,
    icon: null,
  };

  return (
    <Badge variant={variant} className="gap-1">
      {icon}
      {status.replace(/_/g, " ")}
    </Badge>
  );
}
