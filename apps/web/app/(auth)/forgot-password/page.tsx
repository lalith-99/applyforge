"use client";

import { useState } from "react";
import Link from "next/link";
import { api, ApiError } from "@/lib/api";

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setPending(true);
    setError(null);
    try {
      await api.post("/auth/password-reset/request", { email });
      setSent(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not submit reset request.");
    } finally {
      setPending(false);
    }
  }

  return (
    <main className="mx-auto flex min-h-screen w-full max-w-md flex-col justify-center gap-6 p-8">
      <div>
        <h1 className="text-2xl font-semibold">Reset your password</h1>
        <p className="mt-1 text-sm text-black/60 dark:text-white/60">
          Enter your account email. We’ll send a one-time reset link if an account exists.
        </p>
      </div>
      {sent ? (
        <div className="rounded-md border border-black/10 p-4 text-sm dark:border-white/15">
          Check your inbox. The response is intentionally the same whether or not that email is registered.
        </div>
      ) : (
        <form onSubmit={submit} className="flex flex-col gap-4">
          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium">Email</span>
            <input
              type="email"
              required
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              className="rounded-md border border-black/10 bg-transparent px-3 py-2 dark:border-white/15"
            />
          </label>
          {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
          <button disabled={pending} className="rounded-md bg-foreground px-4 py-2 text-sm font-medium text-background disabled:opacity-60">
            {pending ? "Sending…" : "Send reset link"}
          </button>
        </form>
      )}
      <Link href="/login" className="text-sm underline">Back to login</Link>
    </main>
  );
}
