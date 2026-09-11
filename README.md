# PriceWatch

PriceWatch is a full-stack price tracking application that lets users monitor products, set target prices, view price history, and receive notifications when a product reaches its target price.

## Live Demo

- **Frontend:** [Live Demo!](https://pricewatch-frontend-uwc2.onrender.com/)
- **Backend API:** https://pricewatch-api-chrt.onrender.com
- **Database:** Neon PostgreSQL

> Render's free services may sleep after inactivity, so the first request can take a little longer.

## Preview

<img width="1096" height="880" alt="image" src="https://github.com/user-attachments/assets/fa0274cf-a8d1-4e09-b622-6cd25d234048" />


## Features

- User registration and JWT authentication
- Add, update, and delete tracked products
- Set target prices and monitor products
- Track current prices and price history
- Price-drop notifications
- Background price-checking worker
- REST API
- PostgreSQL persistence
- Docker support
- Unit and integration tests

## Tech Stack

**Backend:** Go, PostgreSQL, pgx, JWT, bcrypt  
**Frontend:** HTML, CSS, JavaScript  
**DevOps:** Docker, Docker Compose, Render, Neon

## Architecture

    Frontend → Go REST API → PostgreSQL (Neon)
                           ↑
                    Price Check Worker

## Project Structure

    PriceWatch/
    ├── cmd/
    │   ├── api/
    │   └── worker/
    ├── internal/
    │   ├── auth/
    │   ├── db/
    │   ├── handlers/
    │   ├── models/
    │   └── worker/
    ├── frontend/
    ├── test/
    ├── Dockerfile.api
    ├── Dockerfile.worker
    ├── docker-compose.yml
    └── README.md

## Run Locally

Set the required environment variables:

    DATABASE_URL=
    JWT_SECRET=
    API_ADDR=:8080
    WORKER_INTERVAL_SECONDS=60

Run the API:

    go run ./cmd/api

Run the worker:

    go run ./cmd/worker

Run tests:

    go test ./...

Or run with Docker:

    docker compose up --build

## Price Checking

The current implementation uses a deterministic mock price provider for demonstration and testing. This allows price changes and target-price notifications to be reproduced without depending on external retailer APIs.

## Deployment

The API is deployed on Render, the frontend is deployed as a Render Static Site, and PostgreSQL is hosted on Neon. Production secrets are configured through environment variables and are not committed to the repository.
