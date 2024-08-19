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

s = Back.CYAN + "Fleet" + Style.RESET_ALL + " " + \
    "is an open-source multi-cluster management solution, which allows you " + \
    "to manage multiple Kubernetes clusters seamlessly, on-premises and/or " + \
    "on the cloud."
print_char_by_char(s)
print_char_by_char("\n\n")
s = "A Fleet deployment features: \n" + \
    "* " + \
    Back.GREEN + "A hub cluster" + Style.RESET_ALL + \
    ", which serves as one unified portal for management; and\n" + \
    "* " + \
    Back.GREEN + "Up to hundreds of member clusters" + Style.RESET_ALL + \
    ", which run your workloads."
print_char_by_char(s)
print_char_by_char("\n\n")
s = "In this demo, we will use a Fleet with one member cluster, called " + \
    Back.GREEN + "bravelion" + Style.RESET_ALL + "."
print_char_by_char(s)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "kubectl get memberclusters"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = "Name       NodeCount   CPUCost   MemoryCost   CPU  CPULeft   Mem    MemLeft\n" + \
    "bravelion  1           0.057     0.014        8    6         24Gi   12Gi"
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")

s = "The Fleet code we use features experimental support for the " + \
    Back.GREEN + "Cluster Inventory API" + Style.RESET_ALL + "." + \
    " Each Fleet member cluster will have its corresponding ClusterProfile object."
print_char_by_char(s)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "kubectl get cp -n fleet-system"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = "NAMESPACE       NAME        AGE\n" + \
    "fleet-system    bravelion   3s"
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "kubectl get cp bravelion -n fleet-system -o yaml"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = """
apiVersion: multicluster.x-k8s.io/v1alpha1
kind: ClusterProfile
metadata:
  creationTimestamp: "2024-08-18T17:16:44Z"
  generation: 1
  labels:
    x-k8s.io/cluster-manager: fleet
  name: bravelion
  namespace: fleet-system
  resourceVersion: "10573"
  uid: 9fc335e4-f214-44aa-b97f-0007b119f247
spec:
  clusterManager:
    name: fleet
  displayName: bravelion
status:
  conditions:
  - lastTransitionTime: "2024-08-18T17:16:44Z"
    message: The Fleet member agent reports that the API server is healthy
    observedGeneration: 1
    reason: MemberClusterAPIServerHealthy
    status: True
    type: ControlPlaneHealthy
  version: {}
"""
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")

s = "We can now request a credential for the " + \
    Back.GREEN + "bravelion" + Style.RESET_ALL + \
    " cluster with the cluster-admin permissions using the " + \
    Back.GREEN + "AuthTokenRequest API" + Style.RESET_ALL + \
    " once we know its cluster profile."
print_char_by_char(s)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "cat atr.yaml"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = """
apiVersion: multicluster.x-k8s.io/v1alpha1
kind: AuthTokenRequest
metadata:
  name: example
  namespace: work
spec:
  targetClusterProfile:
    apiGroup: multicluster.x-k8s.io/v1alpha1
    kind: ClusterProfile
    name: bravelion
  serviceAccountName: work
  serviceAccountNamespace: default
  clusterRoles:
  - name: cluster-admin
"""
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "kubectl apply -f atr.yaml"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = "In a few seconds, Fleet will retrieve the credential for cluster " + \
    Back.GREEN + "bravelion" + Style.RESET_ALL + \
    ", and one can inspect the progress by checking the status of the AuthTokenRequest object " + \
    "(assuming that preparation of such credential is always allowed)."
print_char_by_char(s)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "kubectl get atr example -n work -o yaml"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = """
apiVersion: multicluster.x-k8s.io/v1alpha1
kind: AuthTokenRequest
metadata: ...
spec: ...
status:
  conditions:
  - lastTransitionTime: "2024-08-18T17:32:32Z"
    message: Service account already exists
    observedGeneration: 1
    reason: ServiceAccountFound
    status: "True"
    type: ServiceAccountFoundOrCreated
  - lastTransitionTime: "2024-08-18T17:32:32Z"
    message: All roles have been set up successfully
    observedGeneration: 1
    reason: RoleSetupSucceeded
    status: "True"
    type: RoleFoundOrCreated
  - lastTransitionTime: "2024-08-18T17:32:33Z"
    message: All cluster roles have been set up successfully
    observedGeneration: 1
    reason: ClusterRoleSetupSucceeded
    status: "True"
    type: ClusterRoleFoundOrCreated
  - lastTransitionTime: "2024-08-18T17:32:43Z"
    message: Token is retrieved
    observedGeneration: 1
    reason: TokenRetrieved
    status: "True"
    type: ServiceAccountTokenRetrieved
  tokenResponse:
    apiVersion: v1
    kind: ConfigMap
    name: example
    namespace: work
"""
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")

s = "Now we know that the credential is kept in a config map named " + \
    Back.GREEN + "example" + Style.RESET_ALL + \
    " from the same namespace as the AuthTokenRequest object."
print_char_by_char(s)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "kubectl get cm example -n work -o yaml"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = """
apiVersion: v1
data:
  kubeconfig: |
    apiVersion: v1
    clusters:
    - cluster:
        certificate-authority-data: ...
        server: ...
      name: bravelion
    contexts:
    - context:
        cluster: bravelion
        user: bravelion
      name: bravelion
    current-context: bravelion
    kind: Config
    preferences: {}
    users:
    - name: bravelion
      user:
        token: ...
kind: ConfigMap
metadata:
  name: example
  namespace: work
  ...
"""
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")

s = "And a user or an application can use that credential to access cluster " + \
    Back.GREEN + "bravelion" + Style.RESET_ALL + \
    " as appropriate."
print_char_by_char(s)
print_char_by_char("\n\n")

s = "The cluster profile for cluster " + \
    Back.GREEN + "bravelion" + Style.RESET_ALL + \
    " will also include a reference to the credential in the status, just for reference."
print_char_by_char(s)
print_char_by_char("\n\n")

print_char_by_char(Fore.YELLOW)
s = "kubectl get cp bravelion -n fleet-system -o yaml"
print_char_by_char(s)
print_char_by_char("\n")
print_char_by_char(Style.RESET_ALL)

s = """
apiVersion: multicluster.x-k8s.io/v1alpha1
kind: ClusterProfile
metadata: ...
spec: ...
status:
  conditions: ...
  credentials:
  - accessRef:
      apiVersion: v1
      kind: ConfigMap
      name: example
      namespace: work
    name: work-example
"""
# Add a delay for simulation purposes.
print_char_by_char(Fore.WHITE)
print_char_by_char(s, 0)
print_char_by_char(Style.RESET_ALL)
print_char_by_char("\n\n")