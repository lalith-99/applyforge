import { z } from "zod";

export const credentialsSchema = z.object({
  email: z.string().email("Enter a valid email address"),
  password: z.string().min(8, "Password must be at least 8 characters"),
});

export type CredentialsInput = z.infer<typeof credentialsSchema>;
