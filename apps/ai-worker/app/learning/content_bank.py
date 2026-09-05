"""Curated Quick Prep content for common technologies.

Deterministic stand-in for real AI-generated Quick Prep content (see
MASTER_REQUIREMENTS.md §31 and app/resume/parsing.py for the same rationale).
Skills not in this dictionary fall back to a generic template
(see app/learning/quick_prep.py:_generic_module).
"""

from __future__ import annotations

CONTENT_BANK: dict[str, dict] = {
    "amazon sqs": {
        "what_it_is": "A fully managed message queuing service for decoupling and scaling microservices.",
        "why_it_matters": "Common choice for asynchronous, durable message delivery between services without managing broker infrastructure.",
        "core_concepts": [
            "standard vs FIFO queues",
            "visibility timeout",
            "dead-letter queues",
            "at-least-once delivery",
            "long polling",
        ],
        "screening_points": [
            "Explain the difference between standard and FIFO queues.",
            "Explain how consumers avoid processing the same message twice.",
            "Explain what happens when a consumer fails to process a message.",
        ],
        "questions": [
            {
                "question": "Why SQS instead of Kafka?",
                "concise_answer": "SQS is a simpler, fully managed queue; Kafka is a distributed log suited for high-throughput streaming and replay.",
                "deeper_explanation": "SQS excels at decoupling producers/consumers with minimal operational overhead. Kafka offers ordered, replayable logs and much higher throughput but requires more operational investment.",
            },
            {
                "question": "Standard vs FIFO queues?",
                "concise_answer": "Standard queues offer at-least-once delivery with best-effort ordering; FIFO queues guarantee exactly-once processing and strict ordering within a message group.",
                "deeper_explanation": "FIFO queues trade some throughput for strict ordering and deduplication, useful when order/duplication matters (e.g. financial transactions).",
            },
            {
                "question": "How are duplicate messages handled?",
                "concise_answer": "Consumers should be idempotent since standard queues can deliver a message more than once.",
                "deeper_explanation": "Idempotency is typically achieved by tracking processed message IDs or using natural idempotency keys in the business logic.",
            },
            {
                "question": "What is visibility timeout?",
                "concise_answer": "The window during which a received message is hidden from other consumers while being processed.",
                "deeper_explanation": "If processing isn't acknowledged (deleted) within the timeout, the message becomes visible again and may be redelivered.",
            },
            {
                "question": "What is a dead-letter queue?",
                "concise_answer": "A separate queue that captures messages that repeatedly fail processing, for later inspection.",
                "deeper_explanation": "DLQs prevent poison messages from blocking a queue indefinitely and give visibility into recurring failures.",
            },
            {
                "question": "How would you scale consumers?",
                "concise_answer": "Add more consumer instances; SQS scales horizontally without additional configuration.",
                "deeper_explanation": "Unlike Kafka partitions, SQS doesn't require pre-planned partitioning — any number of consumers can poll the same queue.",
            },
        ],
        "common_mistakes": [
            "Assuming exactly-once delivery on standard queues",
            "Not setting an appropriate visibility timeout for long-running processing",
            "Ignoring dead-letter queue monitoring",
        ],
        "architecture_questions": [
            "How would you design an event-driven pipeline using SQS and a DLQ?",
            "How would you monitor queue depth and consumer lag?",
        ],
    },
    "kafka": {
        "what_it_is": "A distributed, partitioned, replicated commit log used for high-throughput event streaming.",
        "why_it_matters": "Powers event-driven architectures needing ordered, replayable, high-volume message streams.",
        "core_concepts": [
            "topics and partitions",
            "consumer groups",
            "offsets",
            "replication",
            "at-least-once/exactly-once semantics",
        ],
        "screening_points": [
            "Explain how partitioning affects ordering guarantees.",
            "Explain how consumer groups enable parallel processing.",
        ],
        "questions": [
            {
                "question": "How does Kafka guarantee ordering?",
                "concise_answer": "Ordering is guaranteed only within a single partition, not across an entire topic.",
                "deeper_explanation": "Producers can use a partition key so related messages land on the same partition, preserving relative order for that key.",
            },
            {
                "question": "What is a consumer group?",
                "concise_answer": "A set of consumers that split up partitions of a topic to process messages in parallel.",
                "deeper_explanation": "Each partition is only consumed by one member of a group at a time, enabling horizontal scaling of consumption.",
            },
            {
                "question": "How does Kafka achieve durability?",
                "concise_answer": "Messages are replicated across multiple brokers before being acknowledged.",
                "deeper_explanation": "The replication factor controls how many broker copies exist; a leader handles reads/writes with followers replicating.",
            },
        ],
        "common_mistakes": [
            "Assuming global ordering across all partitions",
            "Not handling consumer rebalances gracefully",
        ],
        "architecture_questions": [
            "How would you design exactly-once processing with Kafka?",
            "How would you handle a slow consumer without blocking others?",
        ],
    },
    "kubernetes": {
        "what_it_is": "A container orchestration platform for deploying, scaling, and managing containerized applications.",
        "why_it_matters": "The de facto standard for running production containerized workloads at scale.",
        "core_concepts": [
            "pods",
            "deployments",
            "services",
            "namespaces",
            "declarative config (YAML manifests)",
        ],
        "screening_points": [
            "Explain the difference between a pod and a deployment.",
            "Explain how a service routes traffic to pods.",
        ],
        "questions": [
            {
                "question": "What is a pod?",
                "concise_answer": "The smallest deployable unit in Kubernetes, usually wrapping one container (sometimes more).",
                "deeper_explanation": "Pods share networking/storage between their containers and are ephemeral — Kubernetes replaces failed pods rather than restarting them in place.",
            },
            {
                "question": "How does a Deployment differ from a Pod?",
                "concise_answer": "A Deployment manages a desired set of replica Pods and handles rolling updates/rollbacks.",
                "deeper_explanation": "Deployments create and supervise ReplicaSets, which in turn ensure the specified number of Pods are running.",
            },
            {
                "question": "How does a Service expose Pods?",
                "concise_answer": "A stable virtual IP/DNS name that load-balances traffic across a dynamic set of matching Pods.",
                "deeper_explanation": "Services use label selectors to track which Pods should receive traffic, decoupling clients from ever-changing Pod IPs.",
            },
        ],
        "common_mistakes": [
            "Not setting resource requests/limits",
            "Treating Pods as long-lived, stateful entities",
        ],
        "architecture_questions": [
            "How would you roll out a zero-downtime deployment?",
            "How would you handle configuration/secrets management?",
        ],
    },
    "docker": {
        "what_it_is": "A platform for packaging applications and their dependencies into portable containers.",
        "why_it_matters": "Standardizes application packaging and deployment across environments.",
        "core_concepts": [
            "images and layers",
            "containers vs VMs",
            "Dockerfile",
            "container registries",
        ],
        "screening_points": [
            "Explain the difference between an image and a container.",
            "Explain how layer caching speeds up builds.",
        ],
        "questions": [
            {
                "question": "How does a container differ from a VM?",
                "concise_answer": "Containers share the host OS kernel and are more lightweight; VMs virtualize an entire OS.",
                "deeper_explanation": "This makes containers start faster and use fewer resources, at the cost of weaker isolation compared to VMs.",
            },
            {
                "question": "What is a Dockerfile layer?",
                "concise_answer": "Each instruction in a Dockerfile creates a cached, reusable layer in the resulting image.",
                "deeper_explanation": "Ordering instructions so rarely-changing steps come first (e.g. dependency installs) improves build cache reuse.",
            },
        ],
        "common_mistakes": [
            "Baking secrets into images",
            "Not using multi-stage builds to shrink final image size",
        ],
        "architecture_questions": ["How would you minimize image size for a production service?"],
    },
    "postgresql": {
        "what_it_is": "An open-source, ACID-compliant relational database.",
        "why_it_matters": "One of the most widely used production relational databases, known for reliability and extensibility.",
        "core_concepts": ["ACID transactions", "indexes", "query planning", "MVCC concurrency"],
        "screening_points": [
            "Explain when you'd add an index and the tradeoffs.",
            "Explain how transactions provide isolation.",
        ],
        "questions": [
            {
                "question": "How does PostgreSQL handle concurrent writes?",
                "concise_answer": "Via MVCC (multi-version concurrency control), giving readers a consistent snapshot without blocking writers.",
                "deeper_explanation": "Each transaction sees a snapshot of the data; conflicting writes are resolved at commit time rather than through read locks.",
            },
            {
                "question": "When would you add an index?",
                "concise_answer": "When a column is frequently used in WHERE/JOIN/ORDER BY clauses and the table is large enough that scans are costly.",
                "deeper_explanation": "Indexes speed up reads but add overhead to writes and storage — they should be added deliberately, not by default.",
            },
        ],
        "common_mistakes": [
            "Over-indexing write-heavy tables",
            "Not using EXPLAIN ANALYZE to diagnose slow queries",
        ],
        "architecture_questions": ["How would you shard a PostgreSQL database as it grows?"],
    },
    "dynamodb": {
        "what_it_is": "A fully managed, serverless NoSQL key-value/document database.",
        "why_it_matters": "Used for applications needing predictable low-latency access at very large scale.",
        "core_concepts": [
            "partition key / sort key design",
            "single-table design",
            "eventual vs strong consistency",
            "no joins",
        ],
        "screening_points": [
            "Explain how partition key choice affects performance.",
            "Explain the tradeoffs of single-table design.",
        ],
        "questions": [
            {
                "question": "How is DynamoDB different from a relational database?",
                "concise_answer": "It's a schema-less key-value/document store with no joins — access patterns must be designed upfront.",
                "deeper_explanation": "Unlike PostgreSQL, you model data around your query patterns first, often denormalizing into a single table.",
            },
            {
                "question": "What is single-table design?",
                "concise_answer": "Storing multiple entity types in one DynamoDB table, using composite keys to support multiple access patterns.",
                "deeper_explanation": "This avoids the need for joins (which DynamoDB doesn't support) at the cost of more complex key design upfront.",
            },
        ],
        "common_mistakes": [
            "Choosing a low-cardinality partition key, causing hot partitions",
            "Trying to model relational joins directly",
        ],
        "architecture_questions": [
            "How would you design a DynamoDB table for a multi-tenant application?"
        ],
    },
    "grpc": {
        "what_it_is": "A high-performance RPC framework using HTTP/2 and Protocol Buffers for strongly-typed service contracts.",
        "why_it_matters": "Common for low-latency internal service-to-service communication.",
        "core_concepts": [
            "protocol buffers",
            "HTTP/2 streaming",
            "strict schema contracts",
            "code generation",
        ],
        "screening_points": [
            "Explain how gRPC differs from REST.",
            "Explain what streaming RPCs enable.",
        ],
        "questions": [
            {
                "question": "Why gRPC instead of REST?",
                "concise_answer": "gRPC offers smaller/faster binary payloads, strict typed contracts, and native streaming support.",
                "deeper_explanation": "REST/JSON is simpler and more universally supported (e.g. from browsers); gRPC is better suited to internal, high-throughput service communication.",
            },
        ],
        "common_mistakes": [
            "Exposing gRPC directly to browser clients without a gateway",
            "Not versioning .proto contracts carefully",
        ],
        "architecture_questions": [
            "How would you evolve a gRPC API without breaking existing clients?"
        ],
    },
    "rest": {
        "what_it_is": "An architectural style for networked APIs built around resources and standard HTTP verbs.",
        "why_it_matters": "The most common style for public and browser-facing APIs.",
        "core_concepts": [
            "resources and URIs",
            "HTTP verbs (GET/POST/PUT/PATCH/DELETE)",
            "statelessness",
            "status codes",
        ],
        "screening_points": [
            "Explain what makes an API RESTful.",
            "Explain the difference between PUT and PATCH.",
        ],
        "questions": [
            {
                "question": "What does 'stateless' mean for REST APIs?",
                "concise_answer": "Each request contains all the information needed to process it — the server holds no client session state.",
                "deeper_explanation": "This enables horizontal scaling since any server instance can handle any request.",
            },
        ],
        "common_mistakes": [
            "Using verbs in URLs instead of resources",
            "Overusing 200 OK for error conditions",
        ],
        "architecture_questions": ["How would you version a public REST API?"],
    },
}


def lookup(skill: str) -> dict | None:
    return CONTENT_BANK.get(skill.strip().lower())
