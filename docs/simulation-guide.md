# Simulation Guide: Large-Scale Local Experiments with GoBandit

This guide shows how to drive the GoBandit engine from a local script to run
large-scale simulations — i.e. replaying the bandit loop millions of times
against a hidden "ground truth" to measure how fast it converges, how much
regret it accrues, and how its allocation share moves toward the best arm.

It covers two ways to simulate, with a clear recommendation:

| Path | What it exercises | Speed | Use when |
|------|-------------------|-------|----------|
| **In-process (Go library)** | The Thompson Sampling algorithm (`internal/service`) | 10⁶–10⁹ pulls in seconds | You want to study the algorithm itself across many configs |
| **HTTP API (any language)** | The real deployed server + Postgres | ~10²–10⁴ pulls/sec | You want to validate the live system end-to-end (the real `SELECT`/`UPDATE`, the JSON contract, persistence) |

Start with the **in-process** path for throughput studies; use the **HTTP** path
when you need to confirm the running server behaves the way the algorithm does.

---

## 1. How the engine actually works

Read this before simulating — several details below are load-bearing and not
obvious from the README.

### What runs today

Three binaries exist; all of them wire up the **Test/Arm** model:

- `assignment-api` (`cmd/assignment-api`, port **8080`) — the assignment loop.
- `control-plane` (`cmd/control-plane`, port **8081`) — the HTMX web UI.
- root `gobandit` (`main.go`, port **8080**) — both of the above in one process.

> **Note:** The codebase also contains `Experiment`/`Variant` domain types,
> repository implementations, and migrations (see `docs/prd.md`,
> `internal/repository/postgres/experiment.go`). These are **scaffolded but not
> wired into any `main`**. A simulator must target the **Test/Arm** API, which is
> the only model that actually runs.

### The API contract a simulator drives

| Method & path | Body | Returns | Notes |
|---------------|------|---------|-------|
| `POST /tests` | `application/x-www-form-urlencoded`: `name`, `description`, `numArms` | **HTML** (`TestCard`) | Server mints the test ID and all arm IDs. There is **no JSON envelope** — see §3 for extracting IDs. Requires `numArms >= 2`. |
| `GET /tests/{testID}/arm` | — | JSON: the chosen `Arm` (with `id`, `name`, `successes`, `failures`, …) | This both **selects** (Thompson) and **returns** the arm. Each call is one pull. |
| `POST /tests/{testID}/arms/{armID}/result` | JSON `{"success": bool}` | JSON `{"successes": N, "failures": M}` | Records the outcome. Both `testID` and `armID` are in the URL. |
| `GET /tests/{testID}/arms` | — | HTML stats | |

> The README lists `GET /tests/{testID}` (single test as JSON). That route is
> **not registered** in the live handlers — don't rely on it. Use
> `GET /tests/{testID}/arms` or query Postgres directly (§4).

### The algorithm (the part you'll touch most)

Thompson Sampling lives in `internal/service/thompson.go`. For each arm it keeps
a Beta(α, β) posterior where α = `successes + 1`, β = `failures + 1`. To choose,
it draws one sample per arm and picks the maximum:

```
sample_i ~ Beta(successes_i + 1, failures_i + 1)
choose    = argmax_i sample_i
```

The Beta draws are produced via Gamma variates (Marsaglia–Tsang for α ≥ 1,
Ahrens–Dieter boost for α < 1). `SelectArm` is **pure and in-memory** — it takes
a `[]experiment.Arm` and returns a `*experiment.Arm` pointing into that slice.

Two gotchas that matter at scale:

- **Single global RNG.** `thompson.go` keeps one `*rand.Rand` in a package
  variable, seeded from the wall clock in `init()`, guarded by a mutex. Every
  `SelectArm` call serializes on that mutex. Fine for correctness; it caps
  parallelism if you fan out concurrent in-process sims.
- **Not reproducible by default.** The seed is wall-clock time and the RNG is
  unexported, so you cannot reseed it from outside the package. For reproducible
  runs, either accept the wall-clock seed, or use the self-contained sampler in
  §5 (which lets you own the seed).

### What "simulating" means here

The engine stores only observed `successes`/`failures`. It has no notion of a
"true" conversion rate. A simulation therefore holds a **hidden ground truth**
outside the engine — a per-arm probability `p_i` — and uses it to generate the
Bernoulli outcome for whichever arm the bandit picks:

