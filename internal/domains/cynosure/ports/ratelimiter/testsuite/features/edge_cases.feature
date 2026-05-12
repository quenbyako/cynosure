Feature: Edge Case Rate Limiting
  As a System Guard
  I want to handle unusual scenarios correctly
  To ensure robustness of the rate limiting system

  Background:
    Given input token limit is set to 10 tokens per 10s
    And output token limit is set to 100 tokens per 10s

  Scenario: Zero limit strictly blocks consumption
    Given input token limit is set to 0 tokens per 10s
    When user consumes 1 input tokens for "cheap" model
    Then rate limit exceeded error is returned

  Scenario: Zero limit allows 0 tokens consumption
    Given input token limit is set to 0 tokens per 10s
    When user consumes 0 input tokens for "cheap" model
    Then operation is successful

  Scenario: Input tokens are always a hard limit regardless of maxWait
    Given input token limit is set to 10 tokens per 10s
    And maximum wait time is set to 30s
    And user has already consumed 10 input tokens for "cheap" model
    When user consumes 1 input tokens for "cheap" model
    Then rate limit exceeded error is returned

  Scenario: Embedding tokens are always a hard limit regardless of maxWait
    Given embedding token limit is set to 100 tokens per 10s
    And maximum wait time is set to 30s
    And user has already consumed 100 embedding tokens for "cheap" model
    When user consumes 1 embedding tokens for "cheap" model
    Then rate limit exceeded error is returned

  Scenario: Hard limit (maxWait=0) does not cap debt for output tokens
    Given output token limit is set to 1 tokens per 100h
    And maximum wait time is set to 0s
    And user has already consumed 10001 output tokens for "cheap" model
    When user consumes 1 input tokens for "cheap" model
    Then rate limit exceeded error is returned
    And retry is after 1000000h
