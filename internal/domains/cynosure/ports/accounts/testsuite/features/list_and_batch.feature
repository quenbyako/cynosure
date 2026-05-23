Feature: List and Batch Retrieve Accounts
  As a System Administrator
  I want to fetch accounts in bulk and lists
  To manage multiple server connections efficiently

  Scenario: List user accounts
    Given account "acc1" is saved with name "Account 1" and description "Desc 1"
    And account "acc2" is saved with name "Account 2" and description "Desc 2"
    When I list accounts
    Then the list contains "acc1"
    And the list contains "acc2"

  Scenario: Get accounts batch
    Given account "acc1" is saved with name "Account 1" and description "Desc 1"
    And account "acc2" is saved with name "Account 2" and description "Desc 2"
    When I get accounts batch for "acc1, acc2, nonexistent"
    Then the batch contains "acc1"
    And the batch contains "acc2"
    And the batch does not contain "nonexistent"
