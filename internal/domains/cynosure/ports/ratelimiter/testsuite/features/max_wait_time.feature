Feature: Maximum Wait Time
  As a Product Owner
  I want to limit the maximum wait time
  To prevent permanent user churn due to massive usage

  Scenario: Wait time is capped even with high usage
    Given output token limit is set to 100 tokens per 1h
    And maximum wait time is set to 4h
    And user has already consumed 10000 output tokens for "pro" model
    When user consumes 1 input tokens for "pro" model
    Then rate limit exceeded error is returned
    And retry is after 4h
