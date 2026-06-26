# mactop-desktop Roadmap

Product direction and migration phases are defined in [goals.md](goals.md). This file tracks implementation status.

## Phase 1: Simple view — **done**

- [x] Headless JSON bridge (`mactop --headless` polled by Electron)
- [x] `thermal_level` field on `HeadlessOutput` for locale-safe UI styling
- [x] Glanceable Simple view: CPU, GPU, memory, power, temperature, thermal
- [x] 3×2 grid layout, usage gauges, thermal severity pills
- [x] Summary strip (chip name, core count, last updated)
- [x] Advanced / Complex tabs disabled with “Coming soon” placeholders

## Phase 2: Advanced view — **not started**

- [ ] Enable Advanced tab
- [ ] SoC panel: per-cluster power, GPU frequency, cluster activity
- [ ] Per-core usage (E, P, S on M5+)
- [ ] Memory: used, available, swap
- [ ] DRAM bandwidth (read/write GB/s)
- [ ] Network and disk throughput
- [ ] Top N processes (sortable, filterable)
- [ ] Fans: RPM, target, mode
- [ ] Battery level and charging state (MacBooks)
- [ ] Charts / history for CPU, GPU, memory, power

## Phase 3: Complex view — **not started**

- [ ] Enable Complex tab
- [ ] Full `HeadlessOutput` explorer (structured + raw JSON)
- [ ] Thunderbolt device tree and per-interface throughput
- [ ] RDMA availability and status
- [ ] All mounted volumes with usage
- [ ] Full SMC temperature sensor groups
- [ ] Network link PHY details (Ethernet, Wi-Fi)
- [ ] Diagnostics hooks (`--dump-temps`, `--dump-debug`, `--dump-fps`)
- [ ] Settings: update interval, units, language

## Phase 4: Persistent sidecar API — **not started**

- [ ] Replace per-poll `spawn` with long-lived Go process
- [ ] Local HTTP or WebSocket API for metrics streaming
- [ ] Graceful reconnect and error recovery in Electron main process

## Phase 5: Extract TUI package — **not started**

- [ ] Move remaining gotui code under `internal/tui/` (optional isolation)
- [ ] Ensure `go test ./internal/app/...` passes with TUI isolated

## Phase 6: Delete TUI — **not started**

- [ ] Data parity: every TUI metric available in at least one desktop view
- [ ] Interaction parity: process list, settings persistence, update interval
- [ ] Remove `gotui`, `tcell`, layout/events/theme TUI code
- [ ] Default `app.Run()` path becomes desktop/sidecar mode
- [ ] README and docs describe desktop-first product

## Infrastructure (ongoing)

- [x] CI workflow (`.github/workflows/ci.yml`)
- [x] Dependabot (`.github/dependabot.yml`)
- [x] CodeRabbit config (`.coderabbit.yaml`)
- [ ] Desktop npm test / formatter tooling
- [ ] README: mark Simple view as live (still says “placeholders”)

## Explicit non-goals (desktop v1)

- Party mode
- 20 layout cycling via keyboard
- Vim-style terminal navigation
- Literal `theme.json` key migration from TUI
