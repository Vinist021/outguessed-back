# AGENTS.md

## Project Overview

This repository contains the backend for a multiplayer question/card game.

Known project direction:

* NestJS;
* TypeScript;
* Drizzle ORM;
* relational database;
* React frontend maintained separately;
* realtime multiplayer communication through WebSocket;
* architecture inspired by Clean Architecture / Hexagonal Architecture;
* modular monolith as the preferred deployment model.

Core domain areas include accounts, authentication, guests, decks, cards, matches, game rules, board state, realtime chat, and multiple answer-validation strategies.

Some features may still be planned rather than implemented. **Existing repository code and configuration are the source of truth for current behavior.**

Before relying on any command, path, dependency, or implementation detail not explicitly defined here, verify it in the repository.

---

## Instruction Priority

When instructions conflict, use this priority:

1. Explicit instructions from the current task.
2. The closest applicable `AGENTS.md`.
3. Project documentation and explicit contracts.
4. Existing code patterns.
5. General conventions of NestJS, TypeScript, Drizzle, and the surrounding ecosystem.

MUST NOT violate an explicit architectural restriction merely to reproduce a legacy pattern that conflicts with it.

---

## Before Making Changes

Before implementing a change:

1. Inspect the relevant module and neighboring implementations.
2. Identify an existing feature with similar behavior when possible.
3. Inspect affected  schemas, DTOs, repositories, use cases, controllers, and gateways.
4. Verify relevant scripts and dependencies in `package.json`.
5. Prefer the smallest correct change.
6. Preserve existing public contracts unless the task explicitly changes them.

MUST NOT perform unrelated refactors.

MUST NOT introduce a new abstraction before checking whether the repository already provides one.

---

## Architecture

The intended architecture is a **modular monolith** with domain-oriented modules.

Complex domains SHOULD follow Clean/Hexagonal boundaries:

```text
Presentation
    ↓
Application
    ↓
Domain
    ↑
Infrastructure
```

Dependency direction:

```text
presentation ──→ application ──→ domain
infrastructure ────────────────→ domain/application ports
```

### Domain

Contains business rules and domain concepts.

Typical contents:

```text
domain/
├── entities/
├── value-objects/
├── events/
├── repositories/
└── services/
```

Rules:

* Domain MUST NOT import NestJS.
* Domain MUST NOT import Drizzle.
* Domain MUST NOT import database schemas.
* Domain MUST NOT import controllers, gateways, HTTP, WebSocket, Redis, queues, or external SDKs.
* Domain SHOULD remain executable and testable without infrastructure.
* Business invariants SHOULD live in domain objects or domain services rather than controllers or gateways.

Do not force rich domain modeling onto simple CRUD modules when it adds no value.

### Application

Coordinates application behavior.

Typical contents:

```text
application/
├── use-cases/
├── commands/
├── queries/
└── dto/
```

Application code MAY depend on the domain.

Application code SHOULD depend on infrastructure through interfaces/ports where isolation provides meaningful value.

Use cases SHOULD represent explicit application actions.

Prefer:

```text
CreateMatch
JoinMatch
StartMatch
SubmitAnswer
JudgeAnswer
FinishRound
```

over a large generic service containing unrelated behavior.

### Infrastructure

Contains technical implementations such as:

* Drizzle persistence;
* database access;
* external APIs;
* AI providers;
* Redis;
* queues;
* infrastructure adapters.

Infrastructure MAY depend on domain/application contracts.

Infrastructure MUST NOT become the owner of game rules.

### Presentation

Contains entry points such as:

* HTTP controllers;
* WebSocket gateways;
* request DTOs;
* transport-specific mapping.

Presentation MUST NOT contain core business rules.

Controllers and gateways SHOULD:

1. receive input;
2. authenticate/authorize;
3. validate boundary data;
4. invoke application behavior;
5. map the result to the transport.

---

## Repository Structure

Follow the structure that actually exists in the repository.

For new domain modules, prefer the following structure when consistent with existing code:

```text
src/
├── modules/
│   ├── auth/
│   ├── users/
│   ├── decks/
│   ├── cards/
│   ├── matches/
│   │   ├── domain/
│   │   ├── application/
│   │   ├── infrastructure/
│   │   └── presentation/
│   ├── game/
│   ├── chat/
│   └── ai/
│
├── database/
│   ├── schema/
│   └── migrations/
│
└── shared/
```

This tree describes the intended direction, not proof that every directory currently exists.

