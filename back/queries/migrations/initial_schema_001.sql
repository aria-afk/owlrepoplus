create table if not exists items (
    id text primary key
);

-- TODO: Trigger for on insert only keep 14 most recent entries
create table if not exists item_meta (
    time timestamp primary key,
    item_id text references items(id),
    min_price float not null,
    p25_price float not null,
    p50_price float not null,
    p75_price float not null,
    max_price float not null
);

-- This table is meant to service the most recent prices and locations 
-- since after 24hrs the location is stale and its nice to have a list
-- of prices since somone putting a 1 meso item messes the avg
create table if not exists item_meta_recents (
    uuid uuid primary key default gen_random_uuid(),
    time timestamp not null,
    item_id text references items(id),
    owner text not null,
    store_name text not null,
    bundle int not null,
    price float not null,
    quantity int not null
);

create function delete_old_item_meta() returns trigger as $delete_old_items_meta$
declare 
r record;
i int;
begin
    i := 0;
    for r in select * from item_meta order by time desc loop
        if (i < 14) then 
            i = i + 1;
        else
            delete from item_meta where time = r.timestamp;
        end if;
    end loop;
end; $delete_old_items_meta$ language plpgsql;

create trigger delete_old_items_meta after insert or update on item_meta 
for each statement execute function delete_old_item_meta();

create function delete_old_item_meta_recents() returns trigger as $delete_old_item_meta_recents$
declare 
r record;
begin
    select * from item_meta_recents order by time desc limit 1 into r;
    execute delete from item_meta_recents where time != r.time;
end; $delete_old_item_meta_recents$ language plpgsql;

create trigger delete_old_item_meta_recents after insert or update on item_meta_recents
for each statement execute function delete_old_item_meta_recents();
