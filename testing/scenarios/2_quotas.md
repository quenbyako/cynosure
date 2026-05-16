# Scenario: Quotas & Rate Limiting

**Objective**: Validate that the system correctly enforces token-based quotas, provides helpful "retry-after" messages, and respects plan upgrades.

**Non-Goal**:

1. This is not a test of Telegram's internal rate limits (FloodWait) or deep architectural performance of the leaky bucket algorithm.
2. This is not a test of global rate limits for Paid APIs (like Gemini, Ory, and other)

## Parameters

- **Test User ID (TG):** `user54735425`
- **Test User UUID (DB):** Obtain by `nu testing/scripts/toolkit.nu get user-id user54735425`
- **Папка чатов:** `Testing`
- **Название бота:** `Wait until I find you` (`@zhopakotabot`)

## Preconditions

### Testing environment

1. Проверить доступ к MCP Grafana, выполнить любой базовый запрос.
2. Проверить доступ к MCP Neon (project id: `broad-truth-84972196`). Выполнить любой базовый запрос для проверки доступности.

### Plan Setup

Удостовериться, что существуют тестовые планы:

1. plan_key `dummy`
   - input/output лимит: по 200 токенов за 1 день (специально не хватит даже на системный промпт, не говоря о сообщениях)
   - max_await_period: 0 секунд (ожидание может быть до бесконечности)
   - agents_limit: 1
   - mcp_accounts_limit: 1
2. plan_key `cheap`
   - input/output лимит: по 5000 токенов за 1 день
   - max_await_period: 24 часа
   - agents_limit: 10
   - mcp_accounts_limit: 5
2. plan_key `rich`
   - input/output лимит: по 100000 токенов
   - max_await_period: 6 часов
   - agents_limit: 10
   - mcp_accounts_limit: 5

Команда для получения всех доступных планов: `testing/scripts/toolkit.nu list plans`

### User Initialization

1. Сбросить лимиты и установить план: `testing/scripts/toolkit.nu set plan user54735425 <plan_key>`

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

## Scenario

### 1. Hard Block Verification (Dummy Plan)

**Setup**: Выполнить `nu testing/scripts/toolkit.nu set plan user54735425 dummy`

**Steps**:

1. В Telegram написать боту любое короткое сообщение (например, "Привет").
2. Получить мгновенный ответ с ошибкой превышения лимита.
3. Убедиться, что сообщение содержит информацию о возможности повышения лимита через `/premium`.

### 2. Soft Block & Debt Verification (Cheap Plan)

**Setup**: Выполнить `nu testing/scripts/toolkit.nu set plan user54735425 cheap`

**Steps**:

1. Написать боту запрос на генерацию объемного контента: "Напиши подробную статью об истории развития AI на 500 слов".
2. Дождаться полного ответа от бота (запрос должен пройти, так как долг разрешен).
3. Сразу после получения ответа отправить сообщение "Продолжай".
4. Получить мгновенный ответ с ошибкой превышения лимита.
5. Убедиться, что сообщение содержит информацию о возможности повышения лимита через `/premium`.

### 3. Safety Ceiling Verification (Hard Debt)

**Setup**: Выполнить `nu testing/scripts/toolkit.nu set plan user54735425 cheap`

**Steps**:

1. Написать боту любой запрос вне зависимости от ожидаемого размера
1. В БД вручную установить экстремальный уровень потребления: `UPDATE agents.rate_limit_buckets SET level = 1000000 WHERE user_id = '{{ Test User UUID }}'`.
2. Написать боту любое сообщение.
3. Получить ответ с ошибкой 429, несмотря на "крутой" тариф.

## Expected Result

### 1. Quota Enforcement Logic

1. Проверить состояние бакетов в БД после Phase 1:
   - использовать SQL: `nu testing/scripts/toolkit.nu check buckets user54735425`
   - Убедиться, что `level` равен 0 (запрос был отклонен до списания токенов).
2. Проверить состояние бакетов после Phase 2:
   - Убедиться, что `level` значительно превышает `limit` (подтверждение работы механики долга).
3. Проверить логику сброса в Phase 3:
   - Убедиться, что запрос прошел без ручной очистки бакетов, только за счет смены `plan_id`.

### 2. Observability Verification (Grafana Tempo)

1. Найти трейс отклоненного запроса из Phase 1:
   - Спан `ChatModel.stream` (или его родитель) должен содержать ошибку.
   - В атрибутах спана или в логах должна быть информация о `RateLimitExceededError`.
2. Найти трейс успешного запроса "в долг" из Phase 2:
   - Проверить спан `requesting chat quota`: он должен быть успешным.
   - Сразу за ним в следующем запросе этот же спан должен вернуть ошибку.

### 3. Final Checklist

- [ ] **User Experience**:
      - Сообщения о лимитах содержат человекочитаемое время ожидания (например, "через 15 минут").
      - В каждом сообщении об ошибке есть призыв перейти на `/premium`.
      - Ошибки приходят быстро (latency < 500ms для pre-flight проверок).
- [ ] **Database & Logic**:
      - Тарифы переключаются "на лету" и учитываются при следующем же входящем сообщении.
      - Механика `max_await_period` корректно ограничивает максимальный "долг" пользователя.
      - Бакеты корректно инициализируются и обновляются в схеме `agents`.
- [ ] **Observability**:
      - Все случаи Rate Limit логируются в Tempo.
      - В ошибках присутствует Trace ID для возможности быстрой диагностики.

## Comments

1. Лимиты в коде завязаны на **токены**, а не на сообщения. 1 сообщение ≈ системный промпт + история + новый ввод.
2. `max_await_period` определяет глубину "кредитного плеча" пользователя (debt limit).
3. Trace ID в сообщениях бота критически важен для отладки конкретных отказов в Tempo.
