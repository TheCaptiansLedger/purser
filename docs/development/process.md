# Development Process

## Working on a github issue (starting a new feature, fix, test, docs, etc)

1. **Identify the issue** — every feature or fix must have a GitHub issue.

2. **Assign the issue** to yourself:
   ```bash
   gh issue edit <number> --assignee TheCaptiansLedger
   ```
2a. assign the status:in-progress label and comment on the issue "Starting to work on this"

3. **Create a topic branch** from `develop`:
   ```bash
   git checkout develop && git pull
   git checkout -b feature/<number>-short-description
   ```
   Use `fix/` prefix for bug fixes, `chore/` for maintenance.

4. **Link the branch to the issue** — GitHub auto-links when the branch name contains the issue number, or include `Closes #<number>` in the PR body.

5. **Do the development** following the relevant standards docs.

6. **Commit** following `docs/version-control/workflow.md`.

7. **Create the PR**:
   ```bash
   gh pr create --title "type(scope): description" --body "$(cat <<'EOF'
   ## Summary
   - what changed and why

   ## Test plan
   - [ ] tests added/updated
   - [ ] golangci-lint passes
   - [ ] go test ./... passes

   Closes #<number>
   EOF
   )"
   ```

## Pre-Write Code Review

Always present the code or diff in the chat and wait for explicit user confirmation/approval before writing, editing, or creating files.

## Build and Run

```bash
# Backend only
go run ./cmd/purser

# Full build (UI + embed)
cd web && npm run build && cd ..
go build -o purser ./cmd/purser

# Tests
go test ./...

# Regenerate sqlc types after SQL changes
sqlc generate

# Run migrations
go run ./cmd/purser migrate
```
