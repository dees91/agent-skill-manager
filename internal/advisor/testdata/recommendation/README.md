# Recommendation fixtures

Synthetic cases for optional TypeSafe recommendations. Labels were fixed before
threshold tuning. Names and descriptions are invented and contain no real
inventory. Offline `go test` fixtures drive a scripted provider from the
expected labels; they verify request plumbing, chunking, and mapping, not Jev
quality. Live measurement uses the same files with the real API.

## Splits

- `dev`: used while implementing and for optional threshold tweaks
- `heldout`: reserved for live measurement; do not retune on these cases

## Metrics

Live evaluation (opt-in) reports selection errors, useful-selection recall,
no-skill false selections, retrieval recall, summed usage, and latency. Quality
failures are logged and never fail `go test`.

## Live run

```text
SKILL_MANAGER_LIVE_TYPESAFE=1 TYPESAFE_API_KEY=... go test ./internal/advisor/ -run TestLiveRecommendationEvaluation -count=1
```
