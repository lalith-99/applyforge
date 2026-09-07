import { VerifyEmailClient } from "@/features/auth/VerifyEmailClient";

export default async function VerifyEmailPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string }>;
}) {
  const { token = "" } = await searchParams;

  return (
    <main className="mx-auto flex min-h-screen w-full max-w-md flex-col justify-center gap-6 p-8">
      <div>
        <h1 className="text-2xl font-semibold">Verify your email</h1>
        <p className="mt-1 text-sm text-black/60 dark:text-white/60">
          Email verification protects password-based ApplyForge accounts and recovery flows.
        </p>
      </div>
      <VerifyEmailClient token={token} />
    </main>
  );
}
