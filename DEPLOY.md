# 🚀 Инструкция по развертыванию xFreeHack Bot

Эта инструкция поможет вам развернуть бота на чистом сервере (VPS/VDS).

## 🛠 Предварительные требования
Для работы приложения необходимы:
* **Docker**
* **Docker Compose**
* **Git** (для клонирования репозитория)

## 📋 Шаг 1: Подготовка сервера (Ubuntu/Debian)

Если у вас чистый сервер, выполните следующие команды для установки Docker и Git:

1. **Обновите пакеты:**
   ```bash
   sudo apt update && sudo apt upgrade -y
   ```

2. **Установите Git и необходимые утилиты:**
   ```bash
   sudo apt install -y git curl
   ```

3. **Установите Docker:**
   ```bash
   curl -fsSL https://get.docker.com -o get-docker.sh
   sudo sh get-docker.sh
   ```

4. **Установите Docker Compose (если не установился с Docker):**
   ```bash
   sudo apt install -y docker-compose-plugin
   # Проверка
   docker compose version
   ```

## 📥 Шаг 2: Установка приложения

1. **Клонируйте репозиторий:**
   ```bash
   git clone https://github.com/wenkaler/xfreehack.git
   cd xfreehack
   ```

2. **Настройте окружение:**
   Скопируйте пример файла конфигурации:
   ```bash
   cp .env.example .env
   ```

3. **Отредактируйте .env:**
   Откройте файл любым редактором (например, `nano`):
   ```bash
   nano .env
   ```
   **Обязательно заполните:**
   * `TELEGRAM_TOKEN`: Токен вашего бота (из @BotFather).
   * `ACCESS_TOKEN`: Секретный токен для админ-команд (придумайте любой).
   * `POSTGRES_USER`: Имя пользователя БД (по умолчанию `postgres`).
   * `POSTGRES_PASSWORD`: Пароль БД (придумайте надежный).
   * `POSTGRES_DB`: Имя базы данных (по умолчанию `xfreehack`).

## 🚀 Шаг 3: Запуск

Запустите приложение в фоновом режиме:

```bash
docker compose up -d --build
```

Бот начнет работать, автоматически создаст базу данных и применит необходимые таблицы.

## 📦 Шаг 4: Миграция данных (Опционально)

Если переезжаете с SQLite или старой версии:

1. **Если новая база PostgreSQL чистая**: Перезапуск `docker compose` автоматически применит init-скрипты.
2. **Обновление структуры (если база уже существует)**:
   ```bash
   cat migration/alter_chats.sql | docker exec -i xfreehack-db-1 psql -U postgres -d xfreehack
   ```
   *(Замените `xfreehack-db-1` на имя вашего контейнера БД)*

## 🛠 Обслуживание

* **Просмотр логов:**
  ```bash
  docker compose logs -f app
  ```

* **Перезапуск бота:**
  ```bash
  docker compose restart app
  ```

* **Полное обновление (с пересборкой):**
  ```bash
  git pull
  docker compose up -d --build
  ```

## ❓ Проверка состояния сервера
Если вы не уверены, что установлено на сервере, запустите эти команды:

```bash
echo "--- OS Info ---"
cat /etc/os-release
echo -e "\n--- Docker ---"
docker --version || echo "Docker not installed"
echo -e "\n--- Git ---"
git --version || echo "Git not installed"
echo -e "\n--- Memory ---"
free -h
echo -e "\n--- Disk ---"
df -h /
```
