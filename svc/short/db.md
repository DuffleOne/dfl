# database

Uses a postgres database.

```sql
CREATE TABLE resources (
    id text PRIMARY KEY,
    serial SERIAL UNIQUE,
    hash text UNIQUE,
    type text NOT NULL,
    name text,
    owner_id text NOT NULL,
    link text NOT NULL,
    mime_type text,
    shortcuts text[] NOT NULL DEFAULT '{}',
    nsfw boolean NOT NULL DEFAULT false,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    deleted_at timestamp with time zone
);
```
