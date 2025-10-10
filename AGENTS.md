# Agent Guidelines for probo-v1

## Build Commands
- Build all services: `task build`
- Build individual service: `cd <service> && go build ./cmd/<service>`
- Run all services: `task run`
- Run with live reload: `task dev`

## Test Commands
- No test framework configured
- Run single test: `go test -run TestName ./path/to/package`

## Code Style Guidelines

### Formatting
- Use `gofmt` for Go code formatting
- Follow standard Go formatting conventions

### Imports
- Standard library imports first
- Third-party imports second
- Local project imports last
- Use blank lines to separate import groups

### Naming Conventions
- Exported types/functions: PascalCase (e.g., `IncomingMessage`, `InitRedis`)
- Unexported types/functions: camelCase (e.g., `createUser`, `userRoutes`)
- Constants: ALL_CAPS with underscores (e.g., `PENDING`, `BUY`)

### Types and Structs
- Use JSON tags for all struct fields: `json:"fieldName"`
- Define custom types for enums (e.g., `type OrderStatus string`)
- Use `json.RawMessage` for flexible JSON data

### Error Handling
- Check errors immediately after operations
- Use `log.Fatal` for initialization failures
- Return errors in HTTP handlers with appropriate status codes
- Use `log.Println` for non-critical errors

### Project Structure
- Multi-module Go project with shared types
- Services: database, engine, server, socket, shared
- Use local replace directives for shared modules

## Development Guidelines
- HTTP Server → Engine: via Redis Queue + Pub/Sub
- Engine → Database Server: via Redis Queue + Pub/Sub
- Engine → WebSocket Server: via Redis Pub/Sub (for broadcasting updates)
- never run the applicaiton after making changes to the code i will run it my self
- never test the applicaiton after making changes to the code i will test it my self
