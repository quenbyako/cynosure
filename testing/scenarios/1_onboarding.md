# Scenario 1: Headless Onboarding & Auto-Registration

**Objective**: Verify that a completely new user is silently registered and assigned a default quota upon their first interaction, without any explicit `/start` or setup steps.

**Non-Goal**: Verify that application runs locally. Manual testing is performed on real environment, everything is already pre-deployed.

## Parameters

**Никнейм аккаунта, от которого выполняются тесты:** `user54735425`
**Папка чатов:** `Testing`
**Название бота:** `Wait until I find you` (`@zhopakotabot`)
**Базовая модель:** `gemini-2.5-flash`

## Preconditions

### Testing environment

1. Запустить ory cli `ory use project`. Должен быть показан UUID проекта. Прервать тест, если этого UUID нет. Не авторизовываться в Ory Cloud вручную.
2. Проверить доступ к MCP Grafana, выполнить любой базовый запрос.
3. Проверить доступ к MCP Neon (project id: `broad-truth-84972196`). Выполнить любой базовый запрос для проверки доступности.

### Telegram Preconditions

1. Открыть Telegram Web (`https://web.telegram.org/a/`, необходимо удостовериться, что открыта версия `а`, версия `k` и `z` нестабильны)
2. Удостовериться, что произведена авторизация:
   - Виден список чатов с левой стороны, либо список чатов занимает весь экран
   - Если авторизации нет (отображается только QR код), дождаться, пока внешний пользователь не авторизуется
   - Не предпринимать никаких действий, пока пользователь не авторизуется (ожидать до появления списка чатов)
3. Удостовериться, что авторизация прошла именно под ожидаемым пользователем:
   - Нажать на бургер-меню в левом верхнем углу
   - в выпавшем списке нажать `My Profile`
   - в правом сайд меню найти поля Phone и Username
   - Проверить, что в поле Username указан `{{Test User ID}}`
4. Найти бота {{Название бота}} в поиске, в верхней части окна
5. Нажать на бота {{Название бота}}

### 2. Auth Clean Slate

**Setup**: Before starting, ensure the user does not exist in the system.

1. Получить ID пользователя: `testing/scripts/toolkit.nu get user-id {{Ниикнейм аккаунта, от которого выполняются тесты}}`
2. Если ID получен — удалить его: `ory delete identity {{ user_id полученный на предыдущем шаге }}`
   - НЕ ПРИМЕНЯТЬ `jq` ПРИ УДАЛЕНИИ АККАУНТА
3. Убедиться, что аккаунта более не существует (команда из шага 1 должна вернуть ошибку)

## Scenario

### 1. Initial Connection

**Setup**: User is NOT registered in the system (use `DELETE` query from WALKTHROUGH). Having messages in bot before DOES NOT MEAN that user registred. Registration must be checked in Ory Cloud.

**Steps**:

1. Удостовериться, что в чате есть поле текстового ввода внизу
2. Если есть текстовое поле, и предыдущая история, то:
   - явно в текстовом поле написать команду `/start` и отправить
2. Если текстового поля нет, то:
   - Нажать кнопку `Start`, которая заменяер текстовое поле
3. Получить ответ от бота с базовой информацией о боте
   - Ключевая информация: `Hi! I'm ...`, `What I can do`, `How to get started`

### 2. First interaction

**Setup**: Bot sent a first welcome message, and there is a text field to write messages

**Steps**:

1. Написать сообщение "что ты умеешь?"
2. Дождаться ответа бота
3. Получить сообщение от агента, вместо предзаготовленного приветствия.

## Expected Result

### 1. User registration check

1. Проверить, что пользователь появился в Ory Cloud:
   - Использоvaть команду `testing/scripts/toolkit.nu get user-id {{Ниикнейм аккаунта, от которого выполняются тесты}}`
   - Убедиться, что ory имеет данные об этом пользователе
2. Проверить, что в БД появился агент:
   - использовать SQL: `testing/scripts/toolkit.nu check agent {{Ниикнейм аккаунта, от которого выполняются тесты}}`
   - Убедиться, что есть ровно один агент
   - В агенте прописана {{ Базовая модель }}
   - В агенте прописан детальный system message для агента-координатора
3. Проверить, что в БД появился аккаунт Cynosure Admin MCP
   - использовать SQL `select id from agents.mcp_servers where url = '{{Admin MCP URL}}'` для получения идентификатора сервера
   - убедиться, что есть сервер
   - использовать SQL `select * from agents.mcp_accounts where server_id  = '{{mcp_server_id, полученный на предыдущем шаге}}' and user_id = '{{user_id}}'`
   - Убедиться что есть ровно один аккаунт
   - Убедиться что у аккаунта прописан верное описание

### 2. Observability Verification (Grafana Tempo)

Verify that the `ensureUser` logic was triggered and executed correctly.

1. Использовать MCP сервер grafana
2. Произвести поиск по HTTP трейсу на бота
3. Select the latest trace corresponding to your message.
4. **Verify Spans**:
    - Look for `ensureUser` span: It should show that the user was not found and then created.
    - Look for `CreateThread` span.
    - Look for `Chat` span: It should show the interaction with the LLM (Gemini).
    - Check for any `error=true` spans.
5. Удостовериться, что сигналы в трейсе понятные, порзрачные, и полностью, от и до, описывают происходящее в коде.

## Expected Result

- [ ] **User Identity (Ory Cloud)**:
      - A new identity for `@user54735425` is successfully found in Ory Cloud.
      - The identity has a valid UUID that can be used for database verification.
- [ ] **Database Integrity (NeonDB)**:
      - **Agent Settings**: Exactly one record exists in `agents.agent_settings` for the user's ID. The record correctly specifies `gemini-2.5-flash` as the model and contains a non-empty system message for the coordinator.
      - **MCP Infrastructure**: The Admin MCP server is correctly registered in `agents.mcp_servers`.
      - **User MCP Account**: A corresponding record exists in `agents.mcp_accounts` linking the user to the Admin MCP server with an accurate description.
- [ ] **Bot Functional Verification**:
      - The bot successfully processes the `/start` command and provides a standard welcome message.
      - The bot accurately responds to the inquiry "что ты умеешь?" with a description of its features and tools.
      - All interactions are reflected in the `agents_messages` table and are traceable in Grafana Tempo via the `user_id`.

## Comments

1. Нельзя трогать деплоймент, команды и инструменты Railway использовать запрещено.
