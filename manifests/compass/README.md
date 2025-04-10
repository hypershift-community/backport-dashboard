# MongoDB Compass (Web UI) Deployment

This directory contains Kubernetes manifests for deploying MongoDB Compass (using mongo-express) as a web-based UI for MongoDB.

## Components

1. **Service**
   - Type: NodePort
   - Exposes port 8081 externally on port 30081
   - Provides access to the MongoDB Compass web interface

2. **Deployment**
   - Runs mongo-express as a web-based MongoDB UI
   - Connects to the MongoDB instance in the same namespace
   - Uses existing MongoDB credentials
   - Includes basic authentication for web UI access
   - Configures resource limits and requests

3. **Secret**
   - Stores the password for web UI access
   - Values are base64 encoded

## Prerequisites

- MongoDB deployment should be running in the same namespace
- MongoDB secret (mongodb-secret) should be present

## Deployment Instructions

1. First, create your own compass-secret with a proper password:
   ```bash
   # Generate base64 encoded value for the web UI password
   echo -n "your-password" | base64
   
   # Update the value in the compass-secret section of compass.yaml
   ```

2. Apply the manifests:
   ```bash
   kubectl apply -f compass.yaml
   ```

3. Verify the deployment:
   ```bash
   kubectl get pods -l app=mongodb-compass
   kubectl get svc mongodb-compass
   ```

## Accessing the Web UI

1. The MongoDB Compass web interface will be available at:
   ```
   http://<node-ip>:30081
   ```

2. Login credentials:
   - Username: admin
   - Password: (the password you set in compass-secret)

## Configuration

- Default resource limits:
  - CPU: 200m (0.2 CPU)
  - Memory: 256Mi
- Default resource requests:
  - CPU: 100m (0.1 CPU)
  - Memory: 128Mi

## Security Notes

- The web UI is protected by basic authentication
- The service uses NodePort for external access
- The deployment reuses MongoDB credentials from the existing mongodb-secret
- Consider using an Ingress with TLS for production deployments 