Feature: Maximum Wait Time
  As a Product Owner
  I want to limit the maximum wait time
  To prevent permanent user churn due to massive usage

  Scenario: Wait time is capped even with high usage
    Given input token limit is set to 10 tokens per 5s
    And output token limit is set to 100 tokens per 5s
    And maximum wait time is set to 10s
    And user has already consumed 10000 output tokens for "pro" model
    When user consumes 1 input tokens for "pro" model
    Then rate limit exceeded error is returned
    And retry is after 10s

    When time passes for 10s
    And user consumes 1 input tokens for "pro" model
    Then operation is successful

    # locking for the time when user shouldn't wait
    When user consumes 99999 output tokens for "pro" model
    Then operation is successful

    When user consumes 1 input tokens for "pro" model
    Then rate limit exceeded error is returned
    And retry is after 10s
