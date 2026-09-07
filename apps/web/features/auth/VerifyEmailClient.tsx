"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, ApiError } from "@/lib/api";
import type { Profile, User } from "@/types/api";

type VerifyState = "idle" | "verifying" | "verified" | "error";

export function VerifyEmailClient({ token }: { token: string }) {
  const router = useRouter();
  const [state, setState] = useState<VerifyState>(token ? "verifying" : "idle");
  const [message, setMessage] = useState(
    token ? "Verifying your email…" : "Check your inbox for a verification link.",
  );
  const [resending, setResending] = useState(false);

  useEffect(() => {
    if (!token) return;
    let active = true;

    void api
      .post<{ status: string }>("/auth/email-verification/confirm", { token })
      .then(async () => {
        if (!active) return;
        setState("verified");
        setMessage("Email verified. Continuing to ApplyForge…");

        try {
          await api.get<User>("/auth/session");
          const profile = await api.get<Profile>("/profile");
          router.replace(profile.onboarding_completed_at ? "/dashboard" : "/onboarding");
        } catch {
          router.replace("/login?verified=1");
        }
      })
      .catch((error) => {
        if (!active) return;
        setState("error");
        setMessage(
          error instanceof ApiError
            ? error.message
            : "This verification link could not be used.",
        );
      });

    return () => {
      active = false;
    };
  }, [router, token]);

  async function resend() {
    setResending(true);
    setMessage("");
    try {
      const result = await api.post<{ status: string }>("/auth/email-verification/request");
      if (result.status === "already_verified") {
        const profile = await api.get<Profile>("/profile");
        router.replace(profile.onboarding_completed_at ? "/dashboard" : "/onboarding");
        return;
      }
      setState("idle");
      setMessage("A new verification link has been sent.");
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        router.replace("/login");
        return;
      }
      setState("error");
      setMessage(
        error instanceof ApiError ? error.message : "Could not send a verification email.",
      );
    } finally {
      setResending(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <p
        role={state === "error" ? "alert" : undefined}
        className={
          state === "error"
            ? "rounded-md bg-red-50 p-3 text-sm text-red-700"
            : "rounded-md border border-black/10 p-3 text-sm dark:border-white/15"
        }
      >
        {message}
      </p>

      {!token && (
        <button
          type="button"
          onClick={resend}
          disabled={resending}
          className="rounded-md bg-foreground px-4 py-2 text-sm font-medium text-background disabled:opacity-60"
        >
          {resending ? "Sending…" : "Resend verification email"}
        </button>
      )}

      {state === "error" && token && (
        <button
          type="button"
          onClick={() => router.replace("/verify-email")}
          className="rounded-md border border-black/10 px-4 py-2 text-sm font-medium dark:border-white/15"
        >
          Request a new verification link
        </button>
      )}
    </div>
  );
}
