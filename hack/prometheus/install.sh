# Add/update the Helm chart report for the Prometheus/Kubernetes stack.
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Set up a Service that exposes the endpoint for Fleet agent metrics.
kubectl apply -f svc.yaml

# Retrieve its cluster IP.
FLEET_METRICS_CLUSTER_IP=$(kubectl get svc fleet-metrics -n fleet-system -o jsonpath='{.spec.clusterIP}')

# Prepare the Helm values file.
sed -i 's/YOUR\-CLUSTER\-IP/$FLEET_METRICS_CLUSTER_IP/g' values.yaml 

# Install the Helm chart.
kubectl create ns prometheus
helm install fleet-monitoring prometheus-community/kube-prometheus-stack \
    -f values.yaml \
    -n prometheus

kubectl get svc fleet-monitoring-kube-prom-prometheus -n prometheus -o jsonpath='{.status.loadBalancer}'