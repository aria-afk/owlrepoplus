-- TODO: Remove id not exists when done testing

-- Item meta data (not time related)
create table if not exists items (
    id text primary key,
    p0 int not null,
    p25 int not null,
    p50 int not null,
    p75 int not null,
    p100 int not null,
    mean int not null,
    std int not null,
    nowls int not null
);

-- Item time table
-- 1(items) -> Many(item_hist)
create table if not exists item_hist (
    uuid uuid primary key default gen_random_uuid(),
    item_id text references items (id),
    time timestamp not null
);

-- Related entries for a time series entry
-- 1(item_hist) -> Many(item_hist_entries)
create table if not exists item_hist_entries (
    uuid uuid primary key default gen_random_uuid(),
    item_hist_uuid uuid references item_hist (uuid),
    owner text not null,
    store_name text not null,
    bundle int not null,
    price int not null,
    quantity int not null
);
