# BlackShield Security Audit Report — `cloudbase/garm`

- **Scanned Commit:** `1fdee999d5f863331df72d465b86afe5f40d0d14`
- **Actionable Findings:** `5` (`0 Critical`, `5 High`, `17 Medium`, `0 Low`)
- **BlackShield Platform & Continuous Zero-Cost PR Protection:** [https://blackshield.chaplau.com](https://blackshield.chaplau.com)

## Detailed Findings & Remediation Guide

### 1. [HIGH] `CKV_DOCKER_1` — CKV_DOCKER_1: Container Runs as Root (Missing Non-Root USER)

- **Category:** `misconfiguration`
- **Location:** `Dockerfile:1`
- **Details:** Container does not specify a non-root USER, allowing processes to execute with root privileges.
- **Recommended Fix:** Add 'USER <non-root-uid>' instruction before container entrypoint.

### 2. [HIGH] `CKV_DOCKER_1` — CKV_DOCKER_1: Container Runs as Root (Missing Non-Root USER)

- **Category:** `misconfiguration`
- **Location:** `Dockerfile.build-static:1`
- **Details:** Container does not specify a non-root USER, allowing processes to execute with root privileges.
- **Recommended Fix:** Add 'USER <non-root-uid>' instruction before container entrypoint.

### 3. [HIGH] `CKV_DOCKER_1` — CKV_DOCKER_1: Missing Non-Root USER Instruction

- **Category:** `misconfiguration`
- **Location:** `Dockerfile.build-static:14`
- **Details:** Container executes processes as root user by default because no non-root USER instruction was specified.
- **Recommended Fix:** Add 'USER 10001' or a dedicated non-root application user.

### 4. [HIGH] `go-sqli-db-query-sprintf` — go-sqli-db-query-sprintf: SQL Injection via fmt.Sprintf in db.Query

- **Category:** `sast`
- **Location:** `database/sql/migrations/0008_constraint_parity.go:146`
- **Details:** Go application security vulnerability or insecure coding pattern. at database/sql/migrations/0008_constraint_parity.go:146
- **Recommended Fix:** Use parameterized database/sql or GORM placeholder queries (?, $1) instead of fmt.Sprintf or string concatenation.

### 5. [HIGH] `go-sqli-db-query-sprintf` — go-sqli-db-query-sprintf: SQL Injection via fmt.Sprintf in db.Query

- **Category:** `sast`
- **Location:** `database/sql/migrations/0008_constraint_parity.go:151`
- **Details:** Go application security vulnerability or insecure coding pattern. at database/sql/migrations/0008_constraint_parity.go:151
- **Recommended Fix:** Use parameterized database/sql or GORM placeholder queries (?, $1) instead of fmt.Sprintf or string concatenation.

