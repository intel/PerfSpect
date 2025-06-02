## Example DaemonSet for PerfSpect for GKE

This is an example DaemonSet for exposing PerfSpect metrics as a prometheus compatible metrics endpoint. This example assumes the use of Google Kubernetes Engine (GKE) and using the `PodMonitoring` resource to collect metrics from the metrics endpoint.

```
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: perfspect
  namespace: default
  labels:
    name: perfspect
spec:
  selector:
    matchLabels:
      name: perfspect
  template:
    metadata:
      labels:
        name: perfspect
    spec:
      containers:
      - name: perfspect
        image: docker.registry/user-sandbox/ar-us/perfspect
        imagePullPolicy: Always
        securityContext:
          privileged: true
        args:
          - "/perfspect"
          - "metrics"
          - "--log-stdout"
          - "--granularity"
          - "cpu"
          - "--noroot"
          - "--interval"
          - "15"
          - "--prometheus-server-addr"
          - ":9090"
        ports:
        - name: metrics-port # Name of the port, referenced by PodMonitoring
          containerPort: 9090 # The port your application inside the container listens on for metrics
          protocol: TCP
        resources:
          requests:
            memory: "200Mi"
            cpu: "500m"

---
apiVersion: monitoring.googleapis.com/v1
kind: PodMonitoring
metadata:
  name: perfspect-podmonitoring
  namespace: default
  labels:
    name: perfspect
spec:
  selector:
    matchLabels:
      name: perfspect
  endpoints:
  - port: metrics-port
    interval: 30s
```
 * Replace `docker.registry/user-sandbox/ar-us/perfspect` with the location of your perfspect container image.
