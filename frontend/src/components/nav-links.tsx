"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Search, CalendarDays, Settings } from "lucide-react";

const links = [
  { href: "/", label: "Search", icon: Search },
  { href: "/book", label: "Book", icon: CalendarDays },
  { href: "/migrations", label: "Migrations", icon: Settings },
];

export function NavLinks() {
  const pathname = usePathname();

  return (
    <nav className="flex items-center gap-1">
      {links.map(({ href, label, icon: Icon }) => {
        const isActive =
          href === "/" ? pathname === "/" : pathname.startsWith(href);

        return (
          <Link
            key={href}
            href={href}
            className={`flex items-center gap-1.5 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
              isActive
                ? "bg-accent text-foreground"
                : "text-muted-foreground hover:bg-accent hover:text-foreground"
            }`}
          >
            <Icon className="h-4 w-4" />
            {label}
          </Link>
        );
      })}
    </nav>
  );
}
