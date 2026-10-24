-- KEYS[1] = bucket key
-- ARGV[1] = capacity
-- ARGV[2] = refill rate per second
-- ARGV[3] = cost

local t = redis.call('TIME')
local now_ms = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)

local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local cost = tonumber(ARGV[3])

local data = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(data[1])
local last_ts = tonumber(data[2])

if tokens == nil then
    tokens = capacity
    last_ts = now_ms
else
    local elapsed_sec = math.max(0, (now_ms - last_ts) / 1000)
    tokens = math.min(capacity, tokens + elapsed_sec * refill_rate)
end

local allowed = 0
local retry_after = 0

if tokens >= cost then
    tokens = tokens - cost
    allowed = 1
else
    local needed = cost - tokens
    retry_after = math.ceil(needed / refill_rate)
end

redis.call('HSET', KEYS[1], 'tokens', tokens, 'ts', now_ms)
local ttl_ms = math.ceil((capacity / refill_rate) * 2 * 1000)
redis.call('PEXPIRE', KEYS[1], math.max(ttl_ms, 60000))

local reset_sec = math.floor(now_ms / 1000) + retry_after
return { allowed, math.floor(tokens), reset_sec }
