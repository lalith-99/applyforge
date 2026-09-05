import Link from "next/link";
import { AuthForm } from "@/features/auth/AuthForm";

export default function LoginPage() {
  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-6 p-8">
      <div className="flex flex-col items-center gap-2 text-center">
        <h1 className="text-2xl font-semibold">Log in to ApplyForge</h1>
      </div>
      <AuthForm mode="login" />
      <p className="text-sm text-black/60 dark:text-white/60">
        Need an account?{" "}
        <Link href="/signup" className="underline">
          Sign up
        </Link>
      </p>
    </main>
  );
}
