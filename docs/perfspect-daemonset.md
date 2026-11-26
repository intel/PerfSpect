# Example DaemonSet for PerfSpect for GKE

This is an example DaemonSet for exposing PerfSpect metrics as a prometheus compatible metrics endpoint. This example assumes the use of Google Kubernetes Engine (GKE) and using the `PodMonitoring` resource to collect metrics from the metrics endpoint.

``` yaml
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

## Collecting Container-Scoped Metrics

The default `Dockerfile` (`ubuntu:22.04`) does not include `crictl` (or `ctr`/`docker`), so to collect container-scoped metrics in Kubernetes either mount the host's container runtime CLI binary into the pod (for example, `/home/kubernetes/bin/crictl` mounted at `/host/bin/crictl` and selected with `--container-runtime /host/bin/crictl`) or build `crictl`/`ctr` into a custom image. Set `hostPID: true` on the pod spec and mount `/sys/fs/cgroup` and the host container runtime socket matching your node runtime (for example, `/run/containerd/containerd.sock` for containerd or `/run/crio/crio.sock` for CRI-O):

``` yaml
    spec:
      hostPID: true
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
          - "--noroot"
          - "--scope"
          - "container"
          - "--container-runtime"
          - "/host/bin/crictl"
          - "--prometheus-server-addr"
          - ":9090"
        ports:
        - name: metrics-port
          containerPort: 9090
          protocol: TCP
        resources:
          requests:
            memory: "200Mi"
            cpu: "500m"
        volumeMounts:
          - name: cgroup
            mountPath: /sys/fs/cgroup
            readOnly: true
          - name: crictl-bin
            mountPath: /host/bin/crictl
            readOnly: true
          - name: containerd-sock
            mountPath: /run/containerd/containerd.sock
      volumes:
        - name: cgroup
          hostPath:
            path: /sys/fs/cgroup
        - name: crictl-bin
          hostPath:
            path: /home/kubernetes/bin/crictl
        - name: containerd-sock
          hostPath:
            path: /run/containerd/containerd.sock
```
