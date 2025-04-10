# Jira to MongoDB Sync

This program syncs issues from Jira to MongoDB, copying selected fields for each issue.

## Prerequisites

- Go 1.21 or later
- MongoDB instance (see "Running MongoDB Locally" below)
- Jira instance and API token
- The following Go dependencies (automatically installed via go.mod):
  - github.com/andygrunwald/go-jira
  - go.mongodb.org/mongo-driver
  - github.com/spf13/viper

## Running MongoDB Locally

You can run MongoDB locally using Podman with the following steps:

1. Create a directory for MongoDB data persistence:
   ```bash
   mkdir -p ~/mongodb/data
   ```

2. Run MongoDB container:
   ```bash
   podman run -d \
     --name mongodb \
     -p 27017:27017 \
     -v ~/mongodb/data:/data/db:Z \
     -e MONGODB_INITDB_ROOT_USERNAME=admin \
     -e MONGODB_INITDB_ROOT_PASSWORD=password \
     docker.io/mongo:7.0
   ```

3. Verify the container is running:
   ```bash
   podman ps
   ```

4. Connect to MongoDB using the MongoDB shell (optional):
   ```bash
   podman exec -it mongodb mongosh -u admin -p password
   ```

To stop and remove the container:
```bash
podman stop mongodb
podman rm mongodb
```

Note: The data will persist in `~/mongodb/data` even after removing the container.

## Running MongoDB Compass Locally

You can run MongoDB Compass (mongo-express) locally using Podman with the following steps:

1. Run mongo-express container:
   ```bash
   podman run -d \
     --name mongodb-compass \
     -p 8081:8081 \
     -e ME_CONFIG_MONGODB_SERVER=mongodb \
     -e ME_CONFIG_MONGODB_PORT=27017 \
     -e ME_CONFIG_MONGODB_ADMINUSERNAME=admin \
     -e ME_CONFIG_MONGODB_ADMINPASSWORD=password \
     -e ME_CONFIG_BASICAUTH_USERNAME=admin \
     -e ME_CONFIG_BASICAUTH_PASSWORD=compasspassword \
     docker.io/mongo-express:1.0.0-alpha.4
   ```

2. Create a network to connect MongoDB and mongo-express:
   ```bash
   podman network create mongodb-network
   podman network connect mongodb-network mongodb
   podman network connect mongodb-network mongodb-compass
   ```

3. Access the web interface:
   - Open your browser and go to: `http://localhost:8081`
   - Login with:
     - Username: admin
     - Password: compasspassword

4. Verify the connection:
   - You should see the MongoDB instance listed
   - You can browse databases and collections
   - You can view and edit documents

To stop and remove the containers:
```bash
podman stop mongodb-compass
podman rm mongodb-compass
```

Note: Make sure MongoDB is running before starting mongo-express.

## Configuration

1. Copy the `config.yaml` file and update it with your settings:
   ```yaml
   jira:
     url: "https://your-jira-instance.com"
     username: "your-username"
     token: "your-api-token"

   mongodb:
     uri: "mongodb://admin:password@localhost:27017"
     database: "jira_sync"
     collection: "issues"

   fields_to_sync:
     - "key"
     - "summary"
     - "description"
     - "status"
     - "created"
     - "updated"
     - "assignee"
     - "reporter"
     - "labels"
     - "Target Backport Versions"
     - "Target Version"
   ```

## Running the Program

1. Install dependencies:
   ```bash
   go mod download
   ```

2. Run the program:
   ```bash
   go run main.go
   ```

## MongoDB Document Structure

Each issue is stored as a document with the following structure:
```json
{
  "_id": "OCPBUGS-123",
  "key": "OCPBUGS-123",
  "summary": "Main issue summary",
  "description": "Issue description",
  "status": "In Progress",
  "created": "2024-04-08T12:00:00Z",
  "updated": "2024-04-08T14:00:00Z",
  "assignee": "John Doe",
  "reporter": "Jane Smith",
  "labels": ["bug", "priority/high"],
  "target_backport_versions": ["4.18.0", "4.17.0"],
  "target_version": "4.19.0",
  "clones": {
    "4.18.0": {
      "key": "OCPBUGS-124",
      "summary": "Clone for 4.18.0",
      // ... all fields for 4.18.0 clone
    },
    "4.17.0": {
      "key": "OCPBUGS-125",
      "summary": "Clone for 4.17.0",
      // ... all fields for 4.17.0 clone
    }
  }
}
```

## Security Notes

- The MongoDB connection string in the example uses basic authentication
- For production use, consider:
  - Using TLS for MongoDB connections
  - Storing credentials in environment variables or secrets management
  - Running MongoDB in a secured environment
  - Using a more restrictive MongoDB user with minimal required permissions

## Field Mapping

The program maps the following Jira fields to MongoDB:
- `key`: Issue key (also used as MongoDB _id)
- `summary`: Issue summary
- `description`: Issue description
- `status`: Issue status name
- `created`: Creation timestamp
- `updated`: Last update timestamp
- `assignee`: Assignee's display name
- `reporter`: Reporter's display name
- `labels`: Array of labels attached to the issue

Additional fields can be added by modifying the `extractFields` function in `main.go`. 