Feature: Rate Limiting
  In order to protect the system from spam and overloads
  As a Product Owner
  I want to limit the number of messages each user can send

  Scenario: User reaches message limit
    Given rate limit is set to 1 message per second with burst 2
    And  there is a random user "User A"
    When user "User A" consumes 1 message
    Then operation is successful
    When user "User A" consumes 1 message
    Then operation is successful
    When time passes for 1s
    And  user "User A" consumes 1 message
    Then operation is successful
    When user "User A" consumes 1 message
    Then rate limit exceeded error is returned

  Scenario: Limits are independent for different users
    Given rate limit is set to 1 message per second with burst 1
    And  there is a random user "User A"
    And  there is a random user "User B"
    When user "User A" consumes 1 message
    Then operation is successful
    When user "User B" consumes 1 message
    Then operation is successful
    When user "User A" consumes 1 message
    Then rate limit exceeded error is returned