```
repeat:
    chosen      = engine.select_arm()          # GET /tests/{id}/arm  OR  SelectArm(arms)
    outcome     = random() < p[chosen.id]      # your hidden truth, NOT in the engine
    engine.record_result(chosen, outcome)      # POST .../result       OR  mutate counts
    measure(chosen, outcome)
```

This is standard offline bandit evaluation. The engine learns `p_i` only through
the outcomes you feed it; your script is the environment.

---

## 2. Stand the system up

### Database + server

```bash
just db-up                 # Postgres on localhost:5432 (init.sql auto-applied)
just migrate               # apply migrations/*.up.sql (best-effort, idempotent)

# Pick one binary:
just run-assignment-api    # assignment API on :8080  (all a simulator needs)
# or
just run                   # root binary: UI + assignment on :8080
```

Sanity-check the assignment endpoint (it will 500 until a test exists, which is
expected):

```bash
curl -s localhost:8080/tests/00000000-0000-0000-0000-000000000000/arm
```

### Connection string (hardcoded today)

All three binaries hardcode
`host=localhost user=postgres password=postgres dbname=postgres sslmode=disable`
and never tune the pool (`SetMaxOpenConns`, `SetMaxIdleConns`, etc.). For
high-concurrency HTTP simulations, the default pool is the first ceiling you'll
hit — see §6.

---

## 3. HTTP simulation (validate the live server)

Use this when you want the real deployed behaviour: the actual `SELECT … FROM
arms WHERE test_id`, the atomic `UPDATE … RETURNING`, persistence across
restarts, and the JSON contract. Any language works; the example below is Python
because "local script" usually means Python.

### 3.1 Create a test and recover the generated IDs

`POST /tests` returns HTML, so the first job is extracting the server-generated
test ID and arm IDs. The `TestCard` template renders routes containing both
(`/tests/{testID}/arms` and `option value="{armID}"`), and IDs are UUIDs — easy
to pull out with a regex:

```python
# prep.py — create a test, return (test_id, [arm_id, ...]) in creation order
import re, requests

UUID = re.compile(r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}")
BASE = "http://localhost:8080"

def create_test(name, description, num_arms):
    r = requests.post(f"{BASE}/tests", data={
        "name": name, "description": description, "numArms": num_arms,
    })
    r.raise_for_status()
    html = r.text
    test_id = UUID.search(html).group(0)                 # first UUID is the test id
    arm_ids = UUID.findall(html)[1:]                     # the rest are arm ids
    # dedupe while preserving creation order
    seen = set(); ordered = []
    for a in arm_ids:
        if a not in seen:
            seen.add(a); ordered.append(a)
    return test_id, ordered
```

> Creation order is preserved (the template iterates `test.Arms` in the order the
> service builds them: Arm 1, Arm 2, …). Map your hidden rates positionally:
> `true_rate = {arm_ids[i]: p[i] for i in range(num_arms)}`.

### 3.2 The simulation loop (concurrent)

One pull = two HTTP round trips. Fan out with a thread pool to push throughput;
the server and Postgres pool are the limiting factors, not this script.

```python
# sim_http.py
import random
import requests
from concurrent.futures import ThreadPoolExecutor
from prep import create_test, BASE

# Hidden ground truth — the bandit must NOT see this. Arm order == creation order.
TRUE_RATE = {"a": 0.10, "b": 0.13, "c": 0.11}   # "b" is the true best arm
N = 50_000                                         # total pulls

test_id, arm_ids = create_test("sim", "regret study", len(TRUE_RATE))
rate_by_arm = {arm_ids[i]: list(TRUE_RATE.values())[i] for i in range(len(arm_ids))}
best_rate = max(rate_by_arm.values())

def one_pull(_):
    chosen = requests.get(f"{BASE}/tests/{test_id}/arm").json()       # Thompson pick
    aid = chosen["id"]
    success = random.random() < rate_by_arm[aid]                      # Bernoulli(p_i)
    requests.post(f"{BASE}/tests/{test_id}/arms/{aid}/result",
                  json={"success": success})
    return aid, success

# Pull, accumulate raw events, compute metrics afterwards (see §4)
events = []
with ThreadPoolExecutor(max_workers=32) as ex:
    for aid, success in ex.map(one_pull, range(N)):
        events.append((aid, success))
```

> **Threading caveat:** HTTP pulls are independent and safe to parallelize — the
> server reads/writes per arm atomically. But because order is non-deterministic
> under concurrency, your *regret timeline* is only statistically meaningful
> (averaged over pulls). For an ordered, step-indexed regret curve, run the loop
> single-threaded or assign step indices by completion order and accept the jitter.

