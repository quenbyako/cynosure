Feature: Basic Rate Limiting
  As a System Guard
  I want to limit the number of messages each user can send
  To protect the system from spam

  Background:
    Given input token limit is set to 2 tokens per 1s

  Scenario: Allow requests within burst limit
    Given user has already consumed 1 input tokens for "simple" model
    When user consumes 1 input tokens for "simple" model
    Then operation is successful

  Scenario: Block requests exceeding burst limit
    Given user has already consumed 2 input tokens for "simple" model
    When user consumes 1 input tokens for "simple" model
    Then rate limit exceeded error is returned

  Scenario: Refill quota after time passes
    Given user has already consumed 2 input tokens for "simple" model
    When time passes for 1s
    And user consumes 1 input tokens for "simple" model
    Then operation is successful