MUST inspect existing paths before creating a parallel structure.

Do not create folders solely to satisfy this example if the repository already uses an equivalent organization.

---

## Module Boundaries

Organize code by business capability rather than technical type.

Prefer:

```text
modules/
├── decks/
├── matches/
└── users/
```

over global directories such as:

```text
controllers/
services/
repositories/
```

Modules SHOULD expose the smallest API required by other modules.

MUST NOT import another module's internal infrastructure files directly.

Avoid cross-module deep imports such as:

```text
modules/matches/infrastructure/...
```

from unrelated modules.

When modules need to collaborate, prefer an exported application service/use case, explicit port, or domain/application event already supported by the project.

Avoid circular module dependencies.

---

## Domain Concepts

Use terminology consistently.

### Deck

A collection of cards/questions.

Deck concerns include content and metadata. Game rules SHOULD NOT be implicitly coupled to a deck unless the domain explicitly requires it.

### Card

Represents content used during gameplay.

Cards may have different types and type-specific configuration.

Avoid large chains of conditionals spread throughout the code when card behavior varies materially by type.

### Match

Represents a multiplayer game session.

The backend MUST be authoritative over match state.

### Player

Represents a participant in a match.

A player may correspond to an authenticated account or a guest depending on current project behavior.

Do not assume a player always has a persistent user account.

### Game Engine

Complex gameplay rules SHOULD be isolated from transport and persistence concerns.

Conceptually:

```text
GameState + Command
        ↓
    Game Engine
        ↓
New State + Events
```

The game engine SHOULD remain deterministic where possible.

### Board

Represents board state and player positions when the selected game mode uses a board.

### Answer Validation

Validation behavior may include:

* exact-value validation;
* player judge;
* AI interpretation.

Different validation algorithms SHOULD use interchangeable strategies instead of scattered mode conditionals.

---

## Implementing Features

When adding behavior:

1. Find the owning module.
2. Identify the application action.
3. Place business rules in the domain when they represent domain invariants or game behavior.
4. Implement orchestration through a use case/service following existing patterns.
5. Access persistence through the project's established persistence boundary.
6. Add or update the relevant transport adapter.
7. Add tests at the lowest useful layer.
8. Update external contracts only when required.

A feature SHOULD NOT require controllers or gateways to understand persistence details.

---

## Use Cases and Services

Prefer narrowly scoped application behavior.

Good:

```text
CreateDeckUseCase
CreateMatchUseCase
JoinMatchUseCase
SubmitAnswerUseCase
FinishMatchUseCase
```

Avoid creating a single large service responsible for an entire complex domain.

Example to avoid:

```text
MatchService
├── authentication
├── join
├── scoring
├── answer validation
├── board movement
├── WebSocket broadcasting
├── persistence
└── AI integration
```

A `Service` is acceptable when it has one cohesive responsibility.

Follow existing naming conventions before introducing `UseCase` suffixes.

---

## Game State

The server MUST be authoritative for gameplay state.

Clients SHOULD send **intent**, not trusted results.

Prefer receiving:

```json
{
  "matchId": "match-id",
  "answer": "Brasília"
}
```

Do not trust client-supplied derived state such as:

```json
{
  "score": 500,
  "newPosition": 8,
  "answerCorrect": true
}
```

The backend MUST calculate:

* whether an action is allowed;
* whether an answer is correct;
* score changes;
* board movement;
* turn progression;
* round completion;
* match completion.

Validate actions against the current match state.

---

## Match State Machine

Match lifecycle SHOULD be explicit.

Representative states may include:

```text
WAITING
PLAYING
AWAITING_ANSWERS
JUDGING
ROUND_FINISHED
FINISHED
```

Use the states actually implemented by the project.

Actions MUST validate whether they are legal in the current state.

Do not rely solely on frontend UI restrictions to prevent invalid transitions.

State transitions SHOULD be centralized rather than reproduced across controllers, gateways, or consumers.

---

## Answer Validation Strategies

Answer validation SHOULD be modeled behind an explicit strategy when multiple validation modes exist.

Conceptually:

```ts
interface AnswerValidator {
  validate(input: ValidationInput): Promise<ValidationResult>;
}
```

Possible implementations:

```text
ExactAnswerValidator
PlayerJudgeValidator
AiAnswerValidator
```

MUST NOT spread repeated logic such as:

```ts
if (mode === 'exact') { ... }
else if (mode === 'ai') { ... }
else if (mode === 'player-judge') { ... }
```

