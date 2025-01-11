# Go Web Application / Веб-приложение на Go

This is a web application built with Go, featuring user authentication, messaging, notifications, and profile management.

Это веб-приложение, построенное на Go, с функциями аутентификации пользователей, обмена сообщениями, уведомлений и управления профилем.

## Project Structure / Структура проекта

```
/e:/src/go/
├── handlers.go         # HTTP handlers for various routes / HTTP обработчики для различных маршрутов
├── main.go             # Main entry point of the application / Главная точка входа в приложение
├── models.go           # Data models and related functions / Модели данных и связанные функции
├── notifications.go    # Notification service implementation / Реализация сервиса уведомлений
├── templates/          # HTML templates for the web pages / HTML шаблоны для веб-страниц
│   ├── home.html
│   ├── login.html
│   ├── messages.html
│   ├── profile.html
│   ├── register.html
├── static/             # Static files (CSS, JS, images) / Статические файлы (CSS, JS, изображения)
│   ├── style.css
│   ├── notification.mp3
└── data/               # Data storage (users, messages) / Хранилище данных (пользователи, сообщения)
    ├── users.json
    ├── messages.json
```

## Features / Функции

- **User Authentication**: Register, login, and logout functionality.
- **Аутентификация пользователей**: Регистрация, вход и выход из системы.
- **Messaging**: Send and receive messages, including group messages.
- **Обмен сообщениями**: Отправка и получение сообщений, включая групповые сообщения.
- **Notifications**: Real-time notifications for new messages.
- **Уведомления**: Уведомления в реальном времени о новых сообщениях.
- **Profile Management**: Update profile information and avatar.
- **Управление профилем**: Обновление информации профиля и аватара.
- **File Uploads**: Attach files to messages.
- **Загрузка файлов**: Прикрепление файлов к сообщениям.
- **Markdown Support**: Use Markdown for message content.
- **Поддержка Markdown**: Использование Markdown для содержания сообщений.
- **Message Reactions**: React to messages with emojis.
- **Реакции на сообщения**: Реакции на сообщения с помощью эмодзи.
- **Message Logs**: View logs of message actions (create, edit, delete, react).
- **Журналы сообщений**: Просмотр журналов действий с сообщениями (создание, редактирование, удаление, реакция).

## Setup / Настройка

1. **Clone the repository / Клонируйте репозиторий**:
    ```sh
    git clone https://github.com/yourusername/go-web-app.git
    cd go-web-app
    ```

2. **Install dependencies / Установите зависимости**:
    ```sh
    go mod tidy
    ```

3. **Run the application / Запустите приложение**:
    ```sh
    go run main.go
    ```

4. **Access the application / Откройте приложение**:
    Open your web browser and navigate to `http://localhost:8080`.
    Откройте ваш веб-браузер и перейдите по адресу `http://localhost:8080`.

## API Endpoints / API конечные точки

- **Auth Routes / Маршруты аутентификации**:
  - `GET /register`: Display the registration page. / Отображение страницы регистрации.
  - `POST /register`: Handle user registration. / Обработка регистрации пользователя.
  - `GET /login`: Display the login page. / Отображение страницы входа.
  - `POST /login`: Handle user login. / Обработка входа пользователя.
  - `GET /logout`: Handle user logout. / Обработка выхода пользователя.

- **Message Routes / Маршруты сообщений**:
  - `GET /messages`: Display the messages page. / Отображение страницы сообщений.
  - `POST /send`: Send a new message. / Отправка нового сообщения.

- **Profile Routes / Маршруты профиля**:
  - `GET /profile`: Display the profile page. / Отображение страницы профиля.
  - `POST /profile`: Update profile information. / Обновление информации профиля.

- **Notification Routes / Маршруты уведомлений**:
  - `GET /api/notifications`: Get notifications. / Получение уведомлений.
  - `POST /api/notifications`: Mark notifications as read or clear all. / Отметить уведомления как прочитанные или очистить все.

- **WebSocket Route / Маршрут WebSocket**:
  - `GET /ws`: WebSocket connection for real-time updates. / Соединение WebSocket для обновлений в реальном времени.

- **API Routes / API маршруты**:
  - `GET /api/messages`: Get messages. / Получение сообщений.
  - `POST /api/messages/delete`: Delete a message. / Удаление сообщения.
  - `POST /api/messages/edit`: Edit a message. / Редактирование сообщения.
  - `POST /api/messages/reply`: Reply to a message. / Ответ на сообщение.
  - `GET /api/users/online`: Get online users. / Получение онлайн пользователей.
  - `POST /api/typing`: Broadcast typing status. / Трансляция статуса набора текста.
  - `GET /api/history`: Get message history. / Получение истории сообщений.
  - `POST /api/avatar`: Update user avatar. / Обновление аватара пользователя.
  - `GET /api/messages/export`: Export message history. / Экспорт истории сообщений.
  - `POST /api/messages/upload`: Upload a file. / Загрузка файла.
  - `GET /api/messages/search`: Search message history. / Поиск по истории сообщений.
  - `GET /api/messages/stats`: Get message statistics. / Получение статистики сообщений.
  - `GET /api/users/status`: Get user status. / Получение статуса пользователя.
  - `POST /api/groups/create`: Create a new group. / Создание новой группы.
  - `POST /api/messages/react`: Add a reaction to a message. / Добавление реакции на сообщение.
  - `GET /api/messages/logs`: Get message logs. / Получение журналов сообщений.

## License / Лицензия

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
Этот проект лицензирован по лицензии MIT. См. файл [LICENSE](LICENSE) для подробностей.
