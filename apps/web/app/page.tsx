import Link from "next/link";

export default function Home() {
  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-8 p-8 text-center">
      <div className="flex flex-col gap-3">
        <h1 className="text-4xl font-semibold tracking-tight">
          Stop applying with the same resume.
        </h1>
        <p className="max-w-lg text-black/60 dark:text-white/60">
          ApplyForge discovers high-fit opportunities, analyzes job descriptions,
          tailors your resume, identifies skill gaps, and tracks your applications.
        </p>
      </div>
      <div className="flex gap-3">
        <Link
          href="/signup"
          className="rounded-md bg-foreground px-5 py-2.5 text-sm font-medium text-background"
        >
          Find My Matches
        </Link>
        <Link
          href="/login"
          className="rounded-md border border-black/10 px-5 py-2.5 text-sm font-medium dark:border-white/15"
        >
          Log in
        </Link>
      </div>
    </main>
  );
}

