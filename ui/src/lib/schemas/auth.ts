import * as v from "valibot";
import { msg } from "@lit/localize";

export function getLoginSchema() {
  return v.object({
    email: v.pipe(
      v.string(msg("Email is required")),
      v.nonEmpty(msg("Please enter your email")),
      v.email(msg("Please enter a valid email address")),
    ),
    password: v.pipe(
      v.string(msg("Password is required")),
      v.nonEmpty(msg("Please enter your password")),
    ),
  });
}

export type LoginInput = v.InferInput<ReturnType<typeof getLoginSchema>>;
