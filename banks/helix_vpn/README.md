# HelixVPN HelixQA Test Bank

**Project:** HelixVPN  
**Module:** `digital.vasic.helix_qa`  
**Purpose:** Autonomous-session test bank for the HelixVPN MVP. These test cases drive real CLI/UI flows and score PASS only on captured evidence. They are the paired counterpart to `submodules/challenges/helix_vpn/`.

---

## Scope

This bank contains example test cases for each MVP critical path:

1. **Auth + tunnel establishment** — end-to-end enroll and connect via `helixvpnctl`.
2. **Reconnect / roaming** — drop and recover without manual intervention.
3. **Kill-switch** — assert no plaintext leak when the tunnel drops.
4. **DNS leak prevention** — assert all DNS stays inside the tunnel.
5. **Control-plane HA** — assert fail-static behavior when the control plane is down.
6. **Client UI visual proof** — assert the Access app connect flow renders correctly.

Additional rows cover GAP-6 (DDoS ownership) and the 8 MVP DoD acceptance criteria.

---

## File layout

```
submodules/helix_qa/banks/helix_vpn/
├── README.md
├── helix_vpn_bank.yaml   # Bank metadata + test cases
└── helix_vpn_bank.json   # JSON mirror of the YAML bank
```

---

## Test-case IDs

| ID | Critical path | Category | Evidence | Bound requirement |
|---|---|---|---|---|
| `HVPN-HQA-AUTH-TUNNEL` | Auth + tunnel establishment | e2e | pcap + iperf3 | FR-104, FR-111, FR-701 |
| `HVPN-HQA-RECONNECT-ROAMING` | Reconnect / roaming | chaos | pcap + status log | FR-015, FR-707 |
| `HVPN-HQA-KILL-SWITCH` | Kill-switch no leak | security | gap pcap | FR-502, NFR-404 |
| `HVPN-HQA-DNS-LEAK` | DNS leak prevention | security | DNS pcap | FR-503, NFR-405 |
| `HVPN-HQA-CONTROL-PLANE-HA` | Control-plane fail-static | chaos | HA pcap + log | NFR-200 |
| `HVPN-HQA-CLIENT-UI-VISUAL` | Client UI visual proof | ui | MP4 + vision verdict | FR-1003, FR-1014 |
| `HVPN-HQA-NFR413-API-Rate-Limit` | GAP-6: control-plane rate limiting | ddos | latency + counter CSV | NFR-413 |
| `HVPN-HQA-NFR414-Edge-Flood` | GAP-6: data-plane flood resilience | ddos | liveness + legit latency CSV | NFR-414 |

---

## Running the bank

```bash
# From the HelixVPN project root
make qa-helix_vpn
# Or directly via the HelixQA CLI
go run submodules/helix_qa/cmd/helixqa/main.go \
  --bank submodules/helix_qa/banks/helix_vpn/helix_vpn_bank.yaml
```

---

## References

- `docs/research/mvp/final/10-testing-acceptance-and-qa.md`
- `docs/research/mvp/final/implementation/09-testing-qa/coverage-ledger.md`
- `submodules/challenges/helix_vpn/`
