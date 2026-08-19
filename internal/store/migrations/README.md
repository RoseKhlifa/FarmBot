# SQLite migrations

Add one numbered SQL file per schema change using the form `NNNN_name.sql`.
Files are embedded into the FarmBot binary, applied in numeric order, and
recorded in the `schema_migrations` table. The initial application schema is
owned by P1-02 (`0001_init.sql`).