through multiple layers.

Adding a new validation strategy SHOULD require minimal modification to existing strategies.

---

## Realtime

Realtime communication belongs at the application boundary.

WebSocket gateways SHOULD behave like controllers:

```text
WebSocket event
      ↓
Gateway
      ↓
Application use case
      ↓
Domain
```

Gateways MUST NOT:

* contain scoring rules;
* determine whether an answer is correct;
* mutate game state independently of application/domain logic;
* query Drizzle directly for convenience;
* trust client-provided scores or positions;
* become the source of truth for match state.

Prefer event names that describe domain/application actions and outcomes.

Follow existing event naming conventions before introducing a new convention.

Examples of conceptual events:

```text
match:join
match:start
match:answer:submit
match:answer:judged
match:round:finished
match:finished
chat:message
```

These examples MUST NOT override an existing project convention.

Payloads MUST be validated at the transport boundary.

Authentication and authorization MUST apply to WebSocket actions where appropriate.

---

## Domain/Application Events

Events MAY be used to decouple secondary reactions from core game behavior.

Examples:

```text
PlayerJoined
MatchStarted
AnswerSubmitted
AnswerCorrect
AnswerIncorrect
ScoreChanged
PlayerMoved
RoundFinished
MatchFinished
```

Use events when multiple independent components react to something that already happened.

Do not convert straightforward synchronous control flow into events without a concrete benefit.

Do not use events to hide critical business flow that would be clearer as a direct call.

Event handlers MUST be idempotent when their execution mechanism may deliver an event more than once.

Do not introduce Kafka, RabbitMQ, or other external brokers unless required by the task or existing architecture.

---

## Database

Use Drizzle for relational persistence.

Inspect the repository to determine:

* database engine;
* schema paths;
* Drizzle configuration;
* migration commands;
* naming conventions;
* transaction helpers.

Do not assume these details from this document alone.

### Schemas

Database schemas MUST remain persistence concerns.

MUST NOT import Drizzle schema objects into the domain layer.

Changes to persistence models SHOULD preserve existing naming, constraints, indexes, and relationship conventions.

### Migrations

For schema changes:

1. Modify the appropriate Drizzle schema.
2. Use the migration workflow configured by the repository.
3. Inspect the generated migration before accepting it.
4. Update affected persistence mappings and tests.

MUST NOT manually edit an already-applied migration to change production schema history.

Create an incremental migration instead.

MUST NOT run destructive database operations unless explicitly required and safe within the task.

### Transactions

Use a transaction when multiple database writes form one atomic business operation.

Do not assume multiple independent Drizzle operations are atomic.

Keep transactions as narrow as practical.

---

## Repositories and Queries

Do not create repository abstractions automatically for every table.

Repositories SHOULD be used when they protect meaningful domain/application boundaries.

Example:

```ts
interface MatchRepository {
  findById(id: MatchId): Promise<Match | null>;
  save(match: Match): Promise<void>;
}
```

Infrastructure may provide:

```text
DrizzleMatchRepository
```

Simple read-only queries MAY use a dedicated query service backed directly by Drizzle if that matches existing project conventions.

Examples:

```text
ListPublicDecks
GetRanking
GetMatchHistory
```

Avoid wrappers that merely rename every Drizzle method without adding architectural value.

---

## CQRS

A lightweight separation between commands and queries is encouraged for complex modules.

Commands mutate state:

```text
CreateMatch
JoinMatch
SubmitAnswer
JudgeAnswer
FinishRound
```

Queries read state:

```text
GetMatch
GetDeck
ListDecks
GetRanking
```

MUST NOT introduce full CQRS infrastructure, separate databases, or event sourcing solely because commands and queries are organized separately.

Use `@nestjs/cqrs` only if already present or specifically justified by the task.

---

## DTOs and Validation

External input MUST be validated at the application boundary.

This applies to:

* HTTP request bodies;
* route parameters;
* query parameters;
* WebSocket payloads;
* external webhook/event payloads when applicable.

Use the validation mechanism already configured by the repository.

Do not duplicate domain invariants in DTO validation when they belong to domain behavior.

DTO validation answers:

> Is this external input structurally acceptable?

Domain validation answers:

> Is this operation valid according to business rules?

Do not expose persistence models directly as public API contracts unless that is an intentional existing convention.

---

## API Conventions

Follow existing HTTP conventions before adding endpoints.

Controllers SHOULD remain thin.

