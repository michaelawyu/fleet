export HUB_CLUSTER_CONTEXT="hub"
export HUB_CLUSTER_ADDRESS="https://limit200-dns-i6ywp09y.hcp.eastus.azmk8s.io:443"

export REGISTRY="mcr.microsoft.com/aks/fleet"
export FLEET_VERSION=$(curl "https://api.github.com/repos/Azure/fleet/tags" | jq -r '.[0].name')
export MEMBER_AGENT_IMAGE="member-agent"
export REFRESH_TOKEN_IMAGE="refresh-token"

MEMBER_CLUSTER_ARRAY=(
    #"scale-test-eastus-10-mc"
    "scale-test-eastus2-11-mc"
    "scale-test-westus-12-mc"
    "scale-test-centralus-13-mc"
    "scale-test-eastus-14-mc"
    "scale-test-westus-16-mc"
    "scale-test-eastus2-15-mc"
    "scale-test-centralus-17-mc"
    "scale-test-eastus-18-mc"
    "scale-test-eastus2-19-mc"
    "scale-test-westus-20-mc"
    "scale-test-centralus-21-mc"
    "scale-test-eastus-22-mc"
    "scale-test-eastus2-23-mc"
    "scale-test-westus-24-mc"
    "scale-test-centralus-25-mc"
    "scale-test-eastus-26-mc"
    "scale-test-eastus2-27-mc"
    "scale-test-westus-28-mc"
    "scale-test-centralus-29-mc"
    "scale-test-eastus-30-mc"
    "scale-test-eastus2-31-mc"
    "scale-test-westus-32-mc"
    "scale-test-centralus-33-mc"
    "scale-test-eastus-34-mc"
    "scale-test-eastus2-35-mc"
    "scale-test-westus-36-mc"
    "scale-test-centralus-37-mc"
    "scale-test-eastus-38-mc"
    "scale-test-eastus2-39-mc"
    "scale-test-westus-40-mc"
    "scale-test-centralus-41-mc"
    "scale-test-eastus-42-mc"
    "scale-test-eastus2-43-mc"
    "scale-test-westus-44-mc"
    "scale-test-centralus-45-mc"
    "scale-test-eastus-46-mc"
    "scale-test-eastus2-47-mc"
    "scale-test-westus-48-mc"
    "scale-test-centralus-49-mc"
    "scale-test-eastus-50-mc"
    "scale-test-eastus2-51-mc"
    "scale-test-westus-52-mc"
    "scale-test-centralus-53-mc"
    "scale-test-eastus-54-mc"
    "scale-test-eastus2-55-mc"
    "scale-test-westus-56-mc"
    "scale-test-centralus-57-mc"
    "scale-test-eastus-58-mc"
    "scale-test-eastus2-59-mc"
    "scale-test-westus-60-mc"
    "scale-test-centralus-61-mc"
    "scale-test-eastus-62-mc"
    "scale-test-eastus2-63-mc"
    "scale-test-westus-64-mc"
    "scale-test-centralus-65-mc"
    "scale-test-eastus-66-mc"
    "scale-test-eastus2-67-mc"
    "scale-test-westus-68-mc"
    "scale-test-centralus-69-mc"
    "scale-test-eastus-70-mc"
    "scale-test-eastus2-71-mc"
    "scale-test-westus-72-mc"
    "scale-test-centralus-73-mc"
    "scale-test-eastus-74-mc"
    "scale-test-eastus2-75-mc"
    "scale-test-westus-76-mc"
    "scale-test-centralus-77-mc"
    "scale-test-eastus-78-mc"
    "scale-test-eastus2-79-mc"
    "scale-test-westus-80-mc"
    "scale-test-centralus-81-mc"
    "scale-test-eastus-82-mc"
    "scale-test-eastus2-83-mc"
    "scale-test-westus-84-mc"
    "scale-test-centralus-85-mc"
    "scale-test-eastus-86-mc"
    "scale-test-eastus2-87-mc"
    "scale-test-westus-88-mc"
    "scale-test-centralus-89-mc"
    "scale-test-eastus-90-mc"
    "scale-test-eastus2-91-mc"
    "scale-test-westus-92-mc"
    "scale-test-centralus-93-mc"
    "scale-test-eastus-94-mc"
    "scale-test-eastus2-95-mc"
    "scale-test-westus-96-mc"
    "scale-test-centralus-97-mc"
    "scale-test-eastus-98-mc"
    "scale-test-eastus2-99-mc"
    "scale-test-westus-100-mc"
    "scale-test-centralus-101-mc"
    "scale-test-eastus-102-mc"
    "scale-test-eastus2-103-mc"
    "scale-test-westus-104-mc"
    "scale-test-centralus-105-mc"
    "scale-test-eastus-106-mc"
    "scale-test-eastus2-107-mc"
    "scale-test-westus-108-mc"
    "scale-test-centralus-109-mc"
)

export AZURE_RESOURCE_GROUP=chenyu1-scaletest2

for MEMBER_CLUSTER in "${MEMBER_CLUSTER_ARRAY[@]}"
do
    az aks get-credentials -n $MEMBER_CLUSTER -g $AZURE_RESOURCE_GROUP --admin --overwrite-existing
    export MEMBER_CLUSTER_CONTEXT="$MEMBER_CLUSTER-admin"
    export SERVICE_ACCOUNT="$MEMBER_CLUSTER-alt-hub-cluster-access"

    kubectl config use-context $HUB_CLUSTER_CONTEXT
    kubectl create serviceaccount $SERVICE_ACCOUNT -n fleet-system

    export SERVICE_ACCOUNT_SECRET="$MEMBER_CLUSTER-alt-hub-cluster-access-token"
    cat <<EOF | kubectl apply -f -
    apiVersion: v1
    kind: Secret
    metadata:
        name: $SERVICE_ACCOUNT_SECRET
        namespace: fleet-system
        annotations:
            kubernetes.io/service-account.name: $SERVICE_ACCOUNT
    type: kubernetes.io/service-account-token
EOF

    export TOKEN=$(kubectl get secret $SERVICE_ACCOUNT_SECRET -n fleet-system -o jsonpath='{.data.token}' | base64 -d)
    cat <<EOF | kubectl apply -f -
    apiVersion: cluster.kubernetes-fleet.io/v1beta1
    kind: MemberCluster
    metadata:
        name: $MEMBER_CLUSTER-alt
    spec:
        identity:
            name: $SERVICE_ACCOUNT
            kind: ServiceAccount
            namespace: fleet-system
            apiGroup: ""
        heartbeatPeriodSeconds: 60
EOF

    kubectl config use-context $MEMBER_CLUSTER_CONTEXT
    # Create the secret with the token extracted previously for member agent to use.
    kubectl create secret generic hub-kubeconfig-secret --from-literal=token=$TOKEN
    helm install member-agent ../charts/member-agent/ \
        --set config.hubURL=$HUB_CLUSTER_ADDRESS \
        --set image.repository=$REGISTRY/$MEMBER_AGENT_IMAGE \
        --set image.tag=$FLEET_VERSION \
        --set refreshtoken.repository=$REGISTRY/$REFRESH_TOKEN_IMAGE \
        --set refreshtoken.tag=$FLEET_VERSION \
        --set image.pullPolicy=Always \
        --set refreshtoken.pullPolicy=Always \
        --set config.memberClusterName="$MEMBER_CLUSTER-alt" \
        --set logVerbosity=5 \
        --set namespace=fleet-system \
        --set enableV1Alpha1APIs=false \
        --set enableV1Beta1APIs=true
    
    echo "$MEMBER_CLUSTER-alt joined into the fleet"
done













