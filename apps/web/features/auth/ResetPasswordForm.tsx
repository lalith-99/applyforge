"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api, ApiError } from "@/lib/api";

export function ResetPasswordForm({ token }: { token: string }) {
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (password.length < 8) {
      setError("Password must be at least 8 characters.");
      return;
    }
    if (password !== confirm) {
      setError("Passwords do not match.");
      return;
    }
    setPending(true);
    setError(null);
    try {
      await api.post("/auth/password-reset/confirm", { token, password });
      router.replace("/login?reset=1");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not reset password.");
    } finally {
      setPending(false);
    }
  }

  if (!token) {
    return <p className="text-sm text-red-600">This reset link is missing its one-time token.</p>;
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <label className="flex flex-col gap-1 text-sm">
        <span className="font-medium">New password</span>
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} className="rounded-md border border-black/10 bg-transparent px-3 py-2 dark:border-white/15" />
      </label>
      <label className="flex flex-col gap-1 text-sm">
        <span className="font-medium">Confirm password</span>
        <input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} className="rounded-md border border-black/10 bg-transparent px-3 py-2 dark:border-white/15" />
      </label>
      {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
      <button disabled={pending} className="rounded-md bg-foreground px-4 py-2 text-sm font-medium text-background disabled:opacity-60">
        {pending ? "Resetting…" : "Reset password"}
      </button>
    </form>
  );
}
