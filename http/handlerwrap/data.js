window.BENCHMARK_DATA = {
  "lastUpdate": 1681285952296,
  "repoUrl": "https://github.com/induzo/gocom",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "email": "vincent@serpoul.com",
            "name": "Vincent Serpoul",
            "username": "vincentserpoul"
          },
          "committer": {
            "email": "vincent@serpoul.com",
            "name": "Vincent Serpoul",
            "username": "vincentserpoul"
          },
          "distinct": true,
          "id": "35a02a99b9e14ad1fb617f64c2378c520ab59d50",
          "message": "feat: add bench for handlerwrap",
          "timestamp": "2023-04-12T15:48:35+08:00",
          "tree_id": "ebc60ad4013d2c9496b26e85733e474f84234293",
          "url": "https://github.com/induzo/gocom/commit/35a02a99b9e14ad1fb617f64c2378c520ab59d50"
        },
        "date": 1681285777676,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkParsePaginationQueryParams",
            "value": 580.6,
            "unit": "ns/op\t     496 B/op\t       5 allocs/op",
            "extra": "2088454 times\n2 procs"
          },
          {
            "name": "BenchmarkHTTPWrapper",
            "value": 760.8,
            "unit": "ns/op\t     285 B/op\t       5 allocs/op",
            "extra": "1639256 times\n2 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "vincent@serpoul.com",
            "name": "Vincent Serpoul",
            "username": "vincentserpoul"
          },
          "committer": {
            "email": "vincent@serpoul.com",
            "name": "Vincent Serpoul",
            "username": "vincentserpoul"
          },
          "distinct": true,
          "id": "2cac99c54939e615da63b9134b6000af97d2230d",
          "message": "chore: fetch-depth 1 for most cases is enough",
          "timestamp": "2023-04-12T15:51:25+08:00",
          "tree_id": "211524769c2bb4d885d6e2260d4ba08280dec1fe",
          "url": "https://github.com/induzo/gocom/commit/2cac99c54939e615da63b9134b6000af97d2230d"
        },
        "date": 1681285951769,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkParsePaginationQueryParams",
            "value": 598.8,
            "unit": "ns/op\t     496 B/op\t       5 allocs/op",
            "extra": "2212039 times\n2 procs"
          },
          {
            "name": "BenchmarkHTTPWrapper",
            "value": 785.3,
            "unit": "ns/op\t     287 B/op\t       5 allocs/op",
            "extra": "1610535 times\n2 procs"
          }
        ]
      }
    ]
  }
}