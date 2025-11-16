# Real-Time Chat Rooms (Go + React)

A modern real-time chat application built with **Golang**, **WebSockets**, and a **React** frontend.  
Users can create rooms, join public chat rooms, and chat instantly with others.

---

## 🚀 Features

### 🖥 Backend (Golang)
- JWT authentication middleware  
- Create & join chat rooms  
- Explore all public rooms  
- WebSocket real-time messaging  
- Secure context handling (typed context keys)  

### 💬 Frontend (React)
- Modern and fast UI  
- Explore Rooms page with:
  - Room name  
  - Description  
  - Member count  
  - Room avatar  
- Join room button  
- Responsive layout  

---

## 🛠 Tech Stack

### Backend
- Go 1.22+
- Gorilla Mux
- WebSockets
- PostgreSQL
- JWT authentication

### Frontend
- React 18
- Axios
- Custom CSS

---


### Backend (Go)
```sh
cd backend
go mod tidy
go run main.go

---
```
## Frontend (React)
```sh
cd frontend
npm install
npm run dev

```
## 🔐 Authentication Flow
User logs in → receives JWT
JWT is attached as Authorization: Bearer <token>
Middleware extracts user_id and username into request context
WebSocket connections also validate token

## 🤝 API Endpoint
GET /rooms => Returns all public rooms.
POST /rooms => Create a new room.
POST /join/:roomID => Join an existing room.

## 📄 License
MIT License.