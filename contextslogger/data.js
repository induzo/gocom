window.BENCHMARK_DATA = {
  "lastUpdate": 1680512361591,
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
          "id": "79b87a871679051442698b0b0934060fca649fa4",
          "message": "docs(readme): update latest versions",
          "timestamp": "2023-04-03T16:58:21+08:00",
          "tree_id": "8ddaccd1322856b543029ef2cc726f6c57b100bb",
          "url": "https://github.com/induzo/gocom/commit/79b87a871679051442698b0b0934060fca649fa4"
        },
        "date": 1680512361064,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkNewContext",
            "value": 48.32,
            "unit": "ns/op\t      48 B/op\t       1 allocs/op",
            "extra": "24843904 times\n2 procs"
          },
          {
            "name": "BenchmarkFromContext/logger_in_context",
            "value": 7.314,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "164171785 times\n2 procs"
          },
          {
            "name": "BenchmarkFromContext/no_logger_in_context",
            "value": 4.059,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "296591319 times\n2 procs"
          }
        ]
      }
    ]
  }
}