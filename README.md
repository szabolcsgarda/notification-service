# REST long-polling microservice for near-real-time notification delivery

## Introduction
This is a Go application that provides a JWT authenticated REST API for clients and makes
it possible for them to receive notifications in near-real-time from any service in the
cloud architecture.

### How it works
The service receives the notification from a dedicated AWS SQS queue and delivers it immediately to
the client if it is connected. If the addressee client in not connected, or the delivery
was not successful for whatever reason, the notification will not be deleted from the queue and if SQS
configured so, it will go to a dead-letter-queue. The service will retry the delivery only if the SQS policy set to deliver 
it again for listener.

### Deployment
The service can be deployed in a highly scalable and highly available manner.
The service uses a database to store client sessions, currently it is running on PostgrepSQL (
DynamoDB or Redis are also good fits)

### Authentication
The service supports JWT authentication. JWT tokens are expected in the "Authorization" header of the request,
the URL to the jwks.json file must be provided as an environment variable.

### Session Management
Each instance of the application receives deliverable messages from a dedicated SQS queue. It means, that if a message generator
service wants to send a message to client 'A', it needs to know exactly which notification-service instance A is currently connected to.
That's when the database comes into play. When a client connects to an instance of the notification-service, the service assigns its 
unique ID to the client and stores it in the database. When a message generator service wants to send a message to client 'A', it needs to 
retrieve the corresponding notification-service ID from the database and send the message to the corresponding SQS queue.

## Use cases
Has been designed for a mobile application, to avoid extensive usage of push notifications or websocket connections. The service can be deployed beside any other API
service, preferably behind a load balancer. It can be handy for any mobile application where real-time notifications must be delivered while the 
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
See /api/notification-service.yaml for details!

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
