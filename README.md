# REST long-polling microservice for near-real-time notification delivery

## Introduction
The REST long-polling microservice for near-real-time notification delivery is a
Go application that provides a JWT authenticated REST API for clients and makes
it possible for them to receive notifications in near-real-time from any service in the
cloud architecture.
The service receives the notification from a dedicated AWS SQS queue and delivers it immediately to
the client if it is connected. If the addressee client in not connected, or the delivery
was not successful for whatever reason, the notification will stay in the SQS queue. The
service will retry the delivery only if the SQS policy set to deliver it again for listerner.
The service can be deployed in a highly scalable and highly available manner.
The service uses a database to store client sessions, currently it is running on PostgrepSQL (
DynamoDB or Redis are also good fits)

## Use cases
Has been designed for a mobile application, to avoid extensive usage of push notifications or websocket connections. The serviice can be deployed beside any other API
service, preferably behind a load balancer. It can be handy for any mobile application where real-time notifications must be delived while the 
application is in active state, for long-running IoT software or for browser applications.

## Features
<b>JWT Authentication:</b> User authentication is based on JWT tokens.
<b>Easy to use with AWS SQS</b>
<b>Logging:</b> Using Uber zap library for logging

## Usage
0. Clone the repository
```
git clone https://github.com/szabolcsgarda/notification-service.git'
cd notification-service
```

### Deployment on your machine
13. Install dependencies
```
go install
```

2. Set the required environment variables
   See below the Environment variables section for details.

3. Start the server

```
go run main.go
```

### Build Docker image
1. Build the Docker image
```
docker build -t notification-service -f Dockerfile-dev .
```

2. Run the Docker container
   The application is conveniently deployable as a Docker container, offering flexibility
   and ease of scaling. However, it's important to note that by default, the Docker
   container does not have access to the host machine's AWS secrets. The application requires a
   set of environmental variable (see below), which determines SQS usage, provide database address
   and credentials. Also recommended to use AWS role-based permissions.
```
docker run -p 3000:3000 -e DB_HOST="your-db-host" notification-service
```
## API documentation
See /openapi/notification-service.yaml for details!

## Environmental variables
The following environment variables are used by the application:

| Variable Name                     | Description                             |Required   |Default Value    |
|-----------------------------------|-----------------------------------------|-----------|-----------------|
| `PORT`                            | REST API port                           | No        | 8080            |
| `SQS_QUEUE_NAME_PREFIX`           | Prefix of the SQS queue                 | Yes       | -               |
| `NOTIFICATION_SERVICE_CLIENT_ID`  | Client ID (generated if not provided)   | No        | random          |
| `SQS_QUEUE_URL`                   | Only needed if above param. provided    | No        | -               |
| `COGNITO_JWK_URL`                 | URL to jwks.json to validate user JWK-s | Yes       | -               |
| `DB_HOST`                         | Database host                           | No        | localhost       |
| `DB_USER`                         | Database user                           | No        | my_user         |
| `DB_PASSWORD`                     | Database password                       | No        | my_password     |
| `DB_NAME`                         | Database name                           | No        | message_service |
| `LOGGING_MODE`                    | Logging mode for zap logger             | No        | DEVELOPMENT     |
