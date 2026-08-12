# Activity metric data notes

## Sources and semantics

- `heartbeats.time` is indexed and is the range key for the selected Wrapped calendar year.
- Activity duration is derived from heartbeat intervals capped by the effective profile timeout. The current imported profile uses a 15-minute timeout and `writes_only=true`.
- Daily totals, peak day, streaks, and heatmap intensity are calculated from write-filtered heartbeat intervals in the configured timezone.
- Longest AI task groups consecutive heartbeats with the same non-empty `ai_session`; a gap longer than the effective timeout starts a new task.
- `ai_input_tokens` and `ai_output_tokens` represent usage since the previous heartbeat, so annual token usage is the sum of token-bearing heartbeats inside the selected year and intentionally ignores the activity `writes_only` filter.
- `GET /api/v2/wrapped?year=YYYY` returns these metrics under `activity`; there is no separate Activity Matrix endpoint.

## Schema limitations

The heartbeat schema and sampled `origin_payload` values contain AI session, subscription plan, token, prompt, and line-change telemetry. They do not contain provider quota, credits, remaining resets, or reset timestamps. Those metrics require a new ingestion source and must not be inferred from heartbeat activity.

## Performance

Annual activity and token reads use `idx_heartbeats_time`. Tokens use a range-bound SQL aggregate instead of loading token values into the frontend.
