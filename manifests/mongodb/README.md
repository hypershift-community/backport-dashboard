# MongoDB Kubernetes Deployment

This directory contains Kubernetes manifests for deploying MongoDB.

## Components

1. **PersistentVolumeClaim (PVC)**
   - Requests 10GB of storage
   - Uses ReadWriteOnce access mode

2. **Service**
   - Exposes MongoDB on port 27017
   - Provides stable network identity for the MongoDB pod

3. **Deployment**
   - Runs MongoDB 6.0
   - Uses persistent storage via PVC
   - Configures resource limits and requests
   - Uses secrets for authentication

4. **Secret**
   - Stores MongoDB root username and password
   - Values are base64 encoded

## Deployment Instructions

1. First, create your own secret with proper credentials:
   ```bash
   # Generate base64 encoded values
   echo -n "your-username" | base64
   echo -n "your-password" | base64
   
   # Update the values in the mongodb-secret section of mongodb.yaml
   ```

2. Apply the manifests:
   ```bash
   kubectl apply -f mongodb.yaml
   ```

3. Verify the deployment:
   ```bash
   kubectl get pods -l app=mongodb
   kubectl get pvc mongodb-pvc
   kubectl get svc mongodb
   ```

## Configuration

- The MongoDB instance will be accessible within the cluster at: `mongodb:27017`
- Default resource limits:
  - CPU: 500m (0.5 CPU)
  - Memory: 1Gi
- Default resource requests:
  - CPU: 250m (0.25 CPU)
  - Memory: 256Mi

## Updating the Application Configuration

Update your application's MongoDB connection string to use:
```
mongodb://username:password@mongodb:27017/database_name
```

Replace:
- `username` and `password` with the values you set in the secret
- `database_name` with your desired database name 