alter table histories
    add hash text;

update histories
set hash = md5(user_id || created_at::text)
where 1 = 1;

create unique index idx_histories_hash
    on histories (hash);
