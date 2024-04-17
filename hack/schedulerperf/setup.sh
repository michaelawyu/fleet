export SUBSCRIPTION=YOUR-SUBSCRIPTION-ID
export RG=chenyu1-perf-test
export DEFUALT_LOCATION=eastus
export REGISTRY_NAME=fleetdemo
export REGISTRY=fleetdemo.azurecr.io
export TAG=perf

az login
az account set --subscription $SUBSCRIPTION
az group create -n $RG -l $DEFUALT_LOCATION

export HUB_CLUSTER=hub

az aks create -n $HUB_CLUSTER \
    -g $RG \
    --node-count 2 \
    --node-vm-size standard_d8ads_v5 \
    --attach-acr $REGISTRY_NAME

export MEMBER_CLUSTER_COUNT=25
declare -a MEMBER_CLUSTERS=()
declare -a LOCATIONS=("eastus" "westus" "centralus" "northeurope" "eastasia")
declare -a ENVIRONMENTS=("dev" "staging" "prod")

for (( i=0; i<MEMBER_CLUSTER_COUNT; i++ ));
do
    MEMBER_CLUSTERS+=("member-$i")
done

for (( i=0; i<MEMBER_CLUSTER_COUNT; i++ ));
do
    LOC_IDX=$((i % ${#LOCATIONS[@]}))
    az aks create -n ${MEMBER_CLUSTERS[$i]} \
        -g $RG \
        --location ${LOCATIONS[$LOC_IDX]} \
        --node-count 2 \
        --node-vm-size standard_d2s_v3 \
        --attach-acr $REGISTRY_NAME
done

for (( i=0; i<MEMBER_CLUSTER_COUNT; i++ ));
do
    LOC_IDX=$((i % ${#LOCATIONS[@]}))
    az aks update -n ${MEMBER_CLUSTERS[$i]} \
        -g $RG \
        --attach-acr $REGISTRY_NAME
done

az aks get-credentials -g $RG -n $HUB_CLUSTER --admin
kubectl config use-context $HUB_CLUSTER-admin
helm install hub-agent charts/hub-agent/ \
    --set image.pullPolicy=Always \
    --set image.repository=$REGISTRY/hub-agent \
    --set image.tag=$TAG \
    --set logVerbosity=5 \
    --set namespace=fleet-system \
    --set enableWebhook=false \
    --set webhookClientConnectionType=service \
    --set enableV1Alpha1APIs=false \
    --set enableV1Beta1APIs=true \
    --set resources.limits.cpu=6 \
    --set resources.limits.memory=12Gi \
    --set resources.requests.cpu=4 \
    --set resources.limits.memory=8Gi \
    --set hubAPIQPS=500 \
    --set hubAPIBurst=50000

for (( i=0; i<MEMBER_CLUSTER_COUNT; i++ ));
do
    kubectl create serviceaccount fleet-member-agent-$i -n fleet-system
    cat <<EOF | kubectl apply -f -
    apiVersion: v1
    kind: Secret
    metadata:
        name: fleet-member-agent-$i-sa
        namespace: fleet-system
        annotations:
            kubernetes.io/service-account.name: fleet-member-agent-$i
    type: kubernetes.io/service-account-token
EOF
done

for (( i=0; i<MEMBER_CLUSTER_COUNT; i++ ));
do
    az aks get-credentials -g $RG -n ${MEMBER_CLUSTERS[$i]} --admin
done

for (( i=0; i<MEMBER_CLUSTER_COUNT; i++ ));
do
    kubectl config use-context $HUB_CLUSTER-admin
    TOKEN=$(kubectl get secret fleet-member-agent-$i-sa -n fleet-system -o jsonpath='{.data.token}' | base64 -d)
    kubectl config use-context "${MEMBER_CLUSTERS[$i]}-admin"
    kubectl delete secret hub-kubeconfig-secret --ignore-not-found
    kubectl create secret generic hub-kubeconfig-secret --from-literal=token=$TOKEN
done

export HUB_SERVER_URL=hub-dns-olwxtovd.hcp.eastus.azmk8s.io

for (( i=0; i<MEMBER_CLUSTER_COUNT; i++ ));
do
    kubectl config use-context "${MEMBER_CLUSTERS[$i]}-admin"
    helm install member-agent charts/member-agent/ \
        --set config.hubURL=$HUB_SERVER_URL \
        --set image.repository=$REGISTRY/member-agent \
        --set image.tag=$TAG \
        --set refreshtoken.repository=$REGISTRY/refresh-token \
        --set refreshtoken.tag=$TAG \
        --set image.pullPolicy=Always \
        --set refreshtoken.pullPolicy=Always \
        --set config.memberClusterName="${MEMBER_CLUSTERS[$i]}" \
        --set logVerbosity=5 \
        --set namespace=fleet-system \
        --set enableV1Alpha1APIs=false \
        --set enableV1Beta1APIs=true
done

kubectl config use-context $HUB_CLUSTER-admin
for (( i=0; i<MEMBER_CLUSTER_COUNT; i++ ));
do
    cat <<EOF | kubectl apply -f -
    apiVersion: cluster.kubernetes-fleet.io/v1beta1
    kind: MemberCluster
    metadata:
        name: ${MEMBER_CLUSTERS[$i]}
    spec:
        identity:
            name: fleet-member-agent-$i
            kind: ServiceAccount
            namespace: fleet-system
            apiGroup: ""
EOF
done

for (( i=0; i<MEMBER_CLUSTER_COUNT; i++ ));
do
    LOC_IDX=$((i % ${#LOCATIONS[@]}))
    ENV_IDX=$((i % ${#ENVIRONMENTS[@]}))
    kubectl label membercluster ${MEMBER_CLUSTERS[$i]} environment=${ENVIRONMENTS[$ENV_IDX]} location=${LOCATIONS[$LOC_IDX]}
done

for (( i=0; i<MEMBER_CLUSTER_COUNT; i++ ));
do
    kubectl config use-context "${MEMBER_CLUSTERS[$i]}-admin"
    P=$(kubectl get pods -o jsonpath={..metadata.name} -n fleet-system)
    kubectl delete pod $P -n fleet-system
done