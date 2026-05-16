#!/usr/bin/env nu

# Cynosure Testing Toolkit
# Usage: nu testing/scripts/toolkit.nu <command> [args]

def main []: nothing -> nothing {
    help main
}

# Get User ID from Ory
def "main get user-id" [username: string]: nothing -> string {
    let output = (ory list identities --format json | from json)
    let identity = ($output.identities | where traits.username == $username | first)
    if ($identity == null) {
        error make {msg: $"User ($username) not found"}
    }
    $identity.id
}

# Generate SQL to switch user plan and reset buckets
def "main set plan" [username: string, plan_key: string]: nothing -> string {
    let id = (main get user-id $username)

    let line1 = $"-- Switch ($username) (($id)) to ($plan_key)"
    let line2 = $"DELETE FROM agents.rate_limit_buckets WHERE user_id = '($id)';"
    let line3 = "INSERT INTO agents.user_plans (user_id, plan_id)"
    let line4 = $"SELECT '($id)', id FROM agents.plans WHERE plan_key = '($plan_key)'"
    let line5 = "ON CONFLICT (user_id) DO UPDATE SET plan_id = EXCLUDED.plan_id;"

    ([$line1, $line2, $line3, $line4, $line5] | str join "\n")
}

# Generate SQL to clear user state for onboarding
def "main prepare onboarding" [username: string]: nothing -> string {
    let id = (main get user-id $username)

    let line1 = $"-- Preparation for ($username) (($id))"
    let line2 = $"DELETE FROM agents.rate_limit_buckets WHERE user_id = '($id)';"
    let line3 = $"DELETE FROM agents.user_plans WHERE user_id = '($id)';"
    let line4 = $"DELETE FROM agents.mcp_accounts WHERE user_id = '($id)';"
    let line5 = $"DELETE FROM agents.agent_settings WHERE user_id = '($id)';"

    ([$line1, $line2, $line3, $line4, $line5] | str join "\n")
}

# Generate SQL to check current buckets
def "main check buckets" [username: string]: nothing -> string {
    let id = (main get user-id $username)
    $"SELECT resource_type, level, last_leak_at FROM agents.rate_limit_buckets WHERE user_id = '($id)';"
}

# Generate SQL to check agent
def "main check agent" [username: string]: nothing -> string {
    let id = (main get user-id $username)
    $"SELECT model, system_message FROM agents.agent_settings WHERE user_id = '($id)';"
}

# Generate SQL to list plans
def "main list plans" []: nothing -> string {
    "SELECT id, plan_key, chat_input_limit, chat_output_limit, max_await_period FROM agents.plans ORDER BY chat_input_limit ASC;"
}