### 3.3 Read the learned state back

Cleanest is straight from Postgres:

```bash
just db-shell
#=> SELECT name, successes, failures,
#=>        round(successes::numeric / nullif(successes+failures,0), 4) AS observed
#=> FROM arms WHERE test_id = '<paste test_id>' ORDER BY name;
```

You should see the observed rate of the best arm climb toward its true `p_i`,
and its share of total pulls grow over time.

---

## 4. In-process simulation (study the algorithm, fast)

This is the recommended default for large-scale studies. Import the engine
packages directly, hold the arm counts in memory, and skip HTTP and Postgres
entirely. `SelectArm` is microseconds; 10⁶ pulls across many configs take
seconds.

Create a sibling program (e.g. `examples/sim/main.go`) so you don't litter the
repo root:

```go
package main

import (
	"fmt"
	"math/rand"

	"gobandit/internal/domain/experiment"
	"gobandit/internal/service"
)

func main() {
	// Hidden ground truth — the bandit never sees this.
	trueRate := map[string]float64{"a0": 0.10, "a1": 0.13, "a2": 0.11}
	best := 0.13

	ts := service.NewThompsonSampling()
	arms := []experiment.Arm{{ID: "a0"}, {ID: "a1"}, {ID: "a2"}}

	rng := rand.New(rand.NewSource(7)) // for the ENVIRONMENT only (outcome draws)

	const N = 100_000
	cumReward, cumRegret := 0.0, 0.0
	pulls := map[string]int{}

	for step := 0; step < N; step++ {
		chosen := ts.SelectArm(arms) // pointer into the slice; mutating it updates counts
		pulls[chosen.ID]++

		success := rng.Float64() < trueRate[chosen.ID]
		if success {
			chosen.Successes++
		} else {
			chosen.Failures++
		}

		if success {
			cumReward += 1
		}
		cumRegret += best - trueRate[chosen.ID]
	}

	fmt.Printf("cumulative reward = %.0f / %d\n", cumReward, N)
	fmt.Printf("cumulative regret  = %.2f  (Thompson: should grow ~ O(log N))\n", cumRegret)
	for _, a := range arms {
		fmt.Printf("  arm %s: pulls=%d  observed=%.3f  true=%.2f\n",
			a.ID, pulls[a.ID], a.SuccessRate(), trueRate[a.ID])
	}
}
```

Run it from the repo root so the module resolves:

```bash
mkdir -p examples/sim && cp <the file above> examples/sim/main.go
go run ./examples/sim
```

You should observe: cumulative regret growing sub-linearly (~logarithmic), and
the best arm (`a1`) dominating the pull share while weaker arms still get
exploratory traffic.

### Variant: many configurations in parallel

To sweep configurations (number of arms, gap sizes, horizons), launch one
goroutine per config. Remember the engine's RNG mutex serializes `SelectArm`, so
parallelism helps mainly by overlapping the *environment* work (outcome draws,
bookkeeping) and by letting many configs progress between lock acquisitions:

```go
results := make(chan report, len(configs))
for _, cfg := range configs {
    go func(c config) { results <- simulate(c) }(cfg)
}
```

If the mutex becomes your bottleneck (it will, once environment work is trivial),
either (a) accept it, or (b) drop down to the reproducible sampler in §5, which
removes the shared RNG entirely.

---

## 5. Reproducible runs (own the RNG)

The engine's RNG is wall-clock seeded and unexported, so runs aren't
reproducible as-is. For reproducible experiments, run Thompson Sampling against
your own seeded RNG. The selection rule is small enough to inline — this is a
faithful copy of `service.SelectArm` with a controllable source:

```go
// betaVarate: sample from Beta(alpha, beta) via two Gamma variates.
func gamma(rng *rand.Rand, alpha float64) float64 {
	if alpha <= 0 {
		return 0
	}
	if alpha < 1 {
		return gamma(rng, 1+alpha) * math.Pow(rng.Float64(), 1/alpha)
	}
	d := alpha - 1.0/3.0
	c := 1.0 / math.Sqrt(9.0*d)
	for {
		var x, v float64
		for {
			x = rng.NormFloat64()
			v = 1 + c*x
			if v > 0 {
				break
			}
		}
		v = v * v * v
		u := rng.Float64()
		if u < 1-0.331*x*x*x*x {
			return d * v
		}
		if math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}
func beta(rng *rand.Rand, a, b float64) float64 {
	x, y := gamma(rng, a), gamma(rng, b)
	if x+y == 0 {
		return 0.5
	}
	return x / (x + y)
}

func selectArm(rng *rand.Rand, arms []experiment.Arm) *experiment.Arm {
	var best float64
	var chosen *experiment.Arm
	for i := range arms {
		s := beta(rng, float64(arms[i].Successes+1), float64(arms[i].Failures+1))
		if s > best || chosen == nil {
			best, chosen = s, &arms[i]
		}
	}
	return chosen
}
```

