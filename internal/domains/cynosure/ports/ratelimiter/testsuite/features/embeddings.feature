Feature: Embedding Rate Limiting
  As a System Guard
  I want to limit the number of embedding tokens each user can consume
  To protect the system from resource exhaustion

  Background:
    Given embedding token limit is set to 1000 tokens per 1h

  Scenario: Allow embedding requests within the limit
    When user consumes 500 embedding tokens for "small" model
    Then operation is successful

  Scenario: Block embedding requests exceeding the limit
    When user consumes 1500 embedding tokens for "small" model
    Then rate limit exceeded error is returned

  Scenario: Combined chat and embedding usage
    Given input token limit is set to 5 tokens per 1m
    And output token limit is set to 100 tokens per 1m
    When user consumes 1 input tokens for "cheap" model
    And user consumes 50 output tokens for "cheap" model
    And user consumes 500 embedding tokens for "small" model
    Then operation is successful
