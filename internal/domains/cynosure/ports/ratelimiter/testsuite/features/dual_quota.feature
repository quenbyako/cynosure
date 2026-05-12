Feature: Dual Quota Enforcement
  As a System Guard
  I want to enforce both message count and token limits independently

  Background:
    Given input token limit is set to 2 tokens per 10s
    And output token limit is set to 1000 tokens per 10s

  Scenario: Request is blocked by message limit
    Given user has already consumed 2 input tokens for "cheap" model
    When user consumes 1 input tokens for "cheap" model
    Then rate limit exceeded error is returned

  Scenario: Request is blocked by token limit
    Given output token limit is set to 100 tokens per 10s
    And user has already consumed 500 output tokens for "cheap" model
    When user consumes 1 input tokens for "cheap" model
    Then rate limit exceeded error is returned

  Scenario: Rollback message count when token limit is blocked
    Given output token limit is set to 100 tokens per 10s
    And user has already consumed 150 output tokens for "cheap" model
    # Initial state: 0 messages sent, 2 allowed. 150 tokens spent, 100 allowed
    # (debt=50).
    When user consumes 2 input tokens for "cheap" model
    Then rate limit exceeded error is returned

    # Wait for tokens to recover. Output recovers 50 tokens (debt becomes 0).
    # Input recovers 1 token.
    When time passes for 31s
    # If input leaked, balance is 1. If rolled back, balance is 2.
    When user consumes 2 input tokens for "cheap" model
    And user consumes 10 output tokens for "cheap" model
    Then operation is successful

    When user consumes 1 input tokens for "cheap" model
    Then rate limit exceeded error is returned
