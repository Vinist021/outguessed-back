# AGENTS.md
## Purpose
Backend for a multiplayer question/card game using NestJS, TypeScript, Drizzle ORM, relational persistence, WebSockets, and a modular-monolith architecture inspired by Clean/Hexagonal Architecture.
Existing code/configuration is the source of truth. Verify paths, scripts, dependencies, DB engine, auth, validation, and infrastructure before assuming details.

## Instruction Priority
1. Current task instructions.
2. Closest applicable `AGENTS.md`.
3. Project docs/contracts.
4. Existing code patterns.
5. General NestJS/TypeScript/Drizzle conventions.
Do not reproduce a legacy pattern that violates an explicit rule here.

## Before Changing Code
- Inspect the owning module and a similar implementation.
- Check affected schemas, DTOs, repositories, use cases/services, controllers, and gateways.
- Verify `package.json` scripts/dependencies.
- Reuse existing abstractions before creating new ones.
- Make the smallest correct change; avoid unrelated refactors.
- Preserve public contracts unless the task explicitly changes them.

## Architecture
Preferred deployment: **modular monolith**, organized by business capability.
Dependency direction: `presentation -> application -> domain`; `infrastructure -> application/domain contracts`.

### Domain
Owns business rules, invariants, entities, value objects, domain services/events, and useful repository/port contracts.
**MUST NOT** import NestJS, Drizzle/database schemas, controllers/gateways, HTTP/WebSocket concerns, Redis, queues, brokers, or external SDKs.
Keep domain logic testable without infrastructure. Put game invariants in domain/application logic, not transport code. Do not force rich domain modeling onto simple CRUD where it adds no value.

### Application
Coordinates explicit actions such as `CreateMatch`, `JoinMatch`, `SubmitAnswer`, and `FinishRound`.
- Prefer focused use cases/services over large generic services.
- Depend on infrastructure through ports/interfaces when isolation is meaningful.
- Follow existing naming conventions before introducing `UseCase` suffixes.

### Infrastructure
Contains Drizzle persistence, external-provider adapters, Redis/queues, AI adapters, and other technical implementations.
**MUST NOT** own game rules.

### Presentation
Contains HTTP controllers, WebSocket gateways, transport DTOs, and transport mapping.
Controllers/gateways should only receive input, authenticate/authorize, validate boundary data, invoke application behavior, and map results/errors.
**MUST NOT** contain scoring, answer validation, board movement, match transitions, or direct persistence logic except an established simple-query pattern already used by the project.

## Module Boundaries
- Organize by business capability using the repository's actual structure.
- Inspect existing paths before creating folders; do not create a parallel structure just to match examples.
- Modules should expose the smallest API needed by other modules.
- **MUST NOT** deep-import another module's internal infrastructure.
- Prefer exported application APIs, explicit ports, or existing events for cross-module collaboration.
- Avoid circular dependencies.

## Core Game Rules
- **Match state is server-authoritative.**
- Clients send intent, not trusted derived results.
- Never trust client-provided scores, positions, correctness, turn results, or identity when server context is authoritative.
- Backend calculates action legality, answer correctness, score changes, board movement, turn/round progression, and match completion.
- A player may be authenticated or a guest; do not assume a persistent account.
- Keep complex gameplay rules isolated from transport/persistence and deterministic where practical.

### Match State
Use the states actually implemented by the repository.
- Every state-changing action **MUST** validate legality against current server state.
- Centralize transitions; do not duplicate them across controllers, gateways, consumers, or repositories.
- Never rely on frontend restrictions to enforce transitions.

### Answer Validation
When multiple modes exist (e.g. exact, player judge, AI), keep them behind one explicit strategy/port.
- Do not scatter repeated mode conditionals across layers.
- Adding a strategy should minimally affect existing strategies.
- Treat AI output as untrusted; validate/normalize it before mutating persisted state.

## Realtime
WebSocket gateways are transport adapters, not game engines.
Gateways **MUST NOT** implement scoring/validation, mutate state independently, query Drizzle for convenience, trust client-derived state, or bypass auth.
Validate WebSocket payloads at the boundary and preserve existing event names/contracts.
For concurrent state changes:
- validate current state before mutation;
- prevent duplicate processing where relevant;
- do not trust client event ordering;
- consider retries/reconnects;
- make operations idempotent when duplicate delivery is possible.
Do not treat in-memory process state as durable unless explicitly intended.

