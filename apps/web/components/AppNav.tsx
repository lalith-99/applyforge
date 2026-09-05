"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

const NAV_LINKS = [
  { href: "/dashboard", label: "Dashboard" },
  { href: "/jobs", label: "Jobs" },
  { href: "/applications", label: "Applications" },
  { href: "/analytics", label: "Analytics" },
  { href: "/resume", label: "Resume" },
];

export function AppNav() {
  const router = useRouter();
  const queryClient = useQueryClient();

  const handleLogout = async () => {
    try {
      await api.post("/auth/logout");
    } finally {
      queryClient.clear();
      router.push("/login");
      router.refresh();
    }
  };

  return (
    <nav className="flex items-center gap-6 border-b border-black/10 px-8 py-4 text-sm dark:border-white/15">
      <span className="font-semibold">ApplyForge</span>
      {NAV_LINKS.map((link) => (
        <Link key={link.href} href={link.href} className="text-black/70 hover:text-black dark:text-white/70 dark:hover:text-white">
          {link.label}
        </Link>
      ))}
      <button
        type="button"
        onClick={handleLogout}
        className="ml-auto text-black/70 hover:text-black dark:text-white/70 dark:hover:text-white"
      >
        Log out
      </button>
    </nav>
  );
}
