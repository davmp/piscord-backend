# 🎨 Piscord Backend

Backend service for Piscord, a real-time chat platform. This project was developed using **Go 1.25+**, **Gin Web Framework**, **Gorilla WebSockets**, and **MongoDB**.

> [!TIP]
> For complete system orchestration (Backend + Frontend + Database) via Kubernetes, visit the main repository:
> 👉 **[Piscord App - Main Repository](https://github.com/davmp/piscord-app)**

---

## 🚀 Technologies

- **Language:** [Go 1.25+](https://go.dev/)
- **Framework:** [Gin](https://gin-gonic.com/)
- **WebSockets:** [Gorilla WebSocket](https://github.com/gorilla/websocket)
- **Database:** [MongoDB](https://www.mongodb.com/)
- **Authentication:** JWT (JSON Web Tokens)

## 📋 Prerequisites

Before you begin, ensure you have the following installed on your machine:

- [Go](https://go.dev/dl/) (version 1.25 or higher)
- [MongoDB](https://www.mongodb.com/try/download/community) (Local instance or Atlas URI)

## 🛠️ Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/davmp/piscord-backend.git
   cd piscord-backend
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

## 💻 Development

To run the application in development mode:

1. Set the necessary environment variables (see below).
2. Run the application:
   ```bash
   go run main.go
   ```

The server will start on `http://localhost:8000` (or the defined PORT).

## 🐳 Docker

To build and run the Docker image locally:

1. **Build the image:**
   ```bash
   docker build -t piscord-backend .
   ```

2. **Run the container:**
   ```bash
   docker run -p 8000:8000 \
     -e MONGO_URI="mongodb://host.docker.internal:27017" \
     -e JWT_SECRET="your_secret" \
     piscord-backend
   ```

## ⚙️ Environment Variables

The following environment variables can be configured (especially useful in Docker/Kubernetes):

| Variable | Description | Default Value |
| :--- | :--- | :--- |
| `MONGO_URI` | MongoDB connection URI | `mongodb://localhost:27017` |
| `JWT_SECRET` | Secret key for JWT signing | - |
| `PORT` | Port where the server will run | `8000` |

## 📂 Project Structure

- `handlers/`: HTTP and WebSocket handlers (Controllers).
- `services/`: Business logic and database interactions.
- `models/`: Data structures and database models.
- `middleware/`: HTTP middlewares (e.g., Authentication).
- `config/`: Configuration loading.
- `utils/`: Utility functions.
