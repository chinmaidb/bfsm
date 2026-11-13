# Benefit scheme app

The project is split into an API and a web client:

- [`backend/`](backend/README.md) — Go, Echo, PostgreSQL, Argon2id, RS256 tokens, audit trail, and transactional event outbox.
- [`frontend/`](frontend/README.md) — responsive React/TypeScript interface with Tailwind CSS and shadcn/ui conventions.

Start with the backend README to create the local database, signing keypair,
schema, and first administrator. Then configure `frontend/.env.local` and start
the Vite development server. The frontend expects the API at
`http://localhost:8080/api/v1` by default.

The application records a person chosen for each monthly draw; it does not run
a virtual lottery. Promoter incentive and commission amounts are entered by an
administrator until their calculation policy is agreed.

Customer receipts and supplier payments are supported in a separate audited
cash ledger. Scheme member contributions remain in their dedicated ledger.
