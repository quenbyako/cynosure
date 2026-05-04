Feature: Basic Rate Limiting
  As a System Guard
  I want to limit the number of messages each user can send
  To protect the system from spam

  Background:
    Given rate limit is set to 2 message per 1s

  Scenario: Allow requests within burst limit
    Given user has already sent 1 message for "simple" model
    When user consumes 1 message for "simple" model
    Then operation is successful

  Scenario: Block requests exceeding burst limit
    Given user has already sent 2 messages for "simple" model
    When user consumes 1 message for "simple" model
    Then rate limit exceeded error is returned

  Scenario: Refill quota after time passes
    Given user has already sent 2 messages for "simple" model
    And time passes for 1s
    When user consumes 1 message for "simple" model
    Then operation is successful
