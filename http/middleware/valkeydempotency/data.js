window.BENCHMARK_DATA = {
  "lastUpdate": 1738149032216,
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
            "email": "noreply@github.com",
            "name": "GitHub",
            "username": "web-flow"
          },
          "distinct": true,
          "id": "cdfa363cdec81d322ccc4115d69f26355d8837b2",
          "message": "Merge pull request #14 from induzo/chore/refactor-valkeydempotency\n\nchore: refactor valkeydempotency",
          "timestamp": "2025-01-29T19:09:45+08:00",
          "tree_id": "20d55c9cfdbfb72b71e9815d201f1a8c549318af",
          "url": "https://github.com/induzo/gocom/commit/cdfa363cdec81d322ccc4115d69f26355d8837b2"
        },
        "date": 1738149031925,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkStoreStoreResponse",
            "value": 457005,
            "unit": "ns/op\t    2637 B/op\t      40 allocs/op",
            "extra": "2623 times\n4 procs"
          },
          {
            "name": "BenchmarkStoreStoreResponse - ns/op",
            "value": 457005,
            "unit": "ns/op",
            "extra": "2623 times\n4 procs"
          },
          {
            "name": "BenchmarkStoreStoreResponse - B/op",
            "value": 2637,
            "unit": "B/op",
            "extra": "2623 times\n4 procs"
          },
          {
            "name": "BenchmarkStoreStoreResponse - allocs/op",
            "value": 40,
            "unit": "allocs/op",
            "extra": "2623 times\n4 procs"
          }
        ]
      }
    ]
  }
}