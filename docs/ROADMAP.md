# Roadmap

> **Status:** these are **planned directions, not yet implemented**. Nothing
> here is a commitment or a schedule. The shipped product is documented in the
> other `docs/*.md` files and `README.md`. This page exists only to record
> directions that have been designed or discussed, so they aren't lost.

Plume ships as a single Go binary with an embedded SPA and **zero external
dependencies**. Any future feature must preserve that constraint: no new
external services, and no breaking schema changes without additive migrations.

## Voice screen sharing

Add screen-share video tracks to existing voice channels, reusing the SFU
audio-forwarding path. Add a video transceiver to the existing publisher peer
connection and renegotiate on start/stop. No database changes: state is driven
by WebRTC track detection and broadcast over the existing WebSocket rooms.
Tracked as a future item in
[`api/voice-channels.md`](api/voice-channels.md#future-enhancements).

## Opt-in AI module (BYOK + MCP)

An **optional, fully opt-in** AI module was designed but **not built**:

- Users bring their own provider keys (BYOK) for any OpenAI-compatible
  endpoint. Plume never sells, proxies, or bills for AI.
- Zero cost when off: no goroutines, listeners, or DB connections unless
  enabled (mirrors the existing SMTP/VAPID `Enabled()` no-op precedent).
- Plume can additionally expose itself as an MCP server (separate,
  token-authed listener) so external LLM clients can read/manage projects
  through the user's own credentials.

The detailed design (architecture, provider layer, MCP surface, feature
phases, security model) was written up as a plan and deliberately **not
carried into the shipped docs**, because the feature does not exist yet. If
work starts, the design should be revisited against the current codebase
rather than treated as a fixed spec.
