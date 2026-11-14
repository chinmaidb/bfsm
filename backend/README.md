# Benefit Scheme API

Go 1.25+ API using Echo v5, PostgreSQL, Argon2id password hashes, and RSA
RS256 access tokens. The API stores a manually recorded monthly draw result; it
does not run or simulate a lottery.

## Local setup

From `backend/`:

```sh
cp .env.example .env
mkdir -p secrets
go run ./cmd/keygen ./secrets/jwt-private.pem ./secrets/jwt-public.pem
psql 'postgres://schemeapp:schemeapp@localhost:5432/schemeapp?sslmode=disable' -f migrations/001_foundation.sql
psql 'postgres://schemeapp:schemeapp@localhost:5432/schemeapp?sslmode=disable' -f migrations/002_scheme_domain.sql
psql 'postgres://schemeapp:schemeapp@localhost:5432/schemeapp?sslmode=disable' -f migrations/003_customer_supplier_ledger.sql
psql 'postgres://schemeapp:schemeapp@localhost:5432/schemeapp?sslmode=disable' -f migrations/004_access_control.sql
psql 'postgres://schemeapp:schemeapp@localhost:5432/schemeapp?sslmode=disable' -f migrations/005_limit_company_admin.sql
```

Edit `.env` with credentials for your local PostgreSQL instance before using
these commands.

The local setup expects a PostgreSQL instance reachable from your development
machine.

## Production without Docker

The Go API can serve the built frontend from the same origin. Build the frontend
first; Vite's default API URL is same-origin (`/api/v1`), while `npm run dev`
proxies API requests to `http://localhost:8080`:

```sh
cd ../frontend
pnpm install --frozen-lockfile
pnpm build
cd ../backend
```

For a non-Docker production deployment, prepare the environment file, update
`DATABASE_URL`, and let `run.sh` build the frontend and Go app, generate a JWT
keypair if missing, apply migrations, and launch the app under PM2:

```sh
bash run.sh env
# Edit .env and set the production DATABASE_URL.
bash run.sh all
```

The API serves static assets and frontend routes from `WEB_DIR`, falling back
to `index.html` for browser routes. Unknown `/api/...` endpoints remain API
404s. The PM2 config serves `build/bfsm-web` and listens on port 9562. If
`.env` already exists, it is left unchanged. Run the migrations before starting
the API so the company administrator role exists for super-admin provisioning.

To create a separate admin account, export the values from `.env` in your
shell, then run:

```sh
export DATABASE_URL='postgres://schemeapp:schemeapp@localhost:5432/schemeapp?sslmode=disable'
export JWT_PRIVATE_KEY=./secrets/jwt-private.pem
export NEW_USERNAME=admin
export NEW_PASSWORD='use-a-unique-password-of-12-or-more-characters'
export NEW_ROLE=admin
go run ./cmd/create-user
go run ./cmd/api
```

To add a promoter or member login later, run `cmd/create-user` with the matching
`NEW_ROLE`, note the printed account ID, then set that `userId` on the promoter
or member record in the admin master screen.

To print an Argon2id hash without creating a database account, run
`go run ./cmd/create-user hash-password` from `backend/` and enter the password
at the prompt. The command writes the hash to standard output and does not
connect to the database.

The example Postgres password is for local development only. Keep production
database credentials and the private signing key in a secrets manager. The API
reads configuration from environment variables: `API_ADDRESS`, `APP_ENV`,
`DATABASE_URL`, optional `WEB_DIR`, `JWT_ISSUER`, `JWT_PRIVATE_KEY`,
`JWT_LIFETIME_MINUTES`, and `CORS_ALLOWED_ORIGINS` (comma-separated exact
origins). In production,
`SUPER_ADMIN_USERNAME` and `SUPER_ADMIN_PASSWORD` are required together; the
password must be at least 16 characters.

### Lifecycle

Run these commands from `backend/`:

```sh
bash run.sh build       # build frontend and linux/amd64 Go executable
bash run.sh frontend    # build/copy frontend only
bash run.sh backend     # cross-compile Go executable only
bash run.sh env         # create .env from .env.example if absent
bash run.sh keygen      # generate keypair only if both keys are absent
bash run.sh migrate     # apply pending database migrations
bash run.sh start       # start or reload app under PM2
bash run.sh restart     # rebuild, migrate, then restart under PM2
bash run.sh stop        # stop PM2 process without removing it
bash run.sh status      # show PM2 status
bash run.sh logs        # follow app logs; accepts PM2 log options
bash run.sh delete      # remove PM2 process (builds and data are preserved)
```

`bash run.sh` or `bash run.sh all` performs the full build, environment and key
setup, migration, and PM2 start sequence. The script requires `pnpm`, Go,
OpenSSL, and PM2. The Go executable is cross-compiled for Linux amd64 with CGO
disabled. Keep `.env` and `secrets/jwt-private.pem` protected and backed up;
the generated super-admin password is stored in the protected `.env` file.

Migrations are applied in filename order and recorded in `schema_migrations`;
each SQL migration and its version record commit in one transaction. This works
for a fresh database. When running `go run ./cmd/migrate` from `backend/`, the
migrator reads `migrations/`; set `MIGRATIONS_DIR` to use another directory.
The `.env` file is optional when `DATABASE_URL` is already exported or provided
by the runtime. If the original schema files `001`–`003` were already
applied manually, baseline those exact versions before the first production
run so they are not re-applied; leave `004_access_control.sql` and
`005_limit_company_admin.sql` unrecorded so the access-control schema and
restricted administrator grants are installed:

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
  version text PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO schema_migrations(version) VALUES
  ('001_foundation.sql'),
  ('002_scheme_domain.sql'),
  ('003_customer_supplier_ledger.sql');
