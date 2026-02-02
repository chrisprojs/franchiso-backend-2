# Franchiso 🏪
Franchiso is a web-based marketplace platform designed to facilitate the buying and selling of franchise businesses in Indonesia by connecting franchisors and potential franchisees in a single digital ecosystem. The platform allows franchisors to publish detailed franchise information, including investment costs, return on investment, business documents, and outlet locations, while enabling franchisees to search and compare opportunities using advanced filters and location mapping features. By incorporating business verification, structured information, and secure transaction support, Franchiso aims to increase transparency, trust, and efficiency in the franchise trading process, making it easier for users to find, evaluate, and acquire franchise business opportunities.

## AI & Search Features

<table border="0">
  <tr>
    <td valign="top" width="50%">
      <b>Searching Using AI</b><br>
      <img src="documentation/1.jpg" width="250"><br>
      Advanced search system leveraging AI to search by query and image.
    </td>
    <td valign="top" width="50%">
      <b>Midtrans Payments</b><br>
      <img src="documentation/2.jpg" width="250"><br>
      Seamless and secure boosting payment transactions.
    </td>
  </tr>
  <tr>
    <td valign="top" width="50%">
      <b>Elasticsearch Filters</b><br>
      <img src="documentation/3.jpg" width="250"><br>
      Combines traditional filters with AI semantic similarity.
    </td>
    <td valign="top" width="50%">
      <b>Location Mapping</b><br>
      <img src="documentation/4.jpg" width="250"><br>
      Geospatial data visualization for franchise outlets.
    </td>
  </tr>
</table>

Make sure `GEMINI_API_KEY` and `GEMINI_ACTIVE=true` are configured and the `ai_module` is reachable for full AI features.

## Franchiso Backend 2

Backend service for the Franchiso platform, built with Go, PostgreSQL, Redis, Elasticsearch, and an AI module for vector/image and text search. This service exposes REST APIs for:

- **Authentication**: register, email verification, login, profile.
- **Franchises**: create, edit, delete, list own franchises, view franchise detail.
- **Search**: filter and AI‑assisted search (text + image + embeddings) over franchises.
- **Boost & Payments**: boost a franchise using Midtrans payments.
- **Admin**: verify submitted franchises.

The app can run either **locally with Docker Compose** or be deployed to **Kubernetes** using `deployment.dev.yaml`.

For the frontend code repository, please clone from https://github.com/chrisprojs/franchiso-frontend-2

---

### Tech Stack

- **Language**: Go (Gin)
- **Database**: PostgreSQL
- **Cache / Queue**: Redis
- **Search**: Elasticsearch (with vector search)
- **AI module**: Python service (`ai_module`) for embeddings / image vectors
- **Payments**: Midtrans
- **Cloud platform**: GCP (GKE, Google Maps API, Gemini API)
- **Infrastructure**: Docker, Kubernetes
- **Email**: SMTP

---

### Prerequisites

- **Docker & Docker Compose** installed.
- **Go** (if you want to run without Docker) – Go 1.21+ recommended.
- Access to the required external services:
  - Midtrans server key.
  - Google Maps API key.
  - Gemini API key (if you want AI search / embeddings).
  - SMTP account (for verification emails).

---

### Environment Variables

Create a `.env` file in the project root. At minimum, you will typically need:

- **Postgres**
  - `PG_USER`
  - `PG_PASSWORD`
  - `PG_DATABASE`
- **Redis**
  - (address is wired from Docker compose: `REDIS_ADDR=redis:6379`)
- **Elasticsearch**
  - (URL is wired from Docker compose: `ELASTIC_URL=http://elasticsearch:9200`)
- **JWT / Security**
  - `JWT_SECRET`
- **Midtrans**
  - `MIDTRANS_SERVER_KEY`
  - `MIDTRANS_ENV` (e.g. `sandbox` or `production`)
- **Google Maps**
  - `GOOGLE_MAPS_API_KEY`
- **Gemini**
  - `GEMINI_API_KEY`
  - `GEMINI_ACTIVE` (`true` / `false`)
- **SMTP**
  - `SMTP_ACC`
  - `SMTP_ACC_PASSWORD`

Values can also be injected through the compose files or Kubernetes secrets. See `docker-compose-dev.yml`, `docker-compose-prod.yml`, and `deployment.dev.yaml` for how they are wired.

---

### Running with Docker Compose (Development)

This is the easiest way to start everything locally (Postgres, Redis, Elasticsearch, backend app, and AI module).

1. **Clone the repo** and `cd` into it:

   ```bash
   git clone <this-repo-url>
   cd franchiso-backend-2
   ```

2. **Create `.env`** in the project root and fill in the environment variables listed above.

3. **Start the stack**:

   ```bash
   docker compose -f docker-compose-dev.yml up --build
   ```

4. **Access the services** (default ports from `docker-compose-dev.yml`):

   - Backend API: `http://localhost:8080`
   - Static/storage proxy: `http://localhost:8081`
   - PostgreSQL: `localhost:5433`
   - Redis: `localhost:6278`
   - Elasticsearch: `http://localhost:9201`
   - AI module: `http://localhost:5000`

5. To stop:

   ```bash
   docker compose -f docker-compose-dev.yml down
   ```

