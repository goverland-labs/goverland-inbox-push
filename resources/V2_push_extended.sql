alter table histories
    add clicked_at timestamp;

alter table histories
    alter column message type jsonb using message::jsonb;

create index histories_id_message_id
    on histories ((message->>'id'));
