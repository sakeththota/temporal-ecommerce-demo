"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import {
  getVersions,
  startMigration,
  getMigrationProgress,
  pauseMigration,
  resumeMigration,
  resetMigrations,
} from "@/lib/api";
import type {
  EmbeddingVersion,
  MigrationProgress,
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
} from "lucide-react";

export default function MigrationsPage() {
  const [versions, setVersions] = useState<EmbeddingVersion[]>([]);
  const [resetting, setResetting] = useState(false);

  const loadVersions = useCallback(async () => {
    try {
      const data = await getVersions();
      setVersions(data);
    } catch (err) {
      console.error("Failed to load versions:", err);
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
              href="http://localhost:8233"
              target="_blank"
              rel="noopener noreferrer"
            >
              <ExternalLink className="h-3.5 w-3.5" />
              Temporal UI
            </a>
          </Button>
          {versions.length > 1 && (
            <Button
              variant="destructive"
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

  const handleStartMigration = async (
    model: (typeof availableModels)[0]
  ) => {
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
      setError(
        err instanceof Error ? err.message : "Failed to start migration"
      );
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
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {availableModels.map((model) => {
            const isThisModelActive =
              activeVersion?.model_name === model.model_name;
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
          <div className="overflow-hidden rounded-lg border">
            <table className="w-full text-sm">
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
  const [progress, setProgress] = useState<MigrationProgress | null>(null);
  const [polling, setPolling] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const isInProgress = v.status === "pending" || v.status === "in_progress";

  useEffect(() => {
    if (!isInProgress) return;

    setPolling(true);
    const poll = async () => {
      try {
        const p = await getMigrationProgress(v.version);
        setProgress(p);
        if (p.status === "completed") {
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
    intervalRef.current = setInterval(poll, 1500);

    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [isInProgress, v.version]);

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

  return (
    <tr className="text-foreground">
      <td className="px-4 py-3">
        <div className="flex items-center gap-2">
          <span className="font-mono font-medium">{v.version}</span>
          {v.is_active && <Badge variant="success">active</Badge>}
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
      </td>
      <td className="px-4 py-3 text-right">
        {polling && displayStatus === "in_progress" && (
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
    pending: {
      variant: "warning",
      icon: <Clock className="h-3 w-3" />,
    },
    paused: {
      variant: "warning",
      icon: <Pause className="h-3 w-3" />,
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
      {status.replace("_", " ")}
    </Badge>
  );
}
