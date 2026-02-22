"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import {
  getVersions,
  startMigration,
  getMigrationProgress,
  pauseMigration,
  resumeMigration,
} from "@/lib/api";
import type {
  EmbeddingVersion,
  MigrationProgress,
  StartMigrationRequest,
} from "@/lib/types";

export default function MigrationsPage() {
  const [versions, setVersions] = useState<EmbeddingVersion[]>([]);

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

  return (
    <div className="space-y-8">
      <h1 className="text-2xl font-bold tracking-tight">
        Migration Dashboard
      </h1>
      <StartMigrationForm
        onStarted={loadVersions}
        existingVersions={versions.map((v) => v.version)}
      />
      <VersionsTable versions={versions} />
    </div>
  );
}

function StartMigrationForm({
  onStarted,
  existingVersions,
}: {
  onStarted: () => void;
  existingVersions: string[];
}) {
  const [version, setVersion] = useState("");
  const [modelName, setModelName] = useState("nomic-embed-text");
  const [dimensions, setDimensions] = useState(768);
  const [batchSize, setBatchSize] = useState(10);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      setError(null);

      if (!version.trim()) {
        setError("Version is required");
        return;
      }
      if (existingVersions.includes(version.trim())) {
        setError(`Version "${version}" already exists`);
        return;
      }

      setSubmitting(true);
      try {
        const req: StartMigrationRequest = {
          version: version.trim(),
          model_name: modelName,
          dimensions,
          batch_size: batchSize,
        };
        await startMigration(req);
        setVersion("");
        onStarted();
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Failed to start migration"
        );
      } finally {
        setSubmitting(false);
      }
    },
    [version, modelName, dimensions, batchSize, existingVersions, onStarted]
  );

  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-900 p-5 space-y-4">
      <h2 className="text-lg font-semibold">Start New Migration</h2>
      <form onSubmit={handleSubmit} className="grid grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-zinc-400">Version</label>
          <input
            type="text"
            value={version}
            onChange={(e) => setVersion(e.target.value)}
            placeholder="e.g. v3"
            className="w-full rounded border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-zinc-500"
          />
        </div>
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-zinc-400">
            Model Name
          </label>
          <input
            type="text"
            value={modelName}
            onChange={(e) => setModelName(e.target.value)}
            className="w-full rounded border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-zinc-500"
          />
        </div>
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-zinc-400">
            Dimensions
          </label>
          <input
            type="number"
            value={dimensions}
            onChange={(e) => setDimensions(Number(e.target.value))}
            className="w-full rounded border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-zinc-500"
          />
        </div>
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-zinc-400">
            Batch Size
          </label>
          <input
            type="number"
            value={batchSize}
            onChange={(e) => setBatchSize(Number(e.target.value))}
            className="w-full rounded border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-zinc-500"
          />
        </div>
        <div className="col-span-2 flex items-center gap-3">
          <button
            type="submit"
            disabled={submitting}
            className="rounded-lg bg-white px-5 py-2 text-sm font-medium text-zinc-900 hover:bg-zinc-200 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          >
            {submitting ? "Starting..." : "Start Migration"}
          </button>
          {error && <p className="text-sm text-red-400">{error}</p>}
        </div>
      </form>
    </div>
  );
}

function VersionsTable({ versions }: { versions: EmbeddingVersion[] }) {
  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">Embedding Versions</h2>
      {versions.length === 0 ? (
        <p className="text-sm text-zinc-500">No versions yet.</p>
      ) : (
        <div className="overflow-hidden rounded-lg border border-zinc-800">
          <table className="w-full text-sm">
            <thead className="bg-zinc-900 text-left text-xs font-medium text-zinc-400">
              <tr>
                <th className="px-4 py-3">Version</th>
                <th className="px-4 py-3">Model</th>
                <th className="px-4 py-3">Dims</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3">Progress</th>
                <th className="px-4 py-3">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-800">
              {versions.map((v) => (
                <VersionRow key={v.version} version={v} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function VersionRow({ version: v }: { version: EmbeddingVersion }) {
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
    } catch (err) {
      console.error("Pause failed:", err);
    }
  };

  const handleResume = async () => {
    try {
      await resumeMigration(v.version);
    } catch (err) {
      console.error("Resume failed:", err);
    }
  };

  return (
    <tr className="text-zinc-300">
      <td className="px-4 py-3 font-mono">
        <div className="flex items-center gap-2">
          {v.version}
          {v.is_active && (
            <span className="rounded-full bg-emerald-900/40 px-2 py-0.5 text-xs text-emerald-400 ring-1 ring-emerald-500/30">
              active
            </span>
          )}
        </div>
      </td>
      <td className="px-4 py-3">{v.model_name}</td>
      <td className="px-4 py-3">{v.dimensions}</td>
      <td className="px-4 py-3">
        <StatusBadge status={displayStatus} />
      </td>
      <td className="px-4 py-3">
        <div className="flex items-center gap-2">
          <div className="h-1.5 w-24 overflow-hidden rounded-full bg-zinc-800">
            <div
              className="h-full rounded-full bg-white transition-all duration-300"
              style={{ width: `${pct}%` }}
            />
          </div>
          <span className="text-xs text-zinc-500">
            {processed}/{total}
          </span>
        </div>
      </td>
      <td className="px-4 py-3">
        {polling && displayStatus === "in_progress" && (
          <button
            onClick={handlePause}
            className="rounded bg-zinc-800 px-3 py-1 text-xs text-zinc-300 hover:bg-zinc-700 transition-colors"
          >
            Pause
          </button>
        )}
        {polling && displayStatus === "paused" && (
          <button
            onClick={handleResume}
            className="rounded bg-zinc-800 px-3 py-1 text-xs text-zinc-300 hover:bg-zinc-700 transition-colors"
          >
            Resume
          </button>
        )}
      </td>
    </tr>
  );
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    completed: "bg-emerald-900/40 text-emerald-400 ring-emerald-500/30",
    in_progress: "bg-blue-900/40 text-blue-400 ring-blue-500/30",
    pending: "bg-yellow-900/40 text-yellow-400 ring-yellow-500/30",
    paused: "bg-orange-900/40 text-orange-400 ring-orange-500/30",
    failed: "bg-red-900/40 text-red-400 ring-red-500/30",
  };

  const cls = styles[status] ?? "bg-zinc-800 text-zinc-400 ring-zinc-600/30";

  return (
    <span
      className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ring-1 ${cls}`}
    >
      {status}
    </span>
  );
}
