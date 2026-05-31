# PostgreSQL SQL Package

This directory is the public-safe PostgreSQL package for FLVX.

The live application source of truth is the GORM model layer in
`go-backend/internal/store/model/model.go`, and the backend runs
`AutoMigrate` at startup. The SQL files here are sanitized bootstrap and
inspection assets for operators who want to pre-create or review the database
schema without exposing production data.

Files:

- `schema.sql`: table and index DDL generated from the public-safe model layer.
- `seed.sql`: sanitized first-run seed data only. It contains the public default
  admin account used by the application and no production users, nodes, domains,
  tokens, IP addresses, traffic data, certificates, or private keys.

For a normal fresh VPS install, use:

```bash
./install-public-safe.sh
```

The install path lets the backend create or reconcile the schema automatically.
Use `schema.sql` manually only when your database process requires a pre-created
schema.