```

Only baseline versions whose schema is already present. If migration `004` was
also applied manually, include `('004_access_control.sql')`. If `005` was also
applied manually, include that version too.
Back up the database before deploying schema changes.

The configured super admin is provisioned when the API starts. Use the account
and role screens to create other company staff and grant operation-specific
access. For local development, `cmd/create-user` can still create admin,
promoter, or member accounts after migrations have been applied.

To create a promoter or member login:

```sh
export NEW_USERNAME=collector
export NEW_PASSWORD='use-a-unique-password-of-12-characters'
export NEW_ROLE=promoter
go run ./cmd/create-user
```

The same command can create member accounts by changing `NEW_ROLE`. Link their
returned user IDs to promoter/member records in the admin screen. Company staff
accounts are created in the access control screen and assigned a custom role.

## Structure

- `cmd/api`: process startup and graceful shutdown
- `cmd/create-user`, `cmd/keygen`, `cmd/migrate`: account provisioning, signing key setup, and versioned schema migrations
- `internal/application/access.go`: audited account and company-role management, permission catalog, super admin provisioning
- `internal/httpapi`: Echo routes, auth middleware, and request/response mapping
- `internal/application`: use cases, transactions, reports, audit, and outbox writes
- `internal/domain`: business event types
- `internal/postgres`: database connection and pool configuration
- `internal/security`: Argon2id and RS256 token primitives
- `migrations`: PostgreSQL schema

Handlers depend on application use cases. Business changes, audit entries, and
outbox events commit together in one PostgreSQL transaction. The outbox is the
event boundary for future consumers; this MVP does not require a broker or run a
background event worker. Reads stay direct SQL queries. Master records use
ordinary REST CRUD; financial and draw facts are recorded as append-only
records.

## API overview

All routes use `/api/v1`. Login is public; all other routes need
`Authorization: Bearer <accessToken>`.

| Role | Routes |
| --- | --- |
| Public | `POST /auth/login`; `GET /catalog/schemes` |
| Company roles | Operation-specific access to master data, enrollments, payments, draw records, incentives, parties, financial ledger, reports, audit, accounts, and roles |
| Admin | `GET/POST /promoters`, `/members`, `/schemes`, `/parties`; `PUT/DELETE /{resource}/:id`; `GET/POST /enrollments`; `PATCH /enrollments/:id/status`; `POST /draw-results`; `POST /promoter-incentives`; `PATCH /promoter-incentives/:id/status`; `GET/POST /cash-receipts`; `GET/POST /supplier-payments`; `GET /financial-entries/:id/receipt`; `GET /reports/{payments,daybook,dues,incentives,draw-results,cashbook,customer-receipts,supplier-payments,party-ledger}?from=YYYY-MM-DD&to=YYYY-MM-DD`; `GET /audit`; `GET /accounts`; `POST/PUT/DELETE /accounts`; `GET /roles`; `POST/PUT/DELETE /roles`; `GET /permissions`; `GET /payments/:id/receipt` |
| Admin/promoter | `POST /payments`; `GET /members/mine`; `GET /payments/mine`; `GET /enrollments/mine`; `GET /promoter/incentives/mine`; `GET /payments/:id/receipt` |
| Member | `GET /member/me`; `GET /member/payments`; `GET /member/payments/:id/receipt`; `GET /member/enrollments` |

Company operators use the `staff` account type and receive a company role.
Routes check operation permissions on every request; the frontend hides screens
outside the session's current grants. The built-in Company administrator role
can manage accounts and role definitions, but has no operational grants by
default. Operational access is assigned through explicit roles. Custom role
grants and account changes are audited, and role managers cannot grant or assign
permissions they do not themselves hold. Account and role deletes are audited
deactivations to retain history and foreign key references. The configured
super admin bypasses permission checks and is managed only through production
configuration.

The company role catalog includes the `company_admin`, `promoter`, and
`customer` role keys. Promoter and customer are seeded as role templates without
operational permissions so they can be used to distinguish future role-specific
screens and links. The catalog is shared by this application deployment; it is
not currently scoped to separate company records.

Master list endpoints accept `limit` (1–200, default 100) and `offset`. Master
deletes are soft deactivations. Financial payment and draw result rows cannot be
edited or deleted; corrections should be represented by a separately audited
adjustment workflow when that policy is defined.

The cash ledger supports `customer` and `supplier` parties. Customer receipts
and supplier payments use separate immutable entries and separate receipt or
voucher numbers; both are idempotent and audited. A customer can optionally link
to a scheme member. Scheme contribution payments stay in their existing ledger
so they cannot be confused with other customer receipts. The cashbook and party
ledger reports include only these general customer/supplier entries.

Amounts are integer minor units (for INR, paise). Incentive and commission
amounts are entered by an admin because the source notes do not define their
calculation rules. Payment requests require a unique `idempotencyKey` to avoid
duplicate receipts when a client retries. KYC stores the document type and last four digits only; do
not send complete government ID numbers to this API. Reports cap output at 5,000
rows.

## Authorization and security

Use Argon2id for account password hashes; passwords are never stored in
plaintext. `cmd/keygen` writes a 3072-bit RSA keypair with restrictive private
key permissions. The API signs with RS256 and verifies against the public key
derived from that private key. Access tokens default to 15 minutes and can be
configured up to 24 hours. Give each account a unique password and keep the
private key outside source control.

Audit state snapshots exclude password data. KYC fields are restricted to the
document type and last four digits. The audit table rejects update/delete
operations. Audit entries and event outbox records are written in the same
transaction as each implemented mutation.