---

### Running with Docker Compose (Production‑like)

Use the production compose file (secured Elasticsearch, dedicated network, named volumes).

```bash
docker compose -f docker-compose-prod.yml up --build -d
```

You must provide all required environment variables (e.g. via `.env` or your orchestration/host) – see `docker-compose-prod.yml` for details.

---

### Running Locally without Docker (Backend only)

You can also run just the Go backend directly, pointing it at existing PostgreSQL, Redis, and Elasticsearch instances.

1. **Install Go dependencies**:

   ```bash
   go mod tidy
   ```

2. Ensure that:

   - PostgreSQL, Redis, and Elasticsearch are running and reachable.
   - All environment variables in the **Environment Variables** section are set in your shell.

3. **Run the backend**:

   ```bash
   go run ./...
   ```

   The server will listen on port `8080` (and `8081` for the storage proxy).

---

### Kubernetes Deployment (Development)

`deployment.dev.yaml` contains:

- **PersistentVolumeClaim** for database/storage.
- **Secrets** for database password, JWT, Midtrans, Google Maps, Gemini, SMTP.
- **Single‑pod Deployment** running:
  - PostgreSQL
  - Redis
  - Elasticsearch
  - Go backend (`backend-app`)
  - AI module
- **Services** to expose:
  - Public LoadBalancer for HTTP/storage (`franchiso-service`).
  - Internal services for Postgres and Elasticsearch.

Basic flow:

1. **Create the secrets** (either:
   - manually apply `deployment.dev.yaml` after customizing `stringData` under `franchiso-secrets`, or
   - create them via `kubectl create secret ...` and keep only the Deployment/Service blocks).

2. **Apply the manifest**:

   ```bash
   kubectl apply -f deployment.dev.yaml
   ```

3. Wait for the pod `franchiso-stack` to become Ready and for the `franchiso-service` LoadBalancer to receive an external IP. Use that IP for the frontend.

---

### API Overview

Below is a high‑level summary of important endpoints (all are `JSON` unless noted). Exact request/response structures are defined in the `service` and `models` packages.

- **Health**
  - `GET /healthcheck` – basic status.

- **Auth**
  - `POST /register` – register user (fields: `name`, `email`, `password`, `role`). Triggers verification email and stores pending data in Redis.
  - `POST /verify-email` – verify registration via email code; creates user, issues access & refresh tokens.
  - `POST /login` – login with email/password, returns access & refresh tokens.
  - `GET /profile` – get current user profile (requires `Authorization: Bearer <access_token>`).

- **Franchise (authenticated as `Franchisor` for owner actions)**
  - `GET /franchise/my_franchises` – list franchises owned by current franchisor.
  - `POST /franchise/upload` – multipart form upload to create a new franchise:
    - Text fields: `category_id`, `brand`, `description`, `investment`, `monthly_revenue`, `roi`, `branch_count`, `year_founded`, `website`, `whatsapp_contact`.
    - Files: `logo`, `ad_photos[]`, `stpw`, `nib`, `npwp`.
  - `PUT /franchise/edit/:id` – edit existing franchise (same fields as upload, all optional).
  - `DELETE /franchise/delete/:id` – delete owned franchise (also removes from Elasticsearch if verified).
  - `GET /franchise/:id` – public franchise detail from Elasticsearch.
  - `GET /franchise/:id?showPrivate=true` – private/owner/admin view with extra fields from Postgres (requires auth).
  - `GET /franchise/categories` – list available categories.
  - `GET /franchise/locations` – list franchise locations.
  - `POST /franchise` – search franchises with filters and optional AI assistance:
    - Filters: `category`, `min_investment`, `max_investment`, `min_monthly_revenue`, `min_roi`, `max_roi`,
      `min_branch_count`, `max_branch_count`, `min_year_founded`, `max_year_founded`.
    - Sorting: `order_by`, `order_direction`.
    - Pagination: `page`, `limit`.
    - AI search:
      - `search_query` (text) – normal text search, with Gemini embedding fallback when no exact match and `GEMINI_ACTIVE=true`.
      - `search_by_image` (file) – image‑based search via logo/ad_photos vectors.

- **Boost & Payments**
  - `POST /boost/:id` – boost a franchise (authenticated franchisor). Uses Midtrans for payments; details in `service/boost.go` and `service/payment.go`.
  - `POST /mid_trans/call_back` – Midtrans callback endpoint for updating payment/boost status.

- **Admin**
  - `GET /admin/verify-franchise` – list franchises waiting for verification (auth + proper admin role required).
  - `PUT /admin/verify-franchise/:id` – approve/reject a franchise and synchronize verified ones into Elasticsearch.

---

### Development Notes

- CORS is configured to allow `http://localhost:3000` by default for the frontend.
- File uploads are proxied through a storage proxy service on port `8081` (see `storage_proxy.go`).
- Database table names are in the `franchiso` schema (e.g. `franchiso.users`, `franchiso.franchises`).
- For detailed implementation, see:
  - `config/` – connections & third‑party configs.
  - `service/` – HTTP handlers.
  - `models/` – database and Elasticsearch models.
  - `utils/` – helpers (JWT, time, cache, vector conversion, image processing).