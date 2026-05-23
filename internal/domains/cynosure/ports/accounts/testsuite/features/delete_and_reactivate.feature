Feature: Delete and Reactivate Accounts
  As a System Administrator
  I want to delete and restore accounts
  To manage connection lifecycle

  Scenario: Soft-delete an account
    Given account "acc1" is saved with name "Account 1" and description "Desc 1"
    When I delete account "acc1"
    Then the operation is successful
    When I get account "acc1"
    Then a not found error is returned
    When I list accounts
    Then the list does not contain "acc1"

  Scenario: Retrieve soft-deleted account with option
    Given account "acc1" is saved with name "Account 1" and description "Desc 1"
    And account "acc1" is deleted
    When I get account "acc1" including deleted
    Then the operation is successful
    And the retrieved account has name "Account 1" and description "Desc 1"

  Scenario: Reactivate a soft-deleted account
    Given account "acc1" is saved with name "Account 1" and description "Desc 1"
    And account "acc1" is deleted
    When I reactivate account "acc1"
    Then the operation is successful
    And I can get account "acc1" details
