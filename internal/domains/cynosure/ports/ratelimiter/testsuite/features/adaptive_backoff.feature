Feature: Adaptive Backoff (Token Management)
  As a System Accountant
  I want to allow users to finish their request even if it exceeds the token limit
  But block them until their balance recovers

  Background:
    Given token limit is set to 100 tokens per 1h

  Scenario: User consumes tokens within the limit
    When user consumes 1 message for "cheap" model
    And user settles 50 tokens for "cheap" model
    Then operation is successful

  Scenario: User is blocked after consuming too many tokens
    Given user has already spent 500 tokens for "cheap" model
    When user consumes 1 message for "cheap" model
    Then rate limit exceeded error is returned
    And retry is after 4h
