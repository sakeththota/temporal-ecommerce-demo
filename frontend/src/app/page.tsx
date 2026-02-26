"use client";

import { useState, useCallback, useEffect } from "react";
import Link from "next/link";
import { searchHotels, getVersions } from "@/lib/api";
import type { SearchResult, EmbeddingVersion } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import {
  Search,
  MapPin,
  Star,
  Loader2,
  Sparkles,
  Building2,
  Wifi,
  Car,
  UtensilsCrossed,
  Waves,
  Dumbbell,
  Wind,
  Coffee,
  Tv,
  Bath,
  TreePine,
} from "lucide-react";

const amenityIcons: Record<string, React.ReactNode> = {
  wifi: <Wifi className="h-3 w-3" />,
  "free wifi": <Wifi className="h-3 w-3" />,
  parking: <Car className="h-3 w-3" />,
  "free parking": <Car className="h-3 w-3" />,
  restaurant: <UtensilsCrossed className="h-3 w-3" />,
  pool: <Waves className="h-3 w-3" />,
  "swimming pool": <Waves className="h-3 w-3" />,
  gym: <Dumbbell className="h-3 w-3" />,
  fitness: <Dumbbell className="h-3 w-3" />,
  "fitness center": <Dumbbell className="h-3 w-3" />,
  spa: <Bath className="h-3 w-3" />,
  "air conditioning": <Wind className="h-3 w-3" />,
  "room service": <Coffee className="h-3 w-3" />,
  tv: <Tv className="h-3 w-3" />,
  garden: <TreePine className="h-3 w-3" />,
};

