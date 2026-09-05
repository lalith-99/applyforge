"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { api, ApiError } from "@/lib/api";
import { credentialsSchema, type CredentialsInput } from "@/lib/schemas/auth";
import type { Profile, User } from "@/types/api";

interface AuthFormProps {
  mode: "signup" | "login";
}

export function AuthForm({ mode }: AuthFormProps) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<CredentialsInput>({ resolver: zodResolver(credentialsSchema) });

  const mutation = useMutation({
    mutationFn: (data: CredentialsInput) =>
      api.post<User>(`/auth/${mode}`, data),
    onSuccess: async (user) => {
      queryClient.setQueryData(["auth", "session"], user);

      // Signups never have a profile yet - always send them to onboarding.
      // Logins must NOT unconditionally do this: it used to send every
      // returning user back through the onboarding wizard, whose blank
      // form then overwrote their saved profile on resubmission.
      if (mode === "signup") {
        router.push("/onboarding");
        return;
      }

      try {
        const profile = await api.get<Profile>("/profile");
        router.push(profile.onboarding_completed_at ? "/dashboard" : "/onboarding");
      } catch {
        router.push("/onboarding");
      }
    },
  });

  return (
    <form
      onSubmit={handleSubmit((data) => mutation.mutate(data))}
      className="flex w-full max-w-sm flex-col gap-4"
    >
      <div className="flex flex-col gap-1">
        <label htmlFor="email" className="text-sm font-medium">
          Email
        </label>
        <input
          id="email"
          type="email"
          autoComplete="email"
          {...register("email")}
          className="rounded-md border border-black/10 bg-transparent px-3 py-2 text-sm outline-none focus:border-black/30 dark:border-white/15 dark:focus:border-white/30"
        />
        {errors.email && (
          <p className="text-xs text-red-600">{errors.email.message}</p>
        )}
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor="password" className="text-sm font-medium">
          Password
        </label>
        <input
          id="password"
          type="password"
          autoComplete={mode === "signup" ? "new-password" : "current-password"}
          {...register("password")}
          className="rounded-md border border-black/10 bg-transparent px-3 py-2 text-sm outline-none focus:border-black/30 dark:border-white/15 dark:focus:border-white/30"
        />
        {errors.password && (
          <p className="text-xs text-red-600">{errors.password.message}</p>
        )}
      </div>

      {mutation.isError && (
        <p className="text-sm text-red-600">
          {mutation.error instanceof ApiError
            ? mutation.error.message
            : "Something went wrong. Please try again."}
        </p>
      )}

      <button
        type="submit"
        disabled={mutation.isPending}
        className="rounded-md bg-foreground px-4 py-2 text-sm font-medium text-background disabled:opacity-60"
      >
        {mutation.isPending
          ? "Please wait…"
          : mode === "signup"
            ? "Create account"
            : "Log in"}
      </button>

      <a
        href={`${process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080/api/v1"}/auth/google/start`}
        className="rounded-md border border-black/10 px-4 py-2 text-center text-sm font-medium dark:border-white/15"
      >
        Continue with Google
      </a>
    </form>
  );
}
