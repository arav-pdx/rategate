-- KEYS[1] = bucket key (rate:fw:{id}:{window_start})
-- ARGV[1] = limit
-- ARGV[2] = window size in milliseconds
-- ARGV[3] = cost

local t = redis.call('TIME')
local now_ms = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local limit = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local cost = tonumber(ARGV[3])

local count = redis.call('INCRBY', KEYS[1], cost)
if count == cost then
    redis.call('PEXPIRE', KEYS[1], window_ms)
end

local ttl = redis.call('PTTL', KEYS[1])
local reset_sec = math.floor((now_ms + ttl) / 1000)

if count > limit then
    return { 0, 0, reset_sec }
else
    return { 1, limit - count, reset_sec }
end
