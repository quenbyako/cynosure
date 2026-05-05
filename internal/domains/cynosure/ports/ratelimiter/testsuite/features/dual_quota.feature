Feature: Dual Quota Enforcement
  As a System Guard
  I want to enforce both message count and token limits independently

  Background:
    Given input token limit is set to 2 tokens per 1h
    And output token limit is set to 10000 tokens per 1h

  Scenario: Request is blocked by message limit
    Given user has already consumed 2 input tokens for "cheap" model
    When user consumes 1 input tokens for "cheap" model
    Then rate limit exceeded error is returned

  Scenario: Request is blocked by token limit
    Given output token limit is set to 100 tokens per 1h
    And user has already consumed 500 output tokens for "cheap" model
    When user consumes 1 input tokens for "cheap" model
    Then rate limit exceeded error is returned

  Scenario: Rollback message count when token limit is blocked
    Given output token limit is set to 100 tokens per 1h
    And user has already consumed 500 output tokens for "cheap" model
    # Initial state: 0 messages sent, 2 allowed. 500 tokens spent, 100 allowed (debt).
    When user consumes 1 input tokens for "cheap" model
    Then rate limit exceeded error is returned

    # Wait for tokens to recover. Message count should NOT have been incremented.
    When time passes for 10h
    # Now we should be able to send 2 messages.
    When user consumes 1 input tokens for "cheap" model
    And user consumes 10 output tokens for "cheap" model
    Then operation is successful

    When user consumes 1 input tokens for "cheap" model
    And user consumes 10 output tokens for "cheap" model
    Then operation is successful

    When user consumes 1 input tokens for "cheap" model
    Then rate limit exceeded error is returned
