"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { AppNav } from "@/components/AppNav";
import { useCurrentUser } from "@/hooks/useCurrentUser";

export default function DashboardPage() {
  const router = useRouter();
  const { data: user, isLoading } = useCurrentUser();

  useEffect(() => {
    if (!isLoading && user === null) router.replace("/login");
  }, [isLoading, user, router]);

  if (isLoading) {
    return <main className="flex flex-1 items-center justify-center p-8">Loading…</main>;
  }

  return (
    <>
      <AppNav />
      <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col items-center justify-center gap-4 p-8 text-center">
        <h1 className="text-2xl font-semibold">Welcome to ApplyForge{user ? `, ${user.email}` : ""}</h1>
        <p className="text-sm text-black/60 dark:text-white/60">
          Upload your resume and browse job matches using the navigation above.
        </p>
      </main>
    </>
  );
}
