-- KEYS[1] = bucket key
-- ARGV[1] = limit
-- ARGV[2] = window size in milliseconds
-- ARGV[3] = cost

local t = redis.call('TIME')
local now_ms = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local limit = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local cost = tonumber(ARGV[3])
local clear_before = now_ms - window_ms

redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', clear_before)

local current_count = redis.call('ZCARD', KEYS[1])
local reset_sec = math.floor((now_ms + window_ms) / 1000)

if current_count + cost > limit then
    return { 0, math.max(0, limit - current_count), reset_sec }
end

for i = 1, cost do
    local member = tostring(now_ms) .. ':' .. tostring(t[2]) .. ':' .. tostring(i)
    redis.call('ZADD', KEYS[1], now_ms, member)
end

redis.call('PEXPIRE', KEYS[1], window_ms)
return { 1, limit - (current_count + cost), reset_sec }
