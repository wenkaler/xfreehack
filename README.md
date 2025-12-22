# xFreeHack Service

Сервис для автоматического сбора бесплатных купонов и промокодов с сайта [lovikod.ru](https://lovikod.ru) и публикации их в Telegram.

## Функциональность
-   **Автоматический сбор**: Парсинг категорий, магазинов и купонов.
-   **PostgreSQL**: Надежное хранение данных.
-   **Telegram Бот**: Рассылка новых купонов подписчикам ежедневно в 18:00 (МСК) или по команде.

## Команды бота
-   `/start` - Регистрация в боте и получение приветственного сообщения.
-   `/print [N]` - Получить `N` свежих купонов (по умолчанию 5).
-   `/stat [token]` - (Админ) Получить статистику активных пользователей (требуется `access_token`).

## Запуск

### Требования
-   Docker & Docker Compose

### Быстрый старт
1.  Создайте файл `config.yaml` на основе примера ниже или используйте переменные окружения.
2.  Запустите сервис:
    ```bash
    docker-compose up -d
    ```

## Конфигурация (`config.yaml`)

```yaml
service_name: "xFreeService"
time_to_send: "18:00"
access_token: "secret_access_token"

db:
  host: "xfree-db"
  port: "5432"
  user: "postgres"
  password: "password"
  name: "xfreehack"
  ssl_mode: "disable"

telegram:
  token: "YOUR_TELEGRAM_BOT_TOKEN"
  update_time: 60
```

## Разработка

#### Структура проекта
-   `cmd/` — Точка входа в приложение.
-   `collector/` — Логика парсинга сайта.
-   `storage/` — Взаимодействие с базой данных (PostgreSQL).
-   `snbot/` — Логика Telegram бота.
-   `model/` — Структуры данных (Models).
-   `migration/` — SQL скрипты инициализации БД.

#### Команды
-   Сборка: `go build -o xfree ./cmd/main.go`
-   Запуск: `./xfree`
