-- migration-check:allow-destructive expand-shipped-in=v1.7.0 reason=status column dropped, only ambiguous join refs remain
ALTER TABLE widgets_a DROP COLUMN status;
