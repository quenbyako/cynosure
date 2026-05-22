Feature: Search Accounts
  As a System Administrator
  I want to search accounts by name
  To find existing integrations quickly

  Scenario: Find accounts by name
    Given account "acc3" is saved with name "Production Server" and description "Desc 3"
    And account "acc3" is deleted
    And account "acc1" is saved with name "Production Server" and description "Desc 1"
    And account "acc2" is saved with name "Staging Server" and description "Desc 2"
    When I find accounts by name "Production Server"
    Then the search results contain active account "acc1"
    And the search results contain deleted account "acc3"
    And the search results do not contain "acc2"
