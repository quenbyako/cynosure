-- Lua script for Leaky Bucket rate limiting with debt support and penalty capping.
-- Based on: https://redis.io/tutorials/howtos/ratelimiting/ (Policing Mode)
-- Keys: [level_key, last_leak_key]
-- Args: [amount, rate, period_ms, capacity, max_wait_ms, now_ms, force_consume]
-- Returns: [allowed: int (0 | 1), retry_after_ms: int]

local level_key = KEYS[1]
local last_leak_key = KEYS[2]

local amount = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local period = tonumber(ARGV[3])
local capacity = tonumber(ARGV[4])
local max_wait = tonumber(ARGV[5])
local now = tonumber(ARGV[6])
local force = tonumber(ARGV[7]) == 1

local level = tonumber(redis.call("GET", level_key)) or 0
local last_leak = tonumber(redis.call("GET", last_leak_key)) or now

local elapsed = math.max(0, now - last_leak)
local leak_rate = rate / period
local leaked = elapsed * leak_rate

-- Calculate new level after leaking
local new_level = math.max(0, level - leaked)

-- Check if we can allow this request
local retry_after = 0
if new_level + amount > capacity then
    retry_after = math.ceil((new_level + amount - capacity) / leak_rate)
end

if not force and max_wait >= 0 then
    if (max_wait == 0 and retry_after > 0) or
       (max_wait > 0 and retry_after > max_wait) then
        return {0, retry_after}
    end
end

-- Consume tokens (increase level)
new_level = new_level + amount

-- Penalty Ceiling: cap the debt so the user can recover within max_wait
if max_wait > 0 and retry_after > max_wait then
    new_level = capacity + (max_wait * leak_rate)
    retry_after = max_wait
end

redis.call("SET", level_key, new_level)
redis.call("SET", last_leak_key, now)

-- TTL: enough time to fully drain the bucket (plus a safety hour)
local debt = math.max(0, new_level - capacity)
local expire = math.floor((capacity + debt) / leak_rate / 1000) + 3600
redis.call("EXPIRE", level_key, expire)
redis.call("EXPIRE", last_leak_key, expire)

return {1, retry_after}
