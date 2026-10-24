-- KEYS[1] = bucket base key
-- ARGV[1] = limit
-- ARGV[2] = window size in milliseconds
-- ARGV[3] = cost

local t = redis.call('TIME')
local now_ms = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local limit = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local cost = tonumber(ARGV[3])

local current_window_idx = math.floor(now_ms / window_ms)
local prev_window_idx = current_window_idx - 1

local curr_key = KEYS[1] .. ':' .. current_window_idx
local prev_key = KEYS[1] .. ':' .. prev_window_idx

local curr_count = tonumber(redis.call('GET', curr_key) or '0')
local prev_count = tonumber(redis.call('GET', prev_key) or '0')

local time_into_current = now_ms % window_ms
local prev_weight = 1.0 - (time_into_current / window_ms)
local estimated_count = math.floor(prev_count * prev_weight + curr_count)

local reset_sec = math.floor((now_ms + (window_ms - time_into_current)) / 1000)

if estimated_count + cost > limit then
    return { 0, math.max(0, limit - estimated_count), reset_sec }
end

redis.call('INCRBY', curr_key, cost)
redis.call('PEXPIRE', curr_key, window_ms * 2)

return { 1, math.max(0, limit - (estimated_count + cost)), reset_sec }
