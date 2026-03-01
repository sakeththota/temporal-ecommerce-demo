# Demo Scenarios

This document describes the Temporal features demonstrated in this project and how to showcase them.

---

## Use Case 1: Embedding Migration (Standard)

**Features Demonstrated:**
- Retry policies
- Signals (pause/resume)
- Queries (progress polling)
- Continue-as-new (history management)
- Crash recovery

### Demo Scenario 1A: Pause and Resume Migration

1. Go to Migration Dashboard
2. Start a migration with the default (non-approval) workflow
3. Watch the progress bar fill up
4. Click **Pause** to pause mid-migration
5. Click **Resume** to continue
6. Show how Temporal UI shows the paused state

### Demo Scenario 1B: Crash Recovery

1. Start a migration with batch size set to something small (e.g., 2-3)
2. While it's running, click **Crash Server** in the backend
3. Show the server restarting (if running locally with Docker)
4. Observe that the migration automatically resumes from where it left off
5. Demonstrate that no data was lost

### Demo Scenario 1C: View Activity History

1. Start a migration
2. Open Temporal Web UI (http://localhost:8233)
3. Find the migration workflow
4. Show the **History** tab with all activities
5. Point out:
   - Retries on failed activities
   - Activity start/end times
   - Retry policy in effect

---

## Use Case 2: Booking Checkout with Saga

**Features Demonstrated:**
- Full compensation/rollback on failure (saga pattern)
- Multiple activities that must all succeed or all rollback
- Compensation on payment failure
- Compensation on reservation failure
- Status queries showing compensation status

### Demo Scenario 2A: Payment Failure Compensation

1. Go to a hotel detail page and start a booking
2. The booking workflow runs with a 20% simulated payment failure rate
3. If payment fails, show:
   - The workflow status shows "failed"
   - Compensation runs automatically (refund)
   - Status query shows `compensation_run: true` and `compensation_step: "payment_refunded"`

### Demo Scenario 2B: Reservation Failure Compensation

1. Run multiple bookings until you hit a failure
2. Show the compensation: payment is refunded automatically

---

## Use Case 3: Approval Workflow (Human-in-the-Loop)

**Features Demonstrated:**
- Workflow signals (approve/reject/update)
- Timer-based auto-cancellation
- Human approval gate
- Rollback on rejection

### Demo Scenario 3A: Approval Workflow End-to-End

1. Go to Migration Dashboard
2. Check "Use approval workflow" checkbox
3. Start a migration
4. Watch progress go to 100%
5. Workflow pauses at "Awaiting Approval" status
6. Click **Approve** to switch the version
7. Show that the new version is now active

### Demo Scenario 3B: Rejection and Rollback

1. Start an approval workflow migration
2. Wait for it to complete generation
3. Click **Reject** instead of Approve
4. Show that:
   - Embeddings are deleted
   - Progress resets to 0
   - Version is not activated

### Demo Scenario 3C: Approval Timeout

1. Start an approval workflow migration
2. Wait for completion but don't approve
3. Wait 60 minutes (or mock the timeout)
4. Show automatic rollback occurs

---

## Key Temporal Concepts to Highlight

### 1. Idempotency
- Activities are designed to be idempotent
- Show how retrying doesn't cause duplicate work

### 2. Observability
- Temporal Web UI shows full workflow history
- Query handlers expose internal state
- Stack traces available for failures

### 3. Durability
- Workflow state persisted in Temporal
- Server crashes don't lose progress
- Exactly-once semantics for activities

### 4. Composability
- Workflows compose activities
- Signals allow external interaction
- Queries allow inspection without modification

---

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/search | Semantic search using active embeddings |
| GET | /api/hotels | List all hotels |
| GET | /api/versions | List embedding versions |
| POST | /api/migrations | Start migration |
| GET | /api/migrations/:version | Get progress |
| POST | /api/migrations/:version/pause | Pause (standard) |
| POST | /api/migrations/:version/resume | Resume (standard) |
| POST | /api/migrations/:version/approve | Approve (approval workflow) |
| POST | /api/migrations/:version/reject | Reject (approval workflow) |
| POST | /api/migrations/reset | Reset to v1 |
| POST | /api/bookings | Create booking |
| GET | /api/bookings/:id | Get booking status |
| POST | /api/bookings/:id/cancel | Cancel booking |
| POST | /api/crash | Crash server for demo |

---

## Running the Demo

```bash
# Start all services
docker compose up --build

# Access points
Frontend:    http://localhost:3000
API:         http://localhost:8080
Temporal UI: http://localhost:8233
PostgreSQL:  localhost:5432
Ollama:      localhost:11434
```
