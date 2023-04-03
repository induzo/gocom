window.BENCHMARK_DATA = {
  "lastUpdate": 1680505080633,
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
          "id": "b674d6feeb830a0cf6714a0df61e232642561f0e",
          "message": "feat: add http/health",
          "timestamp": "2023-04-03T14:57:01+08:00",
          "tree_id": "8cc38e2516e72915926d7113dc5ed9bc7e20e8f7",
          "url": "https://github.com/induzo/gocom/commit/b674d6feeb830a0cf6714a0df61e232642561f0e"
        },
        "date": 1680505079716,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkNewContext",
            "value": 53.28,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "22744608 times\n2 procs"
          },
          {
            "name": "BenchmarkFromContext/logger_in_context",
            "value": 8.967,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "137135962 times\n2 procs"
          },
          {
            "name": "BenchmarkFromContext/no_logger_in_context",
            "value": 5.927,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "204026007 times\n2 procs"
          }
        ]
      }
    ]
  }
}