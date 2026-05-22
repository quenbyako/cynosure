Feature: Save and Retrieve Accounts
  As a System Administrator
  I want to persist and retrieve Cynosure accounts
  To verify basic CRUD storage capabilities

  Scenario: Save and retrieve a new account
    When I save account "acc1" with name "Production Server" and description "Main production tools"
    Then the operation is successful
    And I can get account "acc1" details
    And the retrieved account has name "Production Server" and description "Main production tools"

  Scenario: Get account not found
    When I get account "nonexistent"
    Then a not found error is returned

  Scenario: Get account with invalid account ID
    When I get account with invalid account ID
    Then an invalid account ID error is returned
