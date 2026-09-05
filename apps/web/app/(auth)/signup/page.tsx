import Link from "next/link";
import { AuthForm } from "@/features/auth/AuthForm";

export default function SignupPage() {
  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-6 p-8">
      <div className="flex flex-col items-center gap-2 text-center">
        <h1 className="text-2xl font-semibold">Create your ApplyForge account</h1>
        <p className="text-sm text-black/60 dark:text-white/60">
          Go from job posting to interview-ready.
        </p>
      </div>
      <AuthForm mode="signup" />
      <p className="text-sm text-black/60 dark:text-white/60">
        Already have an account?{" "}
        <Link href="/login" className="underline">
          Log in
        </Link>
      </p>
    </main>
  );
}
