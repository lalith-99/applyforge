"use client";

import { useQuery } from "@tanstack/react-query";
import { api, ApiError } from "@/lib/api";
import type { User } from "@/types/api";

export function useCurrentUser() {
  return useQuery<User | null>({
    queryKey: ["auth", "session"],
    queryFn: async () => {
      try {
        return await api.get<User>("/auth/session");
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) return null;
        throw err;
      }
    },
    retry: false,
  });
}
