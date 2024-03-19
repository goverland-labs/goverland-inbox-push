alter table histories
    alter column clicked_at set default null;

update histories set clicked_at = null
where clicked_at <= now() - INTERVAL '3 month';