## Database / Drizzle
Inspect repository config for DB engine, schema paths, migration commands, naming, and transaction helpers.
- DB schemas are infrastructure concerns; **MUST NOT** leak into domain code.
- Preserve existing naming, constraints, indexes, and relationships.
- Use transactions when multiple writes form one atomic business operation; keep them narrow.
For schema changes:
1. Update the appropriate Drizzle schema.
2. Use the repository's configured migration workflow.
3. Inspect generated migration SQL.
4. Update affected mappings/tests.
**MUST NOT** edit already-applied migrations for new production changes; create an incremental migration.
**MUST NOT** run destructive DB operations unless explicitly required and safe.
Do not create repositories for every table; use them where they protect meaningful boundaries. Simple reads may follow established direct-query/query-service patterns.

## Validation and Public Contracts
Validate all external input with the project's existing mechanism, including HTTP bodies/params/query strings, WebSocket payloads, and external provider/webhook payloads when applicable.
Boundary validation asks "is the input structurally valid?"; domain/application validation asks "is this operation allowed?".
Do not expose persistence models as public contracts unless already intentional.
Do not silently change HTTP paths/status/response shapes, WebSocket event/payload shapes, or public DTO fields. Update affected in-repo consumers/contracts when required.

## Authentication, Authorization, Security
Inspect existing auth before modifying it; do not invent a parallel mechanism.
- Authentication identifies the actor; authorization decides whether the actor may act.
- Derive identity from trusted server context when available.
- Enforce authorization server-side for HTTP and WebSocket actions.
- Guest access may differ from authenticated-user permissions/persistence.
- Treat all client/provider input as untrusted.
- Do not expose raw DB errors, stack traces, or secrets.
- Never commit/log passwords, credentials, access/refresh tokens, API keys, or secrets.
- Use environment configuration for secrets.
- When adding required env vars, follow existing config/validation and update existing examples/docs when present; never add real secrets to examples.

## External Integrations / Async Work
External SDKs/providers belong in infrastructure, behind project-level interfaces when that protects application logic.
- Do not call provider SDKs from domain entities.
- Handle provider failures/timeouts explicitly.
- Validate provider responses before they affect persisted state.
Use synchronous flow when results are needed immediately and work is inexpensive.
Introduce queues/jobs only when required by current behavior/task (e.g. retries, expensive work, durable async side effects) and inspect existing infrastructure first.
**MUST NOT** introduce Kafka, RabbitMQ, Redis, microservices, event sourcing, or distributed coordination solely for architectural preference.

## Errors and Logging
Use the existing error model and logger.
- Represent expected domain/application failures explicitly.
- Avoid NestJS transport exceptions in domain code; map failures at transport boundaries.
- Log useful operational metadata, not full payloads by default.
- Avoid noisy logs on hot realtime paths.
- Never log secrets/credentials.

## Dependencies and Style
Follow existing TypeScript/NestJS conventions and comparable files.
Prefer explicit names, cohesive functions, clear domain terminology, dependency injection at boundaries, and meaningful types.
Avoid `any` when practical, generic `utils` for domain behavior, unnecessary inheritance, unclear boolean parameters, and abstractions justified only by hypothetical future use.
Before adding a dependency, verify existing dependencies/runtime cannot solve the need. Do not replace established libraries due to preference.
Respect configured path aliases; do not add one-off aliases without a broader reason.

## Commands
Before running commands, inspect `package.json` and repository config to determine the package manager and scripts for install, dev, build, tests, lint, type checking, and Drizzle migrations/generation.
Use only configured commands and run checks appropriate to the changed scope.
**MUST NOT** claim a command/build/test/migration passed unless it was actually executed successfully.
**MUST NOT** disable lint, validation, type checking, or tests to make a change pass.
**MUST NOT** weaken tests to accommodate incorrect behavior.

## Agent Workflow
1. Read the task completely.
2. Inspect affected code and similar implementations.
3. Identify the owning architectural layer.
4. Verify repository-specific conventions/commands.
5. Reuse existing abstractions.
6. Implement the smallest correct change.
7. Add/update tests at the lowest useful layer.
8. Run relevant checks; investigate failures instead of bypassing them.
9. Review the final diff for unrelated changes, broken contracts, and boundary violations.

## Definition of Done
Verify before completion:
- requested behavior is implemented;
- architecture boundaries remain intact;
- external input/auth are handled where applicable;
- server authority is preserved for gameplay;
- DB changes use the correct migration workflow;
- relevant tests/lint/type/build checks were run when available;
- no secrets or unrelated refactors were introduced;
- public contracts did not change silently;
- final diff was reviewed.
Prefer clarity and the smallest correct implementation over architectural ceremony.