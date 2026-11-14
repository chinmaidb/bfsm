ALTER TABLE enrollments
    ADD COLUMN ends_on DATE,
    ADD COLUMN payment_frequency TEXT NOT NULL DEFAULT 'weekely'
        CHECK (payment_frequency IN ('daily','weekly', 'biweekly', 'monthly', 'quarterly'));

UPDATE enrollments e
SET ends_on = (e.start_month + make_interval(months => s.duration_months) - interval '1 day')::date
FROM schemes s
WHERE s.id = e.scheme_id;

ALTER TABLE enrollments ALTER COLUMN ends_on SET NOT NULL;
ALTER TABLE enrollments ADD CONSTRAINT enrollments_end_after_start CHECK (ends_on >= start_month);
