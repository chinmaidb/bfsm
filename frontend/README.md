# Ledgerly web app

Responsive React + TypeScript application built with Vite, Tailwind CSS, and
shadcn/ui component conventions. The UI provides role-based workspaces for
administrators, promoters, and members and calls the Go API directly.

## Local development

```sh
cp .env.example .env.local
npm install
npm run dev
```

Configure `VITE_API_BASE_URL` to point to the API's `/api/v1` base URL. The Go
API must allow the frontend origin through `CORS_ALLOWED_ORIGINS` (default:
`http://localhost:5173`).

## Included workflows

- Admin master data for promoters, members, and monthly schemes
- Enrollment creation and status changes
- Payment entry, payment history, and a print-friendly receipt view
- Customer and supplier directories, customer receipt entry, supplier payment
  vouchers, cashbook, and party ledger
- Manual draw-result recording and date-filtered draw history
- Monthly promoter incentive and commission entry, approval, and payout status
- Payment, daybook, due, incentive, and audit reports
- Member profile, enrollment, and payment views

This frontend intentionally does not run a lottery or calculate incentive
formulas. Admins enter the selected draw result and incentive/commission values.
