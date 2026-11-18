

\# ocs-quota



Storage quota enforcement and monitoring for OCP (OpenShift Container Storage) namespaces.



This repository contains the source code, tests (Python/pytest, Java/JUnit 5, Go), and CI

configuration used to demonstrate end-to-end log triage with the

\[OCS Log Analyser](https://github.com/your-org/ocs-log-analyser).



\---



\## Repository layout





ocs-quota/

├── quota/                            # Python package — core quota logic

│   ├── init.py

│   ├── manager.py                    # QuotaManager: tracks per-namespace usage

│   └── k8s\_quota\_client.py          # Thin wrapper for the K8s ResourceQuota API

│

├── tests/                            # Python pytest suite

│   ├── test\_quota\_manager.py

│   └── test\_k8s\_quota\_client.py

│

├── src/

│   ├── main/java/com/ocs/quota/      # Java source

│   │   ├── model/QuotaLimit.java     # Immutable value object (namespace, limitGib, usedGib)

│   │   ├── QuotaManager.java         # Java port of the quota manager

│   │   ├── QuotaExceededException.java

│   │   └── K8sQuotaClient.java       # HTTP client for the K8s ResourceQuota REST API

│   │

│   └── test/java/com/ocs/quota/      # JUnit 5 test suite

│       ├── QuotaManagerTest.java

│       └── K8sQuotaClientTest.java

│

├── quota\_go/                         # Go package

│   ├── go.mod

│   ├── manager.go                    # Go port of the quota manager

│   ├── k8s\_client.go                 # Go HTTP client + ParseQuantityGiB

│   ├── manager\_test.go               # Go tests

│   └── k8s\_client\_test.go

│

├── pytest.ini                        # pytest config: testpaths=tests, pythonpath=.

├── requirements.txt                  # pytest>=8.0

├── pom.xml                           # Maven build: JUnit 5, Jackson, Surefire XML reports

└── .github/

└── workflows/

└── ci.yml                    # GitHub Actions: pytest + Maven + Go jobs





\---



\## Intentional bugs (for log-analyser demo)



Each language has the same two bugs so the analyser can triage failures across languages.



\### Bug 1 — `addUsage` overwrites instead of accumulating



Calling `addUsage` / `AddUsage` more than once for the same namespace replaces the

stored byte count instead of adding to it.



| File | Buggy line |

|---|---|

| `quota/manager.py` | `self.\_usage\[namespace] = bytes\_used` |

| `src/main/java/…/QuotaManager.java` | `usageBytes.put(namespace, bytes)` |

| `quota\_go/manager.go` | `m.usage\[namespace] = bytes` |



\*\*Fix:\*\* accumulate — `self.\_usage\[namespace] = self.\_usage.get(namespace, 0) + bytes\_used`

(Python), `usageBytes.merge(namespace, bytes, Long::sum)` (Java),

`m.usage\[namespace] += bytes` (Go).



\### Bug 2 — `utilisationPct` truncates small percentages to 0



Integer division is used where floating-point division is required. Any utilisation below

1 % returns `0` instead of the correct fractional value.



| File | Buggy line |

|---|---|

| `quota/manager.py` | `return total // (self.limit\_bytes / 100)` |

| `src/main/java/…/QuotaManager.java` | `return totalUsed / limitBytes \* 100` |

| `quota\_go/manager.go` | `return float64(total / m.limitBytes \* 100)` |



\*\*Fix:\*\* use floating-point division — `total \* 100 / self.limit\_bytes` (Python),

`(double) totalUsed / limitBytes \* 100` (Java),

`float64(total) / float64(m.limitBytes) \* 100` (Go).



\### Product bugs (K8s API — test expectations are correct)



| Test | Description |

|---|---|

| `test\_k8s\_quota\_status\_reflects\_pvc\_deletion` | After a PVC is deleted, the OCP 4.14 kube-controller-manager quota reconciler does not decrement `status.used` when the StorageClass uses a Retain reclaim policy. |

| `test\_k8s\_quota\_status\_missing\_storage\_key\_raises` | Some OCP versions return a ResourceQuota with `requests.storage` absent from `status.used` even when PVCs are active, silently under-reporting usage. |



Reference: `https://issues.redhat.com/browse/OCPBUGS-XXXXX`



\---



\## Running the tests locally



\### Python



```bash

python -m venv .venv

\# Windows

.\\.venv\\Scripts\\Activate.ps1

\# Linux / macOS

source .venv/bin/activate



pip install -r requirements.txt

pytest





Java (Maven)

\# Requires Java 17+ and Maven 3.8+

mvn test

\# JUnit XML reports land in: target/surefire-reports/



Go

cd quota\_go

go test ./... -v



GitHub Actions CI

The workflow at .github/workflows/ci.yml runs all three

test suites on every push and pull request to main.



Job	Runner	Test command	Artifact

pytest	ubuntu-latest	pytest tests/ -v	pytest-output.log

maven	ubuntu-latest	mvn --batch-mode test	maven-output.log, surefire-reports/\*.xml

go	ubuntu-latest	go test ./quota\_go/... -v	go-test-output.log

Artifacts are retained for 30 days and are available on the Actions → run → Artifacts

panel. All three jobs upload their log even when tests fail (continue-on-error: true on

the test step, explicit exit 1 at the end).



Triggers:



Push to main, feature/\*\*, fix/\*\*, log-analyser/\*\*

Pull requests targeting main

Manual dispatch from the Actions tab (workflow\_dispatch)

Connecting to the OCS Log Analyser

1\. Register the project

Call add\_project once (UI, MCP tool, or the Flask API):



name        = ocs-quota

path        = ocs-quota            ← sub-folder under PROJECTS\_ROOT

log\_format  = auto                 ← auto-detects pytest / junit / go

github\_repo = your-org/ocs-quota  ← issues and PRs go to this repo only



2\. Required .env entries in the log-analyser

\# Authenticates all GitHub API calls (PAT with repo scope)

GITHUB\_TOKEN=ghp\_...



\# Safety switch — set false only when you are ready to publish real issues/PRs

GITHUB\_DRY\_RUN=false



GITHUB\_REPO in .env is the global fallback for projects that have no

github\_repo field. Because ocs-quota is registered with its own github\_repo,

issues and PRs will go to your-org/ocs-quota and nowhere else.



3\. Feed a CI log into the analyser

Manual: Download the artifact zip from the Actions run, extract the .log or .xml,

and upload it via the UI Submit Log tab or the submit\_log MCP tool.



Automated (optional): Uncomment the notify-log-analyser job at the bottom of

ci.yml, add a LOG\_ANALYSER\_URL repository secret, and every failing run will POST

all log files to /api/submit automatically:



\- name: Post logs to log-analyser

&#x20; env:

&#x20;   LOG\_ANALYSER\_URL: ${{ secrets.LOG\_ANALYSER\_URL }}

&#x20; run: |

&#x20;   for f in ci-logs/\*\*/\*.log ci-logs/\*\*/\*.xml; do

&#x20;     \[ -f "$f" ] || continue

&#x20;     curl -sS -X POST "$LOG\_ANALYSER\_URL/api/submit" \\

&#x20;       -F "project=ocs-quota" \\

&#x20;       -F "log=@$f" \\

&#x20;       -F "name=$(basename $f)"

&#x20;   done



4\. End-to-end triage flow

submit\_log (pytest / maven / go log)

&#x20; └─► analyse\_all\_failures

&#x20;       └─► provide\_info          (if needs\_info = true)

&#x20;             └─► generate\_patch  (if can\_patch = true)

&#x20;                   └─► submit\_rerun

&#x20;                         ├─ passed ──► draft\_pull\_request ──► create\_pull\_request

&#x20;                         └─ failed ──► draft\_issue         ──► create\_issue



create\_issue and create\_pull\_request both require approved=true. An agent must

never pass approved=true without first showing the draft to a human.



Sample log files

Pre-generated log files are available in the log-analyser samples/ directory for

demo and testing without running the full CI:



File	Format	Failures

samples/pytest\_ocs\_quota.log	pytest	2 (accumulate + int division)

samples/pytest\_k8s\_quota\_product\_bug.log	pytest	2 (product bugs)

samples/java\_ocs\_quota\_surefire.xml	JUnit XML (Surefire)	2

samples/go\_ocs\_quota\_test.log	Go go test -v	4

License

Apache 2.0 — see LICENSE.





\---



\*\*Key sections at a glance:\*\*



\- \*\*Layout\*\* — every file explained with its role

\- \*\*Intentional bugs\*\* — both bugs documented with the exact buggy line and the fix, in all three languages

\- \*\*Product bugs\*\* — K8s API regression documented with the OCPBUGS reference

\- \*\*Running locally\*\* — Python, Java, Go commands

\- \*\*GitHub Actions\*\* — job table, artifact names, retention, triggers

\- \*\*Log Analyser wiring\*\* — `add\_project` call, `.env` entries, manual vs automated log feeding, end-to-end flow diagram, approval safety rule

\- \*\*Sample logs\*\* — where to find the pre-built `.log` / `.xml` files for demos