Controllers MUST NOT:

* access Drizzle directly unless the existing architecture explicitly uses a simple query endpoint pattern;
* implement game rules;
* implement authorization through ad-hoc conditionals when established guards/policies exist;
* expose secrets or internal infrastructure errors.

Do not silently change:

* endpoint paths;
* response shapes;
* status codes;
* WebSocket event names;
* public DTO fields.

When a contract change is required, update afected a consumers/contracts where they exist in this repository.

---

## Authentication and Authorization

Inspect existing authentication implementation before modifying it.

MUST NOT invent an authentication mechanism when one already exists.

MUST distinguish authentication from authorization:

```text
Authentication → Who is this actor?
Authorization  → May this actor perform this action?
```

Never trust user/player identifiers supplied by a client when the authenticated connection already provides authoritative identity.

Guest access MUST NOT be assumed to have the same permissions or persistence semantics as authenticated accounts.

Never log:

* passwords;
* plaintext credentials;
* access tokens;
* refresh tokens;
* API keys;
* secrets.

---

## External Integrations and AI

External providers MUST remain outside the domain layer.

Wrap providers behind project-level interfaces when doing so protects application logic from provider-specific APIs.

For AI answer validation:

```text
Submit Answer
      ↓
Application
      ↓
Answer Validation Port
      ↓
AI Infrastructure Adapter
```

MUST NOT place model-provider SDK calls inside domain entities.

Treat AI output as untrusted external input.

Validate and normalize AI responses before allowing them to affect persisted game state.

Handle provider errors and timeouts explicitly.

Do not embed secrets or API keys in source code.

---

## Async Work

Use synchronous application flow when the result is required immediately and processing is inexpensive.

Use background jobs/queues only when justified by existing requirements, such as:

* expensive AI processing;
* retries;
* non-critical side effects;
* work that should survive request disconnection.

MUST inspect existing queue infrastructure before introducing another mechanism.

Do not introduce a queue solely to make code appear more distributed.

---

## Error Handling

Use the existing project error model.

Domain/application code SHOULD represent expected failures explicitly.

Examples:

```text
MatchNotFound
PlayerAlreadyJoined
MatchAlreadyStarted
InvalidMatchState
UnauthorizedMatchAction
```

Avoid throwing transport-specific NestJS exceptions from domain code.

Map domain/application failures to HTTP or WebSocket responses at the appropriate boundary.

Do not expose raw database errors or stack traces to clients.

---

## Logging

Use the project's existing logger.

Logs SHOULD provide operational context such as:

* operation;
* relevant entity ID;
* match ID;
* failure category.

Do not add noisy logs to hot realtime paths without a concrete operational purpose.

MUST NOT log secrets or sensitive credentials.

Avoid logging complete user-generated payloads when structured metadata is sufficient.

## Commands

Do not assume package-manager or script names.

Before executing project commands, inspect:

```text
package.json
```

Use the repository's configured package manager and existing scripts.

Typical validation categories to identify are:

```bash
# Install
<verify package manager and install command>

# Development
<verify package.json>

# Build
<verify package.json>

# Tests
<verify package.json>

# Lint
<verify package.json>

# Type checking
<verify package.json>

# Database generation/migrations
<verify package.json and drizzle config>
```

MUST NOT invent or report a command as successful unless it was actually executed successfully.

Run only commands appropriate to the modified scope.

---

## Code Style

Follow existing TypeScript and NestJS conventions in the repository.

Before creating new conventions, inspect comparable files.

Prefer:

* explicit names;
* small cohesive functions;
* immutable domain state where practical;
* dependency injection at infrastructure/application boundaries;
* clear domain terminology.

Avoid:

* generic `utils` for domain behavior;
* boolean parameters with unclear meaning;
* large conditional blocks encoding multiple strategies;
* `any` when a meaningful type can be expressed;
* unnecessary inheritance;
* abstractions with a single hypothetical future use.

Comments SHOULD explain non-obvious decisions, invariants, or constraints.

Do not add comments that merely restate the code.

---

## Imports

Respect architectural boundaries when importing.

Allowed conceptual direction:

```text
presentation → application → domain
infrastructure → application/domain contracts
```

Forbidden conceptual direction:

```text
domain → infrastructure
domain → presentation
domain → NestJS
domain → Drizzle
```

Avoid importing internal implementation files across module boundaries.

Use configured path aliases if the repository already defines them.

