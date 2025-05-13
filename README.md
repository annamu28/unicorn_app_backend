# Unicorn App Backend

A robust Go-based backend API for the Unicorn mobile application.

## Project Overview

This backend provides a RESTful API for the Unicorn mobile application, handling authentication, user management, squads, chatboards, posts, and more. Built with Go and PostgreSQL, it follows a clean architecture design with middleware-based authentication.

## Technologies

- **Go** - Backend language
- **Gin** - Web framework
- **PostgreSQL** - Database
- **JWT** - Authentication
- **Docker** - Containerization

## Directory Structure

- `handlers/` - HTTP request handlers
- `middleware/` - Authentication and security middleware
- `models/` - Data models
- `routes/` - API route definitions
- `db/` - Database setup and migrations

## Getting Started

### Prerequisites

- Go 1.16+
- PostgreSQL 13+
- Docker and Docker Compose (optional)

### Environment Variables

Create a `.env` file in the root directory with the following variables:

```
PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=unicorn_db
JWT_SECRET=your_jwt_secret
```

### Running Locally

```bash
# Start the application
go run main.go
```

### Running with Docker

```bash
# Build and start containers
docker-compose up --build

# Stop containers
docker-compose down
```

## API Endpoints

### Authentication

- `POST /register` - Register a new user
- `POST /login` - Login and get tokens
- `POST /refresh` - Refresh an access token
- `POST /logout` - Logout and invalidate tokens

### User Management

- `GET /userinfo` - Get current user info
- `POST /avatar` - Upload user avatar

### Squads

- `POST /squads` - Create a new squad
- `GET /squads` - Get user's squads

### Chatboards

- `POST /chatboards` - Create a new chatboard
- `GET /chatboards` - Get user's chatboards
- `GET /chatboards/:id/pending-users` - Get users pending approval

### Posts

- `POST /posts` - Create a new post
- `GET /posts` - Get posts for a chatboard
- `POST /posts/:id/toggle-pin` - Pin/unpin a post

### Comments

- `POST /comments` - Create a new comment
- `GET /comments` - Get comments for a post

## Architecture

The application follows a clean architecture pattern:

1. **Main** - Entry point that sets up the application
2. **Routes** - Defines API endpoints and connects them to handlers
3. **Middleware** - Handles cross-cutting concerns like authentication
4. **Handlers** - Processes HTTP requests and returns responses
5. **Models** - Defines data structures

## Authentication Flow

1. User registers or logs in to receive JWT tokens
2. Access token is used for API requests via Authorization header
3. Refresh token is used to get new tokens when the access token expires
4. Tokens are validated by the AuthMiddleware before accessing protected routes
