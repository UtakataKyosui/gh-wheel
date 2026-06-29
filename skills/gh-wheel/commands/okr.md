### `gh wheel okr`

Compute GitHub activity metrics for OKR key results

Compute GitHub activity metrics for OKR key-result tracking.

Designed to integrate with okr-hub: gh wheel okr metrics emits the metric keys
that okr-hub's okr-metrics-sync skill consumes, aggregated across repositories.

#### `gh wheel okr metrics`

Compute period GitHub metrics (cross-repo) for okr-hub KR sync

Compute GitHub activity metrics over a date range.

By default the search is cross-repo (author:@me / reviewed-by:@me across every
repository you can see), which suits personal OKRs. Pass -R owner/repo to scope
the metrics to a single repository.

The output uses the same metric keys as okr-hub's okr-metrics-sync skill, so it
can replace plugins/okr-progress/scripts/okr_github_metrics.py. Pass --krs with
the JSON produced by okr_parse.py to map each metric onto a key result.

Authentication failures surface as exit code 4 with an error envelope (not an
"available": false body); a successful run always reports "available": true.

```
gh wheel okr metrics --since <YYYY-MM-DD> --until <YYYY-MM-DD> [flags]
```

フラグ:

- `--krs` — Key results to match, as JSON: [{"label","title","metrics_source"}]
- `--since` — Start date (YYYY-MM-DD, inclusive) [required]
- `--until` — End date (YYYY-MM-DD, inclusive) [required]