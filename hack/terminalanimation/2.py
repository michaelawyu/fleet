import time

from colorama import Fore, Back, Style

def print_char_by_char(text, delay=0.05):
    for ch in text:
        print(ch, end="", flush=True)
        time.sleep(delay)

def rainbow_print(text, delay=0.01):
    colors = [Fore.RED, Fore.YELLOW, Fore.LIGHTYELLOW_EX, Fore.GREEN, Fore.CYAN, Fore.BLUE, Fore.MAGENTA]
    color_idx = 0
    for ch in text:
        print(colors[color_idx] + ch, end="", flush=True)
        color_idx = (color_idx + 1) % len(colors)
        time.sleep(delay)
    print(Style.RESET_ALL, end="", flush=True)
    
s = Back.CYAN + "Kueue" + Style.RESET_ALL + " " + \
    "is an open-source Kubernetes-native job queueing system " + \
    "designed for running batch, HPC, and AI/ML workloads " + \
    "in a Kubernetes cluster."
print_char_by_char(s)
print_char_by_char("\n\n")
s = Back.CYAN + "Kueue" + Style.RESET_ALL + " " + \
    "features support for multi-cluster systems; " + \
    "such setup is called a " + \
    Back.CYAN + "MultiKueue" + Style.RESET_ALL + " " + \
    "system."
print_char_by_char(s)
print_char_by_char("\n\n")
s = "Setting up a " + \
    Back.CYAN + "MultiKueue" + Style.RESET_ALL + " " + \
    "environment is often a complicated process that involves " + \
    "the configuration of " + \
    Back.CYAN + "service accounts," + Style.RESET_ALL + " " + \
    Back.CYAN + "secrets," + Style.RESET_ALL + " " + \
    Back.CYAN + "roles + role bindings," + Style.RESET_ALL + " " + \
    Back.CYAN + "cluster roles + cluster role bindings," + Style.RESET_ALL + " and " + \
    "MultiKueue specific APIs."
print_char_by_char(s)
print_char_by_char("\n\n")

s = "For example, to join a cluster, " + \
    Back.CYAN + "bravelion" + Style.RESET_ALL + ", " + \
    "to a MultiKueue system, you need to do as follows:"
print_char_by_char(s)
print_char_by_char("\n\n")

s = """
# Create a service account in the bravelion cluster.
kubectl create -f svcaccount.yaml

# Create a service account token.
kubectl create -f secret.yaml

# Retrieve the token and other information required for accessing the bravelion cluster.
SA_TOKEN=$(kubectl get -n NAMESPACE secrets/SECRET_NAME -o 'jsonpath={.data['token']}' | base64 -d)
CA_CERT=$(kubectl get -n NAMESPACE secrets/$SECRET_NAME -o 'jsonpath={.data['ca.crt']}')

# Extract cluster IP from the current context
CURRENT_CONTEXT=$(kubectl config current-context)
CURRENT_CLUSTER=$(kubectl config view -o jsonpath='{.contexts[?(@.name == \"CURRENT_CONTEXT\"})].context.cluster}')
CURRENT_CLUSTER_ADDR=$(kubectl config view -o jsonpath='{.clusters[?(@.name == \"$CURRENT_CLUSTER\"})].cluster.server}')

# Compose a kubeconfig file.
cat > KUBECONFIG <<EOF
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: CA_CERT
    server: CURRENT_CLUSTER_ADDR
  name: CURRENT_CLUSTER
contexts:
- context:
    cluster: CURRENT_CLUSTER
    user: SA
  name: CURRENT_CONTEXT
current-context: CURRENT_CONTEXT
kind: Config
preferences: {}
users:
- name: SA
  user:
    token: SA_TOKEN
EOF

# Save the kubeconfig to the Kueue management cluster.
kubectl create secret generic worker1-secret -n kueue-system --from-file=kubeconfig=KUBECONFIG

# And create the MultiKueue configuration objects.
"""
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0.01)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")


s = Back.CYAN + "Cluster Inventory API" + Style.RESET_ALL + " " + \
    "can help connect a multi-cluster platform, such as " + \
    Back.CYAN + "OCM" + Style.RESET_ALL + " and " + \
    Back.CYAN + "Fleet" + Style.RESET_ALL + ", " + \
    "with the Kueue setup to automate the setup of a " + \
    Back.CYAN + "MultiKueue" + Style.RESET_ALL + " " + \
    "environment with virtually no manual operations required."
print_char_by_char(s)
print_char_by_char("\n\n")

s = "To get started, simply view all the cluster profiles currently registered in the system:"
print_char_by_char(s)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "kubectl get cp -n fleet-system"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = "NAMESPACE       NAME        AGE\n" + \
    "fleet-system    bravelion   2d" + \
    "fleet-system    smartfish   2d"
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")

s = "And add the clusters that can run Kueue workloads to a MultiKueueConfig object (a MultiKueue API):"
print_char_by_char(s)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "cat mkconfig.yaml"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = """
apiVersion: kueue.x-k8s.io/v1alpha1
kind: MultiKueueConfig
metadata:
  name: test
spec:
  clusters:
  - bravelion
"""
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "kubectl apply -f mkconfig.yaml"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = "The system will automatically set up everything, from tokens + " + \
    "permissions to MultiKueue admission checks, for you:"
print_char_by_char(s)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "kubectl get multikueuecluster -o yaml"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = """
apiVersion: v1
items:
- apiVersion: kueue.x-k8s.io/v1alpha1
  kind: MultiKueueCluster
  metadata:
    name: bravelion
    ...
  spec:
    kubeConfig:
      location: bravelion
      locationType: Secret
  status:
    conditions:
    - lastTransitionTime: "2024-08-18T18:16:23Z"
      message: Connected
      observedGeneration: 1
      reason: Active
      status: "True"
      type: Active
kind: List
metadata:
  resourceVersion: ""
"""
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")

s = "Note that the cluster can be accessed by Kueue immediately thanks to the credentials signed " + \
    "automatically via the AuthTokenRequest API." 
print_char_by_char(s)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "kubectl get admissioncheck"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = "NAME   AGE\n" + \
    "test   2s"
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")

s = "And the MultiKueue system is ready to use right away:"
print_char_by_char(s)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "cat job.yaml"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = """
apiVersion: batch/v1
kind: Job
metadata:
  generateName: example-
  namespace: default
  labels:
    kueue.x-k8s.io/queue-name: user
spec:
  parallelism: 1
  completions: 1
  suspend: true
  template:
    spec:
      containers:
      - name: sleep
        image: gcr.io/k8s-staging-perf-tests/sleep:v0.1.0
        args: ["30s"]
        resources:
          requests:
            cpu: "1"
            memory: "200Mi"
      restartPolicy: Never
"""
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "kubectl create -f job.yaml"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = "The job is sent to the " + \
    Back.CYAN + "bravelion" + Style.RESET_ALL + " " + \
    "cluster for execution:"
print_char_by_char(s)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "kubectl get workload --context bravelion-admin"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = """
NAME                      QUEUE   RESERVED IN   ADMITTED   FINISHED   AGE
job-example-hwr6t-cd20a   user    default       True                  4s
"""
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")

s = "And it will be completed soon:"
print_char_by_char(s)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "kubectl get workload"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = """
NAME                      QUEUE   RESERVED IN   ADMITTED   FINISHED   AGE
job-example-hwr6t-cd20a   user    default                  True       65s
"""
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")
