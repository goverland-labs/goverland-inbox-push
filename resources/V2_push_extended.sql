alter table histories
    add clicked_at timestamp;

alter table histories
    alter column message type jsonb using message::jsonb;

create index histories_id_message_id
    on histories ((message ->> 'id'));

create table send_queue
(
    id          bigserial
        primary key,
    created_at  timestamp with time zone,
    updated_at  timestamp with time zone,
    deleted_at  timestamp with time zone,
    user_id     text,
    dao_id      text,
    proposal_id text,
    action      text,
    sent_at     timestamp with time zone
);

create index idx_send_queue_deleted_at
    on send_queue (deleted_at);

create unique index idx_send_queue_user_dao_proposal_action
    on send_queue (user_id, dao_id, proposal_id, action);
