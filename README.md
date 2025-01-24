# GoBandit 🎯 - High-Performance Multi-Armed Bandit Engine in Go

[![made-with-Go](https://img.shields.io/badge/Made%20with-Go-1f425f.svg)](https://go.dev/)
[![GoReportCard example](https://goreportcard.com/badge/github.com/eoinhurrell/gobandit)](https://goreportcard.com/report/github.com/eoinhurrell/gobandit)
[![GPLv3 license](https://img.shields.io/badge/License-GPLv3-blue.svg)](http://perso.crans.org/besson/LICENSE.html)

GoBandit is a pet project, an experimentation engine that implements Thompson Sampling for intelligent A/B testing at scale. Built with Go, with a frontend written using HTMX, Templ and Tailwind, and PostgreSQL storage. It provides a clean, performant API for managing multiple concurrent experiments with sophisticated exploration/exploitation strategies.

## Local Development

Start PostgreSQL:

```bash
docker compose up -d postgres
```

Start server:

```bash
go run main.go
```

Go to `http://localhost:8080`

## API

```http
POST /tests
Create a new A/B test with multiple arms

GET /tests/{testID}/arm
Get next arm using Thompson Sampling

POST /tests/{testID}/arms/{armID}/result
Record result for an arm pull

GET /tests/{testID}
Get test statistics and configuration
```

## Performance Characteristics

- **Time Complexity**: O(n) for arm selection where n is number of arms (typically small)
- **Space Complexity**: O(1) per request
- **Database**:
  - Single query for arm selection
  - Single-row updates for result recording
  - Proper indexing for O(1) lookups

## Usage Example

```curl
curl -X POST \
  http://localhost:8080/tests \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Random Test Name",
    "description": "This is a randomly generated test",
    "arms": [
      {
        "name": "Arm A",
        "description": "This is the first arm"
      },
      {
        "name": "Arm B",
        "description": "This is the second arm"
      }
    ]
  }'
```

## Progress

- TODO: Architecture (maybe too simple to diagram?)
- TODO: Blogpost
- TODO: documentation
- TODO: Performance benchmarks
- TODO: Deployment guides
- TODO: Example use cases

## Future Enhancements

- Redis caching layer for high-traffic scenarios
- Additional bandit algorithms (UCB1, ε-greedy)
- Real-time analytics dashboard
- Prometheus metrics
