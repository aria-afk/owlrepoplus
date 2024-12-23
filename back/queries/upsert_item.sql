insert into items (id, p0, p25, p50, p75, p100, mean, std, nowls)
values            ($1, $2, $3,  $4,  $5,  $6,   $7,   $8,  $9)
on conflict (id) do update 
set p0 = $2, p25 = $3, p50 = $4, p75 = $5, p100 = $6, mean = $7, std = $8, nowls = $9;