function getAmenityIcon(amenity: string) {
  const key = amenity.toLowerCase();
  return amenityIcons[key] ?? null;
}

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
    <div className="space-y-8">
      {/* Hero section */}
      <div className="relative overflow-hidden rounded-2xl bg-gradient-to-br from-blue-600 via-blue-700 to-indigo-800 px-8 py-12 text-white shadow-lg">
        <div className="absolute inset-0 bg-[url('data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iNjAiIGhlaWdodD0iNjAiIHZpZXdCb3g9IjAgMCA2MCA2MCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj48ZyBmaWxsPSJub25lIiBmaWxsLXJ1bGU9ImV2ZW5vZGQiPjxnIGZpbGw9IiNmZmYiIGZpbGwtb3BhY2l0eT0iMC4wNSI+PGNpcmNsZSBjeD0iMzAiIGN5PSIzMCIgcj0iMiIvPjwvZz48L2c+PC9zdmc+')] opacity-60" />
        <div className="relative space-y-4">
          <div className="flex items-center gap-2">
            <Sparkles className="h-5 w-5 text-blue-200" />
            <span className="text-sm font-medium text-blue-200">
              AI-Powered Semantic Search
            </span>
          </div>
          <h1 className="text-3xl font-bold tracking-tight sm:text-4xl">
            Find your perfect stay
          </h1>
          <p className="max-w-xl text-blue-100">
            Describe what you&apos;re looking for in natural language. Our
            semantic search understands context, not just keywords.
          </p>

          {/* Search bar */}
          <form onSubmit={handleSearch} className="flex gap-3 pt-2">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <Input
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder='Try "beachfront resort with pool" or "cozy mountain cabin"...'
                className="h-11 border-white/20 bg-white/10 pl-10 text-white placeholder:text-blue-200/60 backdrop-blur-sm focus-visible:ring-white/30"
              />
            </div>
            <Button
              type="submit"
              disabled={loading || !query.trim()}
              className="h-11 bg-white px-6 text-blue-700 shadow-md hover:bg-blue-50"
            >
              {loading ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                "Search"
              )}
            </Button>
          </form>

          {/* Active model badge */}
          {active && (
            <div className="flex items-center gap-2 pt-1 text-xs text-blue-200">
              <span className="inline-flex h-1.5 w-1.5 rounded-full bg-emerald-400" />
              Using {active.model_name} ({active.dimensions}d) &middot;{" "}
              {active.version}
            </div>
          )}
        </div>
      </div>

      {/* Results */}
      {searched && results.length === 0 && (
        <div className="rounded-lg border border-dashed p-8 text-center">
          <p className="text-sm text-muted-foreground">
            No hotels matched your search. Try a different description.
          </p>
        </div>
      )}

      {results.length > 0 && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold text-foreground">
              {results.length} {results.length === 1 ? "result" : "results"}{" "}
              found
            </h2>
            <span className="text-xs text-muted-foreground">
              Sorted by relevance
            </span>
          </div>

          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {results.map((r) => (
              <HotelCard key={r.hotel.id} result={r} />
            ))}
          </div>
        </div>
      )}

      {/* Quick suggestions when no search yet */}
      {!searched && (
        <div className="space-y-3">
          <p className="text-sm font-medium text-muted-foreground">
            Popular searches
          </p>
          <div className="flex flex-wrap gap-2">
            {[
              "Beachfront resort with pool",
              "Luxury downtown hotel",
              "Cozy mountain cabin",
              "Family-friendly with kids activities",
              "Pet-friendly with garden",
              "Romantic getaway with spa",
            ].map((suggestion) => (
              <button
                key={suggestion}
                onClick={() => {
                  setQuery(suggestion);
                }}
                className="rounded-full border bg-white px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:border-primary hover:text-primary"
              >
                {suggestion}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function HotelCard({ result }: { result: SearchResult }) {
  const { hotel, similarity } = result;
  const pct = Math.round(similarity * 100);

  return (
    <Card className="group overflow-hidden transition-shadow hover:shadow-md">
      {/* Image */}
      <div className="relative aspect-[4/3] overflow-hidden bg-muted">
        {hotel.image_url ? (
          <img
            src={hotel.image_url}
            alt={hotel.name}
            className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-muted-foreground">
            <Building2 className="h-8 w-8" />
          </div>
        )}
        {/* Match badge */}
        <Badge
          variant="secondary"
          className="absolute right-2 top-2 bg-white/90 font-mono text-xs backdrop-blur-sm"
        >
          {pct}% match
        </Badge>
      </div>

      {/* Content */}
      <div className="space-y-3 p-4">
        <div>
          <div className="flex items-start justify-between gap-2">
            <h3 className="font-semibold leading-tight text-foreground">
              {hotel.name}
            </h3>
            <div className="flex items-center gap-0.5 text-amber-500">
              <Star className="h-3.5 w-3.5 fill-current" />
              <span className="text-xs font-medium">
                {(4 + similarity).toFixed(1)}
              </span>
            </div>
          </div>
          <div className="mt-1 flex items-center gap-1 text-sm text-muted-foreground">
            <MapPin className="h-3 w-3" />
            {hotel.location}
          </div>
        </div>

        <p className="line-clamp-2 text-sm leading-relaxed text-muted-foreground">
          {hotel.description}
        </p>

        {/* Amenities */}
        <div className="flex flex-wrap gap-1.5">
          {hotel.amenities.slice(0, 4).map((a) => (
            <span
              key={a}
              className="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground"
            >
              {getAmenityIcon(a)}
              {a}
            </span>
          ))}
          {hotel.amenities.length > 4 && (
            <span className="rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground">
              +{hotel.amenities.length - 4} more
            </span>
          )}
        </div>

        {/* Price + CTA */}
        <div className="flex items-end justify-between border-t pt-3">
          <div>
            <span className="text-lg font-bold text-foreground">
              ${hotel.price_per_night}
            </span>
            <span className="text-sm text-muted-foreground"> / night</span>
          </div>
          <Button asChild size="sm">
            <Link href={`/book?hotel=${hotel.id}`}>Book Now</Link>
          </Button>
        </div>
      </div>
    </Card>
  );
}


