# Overview
This application is a self-contained service that manages all billing related issues on its own once setup. 

## Tools Used

- Go
- PostgreSQL
- RabbitMQ 
- Docker
- NodeJS and React(Vite): to demonstrate how the server can be communicated with by the end platforms

## Codebase
The entire codebase is structured in such a way that its partitioned in smaller modules that are detached. Where communication between modules is necessary, we injected the necessary components of one module to the other module. Each module follows a recurring structure where it has:

*   **`handler/`** Holds all controllers functions that handle HTTP requests concerning that module
*   **`service/`** Contains all the businness logic that would be called by other modules, http handlers and background workers.
*   **`repository/`** Communicates directly to the database using SQLC generated code
*   **`model/`** Contains structural definitions and domain entities that would be used by other parts of the module
*   **`subscriber/`** Listens for asynchronous events published by other domains to decouple workflows 

## Initialization
Since each module of the entire program is roughly self-contained, we manage their lifecycles and dependencies centrally. We always initialize the repository and service of each module in our entry point (`cmd/main.go`). If a module requires capabilities from another module (e.g., the Subscription module needing access to Accounts and Plans), we inject the necessary repository or service at this initialization stage. Finally, all initialized services are passed down to the Delivery (HTTP) layer. For example:

```go
// 1. Initialize dependencies for a standalone module
accountRepo := accountrepo.New(deps.Pool)
accountSvc := accountsvc.NewWithEvents(accountRepo, eventSvc, smEngine)

// 2. Initialize a module that depends on multiple other modules (Cross-module injection)
subscriptionRepo := subscriptionrepo.New(deps.Pool)
subscriptionSvc := subscriptionsvc.New(
    subscriptionRepo, 
    accountRepo, // Injected from Account module
    planRepo,    // Injected from Plan module
    // ....
)

// 3. Inject all services into the HTTP server
srv := api.NewServer(cfg, api.Deps{
    Pool:                deps.Pool,
    AccountService:      accountSvc,
    SubscriptionService: subscriptionSvc,
    // ...
})
```

## Payment Provider Integration
Communication with third-party payment providers is implemented in the `/internal/ppi` module. Each payment provider must have its own adapter (`internal/ppi/adapters`) implemented and recorded on the database to be supported. Each adapter has its own implementation of attempting payment and handling webhooks. But it will reorganize any external data comming from third-party providers into an internall recognized and provider-agnostic data format for the internal workflow. This makes scaling the supported payment providers significantly easier.


## Database & Migrations
*   **Migrations:** Schema changes are managed in `internal/database/migrations` and run automatically on application startup.
*   **Queries:** We strictly use `sqlc`. We write raw SQL queries, and `sqlc` generates type-safe Go code for our repositories to use.
