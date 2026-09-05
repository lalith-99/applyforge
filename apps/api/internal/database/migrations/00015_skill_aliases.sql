-- +goose Up
CREATE TABLE skill_aliases (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alias           TEXT NOT NULL,
    canonical_name  TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX skill_aliases_alias_lower_idx ON skill_aliases (lower(alias));

INSERT INTO skill_aliases (alias, canonical_name) VALUES
    ('Golang', 'Go'),
    ('K8s', 'Kubernetes'),
    ('K8', 'Kubernetes'),
    ('Postgres', 'PostgreSQL'),
    ('AWS SQS', 'Amazon SQS'),
    ('SQS', 'Amazon SQS'),
    ('AWS SNS', 'Amazon SNS'),
    ('SNS', 'Amazon SNS'),
    ('JS', 'JavaScript'),
    ('TS', 'TypeScript'),
    ('Node', 'Node.js'),
    ('NodeJS', 'Node.js'),
    ('ReactJS', 'React'),
    ('Mongo', 'MongoDB'),
    ('CI/CD', 'CI-CD'),
    ('GH Actions', 'GitHub Actions');

-- +goose Down
DROP TABLE skill_aliases;
