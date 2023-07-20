create table histories
(
    id            bigserial primary key,
    created_at    timestamp with time zone,
    updated_at    timestamp with time zone,
    deleted_at    timestamp with time zone,
    user_id       text,
    message       text,
    push_response text
);

create index idx_histories_deleted_at on histories (deleted_at);
