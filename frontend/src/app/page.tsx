"use client";

import { useState, useCallback, useEffect } from "react";
import { searchHotels, getVersions } from "@/lib/api";
import type { SearchResult, EmbeddingVersion } from "@/lib/types";

export default function SearchPage() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);
  const [active, setActive] = useState<EmbeddingVersion | null>(null);

  useEffect(() => {
    getVersions()
      .then((versions) => setActive(versions.find((v) => v.is_active) ?? null))
      .catch(() => {});
  }, []);

  const handleSearch = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!query.trim()) return;

      setLoading(true);
      try {
        const data = await searchHotels(query.trim());
        setResults(data);
        setSearched(true);
      } catch (err) {
        console.error("Search failed:", err);
      } finally {
        setLoading(false);
      }
    },
    [query]
  );

  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-bold tracking-tight">
          Semantic Hotel Search
        </h1>
        {active && (
          <p className="text-sm text-zinc-500">
            Using <ActiveBadge version={active} />
          </p>
        )}
      </div>

      <form onSubmit={handleSearch} className="flex gap-3">
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder='Try "beachfront resort with pool" or "cozy mountain cabin"...'
          className="flex-1 rounded-lg border border-zinc-700 bg-zinc-900 px-4 py-2.5 text-sm text-zinc-100 placeholder-zinc-500 outline-none focus:border-zinc-500 focus:ring-1 focus:ring-zinc-500 transition-colors"
        />
        <button
          type="submit"
          disabled={loading || !query.trim()}
          className="rounded-lg bg-white px-5 py-2.5 text-sm font-medium text-zinc-900 hover:bg-zinc-200 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          {loading ? "Searching..." : "Search"}
        </button>
      </form>

      {searched && results.length === 0 && (
        <p className="text-sm text-zinc-500">No results found.</p>
      )}

      {results.length > 0 && (
        <div className="space-y-3">
          {results.map((r) => (
            <HotelCard key={r.hotel.id} result={r} />
          ))}
        </div>
      )}
    </div>
  );
}

function ActiveBadge({ version }: { version: EmbeddingVersion }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-900/40 px-2.5 py-0.5 text-xs font-medium text-emerald-400 ring-1 ring-emerald-500/30">
      <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />
      {version.version} &middot; {version.model_name} &middot;{" "}
      {version.dimensions}d
    </span>
  );
}

function HotelCard({ result }: { result: SearchResult }) {
  const { hotel, similarity } = result;
  const pct = (similarity * 100).toFixed(1);

  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-900 p-4 space-y-2">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <h3 className="font-semibold text-zinc-100">{hotel.name}</h3>
          <p className="text-sm text-zinc-400">{hotel.location}</p>
        </div>
        <div className="text-right shrink-0 space-y-1">
          <div className="text-sm font-mono text-zinc-300">{pct}%</div>
          <div className="text-sm text-zinc-500">
            ${hotel.price_per_night}/night
          </div>
        </div>
      </div>
      <p className="text-sm text-zinc-400 leading-relaxed">
        {hotel.description}
      </p>
      {hotel.amenities.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {hotel.amenities.map((a) => (
            <span
              key={a}
              className="rounded bg-zinc-800 px-2 py-0.5 text-xs text-zinc-400"
            >
              {a}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
