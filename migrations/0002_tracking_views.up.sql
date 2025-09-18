create table workers (
    id               serial      primary key,
    created_at        timestamptz not null    default now(),
    name              text        unique not null,
    type              text        not null,
    status            text        default 'online',
    last_seen        timestamptz default current_timestamp,
    orders_processed  integer     default 0
);