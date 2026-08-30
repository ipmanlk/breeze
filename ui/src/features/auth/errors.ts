import { msg } from "@lit/localize";

/**
 * Extracts a user-friendly error message from a thrown SDK error.
 *
 * The @hey-api/client-fetch SDK throws the raw parsed JSON body on error
 * responses (when throwOnError: true). The body shape is
 * `{ error: { code: string, message: string } }` as produced by the backend
 * `transport.ErrorJSON()`. For rate-limit responses the body is
 * `{ error: { code: "rate_limited", message: "too many requests" } }`.
 *
 * Falls back to `err.message` for Error instances and a generic message for
 * other unexpected throw types.
 */
export function extractAuthError(err: unknown, fallback?: string): string {
  // Plain objects thrown by the SDK on JSON error responses
  if (
    err != null &&
    typeof err === "object" &&
    "error" in err &&
    err.error != null &&
    typeof err.error === "object" &&
    "code" in (err.error as Record<string, unknown>)
  ) {
    const apiErr = err as {
      error: { code: string; message: string };
    };
    return mapAuthErrorCode(apiErr.error.code, apiErr.error.message);
  }

  // Raw text thrown when the error body is not valid JSON
  if (typeof err === "string") {
    return err;
  }

  // Standard Error instances (e.g. network failure)
  if (err instanceof Error) {
    return err.message;
  }

  return fallback || msg("Login failed. Please try again.");
}

function mapAuthErrorCode(code: string, fallbackMessage: string): string {
  switch (code) {
    case "rate_limited":
      return msg("Too many attempts. Please wait a minute and try again.");
    case "auth_error":
      return msg("Invalid email or password.");
    case "setup_required":
      return msg("Setup is required before logging in.");
    case "validation_error":
      return fallbackMessage;
    default:
      return fallbackMessage || msg("Login failed. Please try again.");
  }
}