Now `rng := rand.New(rand.NewSource(seed))` gives bit-for-bit reproducible sims.
This duplicates ~10 lines of engine code; trade-off is full seed control and no
shared mutex, so it scales trivially across goroutines.

> Prefer importing `service.SelectArm` when you specifically want to validate
> the engine's behaviour (including its RNG path). Use the sampler above when you
> want reproducible bandit *experiments* and don't need to test the engine's own
> RNG.

---

## 6. Metrics & how to read them

Track these per step or in windows of, say, 1k pulls:

| Metric | Definition | What to expect (Thompson) |
|--------|------------|---------------------------|
| **Cumulative regret** | Σₜ (p_best − p_chosen_at_t) | Grows ~O(log N); a straight-ish line on log-x means healthy |
| **Instantaneous regret** | p_best − p_chosen (smoothed) | Falls toward 0 as the best arm is identified |
| **% pulls to optimal arm** | pulls(best)/t | Rises toward a high plateau (not necessarily 1.0 — Thompson keeps exploring) |
| **Cumulative reward** | Σₜ outcomeₜ | Compare against random allocation and against always-best (oracle) |
| **Best-arm identification** | does argmax(observed rate) == true best? | Should stabilise to "yes" after enough pulls |
| **Sample efficiency** | pulls needed to reach a regret threshold | Compare Thompson vs uniform/random as a baseline |

Always report **baselines**: at minimum, compare Thompson against (a) uniform
random allocation and (b) the oracle that always pulls the true-best arm. A
simulation without a baseline tells you little. Average over many seeds and
report variance bands, especially for small horizons where stochastic effects
dominate.

---

## 7. Scale notes and gotchas

- **HTTP ceiling is the server, not your script.** Each pull is 1 `SELECT all
  arms` + 1 `UPDATE`. Postgres' pool is untuned (`database/sql` defaults). If
  you saturate one server instance, raise concurrency only up to the point where
  latency stops scaling — then shard across multiple tests or drop to in-process.
- **Assignment is stateless per request.** There's no assignment token, no
  sticky user mapping, no idempotency key. The `result` POST simply increments
  the named arm. This is what makes the sim loop trivial, but it also means the
  HTTP path has no notion of "which user got which arm" — fine for an offline
  regret study, not for consistency-sensitive workloads.
- **The RNG mutex caps in-process parallelism.** Every `SelectArm` takes the
  package lock. For maximum throughput across goroutines, use the reproducible
  sampler in §5 (per-goroutine RNG, no shared lock).
- **Connection string is hardcoded** to `localhost:5432`, `postgres/postgres`.
  Don't expect env overrides; edit the `main` you're running if you need to move
  Postgres.
- **`Experiment`/`Variant` is not wired.** Despite the domain types and
  migrations, no binary serves it. Don't build a sim against it until a `main`
  registers those routes.
- **`GET /tests/{testID}` (JSON) is documented but not registered.** Use
  `/tests/{testID}/arms` or query Postgres directly.
- **Clean up between runs.** `just db-reset` drops the volume and re-applies
  `init.sql`; useful to start a simulation campaign from a known-empty state.

---

## 8. Quick reference

```bash
# bring the system up
just db-up && just migrate && just run-assignment-api

# create a test (IDs come back in the HTML — parse them, §3.1)
curl -X POST localhost:8080/tests \
  -d 'name=sim&description=regret&numArms=3'

# one bandit pull → JSON arm
curl -s localhost:8080/tests/<testID>/arm

# record an outcome
curl -X POST localhost:8080/tests/<testID>/arms/<armID>/result \
  -H 'Content-Type: application/json' -d '{"success":true}'

# inspect learned state
just db-shell   # then SELECT name, successes, failures FROM arms WHERE test_id='<testID>';

# fast algorithm study, no server needed
go run ./examples/sim

# reset everything
just db-reset
```
