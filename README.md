# Pigoi

![Pigoi](/assets/icon.png)

Pigoi is a Go CLI that scaffolds HTTP APIs and RabbitMQ workers. It generates a runnable starter with Gin for routing, Uber Fx for dependency injection and lifecycle management, Viper for YAML configuration, MySQL for storage, and Logrus for structured logging.

## Install Pigoi

The CLI and generated projects declare Go 1.27.4 in their `go.mod` files. Install a compatible Go toolchain and make sure `go` is on your `PATH`.

Once the source is published to a public `github.com/sorihi/pigoi` repository, install Pigoi from any directory:

```sh
go install github.com/sorihi/pigoi@latest
```

Go puts the `pigoi` binary in `GOBIN` when set. With the default GOPATH setup, it installs to `$(go env GOPATH)/bin`, usually `~/go/bin`. If your shell cannot find `pigoi`, add the install directory to your `PATH`. For Bash or Zsh:

```sh
go_bin="$(go env GOBIN)"
export PATH="${go_bin:-$(go env GOPATH)/bin}:$PATH"
pigoi --help
```

Add those first two lines to your shell startup file to keep the setting in new terminals. Run the same `go install` command again to update Pigoi. Once you publish a version tag, users can replace `@latest` with that tag for a pinned install. [Go modules reference](https://go.dev/ref/mod#go-install)

From a local source checkout, use `go install .` to install your working copy.

## Create a project

Run this from the parent directory where you want the new project:

```sh
pigoi create-project inventory --module example.com/inventory
cd inventory
go test ./...
```

`create-project` creates a new directory and runs `go mod tidy`. The default template is `go-api`, which you can also select with `--template go-api`. If you omit `--module`, the project name becomes the module path.

## Configure and run the API

Edit `configs/app-config-map.yaml` from the generated project's root. Set `db.host`, `db.port`, `db.name`, `db.usr`, and `db.pw` to match an existing MySQL database and user. The application pings MySQL at startup, so the database must be ready before the API starts.

If you use Docker, this starts a local MySQL instance for the examples:

```sh
docker run -d --name pigoi-mysql \
  -p 127.0.0.1:3306:3306 \
  -e MYSQL_ROOT_PASSWORD=local-root-password \
  -e MYSQL_DATABASE=inventory \
  -e MYSQL_USER=inventory \
  -e MYSQL_PASSWORD=local-db-password \
  mysql:8.4
```

Wait for MySQL to finish initializing, then replace the `db` block with these local settings. [MySQL Docker image](https://hub.docker.com/_/mysql)

```yaml
db:
  host: 127.0.0.1
  port: 3306
  name: inventory
  usr: inventory
  pw: local-db-password
```

From the generated project root, start the application. In another terminal, call the health endpoint:

```sh
go run ./cmd
```

```sh
curl -fsS http://localhost:9090/health
# {"status":"ok"}
```

The default HTTP address is `localhost:9090`. Change `app.host` and `app.port` in the YAML to use a different address. Redis settings are present in the YAML, and the base application does not open a Redis connection. Use the example passwords only for local development and keep real credentials out of version control.

## Add an API feature

From the generated project root, run:

```sh
pigoi create-api-feature products
```

This creates `internal/products/` with `delivery.go`, `route.go`, `service.go`, `repository.go`, and `interface.go`. Pigoi updates `internal/provider.go` to register the feature with Fx and mount its routes under `/products`.

Start or restart the application with `go run ./cmd`, then try the generated endpoint:

```sh
curl -fsS http://localhost:9090/products/dummy
# {"message":"Dummy method executed successfully"}
```

Use `route.go` to define the endpoints, `delivery.go` for HTTP request and response handling, `service.go` for business logic, and `repository.go` for data access. Feature names must be valid Go package identifiers. For example, use `order_items` for a feature with a multiword name.

## Add a RabbitMQ worker

From the generated project root, run:

```sh
pigoi create-worker-feature orders
```

This creates `internal/orders/` with `broker.go`, `consumer.go`, `service.go`, `repository.go`, and `interface.go`. Pigoi registers the worker in `internal/provider.go`. The same `go run ./cmd` command starts the HTTP API and every registered worker. RabbitMQ is required once a worker is registered.

Set `rabbitmq.url` in `configs/app-config-map.yaml` or set `RABBITMQ_URL` in the application environment to override it. To run a local broker with the management UI:

```sh
docker run -d --name pigoi-rabbitmq \
  -p 127.0.0.1:5672:5672 \
  -p 127.0.0.1:15672:15672 \
  -e RABBITMQ_DEFAULT_USER=local \
  -e RABBITMQ_DEFAULT_PASS=local-dev-password \
  rabbitmq:4.2-management
```

Once RabbitMQ and MySQL are ready, start the application:

```sh
RABBITMQ_URL='amqp://local:local-dev-password@localhost:5672/' go run ./cmd
```

The worker declares a durable queue named `orders` and subscribes during startup. Open the management UI at `http://localhost:15672`, sign in with the local credentials above, and check the consumer count on the `orders` queue. You can also publish a test message from that queue's page. [RabbitMQ Docker image](https://hub.docker.com/_/rabbitmq)

The generated handler acknowledges messages after successful processing and rejects failed messages without requeueing. The initial repository method is a stub that returns success. Add your processing logic before pointing the worker at a queue with real messages.

## Test a generated project

Run these commands from the directory containing the generated `go.mod`:

```sh
go test ./...
go test ./... -coverpkg=./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

The base suite exercises the entry point, Fx wiring, health endpoint, YAML configuration, response and HTTP helpers, logger, and connection setup. The tests use temporary files and a loopback HTTP listener and do not require running MySQL or RabbitMQ services. `./...` selects packages in the current module.

## Project layout

A generated project has this structure:

```text
cmd/main.go                     API and worker entry point
configs/app-config-map.yaml     Runtime configuration
internal/provider.go            Fx providers, routes, and worker registration
internal/health/                Health endpoint
internal/<feature>/             Generated API or worker feature
pkg/configuration/              YAML loader and typed settings
pkg/database/                   MySQL connection pool
pkg/logger/                     Structured logging
pkg/rabbitmq/                   Shared RabbitMQ connection
helpers/response/               JSON responses and error handling
helpers/utils/                  HTTP and string helpers
```

In the Pigoi source checkout, `cmd/` defines the CLI and `internal/generator/` renders templates and updates generated providers. The embedded scaffolds are under `templates/go-api/`, `templates/go-api-feature/`, and `templates/go-worker-feature/`.

To build and check the CLI from the Pigoi source root:

```sh
go test ./...
go build -o /tmp/pigoi .
/tmp/pigoi --help
```
