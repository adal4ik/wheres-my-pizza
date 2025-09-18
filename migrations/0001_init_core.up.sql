create table orders (
    id                serial        primary key,
    created_at       timestamptz   not null    default now(),
    updated_at       timestamptz   not null    default now(),
    number            text          unique not null,
    customer_name     text          not null,
    type              text          not null check (type in ('dine_in', 'takeout', 'delivery')),
    table_number      integer,
    delivery_address  text,
    total_amount      decimal(10,2) not null,
    priority         integer       default 1,
    status            text          default 'received',
    processed_by     text,
    completed_at      timestamptz
);


create table order_items (
    id          serial        primary key,
    created_at  timestamptz   not null    default now(),
    order_id    integer       references orders(id),
    name        text          not null,
    quantity    integer       not null,
    price       decimal(8,2)  not null
);

create table order_status_log (
    id         serial        primary key,
    created_at  timestamptz   not null    default now(),
    order_id    integer       references orders(id),
    status     text,
    changed_by  text,
    changed_at  timestamptz   default current_timestamp,
    notes       text
);