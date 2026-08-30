import * as v from "valibot";
import { msg } from "@lit/localize";

export function getSetupSchema() {
  return v.object({
    orgName: v.pipe(
      v.string(msg("Organization name is required")),
      v.nonEmpty(msg("Please enter your organization name")),
    ),
    name: v.pipe(
      v.string(msg("Name is required")),
      v.nonEmpty(msg("Please enter your name")),
    ),
    email: v.pipe(
      v.string(msg("Email is required")),
      v.nonEmpty(msg("Please enter your email")),
      v.email(msg("Please enter a valid email address")),
    ),
    password: v.pipe(
      v.string(msg("Password is required")),
      v.nonEmpty(msg("Please enter your password")),
      v.minLength(8, msg("Password must be at least 8 characters")),
    ),
  });
}
