# Gebeta Billing

Gebeta Billing is a billing API with a browser-based demo application. The Go
service owns billing operations and persistence; the BFF serves as the web
application's API gateway; and the React app provides a local demo UI.

## Local development

### One-command setup

Install Docker Desktop (with Docker Compose), Go, Node.js 20+, and GNU Make.
Then run:

```sh
make setup
```

`make setup` checks that the tools and Docker engine are available, creates
`.env` from `.env.example` if needed, installs the BFF and web dependencies,
downloads Go modules, and builds/starts the development Compose stack. It
preserves an existing `.env`. The local mock identity provider is included in
the development stack. Local development endpoints use a non-production
webhook secret so the demo invoice and payment simulator work out of the box.
The optional `dev` profile also contains MinIO and its bucket initializer.

Open the web UI at `http://localhost:8081` (or the `WEB_HOST_PORT` configured in
`.env`). The mock sign-in is for local development only and does not provide
real authentication.

If a default host port is already in use, change the corresponding `*_HOST_PORT`
setting in `.env`. Container-to-container service ports should normally remain
unchanged.

### Common commands

Run `make help` to list all available targets. Common commands include:

```sh
make up          # Build and start the local Compose stack
make down        # Stop local services
make logs        # Follow service logs
make test        # Run Go tests
make bff-test    # Run BFF tests
make web-build   # Type-check and build the web application
make lint        # Run golangci-lint
make ci          # Run the repository's main Go and BFF checks
```

To run a component outside Docker, install its dependencies first with `npm ci`
in `platform/bff` or `platform/web`; use `npm run dev` in the relevant folder.

## Production configuration

Do not use the mock identity provider or local credentials in production. The
production Compose override requires `AUTH_JWKS_URL`, `AUTH_ISSUER`, and
`AUTH_AUDIENCE` to be set to the real identity provider configuration:

```sh
docker compose -f docker-compose.yaml -f docker-compose.prod.yaml up --build -d
```

Production `AUTH_JWKS_URL` must use HTTPS. The production override disables the
development endpoints and mock sign-in and keeps the mock identity service
inactive. Never reuse the local webhook secret in a deployed environment.
