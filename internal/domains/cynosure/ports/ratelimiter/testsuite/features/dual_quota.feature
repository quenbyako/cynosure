Feature: Dual Quota Enforcement
  As a System Guard
  I want to enforce both message count and token limits independently

  Background:
    Given rate limit is set to 2 messages per 1h
    And token limit is set to 10000 tokens per 1h

  Scenario: Request is blocked by message limit
    Given user has already sent 2 messages for "cheap" model
    When user consumes 1 message for "cheap" model
    Then rate limit exceeded error is returned

  Scenario: Request is blocked by token limit
    Given token limit is set to 100 tokens per 1h
    And user has already spent 500 tokens for "cheap" model
    When user consumes 1 message for "cheap" model
    Then rate limit exceeded error is returned
