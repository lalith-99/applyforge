"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import type { Profile } from "@/types/api";

export default function AuthContinuePage() {
  const router = useRouter();

  useEffect(() => {
    let active = true;
    void api
      .get<Profile>("/profile")
      .then((profile) => {
        if (!active) return;
        router.replace(profile.onboarding_completed_at ? "/dashboard" : "/onboarding");
      })
      .catch(() => {
        if (!active) return;
        router.replace("/onboarding");
      });

    return () => {
      active = false;
    };
  }, [router]);

  return (
    <main className="flex min-h-screen items-center justify-center p-8">
      <p className="text-sm text-black/60 dark:text-white/60">Finishing sign in…</p>
    </main>
  );
}