Do not introduce new aliases for a single feature without a broader project reason.

---

## Dependency Management

Before adding a package:

1. Check whether installed dependencies already provide the capability.
2. Check whether the platform/runtime provides it.
3. Verify that adding the package materially simplifies or improves the implementation.

MUST NOT add dependencies for trivial functionality.

MUST NOT replace an established project dependency merely due to personal preference.

Changes to dependencies SHOULD be directly related to the task.

---

## Security

Treat all client and external-provider input as untrusted.

MUST:

* validate external input;
* enforce authorization server-side;
* derive authenticated identity from trusted server context;
* protect secrets through environment configuration;
* avoid exposing internal errors;
* verify match actions against authoritative server state.

MUST NOT:

* commit credentials;
* trust client-calculated scores;
* trust client-calculated board positions;
* trust client claims that an answer is correct;
* rely on frontend validation for security.

---

## Environment Variables

Inspect existing environment configuration before adding variables.

When adding a required environment variable:

1. use the project's existing configuration mechanism;
2. update the appropriate example/documentation file if one exists;
3. validate required configuration using the project's established pattern;
4. never commit the real secret.

MUST NOT add secret values to `.env.example`.

---

## Realtime Consistency

Multiplayer behavior MUST account for concurrent actions.

When implementing state-changing match actions:

* validate the current state before mutation;
* prevent duplicate processing where relevant;
* avoid trusting event ordering from clients;
* consider reconnect/retry behavior;
* make operations idempotent when duplicate delivery is possible.

Do not use in-memory process state as durable persistence unless explicitly intended.

If match state is held in memory for the current implementation, do not silently assume it will survive process restart or horizontal scaling.

---

## Scaling

The preferred initial deployment is a modular monolith.

MUST NOT introduce microservices solely for conceptual separation.

Separate services only when there is a concrete requirement such as:

* independent scaling;
* fault isolation;
* significantly different workload characteristics;
* independent deployment requirements.

Redis, queues, brokers, and distributed coordination SHOULD be introduced only when required by current behavior or explicitly requested.

Logical module boundaries SHOULD make future extraction possible without requiring distributed infrastructure today.

---

## Agent Workflow

For every change:

1. Read the task completely.
2. Inspect the affected module and related implementations.
3. Verify repository-specific commands and conventions.
4. Identify the owning architectural layer.
5. Reuse existing abstractions where appropriate.
6. Implement the smallest correct change.
7. Run the relevant validation commands.
8. Inspect failures rather than bypassing checks.
9. Review the final diff for unrelated modifications and architectural violations.

---

## Definition of Done

Before declaring a task complete, verify that:

* [ ] The requested behavior is implemented.
* [ ] Relevant existing patterns were followed.
* [ ] Architectural boundaries remain intact.
* [ ] External input is validated where applicable.
* [ ] Server authority is preserved for gameplay behavior.
* [ ] Database changes include the appropriate migration workflow when applicable.
* [ ] Relevant lint/type/build checks pass when available.
* [ ] No secrets or credentials were introduced.
* [ ] No unrelated refactors are included.
* [ ] Public contracts were not changed silently.
* [ ] The final diff was reviewed.

Only claim checks passed when they were actually executed successfully.

---

## Do Not

* MUST NOT put domain/game rules in controllers or WebSocket gateways.
* MUST NOT import NestJS, Drizzle, or infrastructure into domain code.
* MUST NOT access the database from arbitrary layers for convenience.
* MUST NOT trust client-supplied derived game state.
* MUST NOT bypass authorization because the frontend already restricts an action.
* MUST NOT create generic repositories for every table without architectural value.
* MUST NOT introduce microservices, queues, Redis, brokers, or event sourcing prematurely.
* MUST NOT add dependencies when existing capabilities are sufficient.
* MUST NOT duplicate an abstraction before searching for an existing equivalent.
* MUST NOT perform unrelated refactors.
* MUST NOT silently alter public HTTP or WebSocket contracts.
* MUST NOT edit historical applied migrations to represent new schema changes.
* MUST NOT disable lint, validation, type checking, or tests to make an implementation pass.
* MUST NOT weaken tests to accommodate incorrect behavior.
* MUST NOT commit secrets or credentials.
* MUST NOT claim tests, commands, builds, or migrations succeeded unless they were actually run successfully.
* SHOULD NOT introduce patterns based only on hypothetical future requirements.
* SHOULD prefer clarity and the smallest correct implementation over architectural ceremony.
